package auth

import (
	"crypto/rand"
	"encoding/base32"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/husio/feedstream/cache"
	"github.com/husio/feedstream/ui"
	"github.com/husio/web"
)

func SelectLoginHandler(
	authSrv AuthService,
	tmpl ui.Renderer,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := authSrv.CurrentUser(r.Context(), r)

		context := struct {
			Next        string
			Providers   []*Provider
			CurrentUser *User
		}{
			Next:        r.FormValue("next"),
			Providers:   authSrv.Providers(),
			CurrentUser: user,
		}
		tmpl.Render(w, "select_login.tmpl", context, http.StatusOK)
	}
}

func OAuthLoginHandler(
	authSrv AuthService,
	cacheSrv cache.CacheService,
	tmpl ui.Renderer,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 18)
		if n, err := rand.Read(b); err != nil || n != 18 {
			log.Printf("cannot read random value: %s", err)
			renderAuthErr(w, tmpl, "Cannot prepare authentication data.")
			return
		}
		state := strings.ToLower(base32.StdEncoding.EncodeToString(b))

		provider, ok := findProvider(authSrv, web.PathArg(r, 0))
		if !ok {
			log.Printf("provider not found: %s", web.PathArg(r, 0))
			tmpl.RenderStd(w, http.StatusNotFound)
			return
		}

		url := provider.Config(r).AuthCodeURL(state, oauth2.AccessTypeOnline)
		http.SetCookie(w, &http.Cookie{
			Name:    stateCookie,
			Path:    "/",
			Value:   state,
			Expires: time.Now().Add(time.Minute * 15),
		})
		info := authInfo{
			ProviderCodename: provider.Codename,
			State:            state,
			Next:             r.FormValue("next"),
		}
		if err := cacheSrv.Set(r.Context(), "authlogin:"+state, &info, 10*time.Minute); err != nil {
			log.Printf("data not found in cache: %s", err)
			renderAuthErr(w, tmpl, "Authentication data expired.")
			return
		}
		http.Redirect(w, r, url, http.StatusSeeOther)
	}
}

type authInfo struct {
	ProviderCodename string
	State            string
	Next             string
}

const stateCookie = "oauthState"

func OAuthLoginCallbackHandler(
	authSrv AuthService,
	cacheSrv cache.CacheService,
	tmpl ui.Renderer,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var state string
		if c, err := r.Cookie(stateCookie); err != nil || c.Value == "" {
			log.Printf("invalid oauth state: expected %q, got %q", state, r.FormValue("state"))
			renderAuthErr(w, tmpl, "Invalid authentication state.")
			return
		} else {
			state = c.Value
		}

		if r.FormValue("state") != state {
			log.Printf("invalid oauth state: cookie value is %q, form value is %q", state, r.FormValue("state"))
			renderAuthErr(w, tmpl, "Invalid state token.")
			return
		}

		var info authInfo
		switch err := cacheSrv.Get(r.Context(), "authlogin:"+state, &info); err {
		case nil:
			// all good
		case cache.ErrMiss:
			renderAuthErr(w, tmpl, "Authentication token expired")
			return
		default:
			log.Printf("cannot get auth data from cache: %s", err)
			renderAuthErr(w, tmpl, "Tempolary internal error.")
			return
		}

		if info.State != state {
			log.Printf("invalid oauth state: cached value is %q, form value is %q", state, r.FormValue("state"))
			renderAuthErr(w, tmpl, "Invalid state token.")
			return
		}

		provider, ok := findProvider(authSrv, info.ProviderCodename)
		if !ok {
			log.Printf("provider not found: %s", info.ProviderCodename)
			renderAuthErr(w, tmpl, "Selected provider no longer available.")
			return
		}

		conf := provider.Config(r)
		token, err := conf.Exchange(r.Context(), r.FormValue("code"))
		if err != nil {
			log.Printf("oauth2 exchange failed: %s", err)
			renderAuthErr(w, tmpl, "OAuth2 configuration error.")
			return
		}

		user, err := provider.FetchUser(conf.Client(r.Context(), token))
		switch err {
		case nil:
			// all good
		case ErrInvalidProfile:
			log.Printf("cannot GET %s user information: %s", provider.Name, err)
			renderAuthErr(w, tmpl, "Cannot authenticate with selected profile, because it's incomplete.")
			return
		default:
			log.Printf("cannot GET %s user information: %s", provider.Name, err)
			renderAuthErr(w, tmpl, "Cannot get user information from authentication provider.")
			return
		}

		if err := authSrv.LoginAsUser(r.Context(), w, user); err != nil {
			log.Printf("cannot set current user: %s", err)
			renderAuthErr(w, tmpl, "Cannot login authenticated user.")
			return
		}

		next := info.Next
		if next == "" {
			next = "/"
		}
		http.Redirect(w, r, next, http.StatusTemporaryRedirect)
	}
}

func renderAuthErr(w http.ResponseWriter, tmpl ui.Renderer, message string) {
	context := struct {
		Message string
	}{
		Message: message,
	}
	tmpl.Render(w, "auth_error.tmpl", context, http.StatusInternalServerError)
}

func findProvider(authSrv AuthService, name string) (*Provider, bool) {
	for _, p := range authSrv.Providers() {
		if p.Codename == name {
			return p, true
		}
	}
	return nil, false
}
