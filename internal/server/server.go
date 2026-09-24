package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"go-unit-mangement/internal/auth"
	"go-unit-mangement/internal/config"
	"go-unit-mangement/internal/units"
)

type Server struct {
	cfg  *config.Config
	auth *auth.Service
	// oidc is nil when SSO is not configured.
	oidc  *auth.OIDCProvider
	units *units.Service
	// shutdown is closed by CloseStreams to end long-lived connections;
	// streams tracks the ones still open.
	shutdown     chan struct{}
	shutdownOnce sync.Once
	streams      sync.WaitGroup
}

func New(cfg *config.Config, authService *auth.Service, oidc *auth.OIDCProvider, unitService *units.Service) *Server {
	return &Server{cfg: cfg, auth: authService, oidc: oidc, units: unitService, shutdown: make(chan struct{})}
}

// CloseStreams ends every open event stream and waits until they are closed
// or ctx is done. http.Server.Shutdown does not track hijacked WebSocket
// connections, so call this after it.
func (s *Server) CloseStreams(ctx context.Context) error {
	s.shutdownOnce.Do(func() { close(s.shutdown) })
	done := make(chan struct{})
	go func() {
		s.streams.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
	mux.Handle("GET /api/groups", admin(s.handleListGroups))

	mux.Handle("GET /api/units", authed(s.handleListUnits))
	mux.Handle("GET /api/units/events", authed(s.handleUnitEvents))
	mux.Handle("POST /api/units", authed(s.handleCreateUnit))
	mux.Handle("GET /api/units/{id}", authed(s.handleGetUnit))
	mux.Handle("GET /api/units/{id}/positions", authed(s.handleUnitPositions))
	mux.Handle("PUT /api/units/{id}", authed(s.handleUpdateUnit))
	mux.Handle("PATCH /api/units/{id}", authed(s.handlePatchUnit))
	mux.Handle("DELETE /api/units/{id}", authed(s.handleDeleteUnit))

	mux.HandleFunc("GET /api/version", s.handleVersion)
	mux.HandleFunc("GET /api/instance", s.handleInstance)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})

	mux.Handle("/", spaHandler(frontend))

	return realIP(s.cfg.TrustedProxies, logRequests(mux))
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

// Unwrap lets http.ResponseController reach the underlying writer, which the
// WebSocket upgrade needs to hijack the connection.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "remote", r.RemoteAddr, "duration", time.Since(start))
	})
}
