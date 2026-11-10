package platform

import (
	"net/http"
	"net/url"
	"slices"
)

// CORS allows the listed origins exactly, plus any http://localhost:<port> and http://127.0.0.1:<port> (local dev).
func CORS(allowed []string) func(http.Handler) http.Handler {
	ok := func(origin string) bool {
		if origin == "" {
			return false
		}
		if slices.Contains(allowed, origin) {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil || u.Scheme != "http" || u.Port() == "" || u.Path != "" {
			return false
		}
		return u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if !ok(origin) {
				next.ServeHTTP(w, r)
				return
			}
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			h.Set("Access-Control-Max-Age", "600")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
