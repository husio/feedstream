package main

import (
	"log"
	"net/http"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/husio/envconf"
	"github.com/husio/feedstream/auth"
	"github.com/husio/feedstream/cache"
	"github.com/husio/feedstream/pg"
	"github.com/husio/feedstream/stream"
	"github.com/husio/feedstream/ui"
	"github.com/husio/web"
	_ "github.com/lib/pq"
)

func main() {
	conf := struct {
		HTTPPort           string `envconf:"PORT"`
		Site               string
		Postgres           string `envconf:"DATABASE_URL"`
		Redis              string
		StaticsDir         string
		TemplatesGlob      string
		Debug              bool
		NewspaperApi       string `envconf:"NEWSPAPER_API"`
		NewspaperApiSecret string `envconf:"NEWSPAPER_API_SECRET"`

		RedditOAuth2ClientID     string `envconf:"REDDIT_OAUTH2_CLIENT_ID"`
		RedditOAuth2ClientSecret string `envconf:"REDDIT_OAUTH2_CLIENT_SECRET"`
		GithubOAuth2ClientID     string `envconf:"GITHUB_OAUTH2_CLIENT_ID"`
		GithubOAuth2ClientSecret string `envconf:"GITHUB_OAUTH2_CLIENT_SECRET"`
		GoogleOAuth2ClientID     string `envconf:"GOOGLE_OAUTH2_CLIENT_ID"`
		GoogleOAuth2ClientSecret string `envconf:"GOOGLE_OAUTH2_CLIENT_SECRET"`
	}{
		HTTPPort:      "8080",
		Postgres:      "dbname=postgres user=postgres sslmode=disable",
		Redis:         "redis://localhost:6379/0",
		StaticsDir:    "./static",
		TemplatesGlob: "./templates/**/*.tmpl",
		NewspaperApi:  "https://articlemeta-api.herokuapp.com",
	}
	log.SetFlags(log.Lshortfile | log.Ltime)
	envconf.Parse(&conf)

	db, err := pg.Connect(conf.Postgres)
	if err != nil {
		log.Fatalf("cannot connect to postgres: %s", err)
	}
	defer db.Close()

	pg.MustLoadSchema(db, auth.Schema)
	pg.MustLoadSchema(db, stream.Schema)

	newspaper := stream.NewNewspaperClient(conf.NewspaperApi, conf.NewspaperApiSecret)
	bookmarklet := &stream.Bookmarklet{
		Site: conf.Site,
	}
	rp := redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 2 * time.Minute,
		Dial: func() (redis.Conn, error) {
			return redis.DialURL(conf.Redis)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}
	defer rp.Close()
	cacheSrv := cache.NewRedisCacheService(&rp)
	oauthRedirectUrl := conf.Site + "/login/success"
	var providers []*auth.Provider
	if conf.GoogleOAuth2ClientID != "" && conf.GoogleOAuth2ClientSecret != "" {
		providers = append(providers, auth.GoogleProvider(conf.GoogleOAuth2ClientID, conf.GoogleOAuth2ClientSecret, oauthRedirectUrl))
	}
	authSrv := auth.NewAuthService(db, cacheSrv, providers)

	streamManager := stream.NewManager(db, &rp, newspaper)
	tmpl, err := ui.NewHTMLRenderer(conf.TemplatesGlob, conf.Debug)
	if err != nil {
		log.Fatalf("cannot create render service: %s", err)
	}

	rt := web.NewRouter()
	rt.Add(`/`, "GET", stream.EntriesHandler(streamManager, authSrv, tmpl))
	rt.Add(`/subscriptions`, "GET,POST", stream.SubscriptionHandler(streamManager, bookmarklet, authSrv, tmpl))
	rt.Add(`/subscriptions/(subscription-id)/remove`, "POST", stream.RemoveSubscriptionHandler(streamManager, authSrv, tmpl))
	rt.Add(`/bookmarks`, "OPTIONS,POST", stream.BookmarkHandler(streamManager, authSrv))
	rt.Add(`/bookmarklet`, "GET", stream.BookmarkletHandler())

	rt.Add(`/login`, "GET", auth.SelectLoginHandler(authSrv, tmpl))
	rt.Add(`/login/success`, "GET", auth.OAuthLoginCallbackHandler(authSrv, cacheSrv, tmpl))
	rt.Add(`/login/(provider)`, "GET", auth.OAuthLoginHandler(authSrv, cacheSrv, tmpl))

	rt.Add(`/static/.*`, "GET", http.StripPrefix("/static", http.FileServer(http.Dir(conf.StaticsDir))))

	// XXX
	rt.Add(`/_/updateoutdated`, "POST", stream.UpdateOutdatedHandler(streamManager))

	if err := http.ListenAndServe("localhost:"+conf.HTTPPort, rt); err != nil {
		log.Fatalf("server error: %s", err)
	}
}
