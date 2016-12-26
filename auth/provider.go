package auth

import (
	"errors"
	"net/http"

	"golang.org/x/oauth2"
)

type Provider struct {
	Codename  string
	Name      string
	fetchUser func(*http.Client) (*User, error)
	conf      *oauth2.Config
}

func (p *Provider) Config(r *http.Request) *oauth2.Config {
	return p.conf
}

func (p *Provider) FetchUser(c *http.Client) (*User, error) {
	return p.fetchUser(c)
}

var ErrInvalidProfile = errors.New("invalid provider's profile")
