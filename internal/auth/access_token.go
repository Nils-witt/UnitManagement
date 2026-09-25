package auth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"gorm.io/gorm"

	"go-unit-mangement/internal/models"
)

// accessTokenCacheTTL bounds how long a verified access token's resolved
// account is reused without verifying the token again and re-syncing the
// account's groups. A token's own expiry caps it further.
const accessTokenCacheTTL = time.Minute

// accessTokenCachePruneSize is the cache size above which inserting a new
// entry first sweeps out expired ones, keeping the cache from growing
// without bound under a steady stream of distinct tokens.
const accessTokenCachePruneSize = 1024

// accessTokenAlgorithms are the signature algorithms an OIDC provider may
// sign access tokens with. HS256, which this app's own tokens use, is not
// among them, so those never reach an OIDC verifier.
var accessTokenAlgorithms = []jose.SignatureAlgorithm{
	jose.RS256, jose.RS384, jose.RS512,
	jose.PS256, jose.PS384, jose.PS512,
	jose.ES256, jose.ES384, jose.ES512,
	jose.EdDSA,
}

// OIDCAccessTokens authenticates API requests that present a JWT access
// token issued by a trusted OIDC provider directly as their bearer token,
// instead of first exchanging it for a session via the browser sign-in. The
// token must be issued by one of the trusted issuers, signed with one of
// that issuer's published keys (its JWKS), unexpired, and carry at least one
// of the accepted audiences. A nil *OIDCAccessTokens is valid and never
// handles any token, meaning direct access-token authentication is disabled.
type OIDCAccessTokens struct {
	// verifiers maps each trusted issuer to its token verifier. The
	// verifiers skip go-oidc's single-audience check; audiences is checked
	// by hand instead (see verify), since any one of them is acceptable.
	verifiers   map[string]*oidc.IDTokenVerifier
	audiences   []string
	groupsClaim string
	adminGroup  string

	mu    sync.Mutex
	cache map[[sha256.Size]byte]cachedAccessToken
}

type cachedAccessToken struct {
	userID  uint
	expires time.Time
}

// NewOIDCAccessTokens returns an authenticator for access tokens issued by
// login's provider (if login is non-nil) or any of cfg.AccessTokenIssuers,
// and carrying any of cfg.AccessTokenAudiences. Each additional issuer is
// discovered via its /.well-known/openid-configuration document. It returns
// nil (and nil error) if no audience is set, meaning direct access-token
// authentication isn't enabled; audiences without any issuer to trust, or
// issuers without any audience, is a configuration error.
func NewOIDCAccessTokens(ctx context.Context, login *OIDCProvider, cfg OIDCConfig) (*OIDCAccessTokens, error) {
	if len(cfg.AccessTokenAudiences) == 0 {
		if len(cfg.AccessTokenIssuers) > 0 {
			return nil, errors.New("OIDC_ACCESS_TOKEN_ISSUERS requires OIDC_ACCESS_TOKEN_AUDIENCE to be set")
		}
		return nil, nil
	}

	verifierConfig := &oidc.Config{SkipClientIDCheck: true}
	verifiers := make(map[string]*oidc.IDTokenVerifier, len(cfg.AccessTokenIssuers)+1)
	if login != nil {
		verifiers[login.issuer] = login.provider.Verifier(verifierConfig)
	}
	for _, issuer := range cfg.AccessTokenIssuers {
		if _, ok := verifiers[issuer]; ok {
			continue
		}
		provider, err := oidc.NewProvider(ctx, issuer)
		if err != nil {
			return nil, fmt.Errorf("discover oidc access token issuer %q: %w", issuer, err)
		}
		verifiers[issuer] = provider.Verifier(verifierConfig)
	}
	if len(verifiers) == 0 {
		return nil, errors.New("OIDC_ACCESS_TOKEN_AUDIENCE requires an issuer to trust: configure SSO (OIDC_ISSUER_URL, ...) or set OIDC_ACCESS_TOKEN_ISSUERS")
	}

	return &OIDCAccessTokens{
		verifiers:   verifiers,
		audiences:   cfg.AccessTokenAudiences,
		groupsClaim: cfg.GroupsClaim,
		adminGroup:  cfg.AdminGroup,
		cache:       make(map[[sha256.Size]byte]cachedAccessToken),
	}, nil
}

// SetOIDCAccessTokens makes UserForToken also accept access tokens verified
// by a, which may be nil to accept only this app's own tokens.
func (s *Service) SetOIDCAccessTokens(a *OIDCAccessTokens) { s.accessTokens = a }

// userForAccessToken reports whether raw is a token from one of the trusted
// OIDC issuers (handled) and, if so, the account it authenticates as,
// provisioning one on first use. A token from any other issuer — this app's
// own tokens, or anything that isn't a JWT at all — is left unhandled for
// the session lookup. err is non-nil only for a handled token that failed
// verification or account resolution.
func (s *Service) userForAccessToken(ctx context.Context, raw string) (user *models.User, handled bool, err error) {
	a := s.accessTokens
	if a == nil {
		return nil, false, nil
	}
	verifier := a.verifierFor(raw)
	if verifier == nil {
		return nil, false, nil
	}

	key := sha256.Sum256([]byte(raw))
	if userID, ok := a.cached(key); ok {
		// Loaded every time, so a deleted account or changed role applies
		// right away rather than when the cache entry expires.
		var cached models.User
		err := s.db.WithContext(ctx).First(&cached, userID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, true, ErrInvalidSession
		}
		if err != nil {
			return nil, true, err
		}
		return &cached, true, nil
	}

	id, expiry, hasGroups, err := a.verify(ctx, verifier, raw)
	if err != nil {
		slog.Debug("oidc access token rejected", "err", err)
		return nil, true, ErrInvalidSession
	}
	user, err = s.userForOIDCIdentity(ctx, id)
	if err != nil {
		return nil, true, err
	}
	// Many providers only put groups into the ID token; an access token
	// without the claim leaves the account's groups alone instead of
	// wiping them.
	if hasGroups {
		if err := s.syncOIDCGroups(ctx, user, id); err != nil {
			return nil, true, err
		}
	}
	a.store(key, user.ID, expiry)
	return user, true, nil
}

// verify checks raw's signature, issuer and expiry via verifier, that it
// carries one of the accepted audiences, and that it isn't an ID token. It
// returns the identity the token asserts, its expiry, and whether it carries
// the groups claim.
func (a *OIDCAccessTokens) verify(ctx context.Context, verifier *oidc.IDTokenVerifier, raw string) (OIDCIdentity, time.Time, bool, error) {
	token, err := verifier.Verify(ctx, raw)
	if err != nil {
		return OIDCIdentity{}, time.Time{}, false, fmt.Errorf("verify oidc access token: %w", err)
	}
	if !slices.ContainsFunc(token.Audience, func(aud string) bool { return slices.Contains(a.audiences, aud) }) {
		return OIDCIdentity{}, time.Time{}, false, errors.New("oidc access token audience not accepted")
	}

	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		// Typ is Keycloak's token type claim ("Bearer" for access tokens,
		// "ID" for ID tokens).
		Typ string `json:"typ"`
	}
	var all map[string]json.RawMessage
	if err := errors.Join(token.Claims(&claims), token.Claims(&all)); err != nil {
		return OIDCIdentity{}, time.Time{}, false, fmt.Errorf("decode oidc access token claims: %w", err)
	}
	// An ID token isn't a bearer credential for an API, even if its
	// audience happens to match (e.g. when an accepted audience is the
	// client id); rule out the ones that say what they are.
	if claims.Typ == "ID" || token.Nonce != "" {
		return OIDCIdentity{}, time.Time{}, false, errors.New("id tokens are not accepted as access tokens")
	}

	id := OIDCIdentity{
		Issuer:            token.Issuer,
		Subject:           token.Subject,
		PreferredUsername: claims.PreferredUsername,
		Email:             claims.Email,
		Groups:            []string{},
	}
	rawGroups, hasGroups := all[a.groupsClaim]
	if hasGroups {
		if id.Groups, err = parseGroups(rawGroups); err != nil {
			return OIDCIdentity{}, time.Time{}, false, err
		}
		if a.adminGroup != "" {
			isAdmin := slices.Contains(id.Groups, a.adminGroup)
			id.IsAdmin = &isAdmin
		}
	}
	return id, token.Expiry, hasGroups, nil
}

// verifierFor returns the verifier for raw's (not yet verified) iss claim,
// or nil if raw isn't a JWT signed with a provider's algorithm or names an
// issuer that isn't trusted. It only decides which verification path a
// token takes; Verify checks the issuer again, against the signature.
func (a *OIDCAccessTokens) verifierFor(raw string) *oidc.IDTokenVerifier {
	parsed, err := jwt.ParseSigned(raw, accessTokenAlgorithms)
	if err != nil {
		return nil
	}
	var claims jwt.Claims
	if err := parsed.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return nil
	}
	return a.verifiers[claims.Issuer]
}

func (a *OIDCAccessTokens) cached(key [sha256.Size]byte) (uint, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	e, ok := a.cache[key]
	if !ok || time.Now().After(e.expires) {
		return 0, false
	}
	return e.userID, true
}

func (a *OIDCAccessTokens) store(key [sha256.Size]byte, userID uint, tokenExpiry time.Time) {
	expires := time.Now().Add(accessTokenCacheTTL)
	if tokenExpiry.Before(expires) {
		expires = tokenExpiry
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.cache) >= accessTokenCachePruneSize {
		now := time.Now()
		for k, e := range a.cache {
			if now.After(e.expires) {
				delete(a.cache, k)
			}
		}
	}
	a.cache[key] = cachedAccessToken{userID: userID, expires: expires}
}
