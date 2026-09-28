package stakemine

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

func mapStakeMineError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidGameRequest):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid Stake Mines request")
	case errors.Is(err, ErrInsufficientBalance):
		return apierror.Wrap(err, fiber.StatusUnprocessableEntity, "INSUFFICIENT_BALANCE", "Insufficient balance")
	case errors.Is(err, ErrGameNotFound), errors.Is(err, ErrNoActiveGame), errors.Is(err, ErrUserNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Stake Mines resource not found")
	case errors.Is(err, ErrGameForbidden):
		return apierror.Wrap(err, fiber.StatusForbidden, "FORBIDDEN", "Game access denied")
	case errors.Is(err, ErrGameConflict):
		return apierror.Wrap(err, fiber.StatusConflict, "GAME_STATE_CONFLICT", "Game state does not allow this action")
	default:
		return err
	}
}
