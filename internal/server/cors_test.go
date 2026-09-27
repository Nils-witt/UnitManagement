package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-unit-mangement/internal/config"
)

func TestCORS(t *testing.T) {
	t.Parallel()

	s := &Server{cfg: &config.Config{CORSAllowedOrigins: []string{"https://maps.example"}}}
	h := s.cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	tests := []struct {
		name, method, path, origin string
		preflight                  bool
		wantStatus                 int
		wantAllowOrigin            string
	}{
		{"allowed", http.MethodGet, "/api/units", "https://maps.example", false, http.StatusTeapot, "https://maps.example"},
		{"allowed, other case", http.MethodGet, "/api/units", "https://MAPS.example", false, http.StatusTeapot, "https://MAPS.example"},
		{"allowed preflight", http.MethodOptions, "/api/units", "https://maps.example", true, http.StatusNoContent, "https://maps.example"},
		{"other origin", http.MethodGet, "/api/units", "https://evil.example", false, http.StatusTeapot, ""},
		{"other origin preflight", http.MethodOptions, "/api/units", "https://evil.example", true, http.StatusTeapot, ""},
		{"other scheme", http.MethodGet, "/api/units", "http://maps.example", false, http.StatusTeapot, ""},
		{"same origin", http.MethodGet, "/api/units", "", false, http.StatusTeapot, ""},
		{"outside the API", http.MethodGet, "/units", "https://maps.example", false, http.StatusTeapot, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.preflight {
				req.Header.Set("Access-Control-Request-Method", http.MethodGet)
				req.Header.Set("Access-Control-Request-Headers", "authorization")
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.wantAllowOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tc.wantAllowOrigin)
			}
			if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
				t.Errorf("Access-Control-Allow-Credentials = %q, want none", got)
			}
			if tc.wantAllowOrigin != "" && tc.preflight && rec.Header().Get("Access-Control-Allow-Headers") == "" {
				t.Error("preflight: Access-Control-Allow-Headers missing")
			}
		})
	}
}

func TestCORSDisabled(t *testing.T) {
	t.Parallel()

	s := &Server{cfg: &config.Config{}}
	req := httptest.NewRequest(http.MethodGet, "/api/units", nil)
	req.Header.Set("Origin", "https://maps.example")
	rec := httptest.NewRecorder()
	s.cors(http.NotFoundHandler()).ServeHTTP(rec, req)
	if len(rec.Header().Values("Access-Control-Allow-Origin"))+len(rec.Header().Values("Vary")) != 0 {
		t.Errorf("headers = %v, want no CORS headers", rec.Header())
	}
}
