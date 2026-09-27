package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(cfg config.Config) *RedisClient {
	addr := fmt.Sprintf("%s:%d", cfg.GetCache().Host, cfg.GetCache().Port)

	cache := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.GetCache().Password,
	})

	if cache == nil {
		panic("failed to initialize Redis")
	}

	return &RedisClient{client: cache}
}

func (r *RedisClient) SetValue(key string, value interface{}, ttl int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	v, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, v, time.Duration(ttl)*time.Second).Err()
}

func (r *RedisClient) GetValue(key string, value interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	v, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(v), value)
}

// ConsumeValue atomically reads and deletes a JSON value. It is used for
// one-time OAuth state and other single-use cache entries.
func (r *RedisClient) ConsumeValue(key string, value interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	v, err := r.client.GetDel(ctx, key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(v), value)
}

func (r *RedisClient) DeleteValue(key string) error {
	return r.DeleteValues(key)
}

// DeleteValues removes related session records in one Redis command.
// DeleteValues removes related session records in one Redis command.
func (r *RedisClient) DeleteValues(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return r.client.Del(ctx, keys...).Err()
}

// RotateSession commits the new browser session and revokes the prior one in
// one Redis operation. The per-user pointer never contains a raw session ID.
func (r *RedisClient) RotateSession(userKey, sessionKey, previousKey string, value interface{}, ttl int) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return r.client.Eval(
		ctx,
		`local old = redis.call('GET', KEYS[1])
 redis.call('SET', KEYS[2], ARGV[1], 'EX', ARGV[2])
 redis.call('SET', KEYS[1], KEYS[2], 'EX', ARGV[3])
 if old and old ~= KEYS[2] then redis.call('DEL', old) end
 if KEYS[3] ~= KEYS[2] then redis.call('DEL', KEYS[3]) end
 return 1`,
		[]string{userKey, sessionKey, previousKey},
		payload,
		ttl,
		30*24*3600,
	).Err()
}

// ReadAndRenewSession returns the stored record only if it still exists. TTL
// renewal is atomic with the read, so a concurrent logout cannot resurrect it.
func (r *RedisClient) ReadAndRenewSession(key string, now int64, idleSeconds int, value interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
func (r *RedisClient) DeleteSession(key string) error {
	return r.DeleteValues(key)
}

func (r *RedisClient) HasKey(key string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n, err := r.client.Exists(ctx, key).Result()
	return n > 0, err
}
