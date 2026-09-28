package stakemine

import (
	"errors"
	"strconv"
	"strings"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type StakeMineHttpHandler struct {
	service StakeMineService
}

func NewStakeMineHttpHandler(service StakeMineService) *StakeMineHttpHandler {
	return &StakeMineHttpHandler{service: service}
}

func (h *StakeMineHttpHandler) RegisterRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router = router.Group("/mines", mid.AuthMiddleware)

	router.Post("/create", h.CreateGame)
	router.Post("/:id/reveal", h.RevealTile)
	router.Post("/:id/cashout", h.CashOut)
	router.Get("/active", h.GetActiveGame)
	router.Get("/history", h.GetHistory)
	router.Get("/stats", h.GetStats)
	router.Get("/:id", h.GetGame)
}

// @Summary Create a new Stake Mines game
// @Description Start a new Stake Mines game with specified bet amount and risk level
// @Tags StakeMines
// @Accept json
// @Produce json
// @Param request body model.CreateMineGameRequest true "Game creation request"
// @Success 200 {object} model.MineGameDto
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Failure 422 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /mines/create [post]
// @Security BearerAuth
func (h *StakeMineHttpHandler) CreateGame(c *fiber.Ctx) error {
	profile := utils.GetUserProfileFromCtx(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	var req model.CreateMineGameRequest
	if err := apierror.BindJSON(c, &req); err != nil {
		return err
	}

	game, err := h.service.CreateGame(profile.Id, &req)
	if err != nil {
		return mapStakeMineError(err)
	}

	return c.Status(fiber.StatusOK).JSON(game)
}

// @Summary Reveal a tile in the game
// @Description Reveal a specific tile in an active Stake Mines game
// @Tags StakeMines
// @Accept json
// @Produce json
// @Param id path string true "Game ID"
// @Param request body model.RevealMineTileRequest true "Reveal tile request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /mines/{id}/reveal [post]
// @Security BearerAuth
func (h *StakeMineHttpHandler) RevealTile(c *fiber.Ctx) error {
	profile := utils.GetUserProfileFromCtx(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	gameId := c.Params("id")
	if strings.TrimSpace(gameId) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	var req model.RevealMineTileRequest
	if err := apierror.BindJSON(c, &req); err != nil {
		return err
	}

	game, message, err := h.service.RevealTile(profile.Id, gameId, &req)
	if err != nil {
		return mapStakeMineError(err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": message,
		"game":    game,
	})
}

// @Summary Cash out from the current game
// @Description Cash out and take winnings from an active Stake Mines game
// @Tags StakeMines
// @Produce json
// @Param id path string true "Game ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /mines/{id}/cashout [post]
// @Security BearerAuth
func (h *StakeMineHttpHandler) CashOut(c *fiber.Ctx) error {
	profile := utils.GetUserProfileFromCtx(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	gameId := c.Params("id")
	if strings.TrimSpace(gameId) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	game, err := h.service.CashOut(profile.Id, gameId)
	if err != nil {
		return mapStakeMineError(err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Successfully cashed out!",
		"game":    game,
	})
}

// @Summary Get game details
// @Description Get details of a specific Stake Mines game by ID
// @Tags StakeMines
// @Produce json
// @Param id path string true "Game ID"
// @Success 200 {object} model.MineGameDto
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Router /mines/{id} [get]
// @Security BearerAuth
func (h *StakeMineHttpHandler) GetGame(c *fiber.Ctx) error {
	profile := utils.GetUserProfileFromCtx(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	gameId := c.Params("id")
	if strings.TrimSpace(gameId) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	game, err := h.service.GetGame(profile.Id, gameId)
	if err != nil {
		return mapStakeMineError(err)
	}

	return c.Status(fiber.StatusOK).JSON(game)
}

// @Summary Get active game
// @Description Get the current active Stake Mines game for the user
// @Tags StakeMines
// @Produce json
// @Success 200 {object} model.MineGameDto
// @Failure 404 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /mines/active [get]
// @Security BearerAuth
func (h *StakeMineHttpHandler) GetActiveGame(c *fiber.Ctx) error {
	profile := utils.GetUserProfileFromCtx(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	game, err := h.service.GetActiveGame(profile.Id)
	if err != nil {
		return mapStakeMineError(err)
	}

	return c.Status(fiber.StatusOK).JSON(game)
}

// @Summary Get game history
// @Description Get user's Stake Mines game history with pagination
// @Tags StakeMines
// @Produce json
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /mines/history [get]
// @Security BearerAuth
func (h *StakeMineHttpHandler) GetHistory(c *fiber.Ctx) error {
	profile := utils.GetUserProfileFromCtx(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	limit, err := strconv.Atoi(c.Query("limit", "20"))
	if err != nil || limit < 1 {
		return apierror.Invalid(map[string]string{"limit": "must be a positive integer"})
	}
	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil || offset < 0 {
		return apierror.Invalid(map[string]string{"offset": "must be a non-negative integer"})
	}

	if limit > 100 {
		limit = 100
	}

	history, err := h.service.GetGameHistory(profile.Id, limit, offset)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data":   history,
		"limit":  limit,
		"offset": offset,
	})
}

// @Summary Get user statistics
// @Description Get comprehensive statistics for the user's Stake Mines games
// @Tags StakeMines
// @Produce json
// @Success 200 {object} model.MineGameStatsDto
// @Failure 401 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /mines/stats [get]
// @Security BearerAuth
func (h *StakeMineHttpHandler) GetStats(c *fiber.Ctx) error {
	profile := utils.GetUserProfileFromCtx(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	stats, err := h.service.GetStats(profile.Id)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(stats)
}

func mapStakeMineError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidGameRequest):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid Stake Mines request")
	case errors.Is(err, ErrInsufficientBalance):
		return apierror.Wrap(err, fiber.StatusUnprocessableEntity, "INSUFFICIENT_BALANCE", "Insufficient balance")
	case errors.Is(err, ErrGameNotFound), errors.Is(err, ErrNoActiveGame), errors.Is(err, ErrUserNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Stake Mines resource not found")
	case errors.Is(err, ErrGameForbidden):
		return apierror.Wrap(err, fiber.StatusForbidden, "FORBIDDEN", "Game access denied")
	case errors.Is(err, ErrGameConflict):
		return apierror.Wrap(err, fiber.StatusConflict, "GAME_STATE_CONFLICT", "Game state does not allow this action")
	default:
		return err
	}
}
