package middleware

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
)

// RedisSessionStore loads and renews the persisted authentication records.
type RedisSessionStore struct{ client *cache.RedisClient }

// NewRedisSessionStore binds session persistence to its required Redis client.
func NewRedisSessionStore(client *cache.RedisClient) *RedisSessionStore {
	return &RedisSessionStore{client: client}
}

type sessionRecord struct {
	UserID    string `json:"user_id"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
	CSRFToken string `json:"csrf_token"`
}

// ReadAndRenewSession atomically loads a session and extends its bounded idle lifetime.
func (r *RedisSessionStore) ReadAndRenewSession(ctx context.Context, key string, now int64, idle int) (*security.Session, error) {
	var record sessionRecord
	if err := r.client.ReadAndRenewSession(ctx, key, now, idle, &record); err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("%w: %w", ErrSessionMissing, err)
		}
		return nil, err
	}
	return &security.Session{
		UserID:    record.UserID,
		CreatedAt: record.CreatedAt,
		ExpiresAt: record.ExpiresAt,
		CSRFToken: record.CSRFToken,
	}, nil
}

// GetExternalSubject returns the subject bound to an active external JWT.
func (r *RedisSessionStore) GetExternalSubject(ctx context.Context, key string) (string, error) {
	var record struct {
		SubjectID string `json:"subject_id"`
	}
	if err := r.client.GetValue(ctx, key, &record); err != nil {
		if errors.Is(err, redis.Nil) {
			return "", fmt.Errorf("%w: %w", ErrExternalMissing, err)
		}
		return "", err
	}
	return record.SubjectID, nil
}

// ReadDelegation translates cache expiry into an authentication failure.
func (r *RedisSessionStore) ReadDelegation(ctx context.Context, id string, idle int) (*security.Delegation, error) {
	store := security.DelegationStore{Cache: r.client}
	grant, err := store.Read(ctx, id, idle)
	if errors.Is(err, redis.Nil) {
		return nil, ErrExternalMissing
	}

	return grant, err
}
