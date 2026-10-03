package server

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

const (
	rateLimitWindow                      = time.Minute
	globalRequestsPerMinute              = 300
	externalClientRequestsPerMinute      = 10000
	externalProfileRequestsPerMinute     = 60
	externalTransactionRequestsPerMinute = 10
	externalClientIDLocalKey             = "external_rate_limit_client_id"
)

// ExternalRateLimiters holds shared client and endpoint-specific limiters.
type ExternalRateLimiters struct {
	Client      fiber.Handler
	Profile     fiber.Handler
	Transaction fiber.Handler
}

// NewExternalRateLimiters creates one shared client limiter and per-route user limiters.
func NewExternalRateLimiters(cfg config.Config) ExternalRateLimiters {
	clientLimiter := limiter.New(rateLimiterConfig(
		externalClientRequestsPerMinute,
		externalClientRateLimitKey,
	))

	return ExternalRateLimiters{
		Client: externalClientRateLimitMiddleware(clientLimiter, cfg),
		Profile: limiter.New(rateLimiterConfig(
			externalProfileRequestsPerMinute,
			externalUserRateLimitKey,
		)),
		Transaction: limiter.New(rateLimiterConfig(
			externalTransactionRequestsPerMinute,
			externalUserRateLimitKey,
		)),
	}
}

func registerRateLimiters(router fiber.Router) {
	globalLimiter := limiter.New(rateLimiterConfig(
		globalRequestsPerMinute,
		func(c *fiber.Ctx) string {
			return c.IP()
		},
	))
	router.Use(func(c *fiber.Ctx) error {
		if isExternalPath(c.Path()) {
			return c.Next()
		}

		return globalLimiter(c)
	})
}

func rateLimiterConfig(max int, keyGenerator func(*fiber.Ctx) string) limiter.Config {
	return limiter.Config{
		Max:               max,
		Expiration:        rateLimitWindow,
		KeyGenerator:      keyGenerator,
		LimitReached:      rateLimitReached,
		LimiterMiddleware: limiter.SlidingWindow{},
	}
}

func rateLimitReached(c *fiber.Ctx) error {
	return apierror.New(fiber.StatusTooManyRequests, apierror.CodeTooManyRequests, "Too many requests")
}

func externalClientRateLimitMiddleware(clientLimiter fiber.Handler, cfg config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if claims := delegatedClaimsFromRequest(c, cfg); claims != nil {
			c.Locals(externalClientIDLocalKey, claims.ClientID)
		}

		return clientLimiter(c)
	}
}

func externalClientRateLimitKey(c *fiber.Ctx) string {
	if clientID, _ := c.Locals(externalClientIDLocalKey).(string); clientID != "" {
		return clientID
	}

	return c.IP()
}

func externalUserRateLimitKey(c *fiber.Ctx) string {
	profile := httpidentity.GetProfile(c)
	if profile != nil && profile.ID != "" {
		return profile.ID
	}

	return c.IP()
}

func delegatedClaimsFromRequest(c *fiber.Ctx, cfg config.Config) *security.DelegatedClaims {
	authorization := strings.Fields(c.Get(fiber.HeaderAuthorization))
	if len(authorization) != 2 || !strings.EqualFold(authorization[0], "Bearer") {
		return nil
	}

	claims, err := security.ParseDelegatedToken(
		authorization[1],
		cfg.GetJWT().AccessTokenSecret,
		cfg.GetServer().Name,
	)
	if err != nil {
		return nil
	}

	return claims
}
