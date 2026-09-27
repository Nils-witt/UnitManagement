package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-unit-mangement/internal/models"
)

const (
	// MinAPIKeyBits is the smallest accepted RSA modulus of an API key.
	MinAPIKeyBits = 2048
	// MaxAPIKeyNameLength bounds an API key's label.
	MaxAPIKeyNameLength = 64
	// MaxAPIKeyTokenLifetime is the longest validity (exp - iat) accepted for
	// a JWT signed with an API key. The caller signs its own tokens, so this
	// is what keeps a leaked one from working for long.
	MaxAPIKeyTokenLifetime = time.Hour
	// apiKeyClockSkew tolerates clocks of the signing server running slightly
	// ahead of or behind this one.
	apiKeyClockSkew = time.Minute
	// apiKeyLastUsedInterval throttles LastUsedAt updates to one write per
	// key and interval, instead of one per request.
	apiKeyLastUsedInterval = time.Minute
)

var (
	ErrAPIKeyNotFound      = errors.New("api key not found")
	ErrAPIKeyExists        = errors.New("an api key with this id already exists")
	ErrInvalidAPIKeyName   = fmt.Errorf("api key name must be 1 to %d characters", MaxAPIKeyNameLength)
	ErrInvalidAPIKeyID     = errors.New("api key id must be a UUID")
	ErrInvalidAPIKeyPublic = fmt.Errorf("public key must be a PEM-encoded RSA public key of at least %d bits", MinAPIKeyBits)
)

// apiKeySigningAlgorithms are what an API key may sign with. Neither this
// app's own HS256 tokens nor OIDC access tokens with a non-UUID "kid" are
// taken for one.
var apiKeySigningAlgorithms = []jose.SignatureAlgorithm{jose.RS256}

// CreateAPIKey registers publicKeyPEM for the user under id. id is chosen by
// the caller: for another instance syncing from this one it must be that
// instance's server UUID, which it signs its JWTs' "kid" with.
func (s *Service) CreateAPIKey(ctx context.Context, userID uint, id uuid.UUID, name, publicKeyPEM string) (*models.APIKey, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxAPIKeyNameLength {
		return nil, ErrInvalidAPIKeyName
	}
	if id == uuid.Nil {
		return nil, ErrInvalidAPIKeyID
	}
	publicKeyPEM = strings.TrimSpace(publicKeyPEM) + "\n"
	if _, err := parseAPIKeyPublicKey(publicKeyPEM); err != nil {
		return nil, err
	}
	user, err := s.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	key := &models.APIKey{ID: id, UserID: user.ID, Name: name, PublicKeyPEM: publicKeyPEM}
	if err := s.db.WithContext(ctx).Create(key).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrAPIKeyExists
		}
		return nil, fmt.Errorf("create api key: %w", err)
	}
	return key, nil
}

// ListAPIKeys returns the user's API keys, newest first.
func (s *Service) ListAPIKeys(ctx context.Context, userID uint) ([]models.APIKey, error) {
	if _, err := s.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	var keys []models.APIKey
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&keys).Error
	if err != nil {
		return nil, fmt.Errorf("list api keys of user %d: %w", userID, err)
	}
	return keys, nil
}

// DeleteAPIKey removes the user's API key with keyID and returns it as it
// was. JWTs signed with it stop working right away.
func (s *Service) DeleteAPIKey(ctx context.Context, userID uint, keyID uuid.UUID) (*models.APIKey, error) {
	var key models.APIKey
	res := s.db.WithContext(ctx).Clauses(clause.Returning{}).Where("user_id = ? AND id = ?", userID, keyID).Delete(&key)
	if res.Error != nil {
		return nil, fmt.Errorf("delete api key %s: %w", keyID, res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, ErrAPIKeyNotFound
	}
	return &key, nil
}

// userForAPIKey reports whether raw is a JWT signed with an API key (an
// RS256 JWT whose "kid" is a UUID) and, if so, the user the key acts as. Any
// other token is left unhandled.
func (s *Service) userForAPIKey(ctx context.Context, raw string) (user *models.User, handled bool, err error) {
	parsed, err := jwt.ParseSigned(raw, apiKeySigningAlgorithms)
	if err != nil || len(parsed.Headers) != 1 {
		return nil, false, nil
	}
	keyID, err := uuid.Parse(parsed.Headers[0].KeyID)
	if err != nil {
		return nil, false, nil
	}

	var key models.APIKey
	err = s.db.WithContext(ctx).Joins("User").Where("api_keys.id = ?", keyID).First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, true, ErrInvalidSession
	}
	if err != nil {
		return nil, true, err
	}
	pub, err := parseAPIKeyPublicKey(key.PublicKeyPEM)
	if err != nil {
		return nil, true, err
	}
	var claims jwt.Claims
	if err := parsed.Claims(pub, &claims); err != nil {
		slog.Debug("api key token rejected", "key", keyID, "err", err)
		return nil, true, ErrInvalidSession
	}
	if claims.IssuedAt == nil || claims.Expiry == nil ||
		claims.Expiry.Time().Sub(claims.IssuedAt.Time()) > MaxAPIKeyTokenLifetime {
		return nil, true, ErrInvalidSession
	}
	if err := claims.ValidateWithLeeway(jwt.Expected{Time: time.Now()}, apiKeyClockSkew); err != nil {
		return nil, true, ErrInvalidSession
	}

	err = s.db.WithContext(ctx).Model(&models.APIKey{}).
		Where("id = ? AND (last_used_at IS NULL OR last_used_at < ?)", keyID, time.Now().Add(-apiKeyLastUsedInterval)).
		Update("last_used_at", time.Now()).Error
	if err != nil {
		slog.Warn("record api key use", "key", keyID, "err", err)
	}
	return &key.User, true, nil
}

// parseAPIKeyPublicKey parses a PEM-encoded PKIX RSA public key of at least
// MinAPIKeyBits.
func parseAPIKeyPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, ErrInvalidAPIKeyPublic
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, ErrInvalidAPIKeyPublic
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok || rsaPub.N.BitLen() < MinAPIKeyBits {
		return nil, ErrInvalidAPIKeyPublic
	}
	return rsaPub, nil
}

// SignAPIKeyToken returns a JWT signed with key that authenticates as the API
// key registered under keyID, valid for ttl (at most MaxAPIKeyTokenLifetime).
// internal/sync signs its requests to other instances with it.
func SignAPIKeyToken(key *rsa.PrivateKey, keyID uuid.UUID, ttl time.Duration) (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader(jose.HeaderKey("kid"), keyID.String()),
	)
	if err != nil {
		return "", fmt.Errorf("create api key signer: %w", err)
	}
	now := time.Now()
	token, err := jwt.Signed(signer).Claims(jwt.Claims{
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(ttl)),
	}).Serialize()
	if err != nil {
		return "", fmt.Errorf("sign api key token: %w", err)
	}
	return token, nil
}
