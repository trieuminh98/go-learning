package logic

import (
	"context"

	"go-learning/internal/core"
)

// UserService orchestrates user use-cases using a UserRepository.
type UserService struct {
	repo UserRepository
}

// NewUserService constructs a UserService with its required repository dependency.
func NewUserService(repo UserRepository) *UserService {
	return &UserService{
		repo: repo,
	}
}

// ListUsers returns all users currently stored via the repository.
func (s *UserService) ListUsers(ctx context.Context) ([]core.User, error) {
	return s.repo.List(ctx)
}

// CreateUser coordinates domain validation and delegates storage to the repository.
func (s *UserService) CreateUser(ctx context.Context, u core.User) (core.User, error) {
	// 1. Domain invariant validation
	if err := u.Validate(); err != nil {
		return core.User{}, err
	}

	// 2. Delegate persistence to repository
	return s.repo.Create(ctx, u)
}
