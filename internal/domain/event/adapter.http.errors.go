package event

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

func mapEventError(err error) error {
	switch {
	case errors.Is(err, ErrDailyRewardAlreadyClaimed):
		return apierror.Wrap(err, fiber.StatusConflict, "DAILY_REWARD_ALREADY_CLAIMED", "Daily reward already claimed")
	case errors.Is(err, ErrInsufficientBalance):
		return apierror.Wrap(err, fiber.StatusUnprocessableEntity, "INSUFFICIENT_BALANCE", "Insufficient balance")
	case errors.Is(err, ErrStealTokenConflict):
		return apierror.Wrap(err, fiber.StatusConflict, "STEAL_TOKEN_CONFLICT", "Steal token has already been used")
	case errors.Is(err, ErrStealTokenInvalid):
		return apierror.Wrap(err, fiber.StatusConflict, "STEAL_TOKEN_INVALID", "Steal token is invalid or expired")
	case errors.Is(err, ErrStealTokenForbidden):
		return apierror.Wrap(err, fiber.StatusForbidden, "FORBIDDEN", "Steal token cannot be used by this account")
	case errors.Is(err, ErrInvalidStealRequest):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid victim selection")
	case errors.Is(err, ErrUserNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Requested resource not found")
	default:
		return err
	}
}
