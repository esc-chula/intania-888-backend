package color

import (
	"github.com/google/uuid"

	"github.com/esc-chula/intania-888-backend/internal/apierror"

	"github.com/gofiber/fiber/v2"
)

// HTTPHandler exposes authenticated color and group-stage leaderboard queries.
type HTTPHandler struct {
	service ServicePort
}

// NewHTTPHandler constructs the leaderboard HTTP adapter.
func NewHTTPHandler(service ServicePort) *HTTPHandler {
	return &HTTPHandler{service: service}
}

// RegisterRoutes registers authenticated leaderboard and group-stage read routes.
func (h *HTTPHandler) RegisterRoutes(router fiber.Router, authenticate fiber.Handler) {
	router = router.Group("/colors", authenticate)

	router.Get("/leaderboards", h.GetAllLeaderboards)
	router.Get("/group-stage", h.GetGroupStageTable)
}

// GetAllLeaderboards validates the optional sport identifier and returns the color standings.
func (h *HTTPHandler) GetAllLeaderboards(c *fiber.Ctx) error {
	typeID := c.Query("type_id", "")
	if err := validateOptionalID("type_id", typeID); err != nil {
		return err
	}

	colors, err := h.service.GetAllLeaderboards(c.UserContext(), typeID)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to get leaderboards")
	}

	return c.Status(fiber.StatusOK).JSON(responses(colors))
}

// GetGroupStageTable validates optional sport and group identifiers and returns group-stage standings.
func (h *HTTPHandler) GetGroupStageTable(c *fiber.Ctx) error {
	typeID := c.Query("type_id", "")
	groupID := c.Query("group_id", "")
	if err := validateOptionalID("type_id", typeID); err != nil {
		return err
	}
	if err := validateOptionalID("group_id", groupID); err != nil {
		return err
	}

	colors, err := h.service.GetGroupStageTable(c.UserContext(), typeID, groupID)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to get group stage table")
	}

	return c.Status(fiber.StatusOK).JSON(responses(colors))
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
