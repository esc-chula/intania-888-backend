package middleware

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
	"github.com/esc-chula/intania-888-backend/internal/httplimit"
	"github.com/esc-chula/intania-888-backend/internal/identity"
)

// RequireExternalScope authenticates a delegated bearer credential for the supplied scope.
// Resource permissions are declared at registration, independent of paths and HTTP methods.
func (h *HTTPHandler) RequireExternalScope(scope string, limits ...httplimit.Check) fiber.Handler {
	if scope == "" {
		panic("external route scope must not be empty")
	}

	return func(c *fiber.Ctx) error {
		token, ok := parseBearerToken(c.Get(fiber.HeaderAuthorization))
		if !ok {
			return h.rejectExternalCredential(c)
		}

		id, err := h.service.VerifyExternalGrant(c.UserContext(), token, scope)

		if err != nil {
			if errors.Is(err, ErrExternalScope) {
				return apierror.New(fiber.StatusForbidden, apierror.CodeForbidden, "Required scope is missing")
			}
			if errors.Is(err, ErrExternalMissing) {
				return h.rejectExternalCredential(c)
			}

			return apierror.Wrap(err, fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "Token service is unavailable")
		}

		// Only an active, correctly scoped grant can consume the account budget.
		for _, limit := range limits {
			if limit != nil {
				if err := limit(c, id); err != nil {
					return err
				}
			}
		}

		user, err := h.service.GetExternalProfile(c.UserContext(), id)
		if err != nil {
			if errors.Is(err, identity.ErrUserNotFound) || errors.Is(err, ErrExternalMissing) {
				return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
			}

			return apierror.Wrap(err, fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "User status is unavailable")
		}

		if user == nil {
			return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
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
}

func (h *HTTPHandler) rejectExternalCredential(c *fiber.Ctx) error {
	if h.invalidExternalLimit != nil {
		if err := h.invalidExternalLimit(c, httplimit.ClientIP(c)); err != nil {
			return err
		}
	}
	return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
}
