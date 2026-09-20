package core

import (
	"errors"
	"strings"
)

// Truck is the domain entity.
type Truck struct {
	ID          string `json:"id"`
	Capacity    int    `json:"capacity"`
	CurrentLoad int    `json:"current_load"`
}

// Validate checks basic entity invariants for a truck.
func (t Truck) Validate() error {
	if strings.TrimSpace(t.ID) == "" {
		return ErrInvalidTruckID
	}
	if t.Capacity <= 0 {
		return ErrInvalidCapacity
	}
	if t.CurrentLoad < 0 || t.CurrentLoad > t.Capacity {
		return ErrOverloaded
	}
	return nil
}

// CanLoadCargo enforces domain invariants.
func (t Truck) CanLoadCargo(weight int) error {
	if weight <= 0 {
		return errors.New("cargo weight must be positive")
	}
	if t.CurrentLoad+weight > t.Capacity {
		return ErrOverloaded
	}
	return nil
}
