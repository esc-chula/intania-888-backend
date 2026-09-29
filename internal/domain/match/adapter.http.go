package match

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

// HTTPHandler translates HTTP requests into match application use cases.
type HTTPHandler struct {
	matchService HTTPService
}

// NewHTTPHandler constructs the match HTTP adapter.
func NewHTTPHandler(s HTTPService) *HTTPHandler {
	return &HTTPHandler{matchService: s}
}

// RegisterRoutes registers the existing match routes and authentication guards.
func (h *HTTPHandler) RegisterRoutes(r fiber.Router, auth, admin fiber.Handler) {
	r = r.Group("/matches", auth)
	r.Get("/", h.GetAllMatches)
	r.Get("/current/time", h.GetTime)
	r.Get("/:id", h.GetMatch)

	a := r.Group("", admin)
	a.Post("/", h.CreateMatch)
	a.Put("/:id", h.UpdateMatch)
	a.Patch("/:id/score", h.UpdateMatchScore)
	a.Put("/:id/result", h.SetResult)
	a.Delete("/:id", h.DeleteMatch)
}

// CreateMatch validates match details and creates an administrator-managed fixture.
func (h *HTTPHandler) CreateMatch(c *fiber.Ctx) error {
	var request CreateMatchRequest

	if e := apierror.BindJSON(c, &request); e != nil {
		return e
	}
	v := Input{
		TeamAID:    request.TeamAID,
		TeamBID:    request.TeamBID,
		TypeID:     request.TypeID,
		LocationID: request.LocationID,
		StartTime:  request.StartTime,
		EndTime:    request.EndTime,
	}

	if e := h.matchService.CreateMatch(c.UserContext(), &v); e != nil {
		return mapMatchError(e)
	}

	return c.Status(201).JSON(fiber.Map{"message": "Created match successful"})
}

// GetMatch returns one fixture with its current authoritative betting rates.
func (h *HTTPHandler) GetMatch(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	v, e := h.matchService.GetMatch(c.UserContext(), c.Params("id"))
	if e != nil {
		return mapMatchError(e)
	}

	return c.JSON(matchResultDTO(v))
}

// GetAllMatches validates filters and groups matching fixtures by date and sport type.
func (h *HTTPHandler) GetAllMatches(c *fiber.Ctx) error {
	f := &Filter{TypeID: c.Query("typeId")}
	switch c.Query("schedule") {
	case "schedule":
		f.Schedule = Schedule
	case "result":
		f.Schedule = ScheduleResult

	case "":
	default:
		return apierror.Invalid(map[string]string{"schedule": "must be schedule or result"})
	}

	v, e := h.matchService.GetAllMatches(c.UserContext(), f)
	if e != nil {
		return apierror.Wrap(e, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to fetch matches")
	}

	return c.JSON(groupMatchesByDateAndType(matchResultsDTO(v)))
}

// UpdateMatchScore accepts explicitly supplied nonnegative team scores, including zero.
func (h *HTTPHandler) UpdateMatchScore(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	var v ScoreDTO

	if e := apierror.BindJSON(c, &v); e != nil {
		return e
	}

	if e := h.matchService.UpdateMatchScore(c.UserContext(), c.Params("id"), &ScoreInput{
		TeamAScore: v.TeamAScore,
		TeamBScore: v.TeamBScore,
	}); e != nil {
		return mapMatchError(e)
	}

	return c.JSON(fiber.Map{"message": "Updated match score successfully"})
}

// UpdateMatch applies the supplied fixture fields through the administrator use case.
func (h *HTTPHandler) UpdateMatch(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	var request UpdateMatchRequest

	if e := apierror.BindJSON(c, &request); e != nil {
		return e
	}
	v := Input{}
	if request.TeamAID != nil {
		v.TeamAID = *request.TeamAID
	}
	if request.TeamBID != nil {
		v.TeamBID = *request.TeamBID
	}
	if request.TypeID != nil {
		v.TypeID = *request.TypeID
	}
	if request.LocationID != nil {
		v.LocationID = *request.LocationID
	}
	if request.StartTime != nil {
		v.StartTime = *request.StartTime
	}
	if request.EndTime != nil {
		v.EndTime = *request.EndTime
	}

	if e := h.matchService.UpdateMatch(c.UserContext(), c.Params("id"), &v); e != nil {
		return mapMatchError(e)
	}

	return c.JSON(fiber.Map{"message": "Updated match successfully"})
}

// DeleteMatch removes the selected fixture after the administrator route guard.
func (h *HTTPHandler) DeleteMatch(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	if e := h.matchService.DeleteMatch(c.UserContext(), c.Params("id")); e != nil {
		return mapMatchError(e)
	}

	return c.JSON(fiber.Map{"message": "Deleted match successful"})
}

// GetTime returns the server's current UTC time.
func (h *HTTPHandler) GetTime(c *fiber.Ctx) error {
	v, err := h.matchService.GetTime()
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"currentTime": v})
}

// SetResult validates a terminal winner or draw result and requests atomic settlement.
func (h *HTTPHandler) SetResult(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	var v ResultRequest

	if e := apierror.BindJSON(c, &v); e != nil {
		return e
	}
	if details := v.ValidateRequest(); len(details) > 0 {
		return apierror.Invalid(details)
	}

	e := h.matchService.SetResult(c.UserContext(), c.Params("id"), &ResultInput{
		Outcome:  v.Outcome,
		WinnerID: v.WinnerID,
	})

	switch {
	case e != nil:
		return mapMatchError(e)
	default:
		return c.JSON(fiber.Map{"message": "result accepted"})
	}
}
