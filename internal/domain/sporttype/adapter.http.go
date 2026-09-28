package sporttype

import (
	"github.com/esc-chula/intania-888-backend/internal/apierror"

	"github.com/gofiber/fiber/v2"
)

// HTTPHandler exposes the authenticated sport catalogue route.
type HTTPHandler struct {
	service ServicePort
}

// NewHTTPHandler constructs the sport catalogue HTTP adapter.
func NewHTTPHandler(service ServicePort) *HTTPHandler {
	return &HTTPHandler{service: service}
}

// RegisterRoutes registers the catalogue read route behind the supplied authentication middleware.
func (h *HTTPHandler) RegisterRoutes(router fiber.Router, authenticate fiber.Handler) {
	router = router.Group("/sport-types", authenticate)

	router.Get("/", h.GetAllSportTypes)
}

// GetAllSportTypes returns the complete sport catalogue using its public HTTP representation.
// @Summary Get all sport types
// @Description Get all sport types available in the system
// @Tags SportType
// @Accept json
// @Produce json
// @Success 200 {object} []Response
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
