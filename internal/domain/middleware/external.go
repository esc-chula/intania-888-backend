package middleware

import (
	"errors"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ExternalAPIMiddleware is a middleware for external API endpoints that bypasses browser-only validation
// but keeps JWT authentication, user retrieval, and blacklist enforcement.
func (h *MiddlewareHttpHandler) ExternalAPIMiddleware(c *fiber.Ctx) error {
	token, ok := parseBearerToken(c.Get("Authorization"))
	if !ok {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	// Verify the token and its revocation status.
	id, err := h.service.VerifyExternalToken(token)
	if err != nil {
		if errors.Is(err, ErrExternalMissing) {
			return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		}
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Token service is unavailable")
	}

	// Get the user profile.
	user, err := h.service.GetMe(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		}
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "User status is unavailable")
	}

	// Check blacklist (MUST enforce for security).
	blacklisted, err := h.service.IsBlacklisted(user.Email, user.Id)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Access policy is unavailable")
	}
	if blacklisted {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	// Store user in context for downstream handlers.
	c.Locals("user", user)

	return c.Next()
}
