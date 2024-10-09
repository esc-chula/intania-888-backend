package color

import (
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/gofiber/fiber/v2"
)

type ColorHttpHandler struct {
	service ColorService
}

func NewColorHttpHandler(service ColorService) *ColorHttpHandler {
	return &ColorHttpHandler{service: service}
}

func (h *ColorHttpHandler) RegisterRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router = router.Group("/colors", mid.AuthMiddleware)

	router.Get("/leaderboards", h.GetAllLeaderboards)
	router.Get("/group-stage", h.GetGroupStageTable)
}

// @Summary Get all color leaderboards
// @Description Get all colors with their leaderboard info
// @Tags Color
// @Accept json
// @Produce json
// @Param typeId query string false "Type ID to filter"
// @Success 200 {object} []model.ColorDto
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /colors/leaderboards [get]
func (h *ColorHttpHandler) GetAllLeaderboards(c *fiber.Ctx) error {
	typeId := c.Query("type_id", "")

	colors, err := h.service.GetAllLeaderboards(typeId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Message: "Failed to get leaderboards"})
	}

	return c.Status(fiber.StatusOK).JSON(colors)
}

// @Summary Get group stage table
// @Description Get group stage table with group id and sport type
// @Tags Color
// @Accept json
// @Produce json
// @Param typeId query string false "Type ID to filter"
// @Param groupId query string false "Group ID to filter"
// @Success 200 {object} []model.ColorDto
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /colors/group-stage [get]
func (h *ColorHttpHandler) GetGroupStageTable(c *fiber.Ctx) error {
	typeId := c.Query("type_id", "")
	groupId := c.Query("group_id", "")

	colors, err := h.service.GetGroupStageTable(typeId, groupId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Message: "Failed to get group stage table"})
	}

	return c.Status(fiber.StatusOK).JSON(colors)
}

type ErrorResponse struct {
	Message string `json:"message"`
}
