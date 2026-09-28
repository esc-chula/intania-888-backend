package color

import (
	"github.com/google/uuid"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
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
// @Param type_id query string false "Type ID to filter"
// @Success 200 {array} model.ColorDto
// @Failure 400 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /colors/leaderboards [get]
// @Security BearerAuth
func (h *ColorHttpHandler) GetAllLeaderboards(c *fiber.Ctx) error {
	typeId := c.Query("type_id", "")
	if err := validateOptionalID("type_id", typeId); err != nil {
		return err
	}

	colors, err := h.service.GetAllLeaderboards(typeId)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to get leaderboards")
	}

	return c.Status(fiber.StatusOK).JSON(colors)
}

// @Summary Get group stage table
// @Description Get group stage table with group id and sport type
// @Tags Color
// @Accept json
// @Produce json
// @Param type_id query string false "Type ID to filter"
// @Param group_id query string false "Group ID to filter"
// @Success 200 {array} model.ColorDto
// @Failure 400 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /colors/group-stage [get]
// @Security BearerAuth
func (h *ColorHttpHandler) GetGroupStageTable(c *fiber.Ctx) error {
	typeId := c.Query("type_id", "")
	groupId := c.Query("group_id", "")
	if err := validateOptionalID("type_id", typeId); err != nil {
		return err
	}
	if err := validateOptionalID("group_id", groupId); err != nil {
		return err
	}

	colors, err := h.service.GetGroupStageTable(typeId, groupId)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to get group stage table")
	}

	return c.Status(fiber.StatusOK).JSON(colors)
}

func validateOptionalID(field, value string) error {
	if value == "" {
		return nil
	}
	if _, err := uuid.Parse(value); err != nil {
		return apierror.Invalid(map[string]string{field: "must be a valid ID"})
	}
	return nil
}
