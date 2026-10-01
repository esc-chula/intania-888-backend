package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/esc-chula/intania-888-backend/pkg/config"
)

// RedisClient stores JSON values and coordinates atomic browser session operations.
// Operations preserve caller cancellation and add a five-second timeout.
type RedisClient struct {
	client *redis.Client
}

// NewRedisClient configures a Redis connection without probing the server.
func NewRedisClient(cfg config.Config) *RedisClient {
	addr := fmt.Sprintf("%s:%d", cfg.GetCache().Host, cfg.GetCache().Port)

	cache := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.GetCache().Password,
	})

	return &RedisClient{client: cache}
}

// Ping checks whether Redis is reachable using the caller's deadline.
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close releases the Redis client's connections.
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// SetValue JSON-encodes value and stores it under key. The TTL is measured in seconds.
// Encoding errors are returned before any Redis write occurs.
func (r *RedisClient) SetValue(ctx context.Context, key string, value interface{}, ttl int) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	v, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, v, time.Duration(ttl)*time.Second).Err()
}

// GetValue decodes the stored JSON into value, which must be a writable destination.
// A missing key returns redis.Nil; decoding errors leave the Redis entry unchanged.
func (r *RedisClient) GetValue(ctx context.Context, key string, value interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	v, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(v), value)
}

// ConsumeValue atomically reads and deletes a JSON value. It is used for
// one-time OAuth state and other single-use cache entries.
// The key is deleted before decoding, including when decoding fails.
// A missing key returns redis.Nil.
func (r *RedisClient) ConsumeValue(ctx context.Context, key string, value interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	v, err := r.client.GetDel(ctx, key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(v), value)
}

// DeleteValue removes one key and succeeds when it is already absent.
func (r *RedisClient) DeleteValue(ctx context.Context, key string) error {
	return r.DeleteValues(ctx, key)
}

// DeleteValues removes related session records in one Redis command.
// An empty key list succeeds without contacting Redis.
func (r *RedisClient) DeleteValues(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return r.client.Del(ctx, keys...).Err()
}

// RotateSession commits the new browser session and revokes both the account's
// current session and this browser's prior session in one Redis operation.
// The per-user pointer never contains a raw session ID.
// The session record uses idleTTLSeconds and the user pointer uses absoluteTTLSeconds.
func (r *RedisClient) RotateSession(
	ctx context.Context,
	userKey, sessionKey, previousKey string,
	value interface{},
	idleTTLSeconds, absoluteTTLSeconds int,
) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return r.client.Eval(
		ctx,
		`local currentKey = redis.call('GET', KEYS[1])
 if currentKey and currentKey ~= KEYS[2] then redis.call('DEL', currentKey) end
 if KEYS[3] ~= KEYS[2] then redis.call('DEL', KEYS[3]) end
 redis.call('SET', KEYS[2], ARGV[1], 'EX', ARGV[2])
 redis.call('SET', KEYS[1], KEYS[2], 'EX', ARGV[3])
 return 1`,
		[]string{userKey, sessionKey, previousKey},
		payload,
		idleTTLSeconds,
		absoluteTTLSeconds,
	).Err()
}

// ReadAndRenewSession returns the stored record only if it still exists. TTL
// renewal is atomic with the read, so a concurrent logout cannot resurrect it.
// The stored expires_at and now are Unix seconds; idleSeconds is capped by the
// remaining absolute lifetime. Missing or expired records return redis.Nil.
func (r *RedisClient) ReadAndRenewSession(ctx context.Context, key string, now int64, idleSeconds int, value interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	raw, err := r.client.Eval(
		ctx,
		`local raw = redis.call('GET', KEYS[1])
 if not raw then return nil end
 local record = cjson.decode(raw)
 local remaining = tonumber(record.expires_at) - tonumber(ARGV[1])
 if remaining <= 0 then redis.call('DEL', KEYS[1]); return nil end
 redis.call('EXPIRE', KEYS[1], math.min(remaining, tonumber(ARGV[2])))
 return raw`,
		[]string{key},
		now,
		idleSeconds,
	).Text()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(raw), value)
}

// DeleteSession is idempotent and reports Redis failures to the caller.
func (r *RedisClient) DeleteSession(ctx context.Context, key string) error {
	return r.DeleteValues(ctx, key)
}

// HasKey reports whether key currently exists without reading or renewing its value.
func (r *RedisClient) HasKey(ctx context.Context, key string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	n, err := r.client.Exists(ctx, key).Result()

	return n > 0, err
}

// EvalText executes an atomic authentication operation with a bounded deadline.
func (r *RedisClient) EvalText(ctx context.Context, script string, keys []string, args ...interface{}) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return r.client.Eval(ctx, script, keys, args...).Text()
}

// IsMissing reports a missing authentication record without leaking driver classification.
func IsMissing(err error) bool {
	return errors.Is(err, redis.Nil)
}
