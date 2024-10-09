package event

import (
	"errors"

	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
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
}

// RedeemDailyReward handles the daily reward redemption
// @Summary Redeem daily reward
// @Description Redeem daily reward for the logged-in user
// @Tags Event
// @Accept  json
// @Produce  json
// @Success 200 {object} map[string]string "redeemed daily reward successful"
// @Failure 400 {object} map[string]string "not found user profile in context"
// @Failure 500 {object} map[string]string "internal server error"
// @Router /events/redeem/daily [get]
func (h *EventHttpHandler) RedeemDailyReward(c *fiber.Ctx) error {
	// get user from context
	userProfile := utils.GetUserProfileFromCtx(c)
	if userProfile == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errors.New("not found user profile in context").Error()})
	}

	err := h.eventService.RedeemDailyReward(userProfile)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "redeemed daily reward successful"})
}

func (h *EventHttpHandler) SpinSlotMachine(c *fiber.Ctx) error {
	// Get user from context
	userProfile := utils.GetUserProfileFromCtx(c)
	if userProfile == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "User profile not found"})
	}

	// Call the service to spin the slot machine
	result, err := h.eventService.SpinSlotMachine(userProfile)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(result)
}
