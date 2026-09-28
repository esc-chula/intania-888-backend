package event

import (
	"context"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

// Repository provides event queries and a transaction boundary without exposing a driver.
type Repository interface {
	// WithinTransaction commits the callback only on success; its repository must not escape the callback.
	WithinTransaction(ctx context.Context, fn func(TransactionRepository) error) error
	// ListRewards returns configured daily reward overrides.
	ListRewards(ctx context.Context) ([]DailyReward, error)
	// SetReward creates or replaces a daily reward override.
	SetReward(ctx context.Context, reward DailyReward) error
	// DeleteReward deletes an override or returns ErrDailyRewardOverrideNotFound.
	DeleteReward(ctx context.Context, date string) error
	// DeleteExpiredTokens deletes tokens expired before the supplied instant.
	DeleteExpiredTokens(ctx context.Context, before time.Time) error
	// GetUser reads the actor's current account state, returning ErrUserNotFound if absent.
	GetUser(ctx context.Context, id string) (identity.User, error)
	// GetRandomEligibleUsers selects candidates meeting a service-supplied minimum balance.
	GetRandomEligibleUsers(ctx context.Context, excludeUserID string, minimumBalance int64, limit int) ([]identity.User, error)
}

// TransactionRepository contains queries bound to one atomic event operation.
// User locks are acquired in sorted ID order when more than one user is involved.
type TransactionRepository interface {
	// GetReward returns an override or ErrDailyRewardOverrideNotFound.
	GetReward(ctx context.Context, date string) (DailyReward, error)
	// LockUser locks an account row for update and returns its current state.
	LockUser(ctx context.Context, id string) (identity.User, error)
	// CreateDailyClaim returns false when the same user and date already exist.
	CreateDailyClaim(ctx context.Context, userID, date string, reward value.Money) (bool, error)
	// SetUserBalance writes the absolute balance within the current transaction.
	SetUserBalance(ctx context.Context, id string, balance value.Money) error
	// CreateStealToken inserts a token within the current transaction.
	CreateStealToken(ctx context.Context, token StealToken) error
	// LockStealToken locks a token row or returns ErrStealTokenInvalid.
	LockStealToken(ctx context.Context, token string) (StealToken, error)
	// GetUsersByIDs reads candidate snapshots before balance changes are applied.
	GetUsersByIDs(ctx context.Context, ids []string) ([]identity.User, error)
	// MarkTokenUsed conditionally consumes an unused token and reports whether it changed.
	MarkTokenUsed(ctx context.Context, id string) (bool, error)
}
