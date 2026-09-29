package sporttype

import (
	"github.com/esc-chula/intania-888-backend/internal/apierror"

	"github.com/gofiber/fiber/v2"
)

// HTTPHandler exposes authenticated catalogue reads and administrator mutations.
type HTTPHandler struct {
	service ServicePort
}

// NewHTTPHandler constructs the sport catalogue HTTP adapter.
func NewHTTPHandler(service ServicePort) *HTTPHandler {
	return &HTTPHandler{service: service}
}

// RegisterRoutes authenticates catalogue reads and additionally protects admin mutations.
func (h *HTTPHandler) RegisterRoutes(router fiber.Router, authenticate, admin fiber.Handler) {
	router = router.Group("/sport-types", authenticate)

	router.Get("/", h.GetAllSportTypes)
	router.Post("/admin", admin, h.CreateSportType)
	router.Patch("/admin/:id", admin, h.UpdateSportType)
	router.Delete("/admin/:id", admin, h.DeleteSportType)
	router.Get("/:id", h.GetSportType)
}

// GetAllSportTypes returns the complete sport catalogue using its public HTTP representation.
func (h *HTTPHandler) GetAllSportTypes(c *fiber.Ctx) error {
	sportTypes, err := h.service.GetAllSportTypes(c.UserContext())
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to get sport types")
	}

	return c.Status(fiber.StatusOK).JSON(responses(sportTypes))
}

// GetSportType returns one sport catalogue entry.
func (h *HTTPHandler) GetSportType(c *fiber.Ctx) error {
	row, err := h.service.GetSportType(c.UserContext(), c.Params("id"))
	if err != nil {
		return mapSportTypeError(err)
	}
	return c.JSON(response(row))
}

// CreateSportType creates an administrator-managed catalogue entry.
func (h *HTTPHandler) CreateSportType(c *fiber.Ctx) error {
	var input CreateRequest
	if err := apierror.BindJSON(c, &input); err != nil {
		return err
	}

	row, err := h.service.CreateSportType(c.UserContext(), SportType(input))
	if err != nil {
		return mapSportTypeError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(response(row))
}

// UpdateSportType renames an administrator-managed entry without changing its ID.
func (h *HTTPHandler) UpdateSportType(c *fiber.Ctx) error {
	var input UpdateRequest
	if err := apierror.BindJSON(c, &input); err != nil {
		return err
	}

	row, err := h.service.UpdateSportType(c.UserContext(), c.Params("id"), input.Title)
	if err != nil {
		return mapSportTypeError(err)
	}
	return c.JSON(response(row))
}

// DeleteSportType removes an unused entry while preserving referenced records.
func (h *HTTPHandler) DeleteSportType(c *fiber.Ctx) error {
	if err := h.service.DeleteSportType(c.UserContext(), c.Params("id")); err != nil {
		return mapSportTypeError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
