package middleware

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

// ExternalAPIMiddleware is a middleware for external API endpoints that bypasses browser-only validation
// but keeps JWT authentication, user retrieval, and blacklist enforcement.
func (h *HTTPHandler) ExternalAPIMiddleware(c *fiber.Ctx) error {
	token, ok := parseBearerToken(c.Get(fiber.HeaderAuthorization))
	if !ok {
		return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
	}

	// Each external endpoint requires an explicitly supported delegated scope.
	scope := ""
	if c.Method() == fiber.MethodGet && c.Path() == "/api/v1/external/me" {
		scope = config.ScopeProfileRead
	}
	if c.Method() == fiber.MethodPost && c.Path() == "/api/v1/external/deduct-coin" {
		scope = config.ScopeCoinsSpend
	}
	id, err := h.service.VerifyScopedExternalToken(c.UserContext(), token, scope)

	if err != nil {
		if errors.Is(err, ErrExternalScope) {
			return apierror.New(fiber.StatusForbidden, apierror.CodeForbidden, "Required scope is missing")
		}
		if errors.Is(err, ErrExternalMissing) {
			return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
		}
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "Token service is unavailable")
	}

	// Get the user profile.
	user, err := h.service.GetMe(c.UserContext(), id)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
		}
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "User status is unavailable")
	}

	// Check blacklist (MUST enforce for security).
	blacklisted, err := h.service.IsBlacklisted(c.UserContext(), user.Email, user.ID)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "Access policy is unavailable")
	}
	if blacklisted {
		return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
	}

	// Store user in context for downstream handlers.
	httpidentity.SetProfile(c, user)

	return c.Next()
}
