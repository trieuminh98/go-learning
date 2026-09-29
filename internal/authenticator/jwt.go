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

// ValidateToken parses and validates a signed JWT string against HMAC-SHA256,
// expiration time, configured issuer, and configured audience.
func (a *JWTAuthenticator) ValidateToken(tokenString string) (*logic.TokenClaims, error) {
	var claims jwt.RegisteredClaims

	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithExpirationRequired(),
	}
	if a.issuer != "" {
		opts = append(opts, jwt.WithIssuer(a.issuer))
	}
	if a.audience != "" {
		opts = append(opts, jwt.WithAudience(a.audience))
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return a.secret, nil
		},
		opts...,
	)
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	var exp time.Time
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	}

	var iat time.Time
	if claims.IssuedAt != nil {
		iat = claims.IssuedAt.Time
	}

	var nbf time.Time
	if claims.NotBefore != nil {
		nbf = claims.NotBefore.Time
	}

	var aud string
	if len(claims.Audience) > 0 {
		aud = claims.Audience[0]
	}

	return &logic.TokenClaims{
		UserID:    claims.Subject,
		Issuer:    claims.Issuer,
		Audience:  aud,
		ExpiresAt: exp,
		IssuedAt:  iat,
		NotBefore: nbf,
	}, nil
}
