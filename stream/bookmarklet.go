package stream

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"strings"
)

type BookmarkletRenderer interface {
	RenderAttr(userSecretKey string) (template.HTMLAttr, error)
}

type Bookmarklet struct {
	Site string
}

func (b *Bookmarklet) RenderAttr(userSecretKey string) (template.HTMLAttr, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, b); err != nil {
		return "", fmt.Errorf("cannot render: %s", err)
	}
	href := html.EscapeString(strings.Replace(buf.String(), "\n", ";", -1))
	attr := template.HTMLAttr(fmt.Sprintf(`href="javascript:(function(){%s}())"`, href))
	return attr, nil
}

// XXX - http only during tests
var tmpl = template.Must(template.New("").Parse(`
var f = document.createElement('iframe')
f.style.display = "none"
f.src = '{{.Site}}/bookmarklet'
f.onload = function () {
	var url = location.href
	var canonical = document.querySelector("link[rel='canonical']")
	if (canonical && canonical.href) { url = canonical.href }
	f.contentWindow.postMessage({title: document.title, url: url}, "*")
}
document.body.appendChild(f)
`))
