package location

import (
	"github.com/esc-chula/intania-888-backend/internal/apierror"

	"github.com/gofiber/fiber/v2"
)

// HTTPHandler exposes authenticated catalogue reads and administrator mutations.
type HTTPHandler struct {
	service ServicePort
}

// NewHTTPHandler constructs the location HTTP adapter.
func NewHTTPHandler(service ServicePort) *HTTPHandler {
	return &HTTPHandler{service: service}
}

// RegisterRoutes authenticates catalogue reads and protects admin mutations.
func (h *HTTPHandler) RegisterRoutes(router fiber.Router, authenticate, admin fiber.Handler) {
	router = router.Group("/locations", authenticate)
	router.Get("/", h.GetAllLocations)
	router.Get("/:id", h.GetLocation)
	router.Post("/admin", admin, h.CreateLocation)
	router.Patch("/admin/:id", admin, h.UpdateLocation)
	router.Delete("/admin/:id", admin, h.DeleteLocation)
}

// GetAllLocations returns the configured venue catalogue.
func (h *HTTPHandler) GetAllLocations(c *fiber.Ctx) error {
	rows, err := h.service.GetAllLocations(c.UserContext())
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to get locations")
	}

	return c.JSON(responses(rows))
}

// GetLocation returns one venue by ID.
func (h *HTTPHandler) GetLocation(c *fiber.Ctx) error {
	row, err := h.service.GetLocation(c.UserContext(), c.Params("id"))
	if err != nil {
		return mapLocationError(err)
	}

	return c.JSON(response(row))
}

// CreateLocation creates a venue managed by administrators.
func (h *HTTPHandler) CreateLocation(c *fiber.Ctx) error {
	var input CreateRequest
	if err := apierror.BindJSON(c, &input); err != nil {
		return err
	}

	row, err := h.service.CreateLocation(c.UserContext(), Location(input))
	if err != nil {
		return mapLocationError(err)
	}

	return c.Status(fiber.StatusCreated).JSON(response(row))
}

// UpdateLocation renames a venue without changing its ID.
func (h *HTTPHandler) UpdateLocation(c *fiber.Ctx) error {
	var input UpdateRequest
	if err := apierror.BindJSON(c, &input); err != nil {
		return err
	}

	row, err := h.service.UpdateLocation(c.UserContext(), c.Params("id"), input.Title)
	if err != nil {
		return mapLocationError(err)
	}

	return c.JSON(response(row))
}

// DeleteLocation removes an unused venue.
func (h *HTTPHandler) DeleteLocation(c *fiber.Ctx) error {
	if err := h.service.DeleteLocation(c.UserContext(), c.Params("id")); err != nil {
		return mapLocationError(err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}
