package middleware

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ExternalAPIMiddleware is a middleware for external API endpoints that bypasses browser-only validation
// but keeps JWT authentication, user retrieval, and blacklist enforcement.
func (h *MiddlewareHttpHandler) ExternalAPIMiddleware(c *fiber.Ctx) error {
	token, ok := parseBearerToken(c.Get("Authorization"))
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authorization header"})
	}

	// Verify the token and its revocation status.
	id, err := h.service.VerifyExternalToken(token)
	if err != nil {
		if errors.Is(err, ErrExternalMissing) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "token status unavailable"})
	}

	// Get the user profile.
	user, err := h.service.GetMe(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "user status unavailable"})
	}

	// Check blacklist (MUST enforce for security).
	blacklisted, err := h.service.IsBlacklisted(user.Email, user.Id)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "access policy unavailable"})
	}
	if blacklisted {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	// Store user in context for downstream handlers.
	c.Locals("user", user)

	return c.Next()
}
