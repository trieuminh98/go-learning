package logic

import (
	"strings"

	"go-learning/internal/core"
)

// Simulated in-memory domain storage.
var mockTrucks = map[string]core.Truck{
	"1": {ID: "1", Capacity: 5000, CurrentLoad: 1200},
	"2": {ID: "2", Capacity: 10000, CurrentLoad: 9500},
}

// GetTruck executes the retrieve truck use-case.
// Logic coordinates Core operations and does not know HTTP concepts.
func GetTruck(id string) (core.Truck, error) {
	if strings.TrimSpace(id) == "" {
		return core.Truck{}, core.ErrInvalidTruckID
	}

	truck, exists := mockTrucks[id]
	if !exists {
		return core.Truck{}, core.ErrTruckNotFound
	}

	return truck, nil
}
