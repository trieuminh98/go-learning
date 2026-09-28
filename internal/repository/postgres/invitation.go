package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-learning/internal/core"
	db "go-learning/internal/db/generated"
	"go-learning/internal/logic"
)

var _ logic.InvitationRepository = (*InvitationRepository)(nil)

// InvitationRepository implements logic.InvitationRepository using sqlc queries.
type InvitationRepository struct {
	queries *db.Queries
}

// NewInvitationRepository constructs an InvitationRepository.
func NewInvitationRepository(queries *db.Queries) *InvitationRepository {
	return &InvitationRepository{queries: queries}
}

// Create inserts an invitation record with token hash, user ID, and expiration timestamp.
func (r *InvitationRepository) Create(ctx context.Context, inv core.Invitation) error {
	return r.queries.CreateInvitation(ctx, db.CreateInvitationParams{
		TokenHash: inv.TokenHash,
		UserID:    inv.UserID,
		Expiry: pgtype.Timestamptz{
			Time:  inv.Expiry,
			Valid: true,
		},
	})
}

// DeleteByUserID removes any invitation token associated with the given user ID.
func (r *InvitationRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	return r.queries.DeleteInvitationByUserID(ctx, userID)
}

// GetByTokenHash retrieves an invitation record matching the SHA-256 token hash.
func (r *InvitationRepository) GetByTokenHash(ctx context.Context, tokenHash []byte) (core.Invitation, error) {
	row, err := r.queries.GetInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.Invitation{}, core.ErrInvitationNotFound
		}
		return core.Invitation{}, err
	}

	return core.Invitation{
		TokenHash: row.TokenHash,
		UserID:    row.UserID,
		Expiry:    row.Expiry.Time,
	}, nil
}
