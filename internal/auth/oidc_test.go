package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"maps"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestProvisionedUsername(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", MaxUsernameLength+10)
	tests := []struct {
		name string
		id   OIDCIdentity
		want string
	}{
		{"preferred username", OIDCIdentity{PreferredUsername: "jane", Email: "j@example.com", Subject: "s1"}, "jane"},
		{"falls back to email", OIDCIdentity{PreferredUsername: "  ", Email: "j@example.com", Subject: "s1"}, "j@example.com"},
		{"falls back to subject", OIDCIdentity{Subject: "s1"}, "s1"},
		{"skips a too long claim", OIDCIdentity{PreferredUsername: long, Email: "j@example.com"}, "j@example.com"},
		{"truncates a too long subject", OIDCIdentity{Subject: long}, long[:MaxUsernameLength]},
	}
	for _, tc := range tests {
		if got := provisionedUsername(tc.id); got != tc.want {
			t.Errorf("%s: provisionedUsername() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestParseGroups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw     string
		want    []string
		wantErr bool
	}{
		{`["a","b"]`, []string{"a", "b"}, false},
		{`[]`, []string{}, false},
		{`"a"`, []string{"a"}, false},
		{`42`, nil, true},
		{`[1]`, nil, true},
	}
	for _, tc := range tests {
		got, err := parseGroups(json.RawMessage(tc.raw))
		if (err != nil) != tc.wantErr || !slices.Equal(got, tc.want) {
			t.Errorf("parseGroups(%s) = %q, %v; want %q, error %v", tc.raw, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestExchangeSyncsAdminRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		adminGroup string
		idToken    map[string]any
		userinfo   map[string]any
		want       *bool
	}{
		{"sync off", "", map[string]any{"groups": []string{"admins"}}, nil, nil},
		{"member via id token", "admins", map[string]any{"groups": []string{"staff", "admins"}}, nil, ptr(true)},
		{"not a member", "admins", map[string]any{"groups": []string{"staff"}}, nil, ptr(false)},
		{"single group string", "admins", map[string]any{"groups": "admins"}, nil, ptr(true)},
		{"member via userinfo", "admins", nil, map[string]any{"groups": []string{"admins"}}, ptr(true)},
		{"id token wins over userinfo", "admins", map[string]any{"groups": []string{}}, map[string]any{"groups": []string{"admins"}}, ptr(false)},
		{"claim missing everywhere", "admins", nil, map[string]any{}, ptr(false)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			op := newFakeOP(t, tc.idToken, tc.userinfo)
			p, err := NewOIDCProvider(t.Context(), OIDCConfig{
				IssuerURL:   op.URL,
				ClientID:    "client",
				RedirectURL: "http://app.example" + OIDCCallbackPath,
				AdminGroup:  tc.adminGroup,
				GroupsClaim: "groups",
			})
			if err != nil {
				t.Fatal(err)
			}
			flow := OIDCFlow{State: "state", Nonce: fakeNonce, Verifier: "verifier"}
			id, err := p.Exchange(t.Context(), flow, "state", "code")
			if err != nil {
				t.Fatal(err)
			}
			if id.Subject != "sub-1" {
				t.Errorf("subject = %q", id.Subject)
			}
			if (id.IsAdmin == nil) != (tc.want == nil) || (id.IsAdmin != nil && *id.IsAdmin != *tc.want) {
				t.Errorf("IsAdmin = %v, want %v", deref(id.IsAdmin), deref(tc.want))
			}
		})
	}
}

const fakeNonce = "nonce"

// newFakeOP serves just enough of an OpenID provider for Exchange: discovery,
// keys, a token endpoint issuing an ID token with extraClaims, and userinfo.
func newFakeOP(t *testing.T, extraClaims, userinfo map[string]any) *httptest.Server {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.RawURLEncoding.EncodeToString
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                                srv.URL,
			"authorization_endpoint":                srv.URL + "/auth",
			"token_endpoint":                        srv.URL + "/token",
			"userinfo_endpoint":                     srv.URL + "/userinfo",
			"jwks_uri":                              srv.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"keys": []map[string]any{{
			"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "k1",
			"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		claims := map[string]any{
			"iss": srv.URL, "sub": "sub-1", "aud": "client", "nonce": fakeNonce,
			"iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(),
		}
		maps.Copy(claims, extraClaims)
		header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "k1", "typ": "JWT"})
		payload, _ := json.Marshal(claims)
		signed := b64(header) + "." + b64(payload)
		digest := sha256.Sum256([]byte(signed))
		sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
		if err != nil {
			t.Error(err)
		}
		writeJSON(w, map[string]any{
			"access_token": "access", "token_type": "Bearer", "expires_in": 60,
			"id_token": signed + "." + b64(sig),
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, _ *http.Request) {
		if userinfo == nil {
			t.Error("userinfo fetched although the id token has the groups claim")
		}
		info := map[string]any{"sub": "sub-1"}
		maps.Copy(info, userinfo)
		writeJSON(w, info)
	})
	return srv
}

func ptr[T any](v T) *T { return &v }

func deref(b *bool) any {
	if b == nil {
		return nil
	}
	return *b
}
