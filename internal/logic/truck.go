package logic

import (
	"sort"
	"strings"
	"sync"

	"go-learning/internal/core"
)

var (
	mu         sync.RWMutex
	mockTrucks = map[string]core.Truck{
		"1": {ID: "1", Capacity: 5000, CurrentLoad: 1200},
		"2": {ID: "2", Capacity: 10000, CurrentLoad: 9500},
	}
)

// ListTrucks returns all trucks in storage.
func ListTrucks() []core.Truck {
	mu.RLock()
	defer mu.RUnlock()

	trucks := make([]core.Truck, 0, len(mockTrucks))
	for _, t := range mockTrucks {
		trucks = append(trucks, t)
	}

	// Sort by ID for deterministic output
	sort.Slice(trucks, func(i, j int) bool {
		return trucks[i].ID < trucks[j].ID
	})

	return trucks
}

// GetTruck executes the retrieve truck use-case.
// Logic coordinates Core operations and does not know HTTP concepts.
func GetTruck(id string) (core.Truck, error) {
	if strings.TrimSpace(id) == "" {
		return core.Truck{}, core.ErrInvalidTruckID
	}

	mu.RLock()
	defer mu.RUnlock()

	truck, exists := mockTrucks[id]
	if !exists {
		return core.Truck{}, core.ErrTruckNotFound
	}

	return truck, nil
}

// CreateTruck validates and stores a new truck.
func CreateTruck(t core.Truck) error {
	// 1. Enforce Core domain invariants
	if err := t.Validate(); err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	// 2. Check business uniqueness
	if _, exists := mockTrucks[t.ID]; exists {
		return core.ErrDuplicateTruckID
	}

	mockTrucks[t.ID] = t
	return nil
}
