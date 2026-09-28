package memory

import (
	"context"

	"go-learning/internal/logic"
)

var _ logic.TransactionManager = (*TransactionManager)(nil)

// TransactionManager simulates transaction coordination in memory.
type TransactionManager struct {
	users       *UserRepository
	invitations *InvitationRepository
}

// NewTransactionManager initializes an in-memory transaction coordinator.
func NewTransactionManager(users *UserRepository, invitations *InvitationRepository) *TransactionManager {
	return &TransactionManager{
		users:       users,
		invitations: invitations,
	}
}

// WithinTransaction executes the callback against the in-memory repositories.
func (tm *TransactionManager) WithinTransaction(ctx context.Context, fn func(tx logic.TxStorage) error) error {
	txStorage := logic.TxStorage{
		Users:       tm.users,
		Invitations: tm.invitations,
	}
	return fn(txStorage)
}
