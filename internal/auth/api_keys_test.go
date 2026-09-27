package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"

	"go-unit-mangement/internal/database"
	"go-unit-mangement/internal/models"
)

func testRSAKey(t *testing.T, bits int) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func TestParseAPIKeyPublicKey(t *testing.T) {
	t.Parallel()

	_, good := testRSAKey(t, 2048)
	_, small := testRSAKey(t, 1024)
	for name, pemStr := range map[string]string{"too small": small, "not pem": "hello", "empty": ""} {
		if _, err := parseAPIKeyPublicKey(pemStr); !errors.Is(err, ErrInvalidAPIKeyPublic) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := parseAPIKeyPublicKey(good); err != nil {
		t.Errorf("valid key: %v", err)
	}
}

func TestSignAPIKeyToken(t *testing.T) {
	t.Parallel()

	key, _ := testRSAKey(t, 2048)
	keyID := uuid.New()
	raw, err := SignAPIKeyToken(key, keyID, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := jwt.ParseSigned(raw, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		t.Fatal(err)
	}
	if kid := tok.Headers[0].KeyID; kid != keyID.String() {
		t.Errorf("kid = %q, want %s", kid, keyID)
	}
	var claims jwt.Claims
	if err := tok.Claims(&key.PublicKey, &claims); err != nil {
		t.Fatal(err)
	}
	if d := claims.Expiry.Time().Sub(claims.IssuedAt.Time()); d != 5*time.Minute {
		t.Errorf("lifetime = %s", d)
	}
}

// TestAPIKeyAuthentication needs a scratch PostgreSQL database in
// TEST_DATABASE_URL (see TestSyncOIDCGroups).
func TestAPIKeyAuthentication(t *testing.T) {
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
	ctx := t.Context()
	user := &models.User{Username: "apikey-" + rand.Text()[:8]}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	key, pub := testRSAKey(t, 2048)
	other, _ := testRSAKey(t, 2048)
	keyID := uuid.New()

	if _, err := s.CreateAPIKey(ctx, user.ID, keyID, "peer", pub); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateAPIKey(ctx, user.ID, keyID, "again", pub); !errors.Is(err, ErrAPIKeyExists) {
		t.Errorf("duplicate id: err = %v, want ErrAPIKeyExists", err)
	}

	valid, _ := SignAPIKeyToken(key, keyID, 5*time.Minute)
	got, err := s.UserForToken(ctx, valid)
	if err != nil || got.ID != user.ID {
		t.Fatalf("valid token: %v, %v", got, err)
	}

	for name, token := range map[string]string{
		"wrong key":      must(SignAPIKeyToken(other, keyID, 5*time.Minute)),
		"unknown kid":    must(SignAPIKeyToken(key, uuid.New(), 5*time.Minute)),
		"expired":        must(SignAPIKeyToken(key, keyID, -2*time.Minute)),
		"too long-lived": must(SignAPIKeyToken(key, keyID, 2*MaxAPIKeyTokenLifetime)),
	} {
		if _, err := s.UserForToken(ctx, token); !errors.Is(err, ErrInvalidSession) {
			t.Errorf("%s: err = %v, want ErrInvalidSession", name, err)
		}
	}

	keys, err := s.ListAPIKeys(ctx, user.ID)
	if err != nil || len(keys) != 1 || keys[0].LastUsedAt == nil {
		t.Fatalf("list: %+v, %v", keys, err)
	}
	if _, err := s.DeleteAPIKey(ctx, user.ID, keyID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserForToken(ctx, valid); !errors.Is(err, ErrInvalidSession) {
		t.Errorf("deleted key: err = %v, want ErrInvalidSession", err)
	}
}

func must(s string, err error) string {
	if err != nil {
		panic(err)
	}
	return s
}
