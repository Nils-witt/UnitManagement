package server

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"go-unit-mangement/internal/auth"
	"go-unit-mangement/internal/config"
)

type Server struct {
	cfg  *config.Config
	auth *auth.Service
	// oidc is nil when SSO is not configured.
	oidc *auth.OIDCProvider
}

func New(cfg *config.Config, authService *auth.Service, oidc *auth.OIDCProvider) *Server {
	return &Server{cfg: cfg, auth: authService, oidc: oidc}
}

// Handler builds the HTTP routes: the JSON API under /api and the embedded
// single-page app for everything else.
func (s *Server) Handler(frontend fs.FS) http.Handler {
	mux := http.NewServeMux()
	authed := func(h http.HandlerFunc) http.Handler { return s.auth.RequireAuth(h) }
	admin := func(h http.HandlerFunc) http.Handler { return s.auth.RequireAuth(auth.RequireAdmin(h)) }

	mux.HandleFunc("GET /api/auth/methods", s.handleAuthMethods)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.Handle("GET /api/auth/me", authed(s.handleMe))
	if s.oidc != nil {
		mux.HandleFunc("GET "+oidcLoginPath, s.handleOIDCLogin)
		mux.HandleFunc("GET "+oidcCallbackPath, s.handleOIDCCallback)
	}

	mux.Handle("GET /api/users", admin(s.handleListUsers))
	mux.Handle("POST /api/users", admin(s.handleCreateUser))
	mux.Handle("PUT /api/users/{id}", admin(s.handleUpdateUser))
	mux.Handle("DELETE /api/users/{id}", admin(s.handleDeleteUser))

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})

	mux.Handle("/", spaHandler(frontend))

	return logRequests(mux)
}

// decodeJSON decodes a JSON request body of at most 1 MiB into v, writing a
// 400 response and returning false if it is malformed.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", time.Since(start))
	})
}
