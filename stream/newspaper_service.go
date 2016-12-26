package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type NewspaperService interface {
	Article(ctx context.Context, articleUrl string) (*Article, error)
}

type NewspaperClient struct {
	secret string
	apiURL string

	Client *http.Client
}

var _ NewspaperService = (*NewspaperClient)(nil)

func NewNewspaperClient(apiURL, secret string) *NewspaperClient {
	return &NewspaperClient{
		secret: secret,
		apiURL: apiURL,

		Client: http.DefaultClient,
	}
}

func (n *NewspaperClient) Article(ctx context.Context, articleUrl string) (*Article, error) {
	u := n.apiURL + "/?url=" + url.QueryEscape(articleUrl)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %s", err)
	}
	req.Header.Set("Api-Secret", n.secret)
	resp, err := n.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 1e4))
		return nil, fmt.Errorf("invalid response: %d %s", resp.StatusCode, string(b))
	}

	var meta Article
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1e6)).Decode(&meta); err != nil {
		return nil, fmt.Errorf("cannot decode body: %s", err)
	}
	return &meta, nil
}

type Article struct {
	Canonical string    `json:"canonical"`
	Image     string    `json:"img"`
	Title     string    `json:"title"`
	Authors   []string  `json:"authors"`
	Keywods   []string  `json:"keywords"`
	Tags      []string  `json:"tags"`
	Summary   string    `json:"summary"`
	Text      string    `json:"text"`
	Published time.Time `json:"published"`
}
