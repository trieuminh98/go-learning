package memory

import (
	"context"
	"sort"
	"strings"
	"sync"

	"go-learning/internal/core"
)

// TruckRepository is an in-memory implementation of logic.TruckRepository.
type TruckRepository struct {
	mu     sync.RWMutex
	trucks map[string]core.Truck
}

// NewTruckRepository initializes and returns a thread-safe in-memory truck repository seeded with default data.
func NewTruckRepository() *TruckRepository {
	return &TruckRepository{
		trucks: map[string]core.Truck{
			"1": {ID: "1", Capacity: 5000, CurrentLoad: 1200},
			"2": {ID: "2", Capacity: 10000, CurrentLoad: 9500},
		},
	}
}

// List returns all trucks sorted by ID.
func (r *TruckRepository) List(ctx context.Context) ([]core.Truck, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]core.Truck, 0, len(r.trucks))
	for _, t := range r.trucks {
		result = append(result, t)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}

// GetByID looks up a truck by ID.
func (r *TruckRepository) GetByID(ctx context.Context, id string) (core.Truck, error) {
	if err := ctx.Err(); err != nil {
		return core.Truck{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	truck, exists := r.trucks[id]
	if !exists {
		return core.Truck{}, core.ErrTruckNotFound
	}

	return truck, nil
}

// Create validates uniqueness and persists a truck into memory.
func (r *TruckRepository) Create(ctx context.Context, t core.Truck) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.trucks[strings.TrimSpace(t.ID)]; exists {
		return core.ErrDuplicateTruckID
	}

	r.trucks[t.ID] = t
	return nil
}
