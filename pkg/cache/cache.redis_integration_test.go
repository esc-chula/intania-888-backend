//go:build integration

package cache

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestBrowserSessionRotationAndRevocation(t *testing.T) {
	addr := os.Getenv("INTANIA888_TEST_REDIS_ADDR")
	if addr == "" {
		if os.Getenv("INTANIA888_REQUIRE_INTEGRATION") == "1" {
			t.Fatal("INTANIA888_TEST_REDIS_ADDR is required")
		}
		t.Skip("INTANIA888_TEST_REDIS_ADDR is required")
	}
	client := redis.NewClient(&redis.Options{Addr: addr, Password: os.Getenv("INTANIA888_TEST_REDIS_PASSWORD")})
	defer client.Close()
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	r := &RedisClient{client: client}
	prefix := "test:auth:" + time.Now().Format("20060102150405.000000000")
	userKey := prefix + ":user"
	firstKey := prefix + ":first"
	secondKey := prefix + ":second"
	defer client.Del(ctx, userKey, firstKey, secondKey)
	record := struct {
		ExpiresAt int64  `json:"expires_at"`
		UserID    string `json:"user_id"`
	}{time.Now().Add(30 * 24 * time.Hour).Unix(), "user"}
	if err := r.RotateSession(context.Background(), userKey, firstKey, "test:auth:previous", record, 60, 30*24*3600); err != nil {
		t.Fatal(err)
	}
	var got struct {
		ExpiresAt int64  `json:"expires_at"`
		UserID    string `json:"user_id"`
	}
	if err := r.ReadAndRenewSession(context.Background(), firstKey, time.Now().Unix(), 30, &got); err != nil {
		t.Fatal(err)
	}
	if err := r.RotateSession(context.Background(), userKey, secondKey, firstKey, record, 60, 30*24*3600); err != nil {
		t.Fatal(err)
	}
	if err := r.ReadAndRenewSession(context.Background(), firstKey, time.Now().Unix(), 30, &got); !errors.Is(err, redis.Nil) {
		t.Fatalf("rotated session survived: %v", err)
	}
	if err := r.DeleteSession(context.Background(), secondKey); err != nil {
		t.Fatal(err)
	}
	if err := r.ReadAndRenewSession(context.Background(), secondKey, time.Now().Unix(), 30, &got); !errors.Is(err, redis.Nil) {
		t.Fatalf("revoked session resurrected: %v", err)
	}
	if err := r.RotateSession(context.Background(), userKey, firstKey, "test:auth:previous", record, 60, 30*24*3600); err != nil {
		t.Fatal(err)
	}
	if err := r.ReadAndRenewSession(context.Background(), firstKey, time.Now().Add(31*24*time.Hour).Unix(), 30, &got); !errors.Is(err, redis.Nil) {
		t.Fatalf("absolute expiry ignored: %v", err)
	}
}
