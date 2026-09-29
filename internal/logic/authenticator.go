package logic

import (
	"time"
)

// TokenClaims represents standard identity metadata encoded into authentication tokens.
// Application and logic layers use this typed struct without coupling to external JWT packages.
type TokenClaims struct {
	UserID    string
	Issuer    string
	Audience  string
	ExpiresAt time.Time
	IssuedAt  time.Time
	NotBefore time.Time
}

// Authenticator defines the application-facing contract for cryptographic token generation and validation.
type Authenticator interface {
	GenerateToken(claims TokenClaims) (string, error)
	ValidateToken(tokenString string) (*TokenClaims, error)
}
