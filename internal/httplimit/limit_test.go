package httplimit

import (
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/esc-chula/intania-888-backend/internal/apierror"

	"github.com/esc-chula/intania-888-backend/pkg/config"
)

func TestBurstRefillAndIndependentKeys(t *testing.T) {
	now := time.Unix(1000, 0)
	l := newWithClock("test", config.RatePolicy{PerMinute: 60, Burst: 2}, nil, func() time.Time { return now })
	for i := 0; i < 2; i++ {
		if wait, _, _ := l.take("A"); wait != 0 {
			t.Fatalf("wait = %v", wait)
		}
	}
	if wait, _, _ := l.take("A"); wait != time.Second {
		t.Fatalf("wait = %v", wait)
	}
	if wait, _, _ := l.take("B"); wait != 0 {
		t.Fatalf("independent key wait = %v", wait)
	}
	now = now.Add(500 * time.Millisecond)
	if wait, _, _ := l.take("A"); wait != 500*time.Millisecond {
		t.Fatalf("partial refill wait = %v", wait)
	}
	now = now.Add(500 * time.Millisecond)
	if wait, _, _ := l.take("A"); wait != 0 {
		t.Fatalf("refill wait = %v", wait)
	}
}

func TestMinuteWindowAndRejectedAttemptDoesNotConsumeBurst(t *testing.T) {
	now := time.Unix(1000, 0)
	l := newWithClock("test", config.RatePolicy{PerMinute: 2, Burst: 10}, nil, func() time.Time { return now })
	for i := 0; i < 2; i++ {
		l.take("A")
	}
	if wait, _, _ := l.take("A"); wait != time.Minute {
		t.Fatalf("minute wait = %v", wait)
	}
	if got := l.entries["A"].tokens; got != 8 {
		t.Fatalf("tokens = %v", got)
	}
	now = now.Add(60 * time.Second)
	if wait, _, _ := l.take("A"); wait == 0 {
		t.Fatal("previous window was not weighted")
	}
	now = now.Add(2 * time.Minute)
	if wait, _, _ := l.take("A"); wait != 0 {
		t.Fatalf("expired window wait = %v", wait)
	}
}

func TestConcurrentBurstAndIdleCleanup(t *testing.T) {
	now := time.Unix(1000, 0)
	l := newWithClock("test", config.RatePolicy{PerMinute: 1000, Burst: 10}, nil, func() time.Time { return now })
	var allowed atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if wait, _, _ := l.take("A"); wait == 0 {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if allowed.Load() != 10 {
		t.Fatalf("allowed = %d", allowed.Load())
	}
	now = now.Add(2 * time.Minute)
	l.take("B")
	if _, exists := l.entries["A"]; exists {
		t.Fatal("idle entry retained")
	}
}

func TestRejectedPolicyOwnsHeadersAndLogsRoutePattern(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	limiter := New("account", config.RatePolicy{PerMinute: 1, Burst: 1}, zap.New(core))
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
	app.Use(apierror.RequestID())
	app.Use(New("shared", config.RatePolicy{PerMinute: 60000, Burst: 2000}, nil).Middleware(ClientIP))
	reached := 0
	app.Get("/users/:id", limiter.Middleware(func(*fiber.Ctx) string { return "account" }), func(c *fiber.Ctx) error {
		reached++
		return c.SendStatus(204)
	})
	for _, want := range []int{204, 429} {
		response, err := app.Test(httptest.NewRequest("GET", "/users/private-account", nil))
		if err != nil {
			t.Fatal(err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want || response.Header.Get("X-RateLimit-Limit") != "1" {
			t.Fatalf("status=%d, limit=%q", response.StatusCode, response.Header.Get("X-RateLimit-Limit"))
		}
		if want == 429 && (response.Header.Get("Retry-After") == "" || response.Header.Get("X-RateLimit-Remaining") != "0") {
			t.Fatal("rejection did not identify its exhausted budget")
		}
	}
	events := logs.All()
	if reached != 1 || len(events) != 1 || events[0].ContextMap()["route"] != "/users/:id" {
		t.Fatalf("downstream=%d, rejection events=%+v", reached, events)
	}
}
