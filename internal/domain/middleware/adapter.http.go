package middleware

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type MiddlewareHttpHandler struct {
	service MiddlewareService
	log     *zap.Logger
}

func NewMiddlewareHttpHandler(service MiddlewareService, log *zap.Logger) *MiddlewareHttpHandler {
	return &MiddlewareHttpHandler{
		service: service,
		log:     log,
	}
}

// AuthMiddleware checks the token validity and retrieves the user information.
func (h *MiddlewareHttpHandler) AuthMiddleware(c *fiber.Ctx) error {
	// Extract token from the header
	header := c.Get("Authorization")
	if header == "" {
		h.log.Named("AuthMiddleware").Error("Get header: ", zap.Error(errors.New("missing authorization header")))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing authorization header",
		})
	}

	token := strings.Split(header, " ")
	if len(token) != 2 || token[0] != "Bearer" || token[1] == "" {
		h.log.Named("AuthMiddleware").Error("split Bearer and access_token: ", zap.Error(errors.New("error while split header")))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing authorization header",
		})
	}

	// Verify the token
	userId, err := h.service.VerifyToken(token[1])
	if err != nil {
		h.log.Named("AuthMiddleware").Error("Token verification failed", zap.Error(err))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid or expired token",
		})
	}

	// Get the user profile
	userDto, err := h.service.GetMe(*userId)
	if err != nil {
		h.log.Named("AuthMiddleware").Error("User not found", zap.Error(err))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	// Store user in context for downstream handlers
	c.Locals("user", userDto)

	return c.Next()
}
