package policy

import (
	"context"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/security"
)

const policyCacheTTL = 60

// RedisValues is the Redis functionality needed by the snapshot adapter.
type RedisValues interface {
	// SetValue encodes a cache value with a TTL measured in seconds.
	SetValue(ctx context.Context, key string, value any, ttl int) error
	// GetValue decodes a previously encoded cache value.
	GetValue(ctx context.Context, key string, value any) error
	// DeleteValue invalidates a cache value.
	DeleteValue(ctx context.Context, key string) error
}

type redisSnapshotCache struct{ values RedisValues }

// NewRedisSnapshotCache adapts Redis values to the policy snapshot boundary.
func NewRedisSnapshotCache(values RedisValues) *redisSnapshotCache {
	return &redisSnapshotCache{values: values}
}

// Load reads the established policy snapshot cache format.
func (c *redisSnapshotCache) Load(ctx context.Context) ([]*AccessPolicy, error) {
	var records []*policySnapshotRecord
	if err := c.values.GetValue(ctx, security.ToPolicySnapshotCacheKey(), &records); err != nil {
		return nil, err
	}
	if records == nil {
		return nil, nil
	}
	result := make([]*AccessPolicy, len(records))
	for i, record := range records {
		if record != nil {
			result[i] = &AccessPolicy{ID: record.ID, Kind: record.Kind, PrincipalType: record.PrincipalType,
				Principal: record.Principal, Reason: record.Reason, Enabled: record.Enabled,
				ExpiresAt: record.ExpiresAt, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
		}
	}
	return result, nil
}

// Store writes the established policy snapshot format with its existing TTL.
func (c *redisSnapshotCache) Store(ctx context.Context, policies []*AccessPolicy) error {
	var records []*policySnapshotRecord
	if policies != nil {
		records = make([]*policySnapshotRecord, len(policies))
		for i, policy := range policies {
			if policy != nil {
				records[i] = &policySnapshotRecord{ID: policy.ID, Kind: policy.Kind, PrincipalType: policy.PrincipalType,
					Principal: policy.Principal, Reason: policy.Reason, Enabled: policy.Enabled,
					ExpiresAt: policy.ExpiresAt, CreatedAt: policy.CreatedAt, UpdatedAt: policy.UpdatedAt}
			}
		}
	}
	return c.values.SetValue(ctx, security.ToPolicySnapshotCacheKey(), records, policyCacheTTL)
}

// Delete invalidates the shared policy snapshot.
func (c *redisSnapshotCache) Delete(ctx context.Context) error {
	return c.values.DeleteValue(ctx, security.ToPolicySnapshotCacheKey())
}

type policySnapshotRecord struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	PrincipalType string     `json:"principal_type"`
	Principal     string     `json:"principal"`
	Reason        string     `json:"reason"`
	Enabled       bool       `json:"enabled"`
	ExpiresAt     *time.Time `json:"expires_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
