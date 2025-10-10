package match

import (
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/gofiber/fiber/v2"
)

type MatchHttpHandler struct {
	matchService MatchService
}

func NewMatchHttpHandler(matchService MatchService) *MatchHttpHandler {
	return &MatchHttpHandler{matchService}
}

func (h *MatchHttpHandler) RegisterRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router = router.Group("/matches", mid.AuthMiddleware)

	router.Get("/", h.GetAllMatches)
	router.Get("/:id", h.GetMatch)
	router.Get("/current/time", h.GetTime)

	adminRouter := router.Group("", mid.AdminMiddleware)
	adminRouter.Post("/", h.CreateMatch)
	adminRouter.Patch("/:id/winner/:winner_id", h.UpdateMatchWinner)
	adminRouter.Patch("/:id/score", h.UpdateMatchScore)
	adminRouter.Patch("/:id/draw", h.UpdateMatchDraw)
	adminRouter.Delete("/:id", h.DeleteMatch)
}

// CreateMatch @Summary      Create a new match
// @Summary      Creates a new match
// @Description  Creates a new match and stores it in the system
// @Tags         Match
// @Accept       json
// @Produce      json
// @Param        match  body      model.MatchDto  true  "Match information"
// @Success      201    {object}  map[string]string  "Created match successful"
// @Failure      400    {object}  map[string]string  "Invalid request payload"
// @Failure      500    {object}  map[string]string  "Failed to create match"
// @Router       /matches [post]
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

// GetMatch @Summary      Get match by ID
// @Summary      Retrieves a single match by its ID
// @Description  Retrieves a single match by its ID
// @Tags         Match
// @Produce      json
// @Param        id     path      string  true  "Match ID"
// @Success      200    {object}  model.MatchDto
// @Failure      404    {object}  map[string]string  "Match not found"
// @Router       /matches/{id} [get]
func (h *MatchHttpHandler) GetMatch(c *fiber.Ctx) error {
	match, err := h.matchService.GetMatch(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Match not found"})
	}

	return c.Status(fiber.StatusOK).JSON(match)
}

// GetAllMatches @Summary      Get all matches
// @Summary      Retrieves a list of matches, optionally filtered by type and schedule
// @Description  Retrieves a list of matches, optionally filtered by type and schedule
// @Tags         Match
// @Produce      json
// @Param        typeId     query     string  false  "Filter by sport type ID"
// @Param        schedule   query     string  false  "Filter by schedule (schedule or result)"
// @Success      200    {object}  []model.MatchesByDate  "List of matches grouped by date and sport type"
// @Failure      400    {object}  map[string]string  "Invalid schedule parameter"
// @Failure      500    {object}  map[string]string  "Failed to fetch matches"
// @Router       /matches [get]
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

// UpdateMatchScore @Summary      Update match score
// @Summary  Updates the score of a match
// @Description  Updates the score of a match
// @Tags         Match
// @Accept       json
// @Produce      json
// @Param        id     path      string          true  "Match ID"
// @Param        score  body      model.ScoreDto  true  "Score information"
// @Success      200    {object}  map[string]string  "Updated match score successfully"
// @Failure      400    {object}  map[string]string  "Invalid request payload"
// @Failure      500    {object}  map[string]string  "Failed to update match score"
// @Router       /matches/{id}/score [patch]
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

// UpdateMatchWinner @Summary      Update match winner
// @Summary Updates the winner of a match
// @Description  Updates the winner of a match
// @Tags         Match
// @Produce      json
// @Param        id          path      string  true  "Match ID"
// @Param        winner_id   path      string  true  "Winner Team ID"
// @Success      200    {object}  map[string]string  "Updated match winner successfully"
// @Failure      500    {object}  map[string]string  "Failed to update match winner"
// @Router       /matches/{id}/winner/{winner_id} [patch]
func (h *MatchHttpHandler) UpdateMatchWinner(c *fiber.Ctx) error {
	matchId := c.Params("id")
	winnerId := c.Params("winner_id")

	err := h.matchService.UpdateMatchWinner(matchId, winnerId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update match winner"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Updated match winner successfully"})
}

// DeleteMatch @Summary      Delete match
// @Summary      Deletes a match by its ID
// @Description  Deletes a match by its ID
// @Tags         Match
// @Param        id     path      string  true  "Match ID"
// @Success      200    {object}  map[string]string  "Deleted match successful"
// @Failure      500    {object}  map[string]string  "Failed to delete match"
// @Router       /matches/{id} [delete]
func (h *MatchHttpHandler) DeleteMatch(c *fiber.Ctx) error {
	err := h.matchService.DeleteMatch(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete match"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Deleted match successful"})
}

func (h *MatchHttpHandler) GetTime(c *fiber.Ctx) error {
	time, err := h.matchService.GetTime()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get time"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"currentTime": time})
}

func (h *MatchHttpHandler) UpdateMatchDraw(c *fiber.Ctx) error {
	matchId := c.Params("id")

	err := h.matchService.UpdateMatchDraw(matchId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update match as draw"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Updated match as draw successfully"})
}
