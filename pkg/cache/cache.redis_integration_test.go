//go:build integration

package cache

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestCompareAndSwapValuesAllowsOnlyOneConcurrentRotation(t *testing.T) {
	addr := os.Getenv("INTANIA888_TEST_REDIS_ADDR")
	if addr == "" {
		t.Fatal("INTANIA888_TEST_REDIS_ADDR is required")
	}

	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("connect to integration Redis: %v", err)
	}
	defer client.Close()
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush integration Redis: %v", err)
	}

	r := &RedisClient{client: client}
	expected := map[string]interface{}{
		"auth:v2:refresh:old":     map[string]string{"status": "active"},
		"auth:v2:session:session": map[string]string{"refresh": "old"},
	}
	replacements := map[string]interface{}{
		"auth:v2:refresh:old":     map[string]string{"status": "used"},
		"auth:v2:refresh:new":     map[string]string{"status": "active"},
		"auth:v2:session:session": map[string]string{"refresh": "new"},
	}
	for key, value := range expected {
		if err := r.SetValue(key, value, 60); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}

	results := make(chan bool, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			applied, err := r.CompareAndSwapValues(expected, replacements, 60)
			if err != nil {
				t.Errorf("CompareAndSwapValues() error = %v", err)
			}
			results <- applied
		}()
	}
	wait.Wait()
	close(results)

	applied := 0
	for result := range results {
		if result {
			applied++
		}
	}
	if applied != 1 {
		t.Fatalf("successful compare-and-swaps = %d, want 1", applied)
	}
}
