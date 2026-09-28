package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
)

// RedisRepository stores authentication protocol state in Redis.
type RedisRepository struct{ cache *cache.RedisClient }

// NewRedisRepository binds authentication persistence to its Redis client.
func NewRedisRepository(client *cache.RedisClient) *RedisRepository {
	return &RedisRepository{cache: client}
}

type oauthStateRecord struct {
	CodeVerifier string `json:"code_verifier"`
}

type sessionRecord struct {
	UserID    string `json:"user_id"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
	CSRFToken string `json:"csrf_token"`
}

// externalTokenRecord is the Redis wire record binding an external JWT identifier to its subject.
type externalTokenRecord struct {
	SubjectID string `json:"subject_id"`
}

// StoreOAuthState persists a one-use PKCE verifier with its existing lifetime.
func (r *RedisRepository) StoreOAuthState(ctx context.Context, key string, state OAuthState, ttl int) error {
	return r.cache.SetValue(ctx, key, oauthStateRecord(state), ttl)
}

// ConsumeOAuthState atomically consumes the state and classifies an absent entry.
func (r *RedisRepository) ConsumeOAuthState(ctx context.Context, key string) (OAuthState, error) {
	var record oauthStateRecord
	if err := r.cache.ConsumeValue(ctx, key, &record); err != nil {
		if errors.Is(err, redis.Nil) {
			return OAuthState{}, fmt.Errorf("%w: %w", ErrInvalidOAuthState, err)
		}
		return OAuthState{}, err
	}
	return OAuthState(record), nil
}

// RotateSession atomically replaces the user's session and revokes its predecessor.
func (r *RedisRepository) RotateSession(ctx context.Context, userKey, key, previous string, state security.Session, idle, absolute int) error {
	record := sessionRecord{
		UserID:    state.UserID,
		CreatedAt: state.CreatedAt,
		ExpiresAt: state.ExpiresAt,
		CSRFToken: state.CSRFToken,
	}
	return r.cache.RotateSession(ctx, userKey, key, previous, record, idle, absolute)
}

// DeleteSession revokes a session; deletion is idempotent.
func (r *RedisRepository) DeleteSession(ctx context.Context, key string) error {
	return r.cache.DeleteSession(ctx, key)
}

// StoreExternalToken records the subject permitted to use an external JWT.
func (r *RedisRepository) StoreExternalToken(ctx context.Context, key, subject string, ttl int) error {
	return r.cache.SetValue(ctx, key, externalTokenRecord{SubjectID: subject}, ttl)
}
