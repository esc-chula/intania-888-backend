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
	case errors.Is(err, ErrInvalidProfileRegistration):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid profile registration")
	case errors.Is(err, ErrProfileGroupNotFound):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Profile group does not exist")
	case errors.Is(err, ErrGroupAlreadyAssigned):
		return apierror.Wrap(err, fiber.StatusConflict, "GROUP_ALREADY_ASSIGNED", "A group is already assigned to this user")
	case errors.Is(err, ErrGroupRegistrationConflict):
		return apierror.Wrap(err, fiber.StatusConflict, "GROUP_REGISTRATION_CONFLICT", "Profile changed during group registration")
	case errors.Is(err, ErrUserNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "User not found")
	case errors.Is(err, ErrInsufficientBalance):
		return apierror.Wrap(err, fiber.StatusUnprocessableEntity, "INSUFFICIENT_BALANCE", "Insufficient balance")
	default:
		return err
	}
}
