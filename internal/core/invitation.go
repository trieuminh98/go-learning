package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Invitation represents an invitation / activation token domain entity.
type Invitation struct {
	TokenHash []byte
	UserID    int64
	Expiry    time.Time
}

// GenerateInvitationToken creates an unguessable 32-byte cryptographically secure random token.
// It returns the plaintext token string for email delivery and its SHA-256 hash for database storage.
func GenerateInvitationToken() (plainToken string, tokenHash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	plainToken = hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(plainToken))
	return plainToken, hash[:], nil
}

// HashToken computes the SHA-256 hash of a plaintext token for query verification.
func HashToken(plainToken string) []byte {
	hash := sha256.Sum256([]byte(plainToken))
	return hash[:]
}
