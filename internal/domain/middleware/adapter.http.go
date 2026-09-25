package middleware

import (
	"strings"

	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/utils"
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

func (h *MiddlewareHttpHandler) AuthMiddleware(c *fiber.Ctx) error {
	accessToken := c.Cookies(utils.AccessTokenCookieName)
	if accessToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing access cookie",
		})
	}

	claims, err := h.service.VerifyToken(accessToken)
	if err != nil {
		h.log.Named("AuthMiddleware").Error("Verify access cookie", zap.Error(err))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid or expired session",
		})
	}

	userDto, err := h.service.GetMe(claims.UserId)
	if err != nil {
		h.log.Named("AuthMiddleware").Error("User not found", zap.Error(err))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	if security.IsBlacklisted(userDto.Email, userDto.Id) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	c.Locals("user", userDto)
	c.Locals("session_id", claims.SessionId)
	return c.Next()
}

func (h *MiddlewareHttpHandler) AdminMiddleware(c *fiber.Ctx) error {
	user := utils.GetUserProfileFromCtx(c)
	if user == nil {
		h.log.Named("AdminMiddleware").Error("User not found in context")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	if !security.IsAdmin(user.Email, user.RoleId) {
		h.log.Named("AdminMiddleware").Warn("Non-admin attempted admin action",
			zap.String("role", user.RoleId),
			zap.String("endpoint", c.Path()))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "admin access required",
		})
	}

	return c.Next()
}

func parseBearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
