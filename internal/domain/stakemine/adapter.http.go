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
