package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/husio/feedstream/cache"
	"github.com/husio/feedstream/pg"
	"github.com/husio/feedstream/randstr"
)

type AuthService interface {
	// LoginAsUser create user session and connects it with given user. If
	// account for given user does not yet exist, it's being created.
	LoginAsUser(context.Context, http.ResponseWriter, *User) error

	// CurrentUser returns user connected to current request. It returns
	// ErrNotAuthenticated if client is not authenticated with any account.
	CurrentUser(ctx context.Context, r *http.Request) (*User, error)

	// Providers returns list of all authentication providers registered
	// within service.
	Providers() []*Provider
}

// User represents single user credentials.
type User struct {
	AccountID  int64     `db:"account_id"`
	Provider   string    `db:"provider"`
	Name       string    `db:"name"`
	ProfileURL string    `db:"profile_url"`
	Created    time.Time `db:"created"`
}

type Auth struct {
	db        accountsDatabase
	cache     cache.CacheService
	providers []*Provider
}

var _ AuthService = (*Auth)(nil)

func NewAuthService(db pg.Database, cache cache.CacheService, providers []*Provider) *Auth {
	return &Auth{
		db:        &accountsdb{db: db},
		cache:     cache,
		providers: providers,
	}
}

func (a *Auth) LoginAsUser(ctx context.Context, w http.ResponseWriter, u *User) error {
	u, err := a.db.EnsureExists(ctx, *u)
	if err != nil {
		return err
	}

	key := randstr.New(22)
	const exp = 7 * 24 * time.Hour
	if err := a.cache.Set(ctx, "auth:session:"+key, u, exp); err != nil {
		return fmt.Errorf("cache failed: %s", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:  SessionCookie,
		Value: key,
		Path:  "/",
	})
	return nil
}

const SessionCookie = "s"

// CurrentUser return user attached to given request if exists.
func (a *Auth) CurrentUser(ctx context.Context, r *http.Request) (*User, error) {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		// TODO check other places for authentication token
		return nil, ErrNotAuthenticated
	}

	cacheKey := "auth:session:" + c.Value
	var u User
	switch err := a.cache.Get(ctx, cacheKey, &u); err {
	case nil:
		return &u, nil
	case cache.ErrMiss:
		return nil, ErrNotAuthenticated
	default:
		return nil, fmt.Errorf("storage backend failed: %s", err)
	}
}

func (a *Auth) Providers() []*Provider {
	return append([]*Provider{}, a.providers...) // copy
}

var ErrNotAuthenticated = errors.New("not authenticated")

type accountsDatabase interface {
	EnsureExists(context.Context, User) (*User, error)
}

type accountsdb struct {
	db pg.Database
}

var _ accountsDatabase = (*accountsdb)(nil)

func (a *accountsdb) EnsureExists(ctx context.Context, u User) (*User, error) {
	if u.AccountID == 0 {
		_, err := a.db.Exec(`
			INSERT INTO accounts (provider, name, profile_url, created)
			SELECT $1, $2, $3, $4
			WHERE NOT EXISTS (
				SELECT * FROM accounts
				WHERE provider = $1 AND profile_url = $3
				LIMIT 1
			)
		`, u.Provider, u.Name, u.ProfileURL, time.Now())
		if err != nil {
			return nil, err
		}
	}
	err := a.db.Get(&u, `
			SELECT *
			FROM accounts
			WHERE account_id = $1
				OR (provider = $2 OR profile_url = $3)
			LIMIT 1
	`, u.AccountID, u.Provider, u.ProfileURL)
	if err != nil {
	}
	return &u, err
}
