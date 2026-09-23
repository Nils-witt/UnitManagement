package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"go-unit-mangement/internal/models"
)

// OIDCCallbackPath is where the provider sends the browser back after
// sign-in; OIDC_REDIRECT_URL must point here.
const OIDCCallbackPath = "/api/auth/oidc/callback"

// OIDCConfig configures the optional OpenID Connect (SSO) login.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// OIDCProvider runs the authorization code flow (with PKCE) against one
// provider. A nil *OIDCProvider means SSO is not configured.
type OIDCProvider struct {
	verifier *oidc.IDTokenVerifier
	oauth2   oauth2.Config
}

// NewOIDCProvider discovers the provider via its
// /.well-known/openid-configuration document.
func NewOIDCProvider(ctx context.Context, cfg OIDCConfig) (*OIDCProvider, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider %q: %w", cfg.IssuerURL, err)
	}
	return &OIDCProvider{
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth2: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
	}, nil
}

// OIDCFlow is the per-attempt secret state that the browser carries (in a
// short-lived cookie) from the start of a sign-in to the callback: State
// protects against CSRF, Nonce against replayed ID tokens, and Verifier is
// the PKCE code verifier.
type OIDCFlow struct {
	State    string
	Nonce    string
	Verifier string
}

func NewOIDCFlow() (OIDCFlow, error) {
	state, err := randomValue()
	if err != nil {
		return OIDCFlow{}, err
	}
	nonce, err := randomValue()
	if err != nil {
		return OIDCFlow{}, err
	}
	return OIDCFlow{State: state, Nonce: nonce, Verifier: oauth2.GenerateVerifier()}, nil
}

// AuthCodeURL is where the browser is sent to sign in at the provider.
func (p *OIDCProvider) AuthCodeURL(flow OIDCFlow) string {
	return p.oauth2.AuthCodeURL(flow.State, oidc.Nonce(flow.Nonce), oauth2.S256ChallengeOption(flow.Verifier))
}

// OIDCIdentity is a verified identity from the provider's ID token.
type OIDCIdentity struct {
	Issuer            string
	Subject           string
	PreferredUsername string
	Email             string
}

// Exchange trades the callback's authorization code for an ID token and
// verifies it (signature, audience, expiry and nonce).
func (p *OIDCProvider) Exchange(ctx context.Context, flow OIDCFlow, state, code string) (OIDCIdentity, error) {
	if flow.State == "" || subtle.ConstantTimeCompare([]byte(state), []byte(flow.State)) != 1 {
		return OIDCIdentity{}, errors.New("state mismatch")
	}
	token, err := p.oauth2.Exchange(ctx, code, oauth2.VerifierOption(flow.Verifier))
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("exchange authorization code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return OIDCIdentity{}, errors.New("token response has no id_token")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("verify id token: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(flow.Nonce)) != 1 {
		return OIDCIdentity{}, errors.New("nonce mismatch")
	}
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return OIDCIdentity{}, fmt.Errorf("decode id token claims: %w", err)
	}
	return OIDCIdentity{
		Issuer:            idToken.Issuer,
		Subject:           idToken.Subject,
		PreferredUsername: claims.PreferredUsername,
		Email:             claims.Email,
	}, nil
}

// LoginOIDC signs in the account linked to id, creating it on first sign-in.
// New accounts are not administrators and have no password: least privilege
// for an identity nobody has vetted yet; an administrator can promote them.
func (s *Service) LoginOIDC(ctx context.Context, id OIDCIdentity) (string, *models.User, time.Time, error) {
	user, err := s.userForOIDCIdentity(ctx, id)
	if err != nil {
		return "", nil, time.Time{}, err
	}
	token, expires, err := s.createSession(ctx, user.ID)
	if err != nil {
		return "", nil, time.Time{}, err
	}
	return token, user, expires, nil
}

func (s *Service) userForOIDCIdentity(ctx context.Context, id OIDCIdentity) (*models.User, error) {
	find := func() (*models.User, error) {
		var user models.User
		err := s.db.WithContext(ctx).
			Where("oidc_issuer = ? AND oidc_subject = ?", id.Issuer, id.Subject).
			First(&user).Error
		if err != nil {
			return nil, err
		}
		return &user, nil
	}

	user, err := find()
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return user, err
	}

	username := provisionedUsername(id)
	for attempt := 0; ; attempt++ {
		user = &models.User{Username: username, OIDCIssuer: &id.Issuer, OIDCSubject: &id.Subject}
		err = s.db.WithContext(ctx).Create(user).Error
		if !errors.Is(err, gorm.ErrDuplicatedKey) || attempt == 2 {
			break
		}
		// Either a concurrent first sign-in of the same identity won the
		// race, or the username belongs to someone else.
		if existing, findErr := find(); findErr == nil {
			return existing, nil
		}
		suffix, sErr := randomSuffix()
		if sErr != nil {
			return nil, sErr
		}
		username = truncateRunes(provisionedUsername(id), MaxUsernameLength-len(suffix)-1) + "-" + suffix
	}
	if err != nil {
		return nil, fmt.Errorf("provision sso user: %w", err)
	}
	return user, nil
}

// provisionedUsername picks a readable username for a new SSO account.
func provisionedUsername(id OIDCIdentity) string {
	for _, v := range []string{id.PreferredUsername, id.Email, id.Subject} {
		if v, err := normalizeUsername(v); err == nil {
			return v
		}
	}
	return truncateRunes(id.Subject, MaxUsernameLength)
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func randomValue() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random value: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomSuffix() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate username suffix: %w", err)
	}
	return hex.EncodeToString(b), nil
}
