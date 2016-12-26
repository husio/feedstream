package stream

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/mmcdole/gofeed"
)

type feedinfo struct {
	feed *gofeed.Feed
}

func fetchFeed(ctx context.Context, feedUrl string) (*feedinfo, error) {
	resp, err := http.Get(feedUrl)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch: %s", err)
	}
	defer resp.Body.Close()

	fp := gofeed.NewParser()
	feed, err := fp.Parse(io.LimitReader(resp.Body, 1e6))
	if err != nil {
		return nil, fmt.Errorf("cannot fetch: %s", err)
	}
	return &feedinfo{feed: feed}, nil
}

func (f *feedinfo) FaviconURL(ctx context.Context) (string, error) {
	// try main feed url and if that does not work, one of the article urls
	var feedurl string
	if u, err := url.Parse(f.feed.Link); err != nil {
		feedurl = f.feed.Link
	} else {
		if u.Scheme == "" {
			u.Scheme = "http"
		}
		feedurl = u.String()
	}

	resp, err := http.Get(feedurl)
	if err != nil {
		if len(f.feed.Items) > 0 {
			feedurl = f.feed.Items[0].Link
			resp, err = http.Get(feedurl)
		}
		if err != nil {
			return "", fmt.Errorf("cannot fetch feed: %s", err)
		}
	}
	defer resp.Body.Close()

	if favicon := faviconFromHTML(io.LimitReader(resp.Body, 2e5)); favicon != "" {
		u, err := url.Parse(favicon)
		if err == nil && u.Path != "" && u.Path != "/" {
			if u.Host == "" {
				fu, _ := url.Parse(feedurl)
				u.Host = fu.Host
			}
			u.Scheme = ""
			return u.String(), nil
		}
	}

	// last try - guess favicon
	u, err := url.Parse(feedurl)
	if err != nil {
		return "", fmt.Errorf("cannot parse %q url: %s", feedurl, err)
	}
	u.Path = "/favicon.ico"
	u.RawQuery = ""
	if imageExists(u.String()) {
		u.Scheme = ""
		return u.String(), nil
	}

	// fallback to google figuring out what's the url
	u.Path = ""
	return "//www.google.com/s2/favicons?domain_url=" + u.String(), nil
}

func (f *feedinfo) Title() string {
	return f.feed.Title
}

func (f *feedinfo) Entries() []*Entry {
	var entries []*Entry
	for _, it := range f.feed.Items {
		link := it.Link
		if strings.HasPrefix(link, "//") {
			link = "https:" + link
		}
		e := &Entry{
			Title: it.Title,
			URL:   link,
		}
		if it.PublishedParsed != nil {
			e.Published = *it.PublishedParsed
		} else if it.UpdatedParsed != nil {
			e.Published = *it.UpdatedParsed
		} else {
			e.Published = time.Now()
		}
		entries = append(entries, e)
	}
	return entries
}

func faviconFromHTML(r io.Reader) string {
	tokenizer := html.NewTokenizer(r)
	for {
		switch tt := tokenizer.Next(); tt {
		case html.ErrorToken:
			return ""
		case html.SelfClosingTagToken, html.StartTagToken:
			token := tokenizer.Token()
			if token.Data != "link" {
				continue
			}

			favtok := false
			for _, a := range token.Attr {
				if a.Key == "rel" && strings.Contains(a.Val, "icon") {
					favtok = true
					break
				}
			}
			if favtok {
				for _, a := range token.Attr {
					if a.Key == "href" {
						return a.Val
					}
				}
			}
			return ""
		}
	}
}

// imageExists check if given url returns image. Correctness of the image is
// not validated, only first few bytes.
func imageExists(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	b := make([]byte, 128)
	if n, _ := resp.Body.Read(b); n == 0 {
		return false
	} else {
		b = b[:n]
	}

	switch t := http.DetectContentType(b); t {
	case "image/png", "image/jpeg", "image/gif", "image/bmp", "image/x-icon", "image/vnd.microsoft.icon":
		return true
	default:
		log.Printf("invalid favicon image type: %s", t)
	}
	return false
}
