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
		return apierror.Wrap(err, fiber.StatusBadRequest, apierror.CodeInvalidRequest, "Invalid or expired OAuth state")
	case errors.Is(err, ErrUnverifiedEmail), errors.Is(err, ErrEmailNotAllowed):
		return apierror.Wrap(err, fiber.StatusForbidden, apierror.CodeForbidden, "Email is not allowed")
	case errors.Is(err, security.ErrPolicyUnavailable):
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "Access policy is unavailable")
	default:
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "OAuth login is unavailable")
	}
}
