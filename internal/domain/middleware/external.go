package middleware

import (
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// ExternalAPIMiddleware is a middleware for external API endpoints that bypasses browser-only validation
// but keeps JWT authentication, user retrieval, and blacklist enforcement
func (h *MiddlewareHttpHandler) ExternalAPIMiddleware(c *fiber.Ctx) error {
	token, ok := parseBearerToken(c.Get("Authorization"))
	if !ok {
		h.log.Named("ExternalAPIMiddleware").Error("Missing authorization header")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing authorization header",
		})
	}

	// Verify the token
	claims, err := h.service.VerifyToken(token)
	if err != nil {
		h.log.Named("ExternalAPIMiddleware").Error("Token verification failed", zap.Error(err))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid or expired token",
		})
	}

	// Get the user profile
	userDto, err := h.service.GetMe(claims.UserId)
	if err != nil {
		h.log.Named("ExternalAPIMiddleware").Error("User not found", zap.Error(err))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	// Check blacklist (MUST enforce for security)
	if security.IsBlacklisted(userDto.Email, userDto.Id) {
		h.log.Named("ExternalAPIMiddleware").Warn("Blacklisted user attempted external API access",
			zap.String("endpoint", c.Path()))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// Store user in context for downstream handlers
	c.Locals("user", userDto)
	c.Locals("session_id", claims.SessionId)

	return c.Next()
}
