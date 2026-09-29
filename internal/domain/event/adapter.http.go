package event

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

// HTTPHandler adapts authenticated event requests to service commands.
type HTTPHandler struct {
	eventService ServicePort
}

// NewHTTPHandler constructs the event HTTP adapter.
func NewHTTPHandler(eventService ServicePort) *HTTPHandler {
	return &HTTPHandler{
		eventService: eventService,
	}
}

// RegisterRoutes registers authenticated event routes and administrator reward controls.
func (h *HTTPHandler) RegisterRoutes(router fiber.Router, authMiddleware, adminMiddleware fiber.Handler) {
	router = router.Group("/events", authMiddleware)

	router.Get("/redeem/daily", h.RedeemDailyReward)
	router.Post("/spin/slot", h.SpinSlotMachine)
	router.Post("/use-steal-token", h.UseStealToken)

	adminRouter := router.Group("", adminMiddleware)
	adminRouter.Get("/daily-rewards", h.GetDailyRewardSchedule)
	adminRouter.Put("/daily-rewards/:date", h.SetDailyReward)
	adminRouter.Delete("/daily-rewards/:date", h.DeleteDailyReward)
}

// RedeemDailyReward handles the daily reward redemption
func (h *HTTPHandler) RedeemDailyReward(c *fiber.Ctx) error {
	// get user from context
	userProfile := httpidentity.GetProfile(c)
	if userProfile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	err := h.eventService.RedeemDailyReward(c.UserContext(), userProfile.ID)
	if err != nil {
		return mapEventError(err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "redeemed daily reward successful"})
}

// SpinSlotMachine validates the requested spend and returns the committed slot result.
func (h *HTTPHandler) SpinSlotMachine(c *fiber.Ctx) error {
	// Get user from context
	userProfile := httpidentity.GetProfile(c)
	if userProfile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	// Get spending amount from query
	spendAmountStr := c.Query("spendAmount")
	if spendAmountStr == "" {
		return apierror.Invalid(map[string]string{"spendAmount": "is required"})
	}

	// Parse the spend amount from the query
	spendAmount, err := value.ParseMoney(spendAmountStr)
	if err != nil {
		return apierror.Invalid(map[string]string{"spendAmount": "must be a valid amount"})
	}

	// Check that the spend amount is exactly 50, 100, or 500
	if spendAmount.MinorUnits() != 50_00 && spendAmount.MinorUnits() != 100_00 && spendAmount.MinorUnits() != 500_00 {
		return apierror.Invalid(map[string]string{"spendAmount": "must be 50, 100, or 500"})
	}

	// Call the service to spin the slot machine with the selected spending amount
	result, err := h.eventService.SpinSlotMachine(c.UserContext(), *userProfile, spendAmount)
	if err != nil {
		return mapEventError(err)
	}

	return c.Status(fiber.StatusOK).JSON(spinToResponse(result))
}

// SetDailyReward handles setting daily reward amount
func (h *HTTPHandler) SetDailyReward(c *fiber.Ctx) error {
	date := c.Params("date")
	if !validRewardDate(date) {
		return apierror.Invalid(map[string]string{"date": "must use DD-MM-YYYY format"})
	}

	var req SetDailyRewardRequest
	if err := apierror.BindJSON(c, &req); err != nil {
		return err
	}

	err := h.eventService.SetDailyReward(c.UserContext(), date, req.Amount)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to set daily reward")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Set daily reward successful"})
}

// GetDailyRewardSchedule returns the default daily reward and date-specific overrides.
func (h *HTTPHandler) GetDailyRewardSchedule(c *fiber.Ctx) error {
	response, err := h.eventService.GetDailyRewardSchedule(c.UserContext())
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list daily reward schedule")
	}

	return c.Status(fiber.StatusOK).JSON(scheduleToResponse(response))
}

// DeleteDailyReward removes a date-specific daily reward override.
func (h *HTTPHandler) DeleteDailyReward(c *fiber.Ctx) error {
	date := c.Params("date")
	if !validRewardDate(date) {
		return apierror.Invalid(map[string]string{"date": "must use DD-MM-YYYY format"})
	}

	if err := h.eventService.DeleteDailyReward(c.UserContext(), date); err != nil {
		if errors.Is(err, ErrDailyRewardOverrideNotFound) {
			return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Daily reward override not found")
		}
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to delete daily reward override")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Daily reward override deleted"})
}

// UseStealToken commits the raid against the selected token candidate.
func (h *HTTPHandler) UseStealToken(c *fiber.Ctx) error {
	userProfile := httpidentity.GetProfile(c)
	if userProfile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	var req UseStealTokenRequest
	if err := apierror.BindJSON(c, &req); err != nil {
		return err
	}

	result, err := h.eventService.UseStealToken(c.UserContext(), userProfile.ID, req.Token, req.VictimIndex)
	if err != nil {
		return mapEventError(err)
	}

	return c.Status(fiber.StatusOK).JSON(stealToResponse(result))
}

func validRewardDate(value string) bool {
	parsed, err := time.Parse("02-01-2006", value)
	return err == nil && parsed.Format("02-01-2006") == value && strings.TrimSpace(value) == value
}

// ServicePort is the event functionality consumed by HTTP handlers.
type ServicePort interface {
	// RedeemDailyReward credits the actor's daily claim.
	RedeemDailyReward(ctx context.Context, userID string) error
	// GetDailyRewardSchedule returns the configured reward schedule.
	GetDailyRewardSchedule(ctx context.Context) (*DailyRewardSchedule, error)
	// SpinSlotMachine applies a spin using the authenticated actor.
	SpinSlotMachine(ctx context.Context, actor identity.Profile, spendAmount value.Money) (*SpinResult, error)
	// SetDailyReward creates or replaces a dated override.
	SetDailyReward(ctx context.Context, date string, amount value.Money) error
	// DeleteDailyReward removes a dated override.
	DeleteDailyReward(ctx context.Context, date string) error
	// UseStealToken consumes a token for the selected candidate.
	UseStealToken(ctx context.Context, userID, token string, victimIndex int) (*StealResult, error)
}
