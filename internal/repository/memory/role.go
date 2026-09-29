package memory

import (
	"context"
	"sync"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

var _ logic.RoleRepository = (*RoleRepository)(nil)

// RoleRepository is an in-memory implementation of logic.RoleRepository.
type RoleRepository struct {
	mu    sync.RWMutex
	roles map[string]core.Role
}

// NewRoleRepository initializes and returns a thread-safe in-memory role repository pre-populated with system roles.
func NewRoleRepository() *RoleRepository {
	repo := &RoleRepository{
		roles: make(map[string]core.Role),
	}

	repo.roles["user"] = core.Role{
		ID:          1,
		Name:        "user",
		Level:       1,
		Description: "Default user role with standard permissions",
	}
	repo.roles["moderator"] = core.Role{
		ID:          2,
		Name:        "moderator",
		Level:       2,
		Description: "Moderator role with content moderation permissions",
	}
	repo.roles["admin"] = core.Role{
		ID:          3,
		Name:        "admin",
		Level:       3,
		Description: "Administrator role with full system access",
	}

	return repo
}

// GetByName retrieves a role by its unique semantic name.
func (r *RoleRepository) GetByName(ctx context.Context, name string) (core.Role, error) {
	if err := ctx.Err(); err != nil {
		return core.Role{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	role, ok := r.roles[name]
	if !ok {
		return core.Role{}, core.ErrRoleNotFound
	}

	return role, nil
}
