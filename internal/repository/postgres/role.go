package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"go-learning/internal/core"
	db "go-learning/internal/db/generated"
	"go-learning/internal/logic"
)

var _ logic.RoleRepository = (*RoleRepository)(nil)

// RoleRepository implements logic.RoleRepository using sqlc-generated queries over pgx/v5.
type RoleRepository struct {
	queries *db.Queries
}

// NewRoleRepository constructs a PostgreSQL-backed RoleRepository.
func NewRoleRepository(queries *db.Queries) *RoleRepository {
	return &RoleRepository{
		queries: queries,
	}
}

// GetByName retrieves a role by its unique semantic name.
func (r *RoleRepository) GetByName(ctx context.Context, name string) (core.Role, error) {
	row, err := r.queries.GetRoleByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.Role{}, core.ErrRoleNotFound
		}
		return core.Role{}, err
	}

	return toCoreRole(row.ID, row.Name, row.Level, row.Description), nil
}
