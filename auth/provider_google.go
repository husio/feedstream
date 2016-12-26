package auth

import (
	"encoding/json"
	"net/http"

	"golang.org/x/oauth2"
)

func GoogleProvider(clientID, clientSecret, redirectUrl string) *Provider {
	return &Provider{
		Name:     "Google",
		Codename: "google",

		fetchUser: fetchGoogleUser,
		conf: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectUrl,
			Scopes:       []string{"https://www.googleapis.com/auth/plus.me"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.google.com/o/oauth2/auth",
				TokenURL: "https://accounts.google.com/o/oauth2/token",
			},
		},
	}
}

func fetchGoogleUser(c *http.Client) (*User, error) {
	// https://developers.google.com/identity/protocols/OAuth2
	// https://developers.google.com/identity/protocols/googlescopes#plusv1
	resp, err := c.Get("https://www.googleapis.com/plus/v1/people/me")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user struct {
		DisplayName string `json:"displayName"`
		URL         string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	if user.DisplayName == "" || user.URL == "" {
		return nil, ErrInvalidProfile
	}

	u := &User{
		Provider:   "google",
		Name:       user.DisplayName,
		ProfileURL: user.URL,
	}
	return u, nil
}
