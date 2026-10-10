// Package httplimit applies process-local request budgets without advancing the HTTP stack.
package httplimit

import (
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

const idleLifetime = 2 * time.Minute

type entry struct {
	current  int
	previous int
	reset    time.Time
	tokens   float64
	refill   time.Time
	seen     time.Time
}

// Limiter combines Fiber's weighted two-window algorithm with a token bucket.
// Check can run at a verified-identity boundary without calling c.Next prematurely.
type Limiter struct {
	mu      sync.Mutex
	policy  config.RatePolicy
	name    string
	now     func() time.Time
	logger  *zap.Logger
	entries map[string]*entry
	cleanup time.Time
}

// New constructs an independent policy; callers validate settings during startup.
func New(name string, policy config.RatePolicy, logger *zap.Logger) *Limiter {
	return newWithClock(name, policy, logger, time.Now)
}

func newWithClock(name string, policy config.RatePolicy, logger *zap.Logger, now func() time.Time) *Limiter {
	return &Limiter{
		name: name, policy: policy, logger: logger, now: now,
		entries: make(map[string]*entry),
	}
}

// Check consumes an attempt's minute allowance and an admitted attempt's burst token.
// It returns the existing API error on rejection and never executes a downstream handler.
func (l *Limiter) Check(c *fiber.Ctx, key string) error {
	wait, remaining, reset := l.take(key)
	if wait > 0 {
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		if l.logger != nil {
			l.logger.Warn("http.rate_limit.rejected",
				zap.String("policy", l.name), zap.String("route", c.Path()),
				zap.String("request_id", c.GetRespHeader(apierror.RequestIDHeader)),
			)
		}
		return apierror.New(fiber.StatusTooManyRequests, apierror.CodeTooManyRequests, "Too many requests")
	}
	c.Set("X-RateLimit-Limit", strconv.Itoa(l.policy.PerMinute))
	c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
	c.Set("X-RateLimit-Reset", strconv.Itoa(int(math.Ceil(reset.Seconds()))))
	return nil
}

// Middleware applies a policy before advancing to the next route handler.
func (l *Limiter) Middleware(key func(*fiber.Ctx) string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := l.Check(c, key(c)); err != nil {
			return err
		}
		return c.Next()
	}
}

func (l *Limiter) take(key string) (time.Duration, int, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if !now.Before(l.cleanup) {
		for storedKey, stored := range l.entries {
			if now.Sub(stored.seen) >= idleLifetime {
				delete(l.entries, storedKey)
			}
		}
		l.cleanup = now.Add(time.Minute)
	}
	e := l.entries[key]
	if e == nil {
		e = &entry{reset: now.Add(time.Minute), tokens: float64(l.policy.Burst), refill: now}
		l.entries[key] = e
	}
	e.seen = now
	if !now.Before(e.reset) {
		elapsed := now.Sub(e.reset)
		if elapsed >= time.Minute {
			e.previous = 0
			e.current = 0
			e.reset = now.Add(time.Minute)
		} else {
			e.previous = e.current
			e.current = 0
			e.reset = e.reset.Add(time.Minute)
		}
	}
	e.current++
	reset := e.reset.Sub(now)
	weighted := int(float64(e.previous)*reset.Seconds()/60) + e.current
	remaining := l.policy.PerMinute - weighted
	if remaining < 0 {
		return reset, 0, reset
	}

	// Refill continuously at the minute allowance / 60, capped at burst capacity.
	rate := float64(l.policy.PerMinute) / 60
	elapsed := math.Max(0, now.Sub(e.refill).Seconds())
	e.tokens = math.Min(float64(l.policy.Burst), e.tokens+elapsed*rate)
	e.refill = now
	if e.tokens < 1 {
		return time.Duration(math.Ceil((1 - e.tokens) / rate * float64(time.Second))), remaining, reset
	}
	e.tokens--
	return 0, remaining, reset
}
