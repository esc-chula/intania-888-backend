package auth

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/security"
)

func mapAuthError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidOAuthState):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid or expired OAuth state")
	case errors.Is(err, ErrUnverifiedEmail), errors.Is(err, ErrEmailNotAllowed):
		return apierror.Wrap(err, fiber.StatusForbidden, "FORBIDDEN", "Email is not allowed")
	case errors.Is(err, security.ErrPolicyUnavailable):
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Access policy is unavailable")
	default:
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "OAuth login is unavailable")
	}
}
