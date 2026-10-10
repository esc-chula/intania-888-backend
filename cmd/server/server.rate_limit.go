package server

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/httplimit"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

// ExternalRateLimiters holds checks applied at signed-client and active-grant boundaries.
type ExternalRateLimiters struct {
	Client      fiber.Handler
	Invalid     httplimit.Check
	Profile     httplimit.Check
	Transaction httplimit.Check
}

// NewExternalRateLimiters shares client quotas across endpoints and user quotas across clients.
func NewExternalRateLimiters(cfg config.Config, logger *zap.Logger) ExternalRateLimiters {
	policies := cfg.GetRateLimits()
	client := httplimit.New("external_client", policies.ExternalClient, logger)
	invalid := httplimit.New("invalid_external", policies.InvalidExternal, logger)
	profile := httplimit.New("external_profile", policies.ExternalProfile, logger)
	transaction := httplimit.New("external_transaction", policies.ExternalTransaction, logger)
	return ExternalRateLimiters{
		Client: func(c *fiber.Ctx) error {
			authorization := strings.Fields(c.Get(fiber.HeaderAuthorization))
			if len(authorization) != 2 || !strings.EqualFold(authorization[0], "Bearer") {
				return invalidExternalRequest(c, invalid.Check)
			}
			ctx, claims, err := security.ContextWithDelegatedToken(
				c.UserContext(), authorization[1], cfg.GetJWT().AccessTokenSecret, cfg.GetServer().Name,
			)
			if err != nil {
				return invalidExternalRequest(c, invalid.Check)
			}
			c.SetUserContext(ctx)
			if err := client.Check(c, claims.ClientID); err != nil {
				return err
			}
			return c.Next()
		},
		Invalid:     invalid.Check,
		Profile:     profile.Check,
		Transaction: transaction.Check,
	}
}

func invalidExternalRequest(c *fiber.Ctx, check httplimit.Check) error {
	if err := check(c, httplimit.ClientIP(c)); err != nil {
		return err
	}
	return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
}

func registerRateLimiters(router fiber.Router, cfg config.Config, logger *zap.Logger) {
	router.Use(httplimit.ResolveClientIP(cfg.GetServer().ClientIPMode))
	shared := httplimit.New("shared", cfg.GetRateLimits().Shared, logger)
	router.Use(shared.Middleware(httplimit.ClientIP))
}
