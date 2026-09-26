package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"go-unit-mangement/internal/database"
	"go-unit-mangement/internal/models"
)

// newTestAccessTokens returns an authenticator trusting a fake provider for
// the audiences, and a function minting access tokens from it: issued for
// "api" and expiring in a minute unless claims say otherwise.
func newTestAccessTokens(t *testing.T, adminGroup string, audiences ...string) (*OIDCAccessTokens, string, func(claims map[string]any) string) {
	t.Helper()
	op, sign := newSigningFakeOP(t, nil, nil)
	a, err := NewOIDCAccessTokens(t.Context(), nil, OIDCConfig{
		AccessTokenAudiences: audiences,
		AccessTokenIssuers:   []string{op.URL},
		GroupsClaim:          "groups",
		AdminGroup:           adminGroup,
	})
	if err != nil {
		t.Fatal(err)
	}
	mint := func(claims map[string]any) string {
		all := map[string]any{
			"iss": op.URL, "sub": "sub-1", "aud": "api",
			"iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(),
		}
		maps.Copy(all, claims)
		return sign(all)
	}
	return a, op.URL, mint
}

func TestAccessTokenVerify(t *testing.T) {
	t.Parallel()

	a, issuer, mint := newTestAccessTokens(t, "admins", "api", "other")
	tests := []struct {
		name       string
		claims     map[string]any
		wantErr    bool
		wantGroups []string
		wantAdmin  *bool
	}{
		{"valid", map[string]any{"preferred_username": "jane"}, false, nil, nil},
		{"one of several audiences", map[string]any{"aud": []string{"x", "other"}}, false, nil, nil},
		{"groups and admin role", map[string]any{"groups": []string{"staff", "admins"}}, false, []string{"admins", "staff"}, ptr(true)},
		{"groups without admin group", map[string]any{"groups": []string{"staff"}}, false, []string{"staff"}, ptr(false)},
		{"wrong audience", map[string]any{"aud": "client"}, true, nil, nil},
		{"expired", map[string]any{"exp": time.Now().Add(-time.Minute).Unix()}, true, nil, nil},
		{"keycloak id token", map[string]any{"typ": "ID"}, true, nil, nil},
		{"id token with nonce", map[string]any{"nonce": "n"}, true, nil, nil},
		{"malformed groups", map[string]any{"groups": 42}, true, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw := mint(tc.claims)
			verifier := a.verifierFor(raw)
			if verifier == nil {
				t.Fatal("token from trusted issuer not handled")
			}
			id, expiry, hasGroups, err := a.verify(t.Context(), verifier, raw)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %t", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if id.Issuer != issuer || id.Subject != "sub-1" || expiry.IsZero() {
				t.Errorf("identity = %+v, expiry %s", id, expiry)
			}
			if hasGroups != (tc.wantGroups != nil) || (hasGroups && !slices.Equal(id.Groups, tc.wantGroups)) {
				t.Errorf("groups = %q (claim present %t), want %q", id.Groups, hasGroups, tc.wantGroups)
			}
			if (id.IsAdmin == nil) != (tc.wantAdmin == nil) || (id.IsAdmin != nil && *id.IsAdmin != *tc.wantAdmin) {
				t.Errorf("IsAdmin = %v, want %v", deref(id.IsAdmin), deref(tc.wantAdmin))
			}
		})
	}

	t.Run("bad signature", func(t *testing.T) {
		t.Parallel()
		raw := mint(nil)
		raw = raw[:len(raw)-4] + strings.Repeat("A", 4)
		if _, _, _, err := a.verify(t.Context(), a.verifierFor(raw), raw); err == nil {
			t.Fatal("tampered token verified")
		}
	})
}

func TestAccessTokenNotHandled(t *testing.T) {
	t.Parallel()

	a, _, mint := newTestAccessTokens(t, "", "api")
	signer, err := newTokenSigner([]byte(strings.Repeat("k", MinJWTSecretLength)))
	if err != nil {
		t.Fatal(err)
	}
	own, err := signer.sign("session", 1, time.Now(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string]string{
		"own session token": own,
		"untrusted issuer":  mint(map[string]any{"iss": "https://evil.example"}),
		"not a jwt":         "opaque-token",
	} {
		if a.verifierFor(raw) != nil {
			t.Errorf("%s: handled", name)
		}
	}

	// A disabled authenticator handles nothing.
	s := &Service{}
	if _, handled, _ := s.userForAccessToken(t.Context(), mint(nil)); handled {
		t.Error("handled without access tokens enabled")
	}
}

func TestNewOIDCAccessTokensConfig(t *testing.T) {
	t.Parallel()

	a, err := NewOIDCAccessTokens(t.Context(), nil, OIDCConfig{})
	if a != nil || err != nil {
		t.Errorf("no audience: %v, %v; want disabled", a, err)
	}
	if _, err := NewOIDCAccessTokens(t.Context(), nil, OIDCConfig{AccessTokenIssuers: []string{"https://idp.example"}}); err == nil {
		t.Error("issuers without audience accepted")
	}
	if _, err := NewOIDCAccessTokens(t.Context(), nil, OIDCConfig{AccessTokenAudiences: []string{"api"}}); err == nil {
		t.Error("audience without issuer accepted")
	}
}

func TestAccessTokenCacheExpiry(t *testing.T) {
	t.Parallel()

	a := &OIDCAccessTokens{cache: map[[sha256.Size]byte]cachedAccessToken{}}
	short, long := sha256.Sum256([]byte("short")), sha256.Sum256([]byte("long"))
	a.store(short, 1, time.Now().Add(-time.Second))
	a.store(long, 2, time.Now().Add(time.Hour))
	if _, ok := a.cached(short); ok {
		t.Error("entry outlived its token")
	}
	if id, ok := a.cached(long); !ok || id != 2 {
		t.Errorf("cached = %d, %t; want 2, true", id, ok)
	}
	if e := a.cache[long]; time.Until(e.expires) > accessTokenCacheTTL {
		t.Errorf("entry expires in %s, want at most %s", time.Until(e.expires), accessTokenCacheTTL)
	}
}

// TestUserForAccessToken needs a scratch PostgreSQL database in
// TEST_DATABASE_URL (see TestSyncOIDCGroups).
func TestUserForAccessToken(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := database.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewService(db, time.Hour, []byte(strings.Repeat("k", MinJWTSecretLength)))
	if err != nil {
		t.Fatal(err)
	}
	a, _, mint := newTestAccessTokens(t, "", "api")
	s.SetOIDCAccessTokens(a)
	ctx := t.Context()

	subject := "sub-" + rand.Text()
	group := "grp-" + rand.Text()[:8]
	withGroups := mint(map[string]any{"sub": subject, "preferred_username": subject, "groups": []string{group}})
	user, err := s.UserForToken(ctx, withGroups)
	if err != nil {
		t.Fatal(err)
	}
	if user.OIDCSubject == nil || *user.OIDCSubject != subject || user.IsAdmin {
		t.Fatalf("provisioned %+v", user)
	}

	// A token without the groups claim resolves to the same account and
	// leaves its groups alone.
	withoutGroups := mint(map[string]any{"sub": subject, "exp": time.Now().Add(2 * time.Minute).Unix()})
	again, err := s.UserForToken(ctx, withoutGroups)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != user.ID {
		t.Fatalf("second token resolved to user %d, want %d", again.ID, user.ID)
	}
	loaded, err := s.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Groups) != 1 || loaded.Groups[0].Name != group {
		t.Fatalf("groups = %+v, want [%s]", loaded.Groups, group)
	}

	// A deleted account stops working even while its token is cached.
	if _, err := s.DeleteUser(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserForToken(ctx, withGroups); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("deleted user: err = %v, want ErrInvalidSession", err)
	}
	if err := db.Where("name = ?", group).Delete(&models.Group{}).Error; err != nil {
		t.Fatal(err)
	}
}
