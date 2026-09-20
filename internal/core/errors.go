package core

import "errors"

// Domain errors representing pure business invariants/conditions.
var (
	ErrTruckNotFound  = errors.New("truck not found")
	ErrInvalidTruckID = errors.New("invalid truck ID")
	ErrOverloaded     = errors.New("cargo exceeds truck capacity")
)
