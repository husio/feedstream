package stream

import (
	"strings"
	"testing"
)

func TestFaviconFromHTML(t *testing.T) {
	cases := map[string]struct {
		Fav  string
		HTML string
	}{
		"empty": {
			Fav:  "",
			HTML: "",
		},
		"just-tag": {
			Fav:  "http://foo.com/favicon.ico",
			HTML: `<link rel="icon" href="http://foo.com/favicon.ico">`,
		},
		"self-closing": {
			Fav:  "http://foo.com/favicon.ico",
			HTML: `<link rel="icon" href="http://foo.com/favicon.ico" />`,
		},
		"with-extra-rel": {
			Fav:  "http://foo.com/favicon.ico",
			HTML: `<link rel="shortcut icon" href="http://foo.com/favicon.ico" />`,
		},
		"inside-html": {
			Fav:  "http://foo.com/favicon.ico",
			HTML: `<!doctype html><html><head><link rel="icon shortcut" href="http://foo.com/favicon.ico"></head>`,
		},
	}

	for tname, tc := range cases {
		t.Run(tname, func(t *testing.T) {
			fav := faviconFromHTML(strings.NewReader(tc.HTML))
			if fav != tc.Fav {
				t.Fatalf("want %q, got %q", tc.Fav, fav)
			}
		})
	}
}
