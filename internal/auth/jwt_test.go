package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testSigner(t *testing.T, key string) *tokenSigner {
	t.Helper()
	signer, err := newTokenSigner([]byte(strings.Repeat(key, MinJWTSecretLength)))
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func TestTokenSigner(t *testing.T) {
	t.Parallel()

	signer := testSigner(t, "a")
	now := time.Now()
	token, err := signer.sign("session-id", 42, now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	sessionID, userID, err := signer.verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if sessionID != "session-id" || userID != 42 {
		t.Errorf("verify = %q, %d; want %q, 42", sessionID, userID, "session-id")
	}

	expired, err := signer.sign("session-id", 42, now.Add(-2*time.Hour), now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"garbage", "not-a-jwt"},
		{"expired", expired},
		{"other key", mustSign(t, testSigner(t, "b"), now)},
		{"tampered", parts[0] + "." + parts[1] + "x." + parts[2]},
		{"unsigned", parts[0] + "." + parts[1] + "."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := signer.verify(tc.token); !errors.Is(err, ErrInvalidSession) {
				t.Errorf("verify = %v, want ErrInvalidSession", err)
			}
		})
	}
}

func mustSign(t *testing.T, signer *tokenSigner, now time.Time) string {
	t.Helper()
	token, err := signer.sign("session-id", 42, now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestNewTokenSignerShortSecret(t *testing.T) {
	t.Parallel()
	if _, err := newTokenSigner([]byte("short")); err == nil {
		t.Error("newTokenSigner accepted a short secret")
	}
}

func TestTokenFromRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{"none", nil, ""},
		{"bearer", map[string]string{"Authorization": "Bearer abc.def.ghi"}, "abc.def.ghi"},
		{"lowercase scheme", map[string]string{"Authorization": "bearer abc"}, "abc"},
		{"basic", map[string]string{"Authorization": "Basic dXNlcjpwYXNz"}, ""},
		{"websocket", map[string]string{"Upgrade": "websocket", "Sec-WebSocket-Protocol": "bearer, abc.def.ghi"}, "abc.def.ghi"},
		{"websocket other protocol", map[string]string{"Upgrade": "websocket", "Sec-WebSocket-Protocol": "chat, abc"}, ""},
		{"protocol without upgrade", map[string]string{"Sec-WebSocket-Protocol": "bearer, abc"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			if got := TokenFromRequest(req); got != tc.want {
				t.Errorf("TokenFromRequest = %q, want %q", got, tc.want)
			}
		})
	}
}
