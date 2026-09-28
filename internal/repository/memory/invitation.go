package memory

import (
	"bytes"
	"context"
	"sync"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

var _ logic.InvitationRepository = (*InvitationRepository)(nil)

// InvitationRepository is an in-memory implementation of logic.InvitationRepository.
type InvitationRepository struct {
	mu          sync.RWMutex
	invitations []core.Invitation
}

// NewInvitationRepository initializes an in-memory invitation repository.
func NewInvitationRepository() *InvitationRepository {
	return &InvitationRepository{
		invitations: make([]core.Invitation, 0),
	}
}

// Create inserts an invitation into the in-memory slice.
func (r *InvitationRepository) Create(ctx context.Context, inv core.Invitation) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.invitations = append(r.invitations, inv)
	return nil
}

// DeleteByUserID removes all invitations for the given user ID.
func (r *InvitationRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	filtered := make([]core.Invitation, 0, len(r.invitations))
	for _, inv := range r.invitations {
		if inv.UserID != userID {
			filtered = append(filtered, inv)
		}
	}
	r.invitations = filtered
	return nil
}

// GetByTokenHash looks up an invitation matching the provided token hash bytes.
func (r *InvitationRepository) GetByTokenHash(ctx context.Context, tokenHash []byte) (core.Invitation, error) {
	if err := ctx.Err(); err != nil {
		return core.Invitation{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, inv := range r.invitations {
		if bytes.Equal(inv.TokenHash, tokenHash) {
			return inv, nil
		}
	}

	return core.Invitation{}, core.ErrInvitationNotFound
}
