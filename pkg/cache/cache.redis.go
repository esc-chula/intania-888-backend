package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wiraphatys/intania888/pkg/config"
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

func (r *RedisClient) DeleteValue(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return r.client.Del(ctx, key).Err()
}
