package user

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

func mapUserError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidProfileUpdate):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid profile update")
	case errors.Is(err, ErrProfileGroupNotFound):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Profile group does not exist")
	case errors.Is(err, ErrUserNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "User not found")
	case errors.Is(err, ErrInsufficientBalance):
		return apierror.Wrap(err, fiber.StatusUnprocessableEntity, "INSUFFICIENT_BALANCE", "Insufficient balance")
	default:
		return err
	}
}
