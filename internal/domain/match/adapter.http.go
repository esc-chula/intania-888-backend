package match

import (
	"errors"
	"strings"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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

// CreateMatch godoc
// @Summary Create a match
// @Tags Match
// @Accept json
// @Produce json
// @Param match body model.CreateMatchRequest true "Match"
// @Success 201 {object} map[string]string
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /matches [post]
// @Security BearerAuth
func (h *MatchHttpHandler) CreateMatch(c *fiber.Ctx) error {
	var request model.CreateMatchRequest

	if e := apierror.BindJSON(c, &request); e != nil {
		return e
	}
	v := model.MatchDto{
		TeamAId:   request.TeamAId,
		TeamBId:   request.TeamBId,
		TypeId:    request.TypeId,
		StartTime: request.StartTime,
		EndTime:   request.EndTime,
	}

	if e := h.matchService.CreateMatch(&v); e != nil {
		return mapMatchError(e)
	}

	return c.Status(201).JSON(fiber.Map{"message": "Created match successful"})
}

// GetMatch godoc
// @Summary Get a match by ID
// @Tags Match
// @Produce json
// @Param id path string true "Match ID"
// @Success 200 {object} model.MatchDto
// @Failure 401 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /matches/{id} [get]
// @Security BearerAuth
func (h *MatchHttpHandler) GetMatch(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	v, e := h.matchService.GetMatch(c.Params("id"))
	if e != nil {
		return mapMatchError(e)
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
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /matches [get]
// @Security BearerAuth
func (h *MatchHttpHandler) GetAllMatches(c *fiber.Ctx) error {
	f := &model.MatchFilter{TypeId: c.Query("typeId")}
	if f.TypeId != "" {
		if _, err := uuid.Parse(f.TypeId); err != nil {
			return apierror.Invalid(map[string]string{"typeId": "must be a valid ID"})
		}
	}
	switch c.Query("schedule") {
	case "schedule":
		f.Schedule = model.Schedule
	case "result":
		f.Schedule = model.Result

	case "":
	default:
		return apierror.Invalid(map[string]string{"schedule": "must be schedule or result"})
	}

	v, e := h.matchService.GetAllMatches(f)
	if e != nil {
		return apierror.Wrap(e, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to fetch matches")
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
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /matches/{id}/score [patch]
// @Security BearerAuth
func (h *MatchHttpHandler) UpdateMatchScore(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	var v model.ScoreDto

	if e := apierror.BindJSON(c, &v); e != nil {
		return e
	}

	if e := h.matchService.UpdateMatchScore(c.Params("id"), &v); e != nil {
		return mapMatchError(e)
	}

	return c.JSON(fiber.Map{"message": "Updated match score successfully"})
}

// UpdateMatch godoc
// @Summary Update match details
// @Tags Match
// @Accept json
// @Param id path string true "Match ID"
// @Param match body model.UpdateMatchRequest true "Match fields to update"
// @Success 200 {object} map[string]string
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /matches/{id} [put]
// @Security BearerAuth
func (h *MatchHttpHandler) UpdateMatch(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	var request model.UpdateMatchRequest

	if e := apierror.BindJSON(c, &request); e != nil {
		return e
	}
	v := model.MatchDto{}
	if request.TeamAId != nil {
		v.TeamAId = *request.TeamAId
	}
	if request.TeamBId != nil {
		v.TeamBId = *request.TeamBId
	}
	if request.TypeId != nil {
		v.TypeId = *request.TypeId
	}
	if request.StartTime != nil {
		v.StartTime = *request.StartTime
	}
	if request.EndTime != nil {
		v.EndTime = *request.EndTime
	}

	if e := h.matchService.UpdateMatch(c.Params("id"), &v); e != nil {
		return mapMatchError(e)
	}

	return c.JSON(fiber.Map{"message": "Updated match successfully"})
}

// DeleteMatch godoc
// @Summary Delete a match
// @Tags Match
// @Param id path string true "Match ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /matches/{id} [delete]
// @Security BearerAuth
func (h *MatchHttpHandler) DeleteMatch(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	if e := h.matchService.DeleteMatch(c.Params("id")); e != nil {
		return mapMatchError(e)
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
// @Failure 409 {object} apierror.Response
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 500 {object} apierror.Response
// @Router /matches/{id}/result [put]
// @Security BearerAuth
func (h *MatchHttpHandler) SetResult(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	var v model.MatchResultRequest

	if e := apierror.BindJSON(c, &v); e != nil {
		return e
	}
	if details := v.ValidateRequest(); len(details) > 0 {
		return apierror.Invalid(details)
	}

	e := h.matchService.SetResult(c.Params("id"), &v)

	switch {
	case e != nil:
		return mapMatchError(e)
	default:
		return c.JSON(fiber.Map{"message": "result accepted"})
	}
}

func mapMatchError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidMatch), errors.Is(err, ErrInvalidScore):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid match request")
	case errors.Is(err, ErrInvalidResult):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid match result")
	case errors.Is(err, ErrResultConflict):
		return apierror.Wrap(err, fiber.StatusConflict, "MATCH_RESULT_CONFLICT", "Match result conflicts with existing state")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Match not found")
	default:
		return err
	}
}
