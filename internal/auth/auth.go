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
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"go-unit-mangement/internal/models"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidSession     = errors.New("invalid or expired session")
	// ErrAdminPasswordRequired means no administrator exists and there is no
	// ADMIN_PASSWORD to create one with.
	ErrAdminPasswordRequired = errors.New("no administrator exists: set ADMIN_PASSWORD to create the ADMIN_USERNAME account")
	ErrInvalidTokenTTL       = fmt.Errorf("token lifetime must be between %s and %s", MinTokenTTL, MaxTokenTTL)
	ErrInvalidTokenName      = fmt.Errorf("token name must be 1 to %d characters", MaxTokenNameLength)
	ErrTokenNotFound         = errors.New("token not found")
)

// Bounds of an API token's lifetime (see CreateToken).
const (
	MinTokenTTL = time.Minute
	MaxTokenTTL = 10 * 365 * 24 * time.Hour

	MaxTokenNameLength = 64
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
	tokens     *tokenSigner
	// accessTokens, if set, also accepts OIDC access tokens (see
	// SetOIDCAccessTokens).
	accessTokens *OIDCAccessTokens
}

// NewService signs access tokens with jwtSecret, which must be at least
// MinJWTSecretLength bytes long.
func NewService(db *gorm.DB, sessionTTL time.Duration, jwtSecret []byte) (*Service, error) {
	tokens, err := newTokenSigner(jwtSecret)
	if err != nil {
		return nil, err
	}
	return &Service{db: db, sessionTTL: sessionTTL, tokens: tokens}, nil
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// EnsureAdmin creates username as an administrator with password if no
// account has the role, so there is always someone who can manage users. It
// never promotes an existing account: that could be an SSO account, whose
// username a user at the provider may have picked. It reports whether it
// created the account, ErrAdminPasswordRequired if one is needed but password
// is empty, and ErrUsernameTaken if username belongs to another account.
func (s *Service) EnsureAdmin(ctx context.Context, username, password string) (bool, error) {
	if ok, err := s.HasAdmin(ctx); ok || err != nil {
		return false, err
	}
	if password == "" {
		return false, ErrAdminPasswordRequired
	}
	// Checked up front so the expected case doesn't log a failed INSERT.
	var taken int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("username = ?", strings.TrimSpace(username)).Count(&taken).Error; err != nil {
		return false, err
	}
	if taken > 0 {
		return false, ErrUsernameTaken
	}
	if _, err := s.createUser(ctx, username, password, true); err != nil {
		return false, err
	}
	return true, nil
}

// HasAdmin reports whether any account has the administrator role.
func (s *Service) HasAdmin(ctx context.Context) (bool, error) {
	var admins int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("is_admin").Count(&admins).Error; err != nil {
		return false, err
	}
	return admins > 0, nil
}

// Login verifies credentials and creates a new session. It returns the
// session's access token (a JWT to be sent to the client) and its expiry.
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

	session, err := s.createSession(ctx, models.Session{UserID: user.ID}, s.sessionTTL)
	if err != nil {
		return "", nil, time.Time{}, err
	}
	return session.token, &user, session.ExpiresAt, nil
}

// CreateToken issues a named access token for the user with id that is
// valid for ttl, e.g. for a device or integration acting as that user. It
// ends when it is revoked (RevokeToken), the user is deleted or their
// password is changed.
func (s *Service) CreateToken(ctx context.Context, id uint, name string, ttl time.Duration) (string, *models.Session, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxTokenNameLength {
		return "", nil, ErrInvalidTokenName
	}
	if ttl < MinTokenTTL || ttl > MaxTokenTTL {
		return "", nil, ErrInvalidTokenTTL
	}
	user, err := s.GetUser(ctx, id)
	if err != nil {
		return "", nil, err
	}
	session, err := s.createSession(ctx, models.Session{UserID: user.ID, APIToken: true, Name: name}, ttl)
	if err != nil {
		return "", nil, err
	}
	session.User = *user
	return session.token, &session.Session, nil
}

// ListTokens returns the user's unexpired API tokens, newest first. Their
// token values are not stored, so only the metadata can be listed.
func (s *Service) ListTokens(ctx context.Context, userID uint) ([]models.Session, error) {
	if _, err := s.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	var tokens []models.Session
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND api_token AND expires_at > ?", userID, time.Now()).
		Order("created_at DESC, id DESC").
		Find(&tokens).Error
	if err != nil {
		return nil, fmt.Errorf("list tokens of user %d: %w", userID, err)
	}
	return tokens, nil
}

// RevokeToken ends the user's API token with tokenID. Sign-in sessions
// can't be revoked this way.
func (s *Service) RevokeToken(ctx context.Context, userID, tokenID uint) error {
	res := s.db.WithContext(ctx).Where("user_id = ? AND api_token", userID).Delete(&models.Session{}, tokenID)
	if res.Error != nil {
		return fmt.Errorf("revoke token %d: %w", tokenID, res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrTokenNotFound
	}
	return nil
}

// newSession is a stored session together with its access token.
type newSession struct {
	models.Session
	token string
}

// createSession stores session, valid for ttl from now, and returns it with
// an access token. The token's "jti" is the session's random ID, of which
// only a hash is stored, so a database leak does not expose live sessions.
func (s *Service) createSession(ctx context.Context, session models.Session, ttl time.Duration) (*newSession, error) {
	sessionID, err := newSessionID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	session.TokenHash = hashToken(sessionID)
	// JWT times have second precision; truncating keeps the token's expiry
	// and the session's in step.
	session.ExpiresAt = now.Add(ttl).Truncate(time.Second)
	token, err := s.tokens.sign(sessionID, session.UserID, now, session.ExpiresAt)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return &newSession{Session: session, token: token}, nil
}

// Logout ends the session of an access token. A token that is invalid or
// already expired has no session to end, so that is not an error.
func (s *Service) Logout(ctx context.Context, token string) error {
	sessionID, _, err := s.tokens.verify(token)
	if err != nil {
		return nil
	}
	return s.db.WithContext(ctx).Where("token_hash = ?", hashToken(sessionID)).Delete(&models.Session{}).Error
}

// UserForToken verifies an access token and resolves its session to the
// user. A validly signed token only counts while its session exists, so
// logout and password resets revoke it. This runs on every authenticated
// request, so the user is fetched with a JOIN in one round trip instead of
// Preload's second query. With SetOIDCAccessTokens, a trusted OIDC
// provider's access token is accepted too (see userForAccessToken).
func (s *Service) UserForToken(ctx context.Context, token string) (*models.User, error) {
	if user, handled, err := s.userForAccessToken(ctx, token); handled {
		return user, err
	}
	sessionID, userID, err := s.tokens.verify(token)
	if err != nil {
		return nil, err
	}
	var session models.Session
	err = s.db.WithContext(ctx).
		Joins("User").
		Where("sessions.token_hash = ? AND sessions.user_id = ? AND sessions.expires_at > ?", hashToken(sessionID), userID, time.Now()).
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

func newSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session ID: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
