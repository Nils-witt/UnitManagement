package auth

import (
	"crypto/rand"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"go-unit-mangement/internal/database"
	"go-unit-mangement/internal/models"
)

// TestCreateToken needs a scratch PostgreSQL database in TEST_DATABASE_URL
// (see TestSyncOIDCGroups).
func TestCreateToken(t *testing.T) {
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

	user, err := s.CreateUser(ctx, rand.Text()[:8]+"-device", "password123", false)
	if err != nil {
		t.Fatal(err)
	}

	ttl := 30 * 24 * time.Hour
	token, session, err := s.CreateToken(ctx, user.ID, "  sensor  ", ttl)
	if err != nil {
		t.Fatal(err)
	}
	if session.UserID != user.ID || session.Name != "sensor" || !session.APIToken {
		t.Fatalf("session = user %d, name %q, api token %t; want user %d, name \"sensor\", api token", session.UserID, session.Name, session.APIToken, user.ID)
	}
	if d := time.Until(session.ExpiresAt); d < ttl-time.Minute || d > ttl {
		t.Fatalf("expires in %s, want about %s", d, ttl)
	}
	authed, err := s.UserForToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if authed.ID != user.ID {
		t.Fatalf("token authenticates user %d, want %d", authed.ID, user.ID)
	}

	for _, ttl := range []time.Duration{0, MinTokenTTL - time.Second, MaxTokenTTL + time.Second} {
		if _, _, err := s.CreateToken(ctx, user.ID, "x", ttl); !errors.Is(err, ErrInvalidTokenTTL) {
			t.Errorf("ttl %s: err = %v, want ErrInvalidTokenTTL", ttl, err)
		}
	}
	for _, name := range []string{"", "  ", strings.Repeat("n", MaxTokenNameLength+1)} {
		if _, _, err := s.CreateToken(ctx, user.ID, name, ttl); !errors.Is(err, ErrInvalidTokenName) {
			t.Errorf("name %q: err = %v, want ErrInvalidTokenName", name, err)
		}
	}
	if _, _, err := s.CreateToken(ctx, 0, "x", ttl); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("unknown user: err = %v, want ErrUserNotFound", err)
	}

	// Sign-ins are not listed and can't be revoked as tokens.
	_, _, _, err = s.Login(ctx, user.Username, "password123")
	if err != nil {
		t.Fatal(err)
	}
	second, secondSession, err := s.CreateToken(ctx, user.ID, "tablet", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := s.ListTokens(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, l := range listed {
		names = append(names, l.Name)
	}
	if !slices.Equal(names, []string{"tablet", "sensor"}) {
		t.Fatalf("listed tokens = %q, want [tablet sensor]", names)
	}
	var loginSession models.Session
	if err := db.Where("user_id = ? AND NOT api_token", user.ID).First(&loginSession).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeToken(ctx, user.ID, loginSession.ID); !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("revoke sign-in session: err = %v, want ErrTokenNotFound", err)
	}
	if err := s.RevokeToken(ctx, user.ID+1, secondSession.ID); !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("revoke another user's token: err = %v, want ErrTokenNotFound", err)
	}

	if err := s.RevokeToken(ctx, user.ID, secondSession.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserForToken(ctx, second); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("revoked token: err = %v, want ErrInvalidSession", err)
	}
	if _, err := s.UserForToken(ctx, token); err != nil {
		t.Fatalf("other token after revoke: %v", err)
	}
	if err := s.RevokeToken(ctx, user.ID, secondSession.ID); !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("revoke twice: err = %v, want ErrTokenNotFound", err)
	}

	if _, err := s.UpdateUser(ctx, user.ID, false, "new-password123"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserForToken(ctx, token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("after password change: err = %v, want ErrInvalidSession", err)
	}
}
