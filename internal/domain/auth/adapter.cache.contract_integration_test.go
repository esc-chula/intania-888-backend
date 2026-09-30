//go:build integration

package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/esc-chula/intania-888-backend/internal/domain/auth"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

type cacheContractConfig struct {
	config.Config
	cache config.Cache
}

func (c cacheContractConfig) GetCache() config.Cache { return c.cache }

func TestTypedAuthCacheInteroperatesWithMiddlewareAndPreservesStoredFields(t *testing.T) {
	addr := os.Getenv("INTANIA888_TEST_REDIS_ADDR")
	if addr == "" {
		if os.Getenv("INTANIA888_REQUIRE_INTEGRATION") == "1" {
			t.Fatal("INTANIA888_TEST_REDIS_ADDR is required when INTANIA888_REQUIRE_INTEGRATION=1")
		}
		t.Skip("INTANIA888_TEST_REDIS_ADDR is required")
	}
	host, portString, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}
	password := os.Getenv("INTANIA888_TEST_REDIS_PASSWORD")
	raw := redis.NewClient(&redis.Options{Addr: addr, Password: password})
	t.Cleanup(func() {
		if err := raw.Close(); err != nil {
			t.Errorf("close Redis inspection client: %v", err)
		}
	})
	ctx := context.Background()
	if err := raw.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}

	client := cache.NewRedisClient(cacheContractConfig{cache: config.Cache{Host: host, Port: port, Password: password}})
	writer := auth.NewRedisRepository(client)
	reader := middleware.NewRedisSessionStore(client)
	prefix := "test:typed-auth:" + uuid.NewString()
	userKey, sessionKey, previousKey := prefix+":user", prefix+":session", prefix+":previous"
	stateKey := prefix + ":state"
	t.Cleanup(func() {
		if err := raw.Del(context.Background(), userKey, sessionKey, previousKey, stateKey).Err(); err != nil {
			t.Errorf("remove test authentication records: %v", err)
		}
	})

	now := time.Now().Unix()
	wantSession := security.Session{UserID: "user", CreatedAt: now, ExpiresAt: now + 3600, CSRFToken: "csrf-fixture"}
	if err := writer.RotateSession(ctx, userKey, sessionKey, previousKey, wantSession, 60, 3600); err != nil {
		t.Fatal(err)
	}
	stored, err := raw.Get(ctx, sessionKey).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	var sessionFields map[string]any
	if err := json.Unmarshal(stored, &sessionFields); err != nil {
		t.Fatal(err)
	}
	if len(sessionFields) != 4 || sessionFields["user_id"] != "user" || sessionFields["created_at"] != float64(now) || sessionFields["expires_at"] != float64(now+3600) || sessionFields["csrf_token"] != "csrf-fixture" {
		t.Fatalf("stored session fields changed: %+v", sessionFields)
	}
	gotSession, err := reader.ReadAndRenewSession(ctx, sessionKey, now, 30)
	if err != nil || !reflect.DeepEqual(gotSession, &wantSession) {
		t.Fatalf("typed writer/reader disagreed: session=%+v error=%v", gotSession, err)
	}

	if err := writer.StoreOAuthState(ctx, stateKey, auth.OAuthState{CodeVerifier: "pkce-fixture"}, 60); err != nil {
		t.Fatal(err)
	}
	stateBytes, err := raw.Get(ctx, stateKey).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(stateBytes) != `{"code_verifier":"pkce-fixture"}` {
		t.Fatalf("stored OAuth state shape changed: %s", stateBytes)
	}
	state, err := writer.ConsumeOAuthState(ctx, stateKey)
	if err != nil || state.CodeVerifier != "pkce-fixture" {
		t.Fatalf("consumed OAuth state = %+v, %v", state, err)
	}
	if _, err := writer.ConsumeOAuthState(ctx, stateKey); !errors.Is(err, auth.ErrInvalidOAuthState) || !errors.Is(err, redis.Nil) {
		t.Fatalf("consumed state missing classification lost its cause: %v", err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := reader.ReadAndRenewSession(canceled, sessionKey, now, 30); !errors.Is(err, context.Canceled) || errors.Is(err, middleware.ErrSessionMissing) {
		t.Fatalf("canceled session read was detached or misclassified: %v", err)
	}
	if _, err := writer.ConsumeOAuthState(canceled, stateKey); !errors.Is(err, context.Canceled) || errors.Is(err, auth.ErrInvalidOAuthState) {
		t.Fatalf("canceled OAuth state read was detached or misclassified: %v", err)
	}

	if err := writer.DeleteSession(ctx, sessionKey); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadAndRenewSession(ctx, sessionKey, now, 30); !errors.Is(err, middleware.ErrSessionMissing) || !errors.Is(err, redis.Nil) {
		t.Fatalf("revoked session missing classification lost its cause: %v", err)
	}
}
