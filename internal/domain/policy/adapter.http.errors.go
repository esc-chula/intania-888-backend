package policy

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/security"
)

func mapPolicyError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidPolicy):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid access policy")
	case errors.Is(err, ErrPolicyNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Access policy not found")
	case errors.Is(err, ErrPolicyConflict):
		return apierror.Wrap(err, fiber.StatusConflict, "CONFLICT", "Access policy already exists")
	case errors.Is(err, security.ErrPolicyUnavailable):
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Access policy is unavailable")
	default:
		return err
	}
}
