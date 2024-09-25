package match

import (
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/gofiber/fiber/v2"
)

type MatchHttpHandler struct {
	matchService MatchService
}

func (h *MatchHttpHandler) RegisterRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router = router.Group("/matches", mid.AuthMiddleware)

	router.Post("/", h.CreateMatch)
	router.Get("/", h.GetAllMatches)
	router.Get("/:id", h.GetMatch)
	router.Patch("/:id/winner/:winner_id", h.UpdateMatchWinner)
	router.Patch("/:id/score", h.UpdateMatchScore)
	router.Delete("/:id", h.DeleteMatch)
}

func NewMatchHttpHandler(matchService MatchService) *MatchHttpHandler {
	return &MatchHttpHandler{matchService}
}

func (h *MatchHttpHandler) CreateMatch(c *fiber.Ctx) error {
	matchDto := new(model.MatchDto)

	if err := c.BodyParser(&matchDto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request payload"})
	}

	err := h.matchService.CreateMatch(matchDto)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create match"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "Created match successful"})
}

func (h *MatchHttpHandler) GetMatch(c *fiber.Ctx) error {
	match, err := h.matchService.GetMatch(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Match not found"})
	}

	return c.Status(fiber.StatusOK).JSON(match)
}

func (h *MatchHttpHandler) GetAllMatches(c *fiber.Ctx) error {
	filter := &model.MatchFilter{}

	if typeId := c.Query("typeId"); typeId != "" {
		filter.TypeId = typeId
	}

	if schedule := c.Query("schedule"); schedule != "" {
		switch schedule {
		case "schedule":
			filter.Schedule = model.Schedule
		case "result":
			filter.Schedule = model.Result
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid schedule parameter"})
		}
	}

	matches, err := h.matchService.GetAllMatches(filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch matches"})
	}

	// Group matches by date and sport type
	groupedMatches := groupMatchesByDateAndType(matches)

	return c.Status(fiber.StatusOK).JSON(groupedMatches)
}

func (h *MatchHttpHandler) UpdateMatchScore(c *fiber.Ctx) error {
	matchId := c.Params("id")
	scoreDto := new(model.ScoreDto)

	if err := c.BodyParser(&scoreDto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request payload"})
	}

	err := h.matchService.UpdateMatchScore(matchId, scoreDto)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update match score"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Updated match score successfully"})
}

func (h *MatchHttpHandler) UpdateMatchWinner(c *fiber.Ctx) error {
	matchId := c.Params("id")
	winnerId := c.Params("winner_id")

	err := h.matchService.UpdateMatchWinner(matchId, winnerId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update match winner"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Updated match winner successfully"})
}

func (h *MatchHttpHandler) DeleteMatch(c *fiber.Ctx) error {
	err := h.matchService.DeleteMatch(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete match"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Deleted match successful"})
}
