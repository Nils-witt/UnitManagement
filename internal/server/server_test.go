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
