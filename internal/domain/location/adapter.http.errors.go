package location

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

func mapLocationError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidLocation):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid location")
	case errors.Is(err, ErrLocationNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Location not found")
	case errors.Is(err, ErrLocationConflict):
		return apierror.Wrap(err, fiber.StatusConflict, "CONFLICT", "Location ID already exists")
	case errors.Is(err, ErrLocationInUse):
		return apierror.Wrap(err, fiber.StatusConflict, "LOCATION_IN_USE", "Location is assigned to one or more matches")
	default:
		return err
	}
}
