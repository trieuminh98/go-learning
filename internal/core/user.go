package core

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Password encapsulates password hashing and validation logic.
// The plaintext value is held only in memory during validation/hashing
// and is never serialized, logged, or directly stored.
type Password struct {
	plaintext *string
	hash      []byte
}

// Set computes a secure bcrypt hash for the provided plaintext password.
func (p *Password) Set(plaintext string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), 12)
	if err != nil {
		return err
	}
	p.plaintext = &plaintext
	p.hash = hash
	return nil
}

// Matches verifies whether the provided plaintext password matches the stored bcrypt hash.
func (p *Password) Matches(plaintext string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintext))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

// Hash returns the raw bcrypt hash bytes.
func (p *Password) Hash() []byte {
	return p.hash
}

// NewPasswordFromHash wraps an existing password hash retrieved from storage.
func NewPasswordFromHash(hash []byte) Password {
	return Password{hash: hash}
}

// User represents the pure domain entity for a user.
// Invariants and domain logic live here.
// Notice: No JSON struct tags or HTTP concepts belong in the Core layer.
type User struct {
	ID        string
	FirstName string
	LastName  string
	Email     string
	Password  Password
	IsActive  bool
}

// Validate checks domain rules for User.
func (u User) Validate() error {
	if strings.TrimSpace(u.FirstName) == "" {
		return ErrInvalidFirstName
	}
	if strings.TrimSpace(u.LastName) == "" {
		return ErrInvalidLastName
	}
	return nil
}
