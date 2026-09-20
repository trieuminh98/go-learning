package core

import "strings"

// User represents the pure domain entity for a user.
// Invariants and domain logic live here.
// Notice: No JSON struct tags or HTTP concepts belong in the Core layer.
type User struct {
	ID        string
	FirstName string
	LastName  string
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
