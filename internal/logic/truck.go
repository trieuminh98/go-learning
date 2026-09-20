package logic

import (
	"context"
	"strings"

	"go-learning/internal/core"
)

// TruckService orchestrates truck domain operations using a TruckRepository.
type TruckService struct {
	repo TruckRepository
}

// NewTruckService constructs a TruckService with its required repository dependency.
func NewTruckService(repo TruckRepository) *TruckService {
	return &TruckService{
		repo: repo,
	}
}

// ListTrucks returns all trucks from the repository.
func (s *TruckService) ListTrucks(ctx context.Context) ([]core.Truck, error) {
	return s.repo.List(ctx)
}

// GetTruck retrieves a specific truck by ID after checking input validity.
func (s *TruckService) GetTruck(ctx context.Context, id string) (core.Truck, error) {
	if strings.TrimSpace(id) == "" {
		return core.Truck{}, core.ErrInvalidTruckID
	}

	return s.repo.GetByID(ctx, id)
}

// CreateTruck validates domain invariants and forwards to the repository.
func (s *TruckService) CreateTruck(ctx context.Context, t core.Truck) error {
	// 1. Enforce domain invariants
	if err := t.Validate(); err != nil {
		return err
	}

	// 2. Delegate persistence to repository
	return s.repo.Create(ctx, t)
}
