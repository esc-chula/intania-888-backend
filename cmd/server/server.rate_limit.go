package server

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/httplimit"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

// ExternalRateLimiters holds checks applied at signed-client and active-grant boundaries.
type ExternalRateLimiters struct {
	Client      httplimit.Check
	Invalid     httplimit.Check
	Profile     httplimit.Check
	Transaction httplimit.Check
}

// NewExternalRateLimiters shares client quotas across endpoints and user quotas across clients.
func NewExternalRateLimiters(cfg config.Config, logger *zap.Logger) ExternalRateLimiters {
	policies := cfg.GetRateLimits()

	return ExternalRateLimiters{
		Client:      httplimit.New("external_client", policies.ExternalClient, logger).Check,
		Invalid:     httplimit.New("invalid_external", policies.InvalidExternal, logger).Check,
		Profile:     httplimit.New("external_profile", policies.ExternalProfile, logger).Check,
		Transaction: httplimit.New("external_transaction", policies.ExternalTransaction, logger).Check,
	}
}

func registerRateLimiters(router fiber.Router, cfg config.Config, logger *zap.Logger) {
	router.Use(httplimit.ResolveClientIP(cfg.GetServer().ClientIPMode))
	shared := httplimit.New("shared", cfg.GetRateLimits().Shared, logger)
	router.Use(shared.Middleware(httplimit.ClientIP))
}
