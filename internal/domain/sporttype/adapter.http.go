package sporttype

import (
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/gofiber/fiber/v2"
)

type SportTypeHttpHandler struct {
	service SportTypeService
}

func NewSportTypeHttpHandler(service SportTypeService) *SportTypeHttpHandler {
	return &SportTypeHttpHandler{service: service}
}

func (h *SportTypeHttpHandler) RegisterRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router = router.Group("/sport-types", mid.AuthMiddleware)

	router.Get("/", h.GetAllSportTypes)
}

// @Summary Get all sport types
// @Description Get all sport types available in the system
// @Tags SportType
// @Accept json
// @Produce json
// @Success 200 {object} []model.SportTypeDto
// @Failure 500 {object} ErrorResponse
// @Router /sport-types [get]
func (h *SportTypeHttpHandler) GetAllSportTypes(c *fiber.Ctx) error {
	sportTypes, err := h.service.GetAllSportTypes()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Message: "Failed to get sport types"})
	}

	return c.Status(fiber.StatusOK).JSON(sportTypes)
}

type ErrorResponse struct {
	Message string `json:"message"`
}
