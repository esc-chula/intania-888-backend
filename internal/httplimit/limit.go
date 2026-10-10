// Package httplimit applies process-local request budgets without advancing the HTTP stack.
package httplimit

import (
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

const idleLifetime = 2 * time.Minute

type entry struct {
	bucket *rate.Limiter
	seen   time.Time
}

// Check is a request-budget hook that never advances the HTTP handler chain.
type Check func(*fiber.Ctx, string) error

// Limiter applies an independent token bucket to each identity.
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
		name:    name,
		policy:  policy,
		logger:  logger,
		now:     now,
		entries: make(map[string]*entry),
	}
}

// Check consumes one token when admitted; rejected attempts do not extend the cooldown.
// It returns the existing API error on rejection and never executes a downstream handler.
func (l *Limiter) Check(c *fiber.Ctx, key string) error {
	wait, remaining, reset := l.take(key)
	c.Set("X-RateLimit-Limit", strconv.Itoa(l.policy.Burst))
	c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
	c.Set("X-RateLimit-Reset", strconv.Itoa(int(math.Ceil(reset.Seconds()))))
	if wait > 0 {
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		if l.logger != nil {
			l.logger.Warn("http.rate_limit.rejected",
				zap.String("policy", l.name),
				zap.String("route", c.Route().Path),
				zap.String("request_id", c.GetRespHeader(apierror.RequestIDHeader)),
			)
		}

		return apierror.New(fiber.StatusTooManyRequests, apierror.CodeTooManyRequests, "Too many requests")
	}

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
			if now.Sub(stored.seen) >= idleLifetime && stored.bucket.TokensAt(now) >= float64(l.policy.Burst) {
				delete(l.entries, storedKey)
			}
		}
		l.cleanup = now.Add(time.Minute)
	}

	e := l.entries[key]
	if e == nil {
		e = &entry{
			bucket: rate.NewLimiter(rate.Limit(float64(l.policy.PerMinute)/60), l.policy.Burst),
		}
		l.entries[key] = e
	}
	e.seen = now

	allowed := e.bucket.AllowN(now, 1)
	tokens := e.bucket.TokensAt(now)
	remaining := int(tokens)
	secondsPerToken := 1 / float64(e.bucket.Limit())
	reset := time.Duration(math.Ceil((float64(l.policy.Burst) - tokens) * secondsPerToken * float64(time.Second)))
	if !allowed {
		wait := time.Duration(math.Ceil((1 - tokens) * secondsPerToken * float64(time.Second)))

		return wait, remaining, reset
	}

	return 0, remaining, reset
}
