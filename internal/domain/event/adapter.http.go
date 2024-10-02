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
}

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
