package ui

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"time"
)

type Renderer interface {
	Render(w http.ResponseWriter, templateName string, content interface{}, statusCode int)
	RenderStd(w http.ResponseWriter, statusCode int)
}

type renderService struct {
	render func(io.Writer, string, interface{}) error
}

var _ Renderer = (*renderService)(nil)

func NewHTMLRenderer(glob string, debug bool) (Renderer, error) {
	if !debug {
		r, err := renderer(glob, false)
		if err != nil {
			return nil, err
		}
		return &renderService{render: r}, nil
	}

	srv := &renderService{
		render: func(w io.Writer, n string, c interface{}) error {
			render, err := renderer(glob, true)
			if err != nil {
				return err
			}
			return render(w, n, c)
		},
	}

	return srv, nil
}

func renderer(glob string, debug bool) (func(io.Writer, string, interface{}) error, error) {
	tmpl, err := template.New("").Funcs(map[string]interface{}{
		"debug": func() bool {
			return debug
		},
		"timesince": fnTimesince,
	}).ParseGlob(glob)
	if err != nil {
		return nil, err
	}
	return tmpl.ExecuteTemplate, nil
}

func (s *renderService) Render(
	w http.ResponseWriter,
	template string,
	context interface{},
	status int,
) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var b bytes.Buffer
	start := time.Now()
	if err := s.render(&b, template, context); err != nil {
		log.Printf("cannot render %s: %s", template, err)

		b.Reset()
		if err := s.render(&b, "error-internal.tmpl", nil); err != nil {
			b.Reset()
			fmt.Fprintln(&b, "Internal Server Errror")
		}
		status = http.StatusInternalServerError
	}
	w.Header().Set("Template-Render-Time", time.Now().Sub(start).String())
	w.WriteHeader(status)
	b.WriteTo(w)
}

func (s *renderService) RenderStd(w http.ResponseWriter, statusCode int) {
	switch statusCode {
	case http.StatusInternalServerError:
		s.Render(w, "error-internal.tmpl", nil, http.StatusInternalServerError)
	case http.StatusNotFound:
		s.Render(w, "error-not-found.tmpl", nil, http.StatusUnauthorized)
	case http.StatusUnauthorized:
		s.Render(w, "error-unauthorized.tmpl", nil, http.StatusUnauthorized)
	default:
		http.Error(w, http.StatusText(statusCode), statusCode)
	}
}
