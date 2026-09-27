package server

import (
	"net/http"
	"slices"
	"strings"
)

// corsAllowed reports whether origin, a request's Origin header, belongs to
// another web app allowed to call the API (CORS_ALLOWED_ORIGINS).
func (s *Server) corsAllowed(origin string) bool {
	return origin != "" && slices.Contains(s.cfg.CORSAllowedOrigins, strings.ToLower(origin))
}

// cors lets the allowed origins call the API from the browser. Clients
// authenticate with bearer tokens, never cookies, so credentialed requests
// are not allowed. Requests from other origins pass through unchanged and
// the browser keeps blocking them.
func (s *Server) cors(next http.Handler) http.Handler {
	if len(s.cfg.CORSAllowedOrigins) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		if !s.corsAllowed(origin) {
			next.ServeHTTP(w, r)
			return
		}
		h.Set("Access-Control-Allow-Origin", origin)
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h.Add("Vary", "Access-Control-Request-Method, Access-Control-Request-Headers")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.Set("Access-Control-Expose-Headers", "WWW-Authenticate, Retry-After")
		next.ServeHTTP(w, r)
	})
}
