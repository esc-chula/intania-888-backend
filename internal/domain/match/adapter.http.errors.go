package match

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

func mapMatchError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidMatch), errors.Is(err, ErrInvalidScore):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid match request")
	case errors.Is(err, ErrInvalidLocation):
		return apierror.Invalid(map[string]string{"location_id": "must reference an existing location"})
	case errors.Is(err, ErrInvalidResult):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid match result")
	case errors.Is(err, ErrResultConflict):
		return apierror.Wrap(err, fiber.StatusConflict, "MATCH_RESULT_CONFLICT", "Match result conflicts with existing state")
	case errors.Is(err, ErrNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Match not found")
	default:
		return err
	}
}
