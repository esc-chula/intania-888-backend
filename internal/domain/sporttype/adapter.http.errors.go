package sporttype

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

func mapSportTypeError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidSportType):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid sport type")
	case errors.Is(err, ErrSportTypeNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Sport type not found")
	case errors.Is(err, ErrSportTypeConflict):
		return apierror.Wrap(err, fiber.StatusConflict, "CONFLICT", "Sport type ID already exists")
	case errors.Is(err, ErrSportTypeInUse):
		return apierror.Wrap(
			err,
			fiber.StatusConflict,
			"SPORT_TYPE_IN_USE",
			"Sport type is referenced by matches, tournament groups, or stages",
		)
	default:
		return err
	}
}
