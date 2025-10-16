package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// ExternalAPIMiddleware is a middleware for external API endpoints that bypasses browser-only validation
// but keeps JWT authentication, user retrieval, and blacklist enforcement
func (h *MiddlewareHttpHandler) ExternalAPIMiddleware(c *fiber.Ctx) error {
	// Extract token from the header
	header := c.Get("Authorization")
	if header == "" {
		h.log.Named("ExternalAPIMiddleware").Error("Missing authorization header")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing authorization header",
		})
	}

	token := strings.Split(header, " ")
	if len(token) != 2 || token[0] != "Bearer" || token[1] == "" {
		h.log.Named("ExternalAPIMiddleware").Error("Invalid authorization header format")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid authorization header",
		})
	}

	// Verify the token
	userId, err := h.service.VerifyToken(token[1])
	if err != nil {
		h.log.Named("ExternalAPIMiddleware").Error("Token verification failed", zap.Error(err))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid or expired token",
		})
	}

	// Get the user profile
	userDto, err := h.service.GetMe(*userId)
	if err != nil {
		h.log.Named("ExternalAPIMiddleware").Error("User not found", zap.Error(err))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	// Check blacklist (MUST enforce for security)
	if isInBlacklists(userDto) {
		h.log.Named("ExternalAPIMiddleware").Warn("Blacklisted user attempted external API access",
			zap.String("userId", userDto.Id),
			zap.String("email", userDto.Email))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// Store user in context for downstream handlers
	c.Locals("user", userDto)

	return c.Next()
}
