package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"go-unit-mangement/internal/models"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidSession     = errors.New("invalid or expired session")
)

// dummyHash is compared against when a username does not exist, so that
// login timing does not reveal which usernames are registered. It is computed
// on first use rather than at package init to keep bcrypt off the startup path.
var dummyHash = sync.OnceValue(func() []byte {
	hash, _ := bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)
	return hash
})

type Service struct {
	db         *gorm.DB
	sessionTTL time.Duration
}

func NewService(db *gorm.DB, sessionTTL time.Duration) *Service {
	return &Service{db: db, sessionTTL: sessionTTL}
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// EnsureAdmin creates the initial administrator if the users table is empty.
func (s *Service) EnsureAdmin(ctx context.Context, username, password string) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	if password == "" {
		return false, errors.New("no users exist yet: set ADMIN_PASSWORD to create the initial admin account")
	}
	if _, err := s.createUser(ctx, username, password, true); err != nil {
		return false, err
	}
	return true, nil
}

// PromoteAdminIfNone grants the administrator role to username if no account
// has it yet. Databases created before roles existed have users but no
// administrator, and nobody could manage users without one.
func (s *Service) PromoteAdminIfNone(ctx context.Context, username string) (bool, error) {
	var admins int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("is_admin").Count(&admins).Error; err != nil {
		return false, err
	}
	if admins > 0 {
		return false, nil
	}
	res := s.db.WithContext(ctx).Model(&models.User{}).
		Where("username = ?", username).
		Update("is_admin", true)
	return res.RowsAffected > 0, res.Error
}

// Login verifies credentials and creates a new session. It returns the raw
// session token (to be sent to the client) and the session's expiry.
func (s *Service) Login(ctx context.Context, username, password string) (string, *models.User, time.Time, error) {
	var user models.User
	err := s.db.WithContext(ctx).Preload("Groups", byName).Where("username = ?", strings.TrimSpace(username)).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !user.HasPassword()) {
		// SSO-only accounts have no password; compare anyway so they are
		// indistinguishable by timing from unknown usernames.
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
		return "", nil, time.Time{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", nil, time.Time{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", nil, time.Time{}, ErrInvalidCredentials
	}

	token, expires, err := s.createSession(ctx, user.ID)
	if err != nil {
		return "", nil, time.Time{}, err
	}
	return token, &user, expires, nil
}

func (s *Service) createSession(ctx context.Context, userID uint) (string, time.Time, error) {
	token, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	session := models.Session{
		TokenHash: hashToken(token),
		UserID:    userID,
		ExpiresAt: time.Now().Add(s.sessionTTL),
	}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}
	return token, session.ExpiresAt, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.db.WithContext(ctx).Where("token_hash = ?", hashToken(token)).Delete(&models.Session{}).Error
}

// UserForToken resolves a session token to its user. It runs on every
// authenticated request, so the user is fetched with a JOIN in one round trip
// instead of Preload's second query.
func (s *Service) UserForToken(ctx context.Context, token string) (*models.User, error) {
	var session models.Session
	err := s.db.WithContext(ctx).
		Joins("User").
		Where("sessions.token_hash = ? AND sessions.expires_at > ?", hashToken(token), time.Now()).
		First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInvalidSession
	}
	if err != nil {
		return nil, err
	}
	return &session.User, nil
}

// CleanupExpiredSessions periodically deletes expired sessions until ctx is done.
func (s *Service) CleanupExpiredSessions(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.db.WithContext(ctx).Where("expires_at <= ?", time.Now()).Delete(&models.Session{}).Error; err != nil {
				slog.Error("cleanup expired sessions", "err", err)
			}
		}
	}
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
