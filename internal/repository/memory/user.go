package memory

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

var _ logic.UserRepository = (*UserRepository)(nil)

// UserRepository is an in-memory implementation of logic.UserRepository.
type UserRepository struct {
	mu      sync.RWMutex
	users   map[string]core.User
	userSeq int
}

// NewUserRepository initializes and returns a thread-safe in-memory user repository.
func NewUserRepository() *UserRepository {
	return &UserRepository{
		users: make(map[string]core.User),
	}
}

// List returns all users sorted by ID.
func (r *UserRepository) List(ctx context.Context) ([]core.User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]core.User, 0, len(r.users))
	for _, u := range r.users {
		result = append(result, u)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}

// Create inserts a user into the in-memory store after verifying uniqueness.
func (r *UserRepository) Create(ctx context.Context, u core.User) (core.User, error) {
	if err := ctx.Err(); err != nil {
		return core.User{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.users {
		if strings.EqualFold(existing.FirstName, u.FirstName) && strings.EqualFold(existing.LastName, u.LastName) {
			return core.User{}, core.ErrUserAlreadyExists
		}
	}

	r.userSeq++
	u.ID = fmt.Sprintf("%d", r.userSeq)
	r.users[u.ID] = u

	return u, nil
}

// CreateWithCredentials inserts a user with credentials after verifying email uniqueness.
func (r *UserRepository) CreateWithCredentials(ctx context.Context, u core.User) (core.User, error) {
	if err := ctx.Err(); err != nil {
		return core.User{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if u.Email != "" {
		for _, existing := range r.users {
			if strings.EqualFold(existing.Email, u.Email) {
				return core.User{}, core.ErrEmailAlreadyExists
			}

		}
	}

	r.userSeq++
	u.ID = fmt.Sprintf("%d", r.userSeq)
	r.users[u.ID] = u

	return u, nil
}

// GetByID fetches a user by their stringified ID from memory, returning core.ErrUserNotFound if missing.
func (r *UserRepository) GetByID(ctx context.Context, id int64) (core.User, error) {
	if err := ctx.Err(); err != nil {
		return core.User{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[strconv.FormatInt(id, 10)]
	if !ok {
		return core.User{}, core.ErrUserNotFound
	}

	return user, nil
}

// GetByEmail fetches a user by email from memory.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (core.User, error) {
	if err := ctx.Err(); err != nil {
		return core.User{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}

	return core.User{}, core.ErrUserNotFound
}

// Delete removes a user record by ID.
func (r *UserRepository) Delete(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.users, strconv.FormatInt(id, 10))
	return nil
}
