package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"go-learning/internal/core"
	db "go-learning/internal/db/generated"
	"go-learning/internal/logic"
)

// Ensure TruckRepository satisfies logic.TruckRepository interface at compile time.
var _ logic.TruckRepository = (*TruckRepository)(nil)

// TruckRepository implements logic.TruckRepository using sqlc-generated queries over pgx/v5.
type TruckRepository struct {
	queries *db.Queries
}

// NewTruckRepository constructs a PostgreSQL-backed TruckRepository.
func NewTruckRepository(queries *db.Queries) *TruckRepository {
	return &TruckRepository{
		queries: queries,
	}
}

func toCoreTruck(t db.Truck) core.Truck {
	return core.Truck{
		ID:          t.ID,
		Capacity:    int(t.Capacity),
		CurrentLoad: int(t.CurrentLoad),
	}
}

// List retrieves all trucks ordered by ID.
func (r *TruckRepository) List(ctx context.Context) ([]core.Truck, error) {
	rows, err := r.queries.ListTrucks(ctx)
	if err != nil {
		return nil, err
	}

	trucks := make([]core.Truck, len(rows))
	for i, row := range rows {
		trucks[i] = toCoreTruck(row)
	}
	return trucks, nil
}

// GetByID looks up a specific truck by primary key ID.
func (r *TruckRepository) GetByID(ctx context.Context, id string) (core.Truck, error) {
	row, err := r.queries.GetTruckByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.Truck{}, core.ErrTruckNotFound
		}
		return core.Truck{}, err
	}

	return toCoreTruck(row), nil
}

// Create inserts a new truck and maps primary key collision to ErrDuplicateTruckID.
func (r *TruckRepository) Create(ctx context.Context, t core.Truck) error {
	err := r.queries.CreateTruck(ctx, db.CreateTruckParams{
		ID:          t.ID,
		Capacity:    int32(t.Capacity),
		CurrentLoad: int32(t.CurrentLoad),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return core.ErrDuplicateTruckID
		}
		return err
	}

	return nil
}
