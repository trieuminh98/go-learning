package core

import "errors"

// Domain errors representing pure business invariants/conditions.
var (
	ErrTruckNotFound    = errors.New("truck not found")
	ErrInvalidTruckID   = errors.New("invalid truck ID")
	ErrDuplicateTruckID = errors.New("truck ID already exists")
	ErrInvalidCapacity  = errors.New("truck capacity must be greater than zero")
	ErrOverloaded       = errors.New("cargo exceeds truck capacity")
)
