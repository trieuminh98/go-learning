package core

import "errors"

// Domain errors representing pure business invariants/conditions.
var (
	ErrTruckNotFound    = errors.New("truck not found")
	ErrInvalidTruckID   = errors.New("invalid truck ID")
	ErrDuplicateTruckID = errors.New("truck ID already exists")
	ErrInvalidCapacity  = errors.New("truck capacity must be greater than zero")
	ErrOverloaded       = errors.New("cargo exceeds truck capacity")

	ErrInvalidFirstName  = errors.New("first name must not be empty")
	ErrInvalidLastName   = errors.New("last name must not be empty")
	ErrInvalidEmail      = errors.New("invalid email address")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")
	ErrInvitationNotFound = errors.New("invitation not found or expired")

	ErrInvalidTitle        = errors.New("title must not be empty")
	ErrInvalidContent      = errors.New("content must not be empty")
	ErrPostNotFound        = errors.New("post not found")
	ErrConflict            = errors.New("conflict: resource was modified concurrently")
	ErrCannotFollowSelf    = errors.New("cannot follow self")
)
