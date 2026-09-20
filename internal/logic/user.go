package logic

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"go-learning/internal/core"
)

var (
	userMu    sync.RWMutex
	mockUsers = map[string]core.User{}
	userSeq   = 0
)

// ListUsers returns all users currently stored.
// Notice: Logic does not know http.Request, http.ResponseWriter, or JSON.
func ListUsers() []core.User {
	userMu.RLock()
	defer userMu.RUnlock()

	users := make([]core.User, 0, len(mockUsers))
	for _, u := range mockUsers {
		users = append(users, u)
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].ID < users[j].ID
	})

	return users
}

// CreateUser coordinates domain validation, duplicate verification, and persistence.
func CreateUser(u core.User) (core.User, error) {
	// 1. Domain invariant validation
	if err := u.Validate(); err != nil {
		return core.User{}, err
	}

	userMu.Lock()
	defer userMu.Unlock()

	// 2. Business uniqueness check
	for _, existing := range mockUsers {
		if strings.EqualFold(existing.FirstName, u.FirstName) && strings.EqualFold(existing.LastName, u.LastName) {
			return core.User{}, core.ErrUserAlreadyExists
		}
	}

	// 3. Generate ID and save
	userSeq++
	u.ID = fmt.Sprintf("%d", userSeq)
	mockUsers[u.ID] = u

	return u, nil
}
