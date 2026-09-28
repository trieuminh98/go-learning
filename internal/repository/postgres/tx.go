package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	db "go-learning/internal/db/generated"
	"go-learning/internal/logic"
)

var _ logic.TransactionManager = (*TransactionManager)(nil)

// TransactionManager coordinates PostgreSQL database transactions using pgxpool.Pool.
type TransactionManager struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewTransactionManager constructs a TransactionManager.
func NewTransactionManager(pool *pgxpool.Pool, queries *db.Queries) *TransactionManager {
	return &TransactionManager{
		pool:    pool,
		queries: queries,
	}
}

// WithinTransaction executes a callback within an isolated PostgreSQL transaction.
// It rolls back automatically if the callback returns an error, and commits on success.
func (tm *TransactionManager) WithinTransaction(ctx context.Context, fn func(tx logic.TxStorage) error) error {
	tx, err := tm.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	txQueries := tm.queries.WithTx(tx)
	txStorage := logic.TxStorage{
		Users:       NewUserRepository(txQueries),
		Invitations: NewInvitationRepository(txQueries),
	}

	if err := fn(txStorage); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
