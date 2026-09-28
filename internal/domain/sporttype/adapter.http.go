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
// @Summary Get all sport types
// @Description Get all sport types available in the system
// @Tags SportType
// @Accept json
// @Produce json
// @Success 200 {object} []Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 503 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /sport-types [get]
// @Security CookieSession
func (h *HTTPHandler) GetAllSportTypes(c *fiber.Ctx) error {
	sportTypes, err := h.service.GetAllSportTypes(c.UserContext())
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to get sport types")
	}

	return c.Status(fiber.StatusOK).JSON(responses(sportTypes))
}

// GetSportType returns one sport catalogue entry.
// @Summary Get a sport type
// @Tags SportType
// @Produce json
// @Param id path string true "Sport type ID"
// @Success 200 {object} Response
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Failure 503 {object} apierror.Response
// @Router /sport-types/{id} [get]
// @Security CookieSession
func (h *HTTPHandler) GetSportType(c *fiber.Ctx) error {
	row, err := h.service.GetSportType(c.UserContext(), c.Params("id"))
	if err != nil {
		return mapSportTypeError(err)
	}
	return c.JSON(response(row))
}

// CreateSportType creates an administrator-managed catalogue entry.
// @Summary Create a sport type (admin)
// @Description Requires administrator permission, an allowed Origin, and session CSRF. IDs use 1–100 ASCII letters, digits, underscores, or hyphens. Trimmed titles use 1–100 characters; duplicate titles are allowed.
// @Tags SportType
// @Accept json
// @Produce json
// @Param X-CSRF-Token header string true "Session CSRF token"
// @Param body body CreateRequest true "Sport type"
// @Success 201 {object} Response
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Failure 503 {object} apierror.Response
// @Router /sport-types/admin [post]
// @Security CookieSession
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
// @Summary Rename a sport type (admin)
// @Description Requires administrator permission, an allowed Origin, and session CSRF. Only title is accepted; trim it to 1–100 characters. Duplicate titles are allowed.
// @Tags SportType
// @Accept json
// @Produce json
// @Param id path string true "Immutable sport type ID"
// @Param X-CSRF-Token header string true "Session CSRF token"
// @Param body body UpdateRequest true "New title"
// @Success 200 {object} Response
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Failure 503 {object} apierror.Response
// @Router /sport-types/admin/{id} [patch]
// @Security CookieSession
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
// @Summary Delete an unused sport type (admin)
// @Description Requires administrator permission, an allowed Origin, and session CSRF. Referenced entries return 409 SPORT_TYPE_IN_USE; missing entries return 404 RESOURCE_NOT_FOUND.
// @Tags SportType
// @Param id path string true "Sport type ID"
// @Param X-CSRF-Token header string true "Session CSRF token"
// @Success 204 "Deleted"
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Failure 503 {object} apierror.Response
// @Router /sport-types/admin/{id} [delete]
// @Security CookieSession
func (h *HTTPHandler) DeleteSportType(c *fiber.Ctx) error {
	if err := h.service.DeleteSportType(c.UserContext(), c.Params("id")); err != nil {
		return mapSportTypeError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
