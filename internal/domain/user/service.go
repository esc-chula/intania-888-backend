package user

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

// Service implements account use cases independently of HTTP and database details.
type Service struct {
	repo Repository
	log  *zap.Logger
}

// NewService constructs account use cases using the supplied repository.
func NewService(repo Repository, log *zap.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// CreateUser creates an account with the default 888 coin balance.
func (s *Service) CreateUser(ctx context.Context, input CreateInput) (*identity.Profile, error) {
	user := &identity.User{
		ID:            input.ID,
		Email:         input.Email,
		Name:          input.Name,
		NickName:      input.NickName,
		RoleID:        input.RoleID,
		GroupID:       input.GroupID,
		RemainingCoin: 888_00,
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	s.log.Info("User created successfully", zap.String("user_id", user.ID))
	result := profileFromUser(user)
	// Preserve the existing create response, which reflects the supplied timestamp.
	result.CreatedAt = input.CreatedAt
	return result, nil
}

// GetUser retrieves a profile while preserving the existing single-user response timestamp.
func (s *Service) GetUser(ctx context.Context, id string) (*identity.Profile, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	result := profileFromUser(user)
	result.CreatedAt = zeroTime
	return result, nil
}

// GetAllUsers retrieves account profiles including their creation timestamps.
func (s *Service) GetAllUsers(ctx context.Context) ([]*identity.Profile, error) {
	users, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	profiles := make([]*identity.Profile, len(users))
	for i, user := range users {
		profiles[i] = profileFromUser(user)
	}
	return profiles, nil
}

// UpdateOwnProfile applies supplied profile fields to the authenticated account.
// actorID must come from trusted authentication. Identity, email, role, and balance
// cannot be changed through this use case. The result is read after the update.
func (s *Service) UpdateOwnProfile(ctx context.Context, actorID string, input ProfilePatch) (*identity.Profile, error) {
	if strings.TrimSpace(actorID) == "" || (input.Name == nil && !input.NickNameSet && !input.GroupIDSet) {
		return nil, ErrInvalidProfileUpdate
	}
	if input.Name != nil && strings.TrimSpace(*input.Name) == "" {
		return nil, ErrInvalidProfileUpdate
	}
	if (input.NickName != nil && !input.NickNameSet) || (input.GroupID != nil && !input.GroupIDSet) {
		return nil, ErrInvalidProfileUpdate
	}
	if input.GroupID != nil && strings.TrimSpace(*input.GroupID) == "" {
		return nil, ErrInvalidProfileUpdate
	}

	if err := s.repo.PatchProfile(ctx, actorID, input); err != nil {
		return nil, err
	}

	s.log.Info("User profile updated successfully", zap.String("user_id", actorID))
	return s.GetUser(ctx, actorID)
}

// AdminUpdateUser updates profile fields and balance while preserving role ownership.
func (s *Service) AdminUpdateUser(ctx context.Context, userID string, input AdminUpdateInput) error {
	existed, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Role changes are intentionally excluded from this application API. They
	// are performed by the operator database workflow.
	existed.Name = input.Name
	existed.NickName = input.NickName
	existed.RemainingCoin = input.RemainingCoin.MinorUnits()
	if input.GroupID != nil {
		existed.GroupID = input.GroupID
	}

	if err := s.repo.Update(ctx, existed); err != nil {
		return err
	}

	s.log.Info("User updated by admin successfully", zap.String("user_id", userID))
	return nil
}

// DeductCoin deducts coins from user balance atomically with transaction safety.
func (s *Service) DeductCoin(ctx context.Context, userID string, amount value.Money) (value.Money, error) {
	var remainingBalance value.Money

	err := s.repo.WithinTransaction(ctx, func(tx Transaction) error {
		// 1. Lock user row for update.
		user, err := tx.LockByID(ctx, userID)
		if err != nil {
			return err
		}

		// 2. Validate balance (allow exactly 0, reject negative).
		if user.RemainingCoin < amount.MinorUnits() {
			return ErrInsufficientBalance
		}

		// 3. Atomic deduction using the transaction-bound repository.
		if err := tx.DeductBalance(ctx, userID, amount.MinorUnits()); err != nil {
			return err
		}

		// 4. Calculate remaining balance for response.
		remainingBalance = value.MustMoneyFromMinor(user.RemainingCoin - amount.MinorUnits())
		return nil
	})
	if err != nil {
		return value.Money{}, err
	}

	s.log.Info("Coins deducted successfully",
		zap.String("user_id", userID),
		zap.Int64("amount_minor", amount.MinorUnits()),
		zap.Int64("remaining_minor", remainingBalance.MinorUnits()))
	// Return the balance computed while the locked row was updated.
	return remainingBalance, nil
}
