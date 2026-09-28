package stakemine

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
)

// HTTPHandler serves Stake Mines HTTP endpoints.
type HTTPHandler struct {
	service ServicePort
}

// NewHTTPHandler constructs Stake Mines HTTP handlers.
func NewHTTPHandler(service ServicePort) *HTTPHandler {
	return &HTTPHandler{service: service}
}

// RegisterRoutes installs game routes using the supplied authentication middleware.
func (h *HTTPHandler) RegisterRoutes(router fiber.Router, auth fiber.Handler) {
	router = router.Group("/mines", auth)

	router.Post("/create", h.CreateGame)
	router.Post("/:id/reveal", h.RevealTile)
	router.Post("/:id/cashout", h.CashOut)
	router.Get("/active", h.GetActiveGame)
	router.Get("/history", h.GetHistory)
	router.Get("/stats", h.GetStats)
	router.Get("/:id", h.GetGame)
}

// CreateGame creates an authenticated game.
//
// @Summary Create a new Stake Mines game
// @Description Start a new Stake Mines game with specified bet amount and risk level
// @Tags StakeMines
// @Accept json
// @Produce json
// @Param request body CreateGameRequest true "Game creation request"
// @Success 200 {object} GameResponse
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Failure 422 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /mines/create [post]
// @Security CookieSession
func (h *HTTPHandler) CreateGame(c *fiber.Ctx) error {
	profile := httpidentity.GetProfile(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	var req CreateGameRequest
	if err := apierror.BindJSON(c, &req); err != nil {
		return err
	}

	game, err := h.service.CreateGame(c.UserContext(), profile.ID, CreateInput(req))
	if err != nil {
		return mapStakeMineError(err)
	}

	return c.Status(fiber.StatusOK).JSON(gameResponse(game))
}

// RevealTile reveals a tile for the authenticated game owner.
//
// @Summary Reveal a tile in the game
// @Description Reveal a specific tile in an active Stake Mines game
// @Tags StakeMines
// @Accept json
// @Produce json
// @Param id path string true "Game ID"
// @Param request body RevealTileRequest true "Reveal tile request"
// @Success 200 {object} RevealTileResponse
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /mines/{id}/reveal [post]
// @Security CookieSession
func (h *HTTPHandler) RevealTile(c *fiber.Ctx) error {
	profile := httpidentity.GetProfile(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	gameID := c.Params("id")
	if strings.TrimSpace(gameID) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	var req RevealTileRequest
	if err := apierror.BindJSON(c, &req); err != nil {
		return err
	}

	game, message, err := h.service.RevealTile(c.UserContext(), profile.ID, gameID, RevealInput{Index: req.Index})
	if err != nil {
		return mapStakeMineError(err)
	}

	return c.Status(fiber.StatusOK).JSON(RevealTileResponse{Message: message, Game: gameResponse(game)})
}

// CashOut cashes out an authenticated game.
//
// @Summary Cash out from the current game
// @Description Cash out and take winnings from an active Stake Mines game
// @Tags StakeMines
// @Produce json
// @Param id path string true "Game ID"
// @Success 200 {object} CashOutResponse
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /mines/{id}/cashout [post]
// @Security CookieSession
func (h *HTTPHandler) CashOut(c *fiber.Ctx) error {
	profile := httpidentity.GetProfile(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	gameID := c.Params("id")
	if strings.TrimSpace(gameID) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	game, err := h.service.CashOut(c.UserContext(), profile.ID, gameID)
	if err != nil {
		return mapStakeMineError(err)
	}

	return c.Status(fiber.StatusOK).JSON(CashOutResponse{Message: "Successfully cashed out!", Game: gameResponse(game)})
}

// GetGame serves an owned game.
//
// @Summary Get game details
// @Description Get details of a specific Stake Mines game by ID
// @Tags StakeMines
// @Produce json
// @Param id path string true "Game ID"
// @Success 200 {object} GameResponse
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Router /mines/{id} [get]
// @Security CookieSession
func (h *HTTPHandler) GetGame(c *fiber.Ctx) error {
	profile := httpidentity.GetProfile(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	gameID := c.Params("id")
	if strings.TrimSpace(gameID) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}

	game, err := h.service.GetGame(c.UserContext(), profile.ID, gameID)
	if err != nil {
		return mapStakeMineError(err)
	}

	return c.Status(fiber.StatusOK).JSON(gameResponse(game))
}

// GetActiveGame serves the authenticated account's active game.
//
// @Summary Get active game
// @Description Get the current active Stake Mines game for the user
// @Tags StakeMines
// @Produce json
// @Success 200 {object} GameResponse
// @Failure 404 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /mines/active [get]
// @Security CookieSession
func (h *HTTPHandler) GetActiveGame(c *fiber.Ctx) error {
	profile := httpidentity.GetProfile(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	game, err := h.service.GetActiveGame(c.UserContext(), profile.ID)
	if err != nil {
		return mapStakeMineError(err)
	}

	return c.Status(fiber.StatusOK).JSON(gameResponse(game))
}

// GetHistory serves the existing newest-first game history page.
//
// @Summary Get game history
// @Description Get user's Stake Mines game history with pagination
// @Tags StakeMines
// @Produce json
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} HistoryListResponse
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /mines/history [get]
// @Security CookieSession
func (h *HTTPHandler) GetHistory(c *fiber.Ctx) error {
	profile := httpidentity.GetProfile(c)
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

	history, err := h.service.GetGameHistory(c.UserContext(), profile.ID, limit, offset)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(HistoryListResponse{Data: historyResponses(history), Limit: limit, Offset: offset})
}

// GetStats serves realized game totals and active exposure.
//
// @Summary Get user statistics
// @Description Get comprehensive statistics for the user's Stake Mines games
// @Tags StakeMines
// @Produce json
// @Success 200 {object} StatsResponse
// @Failure 401 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /mines/stats [get]
// @Security CookieSession
func (h *HTTPHandler) GetStats(c *fiber.Ctx) error {
	profile := httpidentity.GetProfile(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	stats, err := h.service.GetStats(c.UserContext(), profile.ID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(statsResponse(stats))
}
