package web

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type Router struct {
	endpoints []endpoint

	// NotFound is called when none of defined handlers match current route.
	NotFound http.Handler

	// MethodNotAllowed is called when there is path match but not for current
	// method.
	MethodNotAllowed http.Handler
}

func NewRouter() *Router {
	return &Router{
		NotFound: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}),
		MethodNotAllowed: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}),
	}
}

// Add registers handler to be called whenever request matching given path
// regexp and method must be handled.
//
// Use (name) or (name:regexp) to match part of the path and pass result to
// handler. First example will use [^/]+ to make match, in second one provided
// regexp is used. Name is not used and is required only for documentation
// purposes.
//
// Using '*' as methods will match any method.
func (r *Router) Add(path, methods string, handler http.Handler) {
	builder := regexp.MustCompile(`\(.*?\)`)
	raw := builder.ReplaceAllStringFunc(path, func(s string) string {
		s = s[1 : len(s)-1]
		// every (<name>) can be optionally contain separate regexp
		// definition using notation (<name>:<regexp>)
		chunks := strings.SplitN(s, ":", 2)
		if len(chunks) == 1 {
			return `([^/]+)`
		}
		return `(` + chunks[1] + `)`
	})
	// replace {} with regular expressions syntax
	rx, err := regexp.Compile(`^` + raw + `$`)
	if err != nil {
		panic(fmt.Sprintf("invalid routing path %q: %s", path, err))
	}

	methodsSet := make(map[string]struct{})
	for _, method := range strings.Split(methods, ",") {
		methodsSet[strings.TrimSpace(method)] = struct{}{}
	}

	r.endpoints = append(r.endpoints, endpoint{
		methods: methodsSet,
		path:    rx,
		handler: handler,
	})

}

func (r *Router) AddFn(path, methods string, handler func(http.ResponseWriter, *http.Request)) {
	r.Add(path, methods, http.HandlerFunc(handler))
}

type endpoint struct {
	methods map[string]struct{}
	path    *regexp.Regexp
	handler http.Handler
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var pathMatch bool

	for _, endpoint := range rt.endpoints {
		match := endpoint.path.FindAllStringSubmatch(r.URL.Path, 1)
		if len(match) == 0 {
			continue
		}

		pathMatch = true

		_, ok := endpoint.methods[r.Method]
		if !ok {
			_, ok = endpoint.methods["*"]
		}
		if !ok {
			continue
		}

		args := match[0][1:]
		r = r.WithContext(context.WithValue(r.Context(), "web:patharg", args))
		endpoint.handler.ServeHTTP(w, r)
		return
	}

	if pathMatch {
		rt.MethodNotAllowed.ServeHTTP(w, r)
	} else {
		rt.NotFound.ServeHTTP(w, r)
	}
}

// PathArg return value as matched by path regexp at given index.
func PathArg(r *http.Request, index int) string {
	args, ok := r.Context().Value("web:patharg").([]string)
	if !ok {
		return ""
	}
	if len(args) <= index {
		return ""
	}
	return args[index]
}
