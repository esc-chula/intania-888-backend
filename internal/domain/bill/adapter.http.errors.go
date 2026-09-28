package bill

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

func mapBillError(err error) error {
	switch {
	case errors.Is(err, ErrMatchNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Match not found")
	case errors.Is(err, ErrInvalidBill):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid bill request")
	case errors.Is(err, ErrInsufficientBalance):
		return apierror.Wrap(err, fiber.StatusUnprocessableEntity, "INSUFFICIENT_BALANCE", "Insufficient balance")
	case errors.Is(err, ErrBillConflict):
		return apierror.Wrap(err, fiber.StatusConflict, "BILL_CONFLICT", "Bill state conflicts with this operation")
	case errors.Is(err, ErrNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Bill not found")
	default:
		return err
	}
}
