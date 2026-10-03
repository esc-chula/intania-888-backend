package server

import (
	"time"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

const globalRequestsPerMinute = 200

func registerRateLimiters(router fiber.Router) {
	router.Use(limiter.New(limiter.Config{
		Max:        globalRequestsPerMinute,
		Expiration: time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return apierror.New(fiber.StatusTooManyRequests, apierror.CodeTooManyRequests, "Too many requests")
		},
	}))
}
