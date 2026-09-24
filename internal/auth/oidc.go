package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	// ExtraScopes are requested in addition to openid, profile and email,
	// e.g. "groups" for providers that only send groups when asked.
	ExtraScopes []string
	// AdminGroup, when set, makes the provider the source of truth for the
	// administrator role of SSO accounts: on every sign-in, the account is
	// an administrator exactly if GroupsClaim lists this group.
	AdminGroup string
	// GroupsClaim names the claim listing the user's groups, which are
	// copied to the account on every sign-in.
	GroupsClaim string
}

// OIDCProvider runs the authorization code flow (with PKCE) against one
// provider. A nil *OIDCProvider means SSO is not configured.
type OIDCProvider struct {
	provider    *oidc.Provider
	verifier    *oidc.IDTokenVerifier
	oauth2      oauth2.Config
	adminGroup  string
	groupsClaim string
}

// NewOIDCProvider discovers the provider via its
// /.well-known/openid-configuration document.
func NewOIDCProvider(ctx context.Context, cfg OIDCConfig) (*OIDCProvider, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider %q: %w", cfg.IssuerURL, err)
	}
	return &OIDCProvider{
		provider:    provider,
		adminGroup:  cfg.AdminGroup,
		groupsClaim: cfg.GroupsClaim,
		verifier:    provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth2: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       append([]string{oidc.ScopeOpenID, "profile", "email"}, cfg.ExtraScopes...),
		},
	}, nil
}

// ManagesAdminRole reports whether group sync decides the administrator role
// of SSO accounts, so it must not be changed by hand.
func (p *OIDCProvider) ManagesAdminRole() bool { return p != nil && p.adminGroup != "" }

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
	// Groups are the user's groups at the provider, sorted and without
	// duplicates; empty when the provider sends none.
	Groups []string
	// IsAdmin is the administrator role granted by the provider's groups,
	// or nil when group sync is off and the role is managed in the app.
	IsAdmin *bool
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
	identity := OIDCIdentity{
		Issuer:            idToken.Issuer,
		Subject:           idToken.Subject,
		PreferredUsername: claims.PreferredUsername,
		Email:             claims.Email,
	}
	groups, err := p.groups(ctx, idToken, token)
	if err != nil {
		return OIDCIdentity{}, err
	}
	identity.Groups = groups
	if p.ManagesAdminRole() {
		isAdmin := slices.Contains(groups, p.adminGroup)
		identity.IsAdmin = &isAdmin
	}
	return identity, nil
}

// groups reads the groups claim from the ID token or, since many providers
// leave it out of the ID token by default, from the userinfo endpoint. A
// user without the claim, or at a provider without userinfo, is in no groups.
func (p *OIDCProvider) groups(ctx context.Context, idToken *oidc.IDToken, token *oauth2.Token) ([]string, error) {
	var claims map[string]json.RawMessage
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("decode id token claims: %w", err)
	}
	raw, ok := claims[p.groupsClaim]
	if !ok {
		if p.provider.UserInfoEndpoint() == "" {
			return []string{}, nil
		}
		info, err := p.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
		if err != nil {
			return nil, fmt.Errorf("fetch userinfo: %w", err)
		}
		claims = nil
		if err := info.Claims(&claims); err != nil {
			return nil, fmt.Errorf("decode userinfo claims: %w", err)
		}
		if raw, ok = claims[p.groupsClaim]; !ok {
			// Only a problem when roles depend on groups; otherwise the
			// provider may simply not be set up to send them.
			if p.ManagesAdminRole() {
				slog.Warn("oidc groups claim missing from id token and userinfo", "claim", p.groupsClaim, "subject", idToken.Subject)
			}
			return []string{}, nil
		}
	}
	return parseGroups(raw)
}

// parseGroups accepts a list of group names or, as some providers send for
// a single group, one string.
func parseGroups(raw json.RawMessage) ([]string, error) {
	var groups []string
	if err := json.Unmarshal(raw, &groups); err != nil {
		var group string
		if err := json.Unmarshal(raw, &group); err != nil {
			return nil, fmt.Errorf("groups claim is neither a string nor a list of strings: %s", raw)
		}
		groups = []string{group}
	}
	// A JSON null decodes to a nil slice.
	groups = slices.DeleteFunc(groups, func(g string) bool { return g == "" })
	slices.Sort(groups)
	return append([]string{}, slices.Compact(groups)...), nil
}

// LoginOIDC signs in the account linked to id, creating it on first sign-in,
// and replaces the account's groups with id.Groups. New accounts have no
// password. Without group sync (id.IsAdmin nil) they are
// not administrators, least privilege for an identity nobody has vetted yet,
// and an administrator can promote them; with it, every sign-in applies the
// role the provider's groups grant.
func (s *Service) LoginOIDC(ctx context.Context, id OIDCIdentity) (string, *models.User, time.Time, error) {
	user, err := s.userForOIDCIdentity(ctx, id)
	if err != nil {
		return "", nil, time.Time{}, err
	}
	if err := s.syncOIDCGroups(ctx, user, id); err != nil {
		return "", nil, time.Time{}, err
	}
	session, err := s.createSession(ctx, models.Session{UserID: user.ID}, s.sessionTTL)
	if err != nil {
		return "", nil, time.Time{}, err
	}
	return session.token, user, session.ExpiresAt, nil
}

// syncOIDCGroups makes the user a member of exactly the provider's groups,
// creating groups seen for the first time, and with group sync of the
// administrator role applies the role they grant.
func (s *Service) syncOIDCGroups(ctx context.Context, user *models.User, id OIDCIdentity) error {
	adminChanged := id.IsAdmin != nil && user.IsAdmin != *id.IsAdmin
	var groups []models.Group
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		if groups, err = ensureGroups(tx, id.Groups); err != nil {
			return err
		}
		ids := make([]uint, len(groups))
		memberships := make([]models.UserGroup, len(groups))
		for i, g := range groups {
			ids[i] = g.ID
			memberships[i] = models.UserGroup{UserID: user.ID, GroupID: g.ID}
		}
		leave := tx.Where("user_id = ?", user.ID)
		if len(ids) > 0 {
			leave = leave.Where("group_id NOT IN ?", ids)
		}
		if err := leave.Delete(&models.UserGroup{}).Error; err != nil {
			return err
		}
		if len(memberships) > 0 {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&memberships).Error; err != nil {
				return err
			}
		}
		if adminChanged {
			return tx.Model(user).Update("is_admin", *id.IsAdmin).Error
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("sync oidc groups of %q: %w", user.Username, err)
	}
	user.Groups = groups
	if adminChanged {
		user.IsAdmin = *id.IsAdmin
		slog.Info("administrator role synced from oidc groups", "user", user.Username, "isAdmin", user.IsAdmin)
	}
	return nil
}

// ensureGroups returns the groups with the given names, sorted by name,
// creating those that don't exist yet.
func ensureGroups(tx *gorm.DB, names []string) ([]models.Group, error) {
	if len(names) == 0 {
		return []models.Group{}, nil
	}
	var groups []models.Group
	if err := tx.Where("name IN ?", names).Order("name").Find(&groups).Error; err != nil {
		return nil, err
	}
	if len(groups) == len(names) {
		return groups, nil
	}
	var missing []models.Group
	for _, name := range names {
		if !slices.ContainsFunc(groups, func(g models.Group) bool { return g.Name == name }) {
			missing = append(missing, models.Group{Name: name})
		}
	}
	// A concurrent sign-in may create the same group first.
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).Create(&missing).Error; err != nil {
		return nil, err
	}
	groups = nil
	err := tx.Where("name IN ?", names).Order("name").Find(&groups).Error
	return groups, err
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
