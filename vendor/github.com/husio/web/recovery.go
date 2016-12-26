package web

import "net/http"

// Recovery wrap handler and stop propagation of all panics from it. If
// possible, Internal Server Error response is written
func Recovery(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				if w.Header().Get("Content-Type") == "" {
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				}
				const code = http.StatusInternalServerError
				http.Error(w, http.StatusText(code), code)
			}
		}()

		h.ServeHTTP(w, r)
	}
}
