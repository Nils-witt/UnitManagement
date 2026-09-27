package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
)

// logRequests wraps the ResponseWriter; the upgrade must still reach the
// underlying connection.
func TestLogRequestsAllowsWebSocketUpgrade(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(logRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})))
	defer srv.Close()

	conn, _, err := websocket.Dial(t.Context(), "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.CloseNow()
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	h := securityHeaders(func() string { return "" }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for _, path := range []string{"/", "/api/health"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		for _, name := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
			if rec.Header().Get(name) == "" {
				t.Errorf("%s: header %s missing", path, name)
			}
		}
	}
}

func TestContentSecurityPolicyAllowsMapOrigin(t *testing.T) {
	t.Parallel()

	if csp := contentSecurityPolicy(""); strings.Contains(csp, "example.com") {
		t.Errorf("unset: %s", csp)
	}
	csp := contentSecurityPolicy("https://tiles.example.com")
	for _, directive := range []string{"img-src", "connect-src"} {
		i := strings.Index(csp, directive)
		if i < 0 || !strings.Contains(csp[i:i+strings.Index(csp[i:], ";")], "https://tiles.example.com") {
			t.Errorf("%s does not allow the map origin: %s", directive, csp)
		}
	}
}
