package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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
func (r *RedisClient) DeleteValues(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return r.client.Del(ctx, keys...).Err()
}

// CompareAndSwapValues atomically replaces JSON values when every expected
// value still matches. It is used for refresh-token rotation: the old refresh
// record, active session, and replacement records are committed together.
func (r *RedisClient) CompareAndSwapValues(expected map[string]interface{}, replacements map[string]interface{}, ttl int) (bool, error) {
	if len(expected) == 0 || len(replacements) == 0 || ttl <= 0 {
		return false, errors.New("cache compare-and-swap requires values and a positive TTL")
	}

	expectedJSON, err := marshalCacheValues(expected)
	if err != nil {
		return false, err
	}
	replacementJSON, err := marshalCacheValues(replacements)
	if err != nil {
		return false, err
	}

	keys := make([]string, 0, len(expectedJSON))
	for key := range expectedJSON {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for attempt := 0; attempt < 3; attempt++ {
		applied := false
		err := r.client.Watch(ctx, func(tx *redis.Tx) error {
			for _, key := range keys {
				actual, err := tx.Get(ctx, key).Bytes()
				if errors.Is(err, redis.Nil) {
					return nil
				}
				if err != nil {
					return err
				}
				if !bytes.Equal(actual, expectedJSON[key]) {
					return nil
				}
			}

			_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				for key, value := range replacementJSON {
					pipe.Set(ctx, key, value, time.Duration(ttl)*time.Second)
				}
				return nil
			})
			if errors.Is(err, redis.TxFailedErr) {
				return err
			}
			if err != nil {
				return err
			}
			applied = true
			return nil
		}, keys...)
		if err == nil {
			return applied, nil
		}
		if !errors.Is(err, redis.TxFailedErr) {
			return false, err
		}
	}

	return false, redis.TxFailedErr
}

func marshalCacheValues(values map[string]interface{}) (map[string][]byte, error) {
	encoded := make(map[string][]byte, len(values))
	for key, value := range values {
		if key == "" {
			return nil, errors.New("cache key cannot be empty")
		}
		payload, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		encoded[key] = payload
	}
	return encoded, nil
}
