package policy

import (
	"context"
	"time"
)

// Repository persists neutral policy snapshots and translates storage failures.
type Repository interface {
	// ListActive returns enabled, unexpired policies in descending creation order.
	ListActive(ctx context.Context, now time.Time) ([]*AccessPolicy, error)
	// List returns a page using the supplied filter.
	List(ctx context.Context, filter ListFilter) (ListResult, error)
	// FindByID returns ErrPolicyNotFound when the policy does not exist.
	FindByID(ctx context.Context, id string) (*AccessPolicy, error)
	// FindByIdentity returns ErrPolicyNotFound when the principal has no matching entry.
	FindByIdentity(ctx context.Context, kind, principalType, principal string) (*AccessPolicy, error)
	// Create inserts a policy and returns ErrPolicyConflict for an existing identity.
	Create(ctx context.Context, policy *AccessPolicy) error
	// Update persists mutable fields without replacing identity fields.
	Update(ctx context.Context, policy *AccessPolicy) error
}

// SnapshotCache stores policy snapshots without exposing a cache wire format.
type SnapshotCache interface {
	// Load retrieves the current snapshot, returning an error on a cache miss.
	Load(ctx context.Context) ([]*AccessPolicy, error)
	// Store replaces the snapshot with the existing short-lived cache policy.
	Store(ctx context.Context, policies []*AccessPolicy) error
	// Delete invalidates a stale snapshot.
	Delete(ctx context.Context) error
}
