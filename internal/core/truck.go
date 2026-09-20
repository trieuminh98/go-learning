package core

import "errors"

// Truck is the domain entity.
type Truck struct {
	ID          string `json:"id"`
	Capacity    int    `json:"capacity"`
	CurrentLoad int    `json:"current_load"`
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
