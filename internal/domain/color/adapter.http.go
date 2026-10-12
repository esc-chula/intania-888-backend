package color

import (
	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/catalogueid"

	"github.com/gofiber/fiber/v2"
)

// HTTPHandler exposes color, group-stage, and team ranking queries.
type HTTPHandler struct {
	service ServicePort
}

// NewHTTPHandler constructs the leaderboard HTTP adapter.
func NewHTTPHandler(service ServicePort) *HTTPHandler {
	return &HTTPHandler{service: service}
}

// RegisterRoutes exposes the leaderboard, ranking, and group-stage read routes.
func (h *HTTPHandler) RegisterRoutes(router fiber.Router) {
	router = router.Group("/colors")

	router.Get("/leaderboards", h.GetAllLeaderboards)
	router.Get("/leaderboards/coins", h.GetCoinRanking)
	router.Get("/leaderboards/predictions", h.GetPredictionRanking)
	router.Get("/group-stage", h.GetGroupStageTable)
}

// GetAllLeaderboards validates the optional sport identifier and returns the color standings.
func (h *HTTPHandler) GetAllLeaderboards(c *fiber.Ctx) error {
	typeID := c.Query("type_id", "")
	if err := validateOptionalID("type_id", typeID, c.Context().QueryArgs().Has("type_id")); err != nil {
		return err
	}

	colors, err := h.service.GetAllLeaderboards(c.UserContext(), typeID)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to get leaderboards")
	}

	return c.Status(fiber.StatusOK).JSON(responses(colors))
}

// GetCoinRanking returns colors ordered by team coins, highest first.
func (h *HTTPHandler) GetCoinRanking(c *fiber.Ctx) error {
	rows, err := h.service.GetCoinRanking(c.UserContext())
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to get color coin ranking")
	}

	return c.Status(fiber.StatusOK).JSON(coinRankResponses(rows))
}

// GetPredictionRanking returns colors ordered by the correct bets of their members, highest first.
func (h *HTTPHandler) GetPredictionRanking(c *fiber.Ctx) error {
	rows, err := h.service.GetPredictionRanking(c.UserContext())
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to get color prediction ranking")
	}

	return c.Status(fiber.StatusOK).JSON(predictionRankResponses(rows))
}

// GetGroupStageTable validates optional sport and group identifiers and returns group-stage standings.
func (h *HTTPHandler) GetGroupStageTable(c *fiber.Ctx) error {
	typeID := c.Query("type_id", "")
	groupID := c.Query("group_id", "")
	if err := validateOptionalID("type_id", typeID, c.Context().QueryArgs().Has("type_id")); err != nil {
		return err
	}
	if err := validateOptionalID("group_id", groupID, c.Context().QueryArgs().Has("group_id")); err != nil {
		return err
	}

	colors, err := h.service.GetGroupStageTable(c.UserContext(), typeID, groupID)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to get group stage table")
	}

	return c.Status(fiber.StatusOK).JSON(responses(colors))
}

func validateOptionalID(field, value string, supplied bool) error {
	if !supplied {
		return nil
	}
	if !catalogueid.Valid(value) {
		return apierror.Invalid(map[string]string{field: "must contain 1–100 ASCII letters, digits, underscores, or hyphens"})
	}

	return nil
}
