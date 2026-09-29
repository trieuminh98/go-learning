package authenticator

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"go-learning/internal/logic"
)

// Ensure JWTAuthenticator satisfies logic.Authenticator at compile time.
var _ logic.Authenticator = (*JWTAuthenticator)(nil)

// JWTAuthenticator implements logic.Authenticator using symmetric HMAC-SHA256 signing.
type JWTAuthenticator struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
}

// NewJWTAuthenticator constructs a new JWTAuthenticator.
// It enforces a minimum secret length of 32 bytes (256-bit) to guard against weak symmetric keys.
func NewJWTAuthenticator(secret, issuer, audience string, ttl time.Duration) (*JWTAuthenticator, error) {
	if len(secret) < 32 {
		return nil, errors.New("jwt secret must be at least 32 bytes long")
	}
	if ttl <= 0 {
		ttl = 60 * time.Minute
	}
	return &JWTAuthenticator{
		secret:   []byte(secret),
		issuer:   issuer,
		audience: audience,
		ttl:      ttl,
	}, nil
}

// GenerateToken builds standard typed claims and signs a JWT using HMAC-SHA256.
func (a *JWTAuthenticator) GenerateToken(claims logic.TokenClaims) (string, error) {
	issuedAt := claims.IssuedAt
	if issuedAt.IsZero() {
		issuedAt = time.Now()
	}

	notBefore := claims.NotBefore
	if notBefore.IsZero() {
		notBefore = issuedAt
	}

	expiresAt := claims.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = issuedAt.Add(a.ttl)
	}

	issuer := claims.Issuer
	if issuer == "" {
		issuer = a.issuer
	}

	audience := claims.Audience
	if audience == "" {
		audience = a.audience
	}

	registeredClaims := jwt.RegisteredClaims{
		Subject:   claims.UserID,
		Issuer:    issuer,
		Audience:  jwt.ClaimStrings{audience},
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		NotBefore: jwt.NewNumericDate(notBefore),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, registeredClaims)
	return token.SignedString(a.secret)
}
