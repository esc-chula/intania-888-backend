package user

import (
	"context"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

// Repository provides account snapshots and transaction boundaries.
// Missing accounts return ErrUserNotFound; other errors retain their underlying cause.
type Repository interface {
	// Create persists an account and updates its generated timestamps.
	Create(context.Context, *identity.User) error
	// GetByID retrieves an account by its identifier.
	GetByID(context.Context, string) (*identity.User, error)
	// GetByEmail retrieves an account by its email address.
	GetByEmail(context.Context, string) (*identity.User, error)
	// GetAll retrieves all account snapshots.
	GetAll(context.Context) ([]*identity.User, error)
	// PatchProfile updates only supplied name, nickname, and group fields for the actor.
	// Explicitly supplied nil nickname and group values clear their stored columns.
	// Missing accounts return ErrUserNotFound.
	PatchProfile(context.Context, string, ProfilePatch) error
	// Update preserves the existing nonzero-field update behavior.
	Update(context.Context, *identity.User) error
	// WithinTransaction commits the callback's writes together or rolls them back.
	WithinTransaction(context.Context, func(Transaction) error) error
}

// Transaction provides balance operations bound to one account transaction.
// Callbacks must not retain this repository after returning.
type Transaction interface {
	// LockByID locks the account until the transaction completes.
	LockByID(context.Context, string) (*identity.User, error)
	// DeductBalance subtracts minor units from the locked account atomically.
	DeductBalance(context.Context, string, int64) error
}

// ServicePort describes the account use cases consumed by HTTP handlers.
type ServicePort interface {
	// CreateUser creates an account with the default balance.
	CreateUser(context.Context, CreateInput) (*identity.Profile, error)
	// GetUser retrieves one account profile.
	GetUser(context.Context, string) (*identity.Profile, error)
	// GetAllUsers retrieves every account profile.
	GetAllUsers(context.Context) ([]*identity.Profile, error)
	// UpdateOwnProfile applies a partial update to the authenticated actor's profile.
	UpdateOwnProfile(context.Context, string, ProfilePatch) (*identity.Profile, error)
	// AdminUpdateUser updates administrator-editable fields.
	AdminUpdateUser(context.Context, string, AdminUpdateInput) error
	// DeductCoin subtracts a balance under a row lock and returns the remaining amount.
	DeductCoin(context.Context, string, value.Money) (value.Money, error)
}
