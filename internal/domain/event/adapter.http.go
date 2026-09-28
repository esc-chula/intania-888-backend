package event

import (
	"errors"
	"strings"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type EventHttpHandler struct {
	eventService EventService
}

func NewEventHttpHandler(eventService EventService) *EventHttpHandler {
	return &EventHttpHandler{
		eventService: eventService,
	}
}

func (h *EventHttpHandler) RegisterRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router = router.Group("/events", mid.AuthMiddleware)

	router.Get("/redeem/daily", h.RedeemDailyReward)
	router.Post("/spin/slot", h.SpinSlotMachine)
	router.Post("/use-steal-token", h.UseStealToken)

	adminRouter := router.Group("", mid.AdminMiddleware)
	adminRouter.Get("/daily-rewards", h.GetDailyRewardSchedule)
	adminRouter.Put("/daily-rewards/:date", h.SetDailyReward)
	adminRouter.Delete("/daily-rewards/:date", h.DeleteDailyReward)
}

// RedeemDailyReward handles the daily reward redemption
// @Summary Redeem daily reward
// @Description Redeem daily reward for the logged-in user
// @Tags Event
// @Accept  json
// @Produce  json
// @Success 200 {object} map[string]string "redeemed daily reward successful"
// @Failure 401 {object} apierror.Response "unauthorized"
// @Failure 404 {object} apierror.Response "user not found"
// @Failure 409 {object} apierror.Response "daily reward already claimed"
// @Failure 500 {object} apierror.Response "internal server error"
// @Router /events/redeem/daily [get]
// @Security BearerAuth
func (h *EventHttpHandler) RedeemDailyReward(c *fiber.Ctx) error {
	// get user from context
	userProfile := utils.GetUserProfileFromCtx(c)
	if userProfile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	err := h.eventService.RedeemDailyReward(userProfile)
	if err != nil {
		return mapEventError(err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "redeemed daily reward successful"})
}

// @Summary Spin the slot machine
// @Description Spins the slot machine using the requested coin amount
// @Tags Event
// @Produce json
// @Param spendAmount query string true "Money string to spend (50, 100, or 500)"
// @Success 200 {object} map[string]interface{} "slot result"
// @Failure 400 {object} apierror.Response "invalid spend amount or user profile"
// @Failure 401 {object} apierror.Response "unauthorized"
// @Failure 404 {object} apierror.Response "user not found"
// @Failure 422 {object} apierror.Response "insufficient balance"
// @Failure 500 {object} apierror.Response "internal server error"
// @Router /events/spin/slot [post]
// @Security BearerAuth
func (h *EventHttpHandler) SpinSlotMachine(c *fiber.Ctx) error {
	// Get user from context
	userProfile := utils.GetUserProfileFromCtx(c)
	if userProfile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	// Get spending amount from query
	spendAmountStr := c.Query("spendAmount")
	if spendAmountStr == "" {
		return apierror.Invalid(map[string]string{"spendAmount": "is required"})
	}

	// Parse the spend amount from the query
	spendAmount, err := model.ParseMoney(spendAmountStr)
	if err != nil {
		return apierror.Invalid(map[string]string{"spendAmount": "must be a valid amount"})
	}

	// Check that the spend amount is exactly 50, 100, or 500
	if spendAmount.MinorUnits() != 50_00 && spendAmount.MinorUnits() != 100_00 && spendAmount.MinorUnits() != 500_00 {
		return apierror.Invalid(map[string]string{"spendAmount": "must be 50, 100, or 500"})
	}

	// Call the service to spin the slot machine with the selected spending amount
	result, err := h.eventService.SpinSlotMachine(userProfile, spendAmount)
	if err != nil {
		return mapEventError(err)
	}

	return c.Status(fiber.StatusOK).JSON(result)
}

// SetDailyReward handles setting daily reward amount
// @Summary Set daily reward
// @Description Creates or replaces the daily reward override for a specific date (admin only). Repeating the same request is safe.
// @Tags Event
// @Accept json
// @Produce json
// @Param date path string true "Reward date in DD-MM-YYYY format"
// @Param request body model.SetDailyRewardRequest true "Daily reward amount"
// @Success 200 {object} map[string]string "Set daily reward successful"
// @Failure 400 {object} apierror.Response "Invalid request payload"
// @Failure 401 {object} apierror.Response "unauthorized"
// @Failure 403 {object} apierror.Response "admin access required"
// @Failure 500 {object} apierror.Response "Failed to set daily reward"
// @Router /events/daily-rewards/{date} [put]
// @Security BearerAuth
func (h *EventHttpHandler) SetDailyReward(c *fiber.Ctx) error {
	date := c.Params("date")
	if !validRewardDate(date) {
		return apierror.Invalid(map[string]string{"date": "must use DD-MM-YYYY format"})
	}

	var req model.SetDailyRewardRequest
	if err := apierror.BindJSON(c, &req); err != nil {
		return err
	}

	err := h.eventService.SetDailyReward(date, req.Amount)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to set daily reward")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Set daily reward successful"})
}

// GetDailyRewardSchedule returns the default daily reward and date-specific overrides.
// @Summary List daily reward schedule
// @Description Returns the configured default daily reward and date-specific overrides (admin only).
// @Tags Event
// @Produce json
// @Success 200 {object} model.DailyRewardScheduleResponse
// @Failure 401 {object} apierror.Response "unauthorized"
// @Failure 403 {object} apierror.Response "admin access required"
// @Failure 500 {object} apierror.Response "Failed to list daily reward schedule"
// @Router /events/daily-rewards [get]
// @Security BearerAuth
func (h *EventHttpHandler) GetDailyRewardSchedule(c *fiber.Ctx) error {
	response, err := h.eventService.GetDailyRewardSchedule()
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to list daily reward schedule")
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// DeleteDailyReward removes a date-specific daily reward override.
// @Summary Delete a daily reward override
// @Description Deletes the date-specific reward override so the default reward applies again (admin only).
// @Tags Event
// @Produce json
// @Param date path string true "Reward date in DD-MM-YYYY format"
// @Success 200 {object} map[string]string "Daily reward override deleted"
// @Failure 400 {object} apierror.Response "Invalid reward date"
// @Failure 401 {object} apierror.Response "unauthorized"
// @Failure 403 {object} apierror.Response "admin access required"
// @Failure 404 {object} apierror.Response "Daily reward override not found"
// @Failure 500 {object} apierror.Response "Failed to delete daily reward override"
// @Router /events/daily-rewards/{date} [delete]
// @Security BearerAuth
func (h *EventHttpHandler) DeleteDailyReward(c *fiber.Ctx) error {
	date := c.Params("date")
	if !validRewardDate(date) {
		return apierror.Invalid(map[string]string{"date": "must use DD-MM-YYYY format"})
	}

	if err := h.eventService.DeleteDailyReward(date); err != nil {
		if errors.Is(err, ErrDailyRewardOverrideNotFound) {
			return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Daily reward override not found")
		}
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Unable to delete daily reward override")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Daily reward override deleted"})
}

// @Summary Use a steal token
// @Description Uses a steal token against one of its eligible victims
// @Tags Event
// @Accept json
// @Produce json
// @Param request body model.UseStealTokenRequestDto true "Steal token request"
// @Success 200 {object} model.UseStealTokenResponseDto "steal result"
// @Failure 400 {object} apierror.Response "invalid request or token"
// @Failure 401 {object} apierror.Response "missing or invalid authorization"
// @Failure 403 {object} apierror.Response "token is not available to this user"
// @Failure 404 {object} apierror.Response "requested resource not found"
// @Failure 409 {object} apierror.Response "token state conflict"
// @Failure 422 {object} apierror.Response "insufficient balance"
// @Failure 500 {object} apierror.Response "internal server error"
// @Router /events/use-steal-token [post]
// @Security BearerAuth
// UseStealToken consumes a steal token to steal a percentage from random users.
func (h *EventHttpHandler) UseStealToken(c *fiber.Ctx) error {
	userProfile := utils.GetUserProfileFromCtx(c)
	if userProfile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	var req model.UseStealTokenRequestDto
	if err := apierror.BindJSON(c, &req); err != nil {
		return err
	}

	result, err := h.eventService.UseStealToken(userProfile.Id, req.Token, req.VictimIndex)
	if err != nil {
		return mapEventError(err)
	}

	return c.Status(fiber.StatusOK).JSON(result)
}

func validRewardDate(value string) bool {
	parsed, err := time.Parse("02-01-2006", value)
	return err == nil && parsed.Format("02-01-2006") == value && strings.TrimSpace(value) == value
}

func mapEventError(err error) error {
	switch {
	case errors.Is(err, ErrDailyRewardAlreadyClaimed):
		return apierror.Wrap(err, fiber.StatusConflict, "DAILY_REWARD_ALREADY_CLAIMED", "Daily reward already claimed")
	case errors.Is(err, ErrInsufficientBalance):
		return apierror.Wrap(err, fiber.StatusUnprocessableEntity, "INSUFFICIENT_BALANCE", "Insufficient balance")
	case errors.Is(err, ErrStealTokenConflict):
		return apierror.Wrap(err, fiber.StatusConflict, "STEAL_TOKEN_CONFLICT", "Steal token has already been used")
	case errors.Is(err, ErrStealTokenInvalid):
		return apierror.Wrap(err, fiber.StatusConflict, "STEAL_TOKEN_INVALID", "Steal token is invalid or expired")
	case errors.Is(err, ErrStealTokenForbidden):
		return apierror.Wrap(err, fiber.StatusForbidden, "FORBIDDEN", "Steal token cannot be used by this account")
	case errors.Is(err, ErrInvalidStealRequest):
		return apierror.Wrap(err, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid victim selection")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apierror.Wrap(err, fiber.StatusNotFound, "RESOURCE_NOT_FOUND", "Requested resource not found")
	default:
		return err
	}
}
