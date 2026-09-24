package auth

import (
	"fmt"
	"strconv"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// MinJWTSecretLength is the shortest accepted signing key: HS256 needs at
// least as many bytes as its hash.
const MinJWTSecretLength = 32

// tokenIssuer is the "iss" claim of every token, so tokens signed with the
// same secret for another purpose are not accepted here.
const tokenIssuer = "go-unit-mangement"

// tokenSigner signs and verifies access tokens: HS256 JWTs whose "jti" is the
// session's random ID. The signature makes a token tamper-proof, and the
// session row behind the ID keeps it revocable.
type tokenSigner struct {
	key    []byte
	signer jose.Signer
}

func newTokenSigner(secret []byte) (*tokenSigner, error) {
	if len(secret) < MinJWTSecretLength {
		return nil, fmt.Errorf("JWT secret must be at least %d bytes", MinJWTSecretLength)
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.HS256, Key: secret},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		return nil, fmt.Errorf("create JWT signer: %w", err)
	}
	return &tokenSigner{key: secret, signer: signer}, nil
}

func (t *tokenSigner) sign(sessionID string, userID uint, issued, expires time.Time) (string, error) {
	claims := jwt.Claims{
		Issuer:   tokenIssuer,
		Subject:  strconv.FormatUint(uint64(userID), 10),
		ID:       sessionID,
		IssuedAt: jwt.NewNumericDate(issued),
		Expiry:   jwt.NewNumericDate(expires),
	}
	token, err := jwt.Signed(t.signer).Claims(claims).Serialize()
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return token, nil
}

// verify checks the token's signature, issuer and expiry and returns its
// session ID and user ID.
func (t *tokenSigner) verify(token string) (string, uint, error) {
	parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{jose.HS256})
	if err != nil {
		return "", 0, ErrInvalidSession
	}
	var claims jwt.Claims
	if err := parsed.Claims(t.key, &claims); err != nil {
		return "", 0, ErrInvalidSession
	}
	if claims.Expiry == nil {
		return "", 0, ErrInvalidSession
	}
	if err := claims.ValidateWithLeeway(jwt.Expected{Issuer: tokenIssuer, Time: time.Now()}, 0); err != nil {
		return "", 0, ErrInvalidSession
	}
	userID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || claims.ID == "" {
		return "", 0, ErrInvalidSession
	}
	return claims.ID, uint(userID), nil
}
