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
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("INTANIA888_TEST_REDIS_PASSWORD"),
	})
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
	previousKey := prefix + ":previous"
	defer client.Del(ctx, userKey, firstKey, secondKey, previousKey)
	record := struct {
		ExpiresAt int64  `json:"expires_at"`
		UserID    string `json:"user_id"`
	}{time.Now().Add(30 * 24 * time.Hour).Unix(), "user"}
	if err := r.RotateSession(context.Background(), userKey, firstKey, previousKey, record, 60, 30*24*3600); err != nil {
		t.Fatal(err)
	}
	var got struct {
		ExpiresAt int64  `json:"expires_at"`
		UserID    string `json:"user_id"`
	}
	if err := r.ReadAndRenewSession(context.Background(), firstKey, time.Now().Unix(), 30, &got); err != nil {
		t.Fatal(err)
	}
	// A second browser has no cookie for the first browser's session.
	if err := r.RotateSession(context.Background(), userKey, secondKey, previousKey, record, 60, 30*24*3600); err != nil {
		t.Fatal(err)
	}
	if err := r.ReadAndRenewSession(context.Background(), firstKey, time.Now().Unix(), 30, &got); !errors.Is(err, redis.Nil) {
		t.Fatalf("rotated session survived: %v", err)
	}
	if pointedKey, err := client.Get(ctx, userKey).Result(); err != nil || pointedKey != secondKey {
		t.Fatalf("account session pointer = %q, %v; want %q", pointedKey, err, secondKey)
	}
	if err := client.Set(ctx, previousKey, "stale browser session", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if err := r.RotateSession(ctx, userKey, firstKey, previousKey, record, 60, 30*24*3600); err != nil {
		t.Fatal(err)
	}
	for _, revokedKey := range []string{secondKey, previousKey} {
		if exists, err := client.Exists(ctx, revokedKey).Result(); err != nil || exists != 0 {
			t.Fatalf("revoked session %q still exists: count=%d error=%v", revokedKey, exists, err)
		}
	}
	if err := r.DeleteSession(context.Background(), firstKey); err != nil {
		t.Fatal(err)
	}
	if err := r.ReadAndRenewSession(context.Background(), firstKey, time.Now().Unix(), 30, &got); !errors.Is(err, redis.Nil) {
		t.Fatalf("revoked session resurrected: %v", err)
	}
	if err := r.RotateSession(context.Background(), userKey, firstKey, previousKey, record, 60, 30*24*3600); err != nil {
		t.Fatal(err)
	}
	if err := r.ReadAndRenewSession(context.Background(), firstKey, time.Now().Add(31*24*time.Hour).Unix(), 30, &got); !errors.Is(err, redis.Nil) {
		t.Fatalf("absolute expiry ignored: %v", err)
	}
}
