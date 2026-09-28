package logic

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go-learning/internal/core"
)

// RegisterUserInput holds domain inputs required to register a new user account.
type RegisterUserInput struct {
	FirstName string
	LastName  string
	Email     string
	Password  string
}

// UserService orchestrates user use-cases using UserRepository, FollowerRepository,
// InvitationRepository, TransactionManager, and Mailer.
type UserService struct {
	repo         UserRepository
	followerRepo FollowerRepository
	invRepo      InvitationRepository
	txManager    TransactionManager
	mailer       Mailer
	tokenExpiry  time.Duration
}

// NewUserService constructs a UserService with its required dependencies.
func NewUserService(
	repo UserRepository,
	followerRepo FollowerRepository,
	invRepo InvitationRepository,
	txManager TransactionManager,
	mailer Mailer,
	tokenExpiry time.Duration,
) *UserService {
	if tokenExpiry <= 0 {
		tokenExpiry = 72 * time.Hour
	}
	return &UserService{
		repo:         repo,
		followerRepo: followerRepo,
		invRepo:      invRepo,
		txManager:    txManager,
		mailer:       mailer,
		tokenExpiry:  tokenExpiry,
	}
}


// ListUsers returns all users currently stored via the repository.
func (s *UserService) ListUsers(ctx context.Context) ([]core.User, error) {
	return s.repo.List(ctx)
}

// GetUser retrieves a single user by ID via the repository.
func (s *UserService) GetUser(ctx context.Context, id int64) (core.User, error) {
	return s.repo.GetByID(ctx, id)
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

// RegisterUser executes the multi-step user registration workflow:
//  1. Validates domain constraints (names, email, minimum password length).
//  2. Encapsulates and hashes password using bcrypt.
//  3. Generates a secure random invitation token and its SHA-256 hash.
//  4. Persists the inactive user and invitation record atomically inside a DB transaction.
//  5. Dispatches an activation email outside the DB transaction.
//  6. If email dispatch fails, executes explicit compensation deleting the invitation and inactive user.
func (s *UserService) RegisterUser(ctx context.Context, input RegisterUserInput) (core.User, error) {
	// 1. Validate domain inputs
	email := strings.TrimSpace(strings.ToLower(input.Email))
	if email == "" {
		return core.User{}, core.ErrInvalidEmail
	}
	if len(input.Password) < 8 {
		return core.User{}, core.ErrInvalidPassword
	}

	userEntity := core.User{
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Email:     email,
		IsActive:  false,
	}
	if err := userEntity.Validate(); err != nil {
		return core.User{}, err
	}

	// 2. Hash password with bcrypt
	if err := userEntity.Password.Set(input.Password); err != nil {
		return core.User{}, err
	}

	// 3. Generate secure invitation token
	plainToken, tokenHash, err := core.GenerateInvitationToken()
	if err != nil {
		return core.User{}, fmt.Errorf("failed to generate invitation token: %w", err)
	}

	// 4. Atomic database transaction
	var createdUser core.User
	err = s.txManager.WithinTransaction(ctx, func(tx TxStorage) error {
		persistedUser, err := tx.Users.CreateWithCredentials(ctx, userEntity)
		if err != nil {
			return err
		}
		createdUser = persistedUser

		userID, err := strconv.ParseInt(persistedUser.ID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid user id format: %w", err)
		}

		invitation := core.Invitation{
			TokenHash: tokenHash,
			UserID:    userID,
			Expiry:    time.Now().Add(s.tokenExpiry),
		}


		return tx.Invitations.Create(ctx, invitation)
	})
	if err != nil {
		return core.User{}, err
	}

	// 5. Send activation email OUTSIDE the DB transaction
	if s.mailer != nil {
		if err := s.mailer.SendActivationEmail(ctx, createdUser.Email, plainToken); err != nil {
			// External dispatch failed post-commit: execute compensating actions in logic layer
			compCtx, compCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer compCancel()

			_ = s.compensateRegistration(compCtx, createdUser.ID)

			return core.User{}, fmt.Errorf("activation email delivery failed: %w", err)
		}
	}

	return createdUser, nil
}

// compensateRegistration reverts committed user and invitation records when post-commit external steps fail.
func (s *UserService) compensateRegistration(ctx context.Context, userIDStr string) error {
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return err
	}

	if s.invRepo != nil {
		_ = s.invRepo.DeleteByUserID(ctx, userID)
	}
	return s.repo.Delete(ctx, userID)
}

// FollowUser coordinates self-follow validation and delegates relation creation to FollowerRepository.
func (s *UserService) FollowUser(ctx context.Context, followerID, followedUserID int64) error {
	if followerID == followedUserID {
		return core.ErrCannotFollowSelf
	}
	return s.followerRepo.Follow(ctx, followerID, followedUserID)
}

// UnfollowUser coordinates self-unfollow validation and delegates relation removal to FollowerRepository.
func (s *UserService) UnfollowUser(ctx context.Context, followerID, followedUserID int64) error {
	if followerID == followedUserID {
		return core.ErrCannotFollowSelf
	}
	return s.followerRepo.Unfollow(ctx, followerID, followedUserID)
}
