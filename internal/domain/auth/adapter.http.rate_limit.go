package auth

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/httplimit"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

// RateLimiters keeps initiation, callback, exchange and revocation allowances independent.
type RateLimiters struct {
	Login         fiber.Handler
	Callback      fiber.Handler
	Token         httplimit.Check
	Revoke        httplimit.Check
	InvalidClient httplimit.Check
}

// NewRateLimiters constructs the authentication policies from the startup settings.
func NewRateLimiters(policies config.RateLimits, logger *zap.Logger) RateLimiters {
	return RateLimiters{
		Login:         httplimit.New("login", policies.Login, logger).Middleware(httplimit.ClientIP),
		Callback:      httplimit.New("callback", policies.Callback, logger).Middleware(httplimit.ClientIP),
		Token:         httplimit.New("token", policies.Token, logger).Check,
		Revoke:        httplimit.New("revoke", policies.Revoke, logger).Check,
		InvalidClient: httplimit.New("invalid_client", policies.InvalidClient, logger).Check,
	}
}

func authRateMiddleware(handler fiber.Handler) fiber.Handler {
	if handler == nil {
		return func(c *fiber.Ctx) error {
			return c.Next()
		}
	}

	return handler
}
