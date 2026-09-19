package match

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type MatchHttpHandler struct {
	matchService MatchService
}

func NewMatchHttpHandler(s MatchService) *MatchHttpHandler {
	return &MatchHttpHandler{matchService: s}
}

func (h *MatchHttpHandler) RegisterRoutes(r fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	r = r.Group("/matches", mid.AuthMiddleware)
	r.Get("/", h.GetAllMatches)
	r.Get("/current/time", h.GetTime)
	r.Get("/:id", h.GetMatch)

	a := r.Group("", mid.AdminMiddleware)
	a.Post("/", h.CreateMatch)
	a.Put("/:id", h.UpdateMatch)
	a.Patch("/:id/score", h.UpdateMatchScore)
	a.Put("/:id/result", h.SetResult)
	a.Delete("/:id", h.DeleteMatch)
}

func decodeResult(c *fiber.Ctx, v any) error {
	d := json.NewDecoder(bytes.NewReader(c.Body()))
	d.DisallowUnknownFields()

	if e := d.Decode(v); e != nil {
		return e
	}

	if e := d.Decode(&struct{}{}); !errors.Is(e, io.EOF) {
		return errors.New("multiple JSON values")
	}

	return nil
}

// CreateMatch godoc
// @Summary Create a match
// @Tags Match
// @Accept json
// @Produce json
// @Param match body model.MatchDto true "Match"
// @Success 201 {object} map[string]string
// @Router /matches [post]
// @Security BearerAuth
func (h *MatchHttpHandler) CreateMatch(c *fiber.Ctx) error {
	var v model.MatchDto

	if e := c.BodyParser(&v); e != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	if e := h.matchService.CreateMatch(&v); e != nil {
		return c.Status(500).JSON(fiber.Map{"error": "create failed"})
	}

	return c.Status(201).JSON(fiber.Map{"message": "Created match successful"})
}

// GetMatch godoc
// @Summary Get a match by ID
// @Tags Match
// @Produce json
// @Param id path string true "Match ID"
// @Success 200 {object} model.MatchDto
// @Router /matches/{id} [get]
// @Security BearerAuth
func (h *MatchHttpHandler) GetMatch(c *fiber.Ctx) error {
	v, e := h.matchService.GetMatch(c.Params("id"))
	if e != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Match not found"})
	}

	return c.JSON(v)
}

// GetAllMatches godoc
// @Summary List matches
// @Tags Match
// @Produce json
// @Param typeId query string false "Sport type ID"
// @Param schedule query string false "schedule or result"
// @Success 200 {array} model.MatchesByDate
// @Router /matches [get]
// @Security BearerAuth
func (h *MatchHttpHandler) GetAllMatches(c *fiber.Ctx) error {
	f := &model.MatchFilter{TypeId: c.Query("typeId")}
	switch c.Query("schedule") {
	case "schedule":
		f.Schedule = model.Schedule
	case "result":
		f.Schedule = model.Result

	case "":
	default:
		return c.Status(400).JSON(fiber.Map{"error": "Invalid schedule parameter"})
	}

	v, e := h.matchService.GetAllMatches(f)
	if e != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch matches"})
	}

	return c.JSON(groupMatchesByDateAndType(v))
}

// UpdateMatchScore godoc
// @Summary Update a match score
// @Tags Match
// @Accept json
// @Param id path string true "Match ID"
// @Param score body model.ScoreDto true "Score"
// @Success 200 {object} map[string]string
// @Router /matches/{id}/score [patch]
// @Security BearerAuth
func (h *MatchHttpHandler) UpdateMatchScore(c *fiber.Ctx) error {
	var v model.ScoreDto

	if e := c.BodyParser(&v); e != nil {
		return c.SendStatus(400)
	}

	if e := h.matchService.UpdateMatchScore(c.Params("id"), &v); e != nil {
		return c.SendStatus(500)
	}

	return c.JSON(fiber.Map{"message": "Updated match score successfully"})
}

// UpdateMatch godoc
// @Summary Update match details
// @Tags Match
// @Accept json
// @Param id path string true "Match ID"
// @Param match body model.MatchDto true "Match"
// @Success 200 {object} map[string]string
// @Router /matches/{id} [put]
// @Security BearerAuth
func (h *MatchHttpHandler) UpdateMatch(c *fiber.Ctx) error {
	var v model.MatchDto

	if e := c.BodyParser(&v); e != nil {
		return c.SendStatus(400)
	}

	if e := h.matchService.UpdateMatch(c.Params("id"), &v); e != nil {
		return c.SendStatus(500)
	}

	return c.JSON(fiber.Map{"message": "Updated match successfully"})
}

// DeleteMatch godoc
// @Summary Delete a match
// @Tags Match
// @Param id path string true "Match ID"
// @Success 200 {object} map[string]string
// @Router /matches/{id} [delete]
// @Security BearerAuth
func (h *MatchHttpHandler) DeleteMatch(c *fiber.Ctx) error {
	if e := h.matchService.DeleteMatch(c.Params("id")); e != nil {
		return c.SendStatus(500)
	}

	return c.JSON(fiber.Map{"message": "Deleted match successful"})
}

// GetTime godoc
// @Summary Get current server match time
// @Tags Match
// @Produce json
// @Success 200 {object} map[string]string
// @Router /matches/current/time [get]
// @Security BearerAuth
func (h *MatchHttpHandler) GetTime(c *fiber.Ctx) error {
	v, _ := h.matchService.GetTime()

	return c.JSON(fiber.Map{"currentTime": v})
}

// SetResult godoc
// @Summary Idempotently set a match result and settle affected bills
// @Tags Match
// @Accept json
// @Produce json
// @Param id path string true "Match ID"
// @Param result body model.MatchResultRequest true "Winner or draw result"
// @Success 200 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /matches/{id}/result [put]
// @Security BearerAuth
func (h *MatchHttpHandler) SetResult(c *fiber.Ctx) error {
	var v model.MatchResultRequest

	if e := decodeResult(c, &v); e != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	e := h.matchService.SetResult(c.Params("id"), &v)

	switch {
	case errors.Is(e, ErrInvalidResult):
		return c.Status(400).JSON(fiber.Map{"error": e.Error()})
	case errors.Is(e, ErrResultConflict):
		return c.Status(409).JSON(fiber.Map{"error": e.Error()})
	case errors.Is(e, gorm.ErrRecordNotFound):
		return c.Status(404).JSON(fiber.Map{"error": "match not found"})
	case e != nil:
		return c.Status(500).JSON(fiber.Map{"error": "settlement failed"})
	default:
		return c.JSON(fiber.Map{"message": "result accepted"})
	}
}
