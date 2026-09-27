package middleware

import (
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type MiddlewareHttpHandler struct {
	service    MiddlewareService
	log        *zap.Logger
	production bool
}

func NewMiddlewareHttpHandler(service MiddlewareService, log *zap.Logger, production ...bool) *MiddlewareHttpHandler {
	prod := false
	if len(production) > 0 {
		prod = production[0]
	}
	return &MiddlewareHttpHandler{service: service, log: log, production: prod}
}

func (h *MiddlewareHttpHandler) CookieName() string {
	return utils.SessionCookieName(h.production)
}

func (h *MiddlewareHttpHandler) Session(sessionID string) (*model.SessionRecord, error) {
	return h.service.GetSession(sessionID)
}

func (h *MiddlewareHttpHandler) AuthMiddleware(c *fiber.Ctx) error {
	id := c.Cookies(h.CookieName())
	if id == "" {
		clearBrowserSessionCookie(c, h.production)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing session"})
	}

	session, err := h.service.GetSession(id)
	if err != nil {
		if errors.Is(err, ErrSessionMissing) {
			clearBrowserSessionCookie(c, h.production)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired session"})
		}
		h.log.Error("Read browser session", zap.Error(err))
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "session store unavailable"})
	}

	user, err := h.service.GetMe(session.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			clearBrowserSessionCookie(c, h.production)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid session user"})
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "user status unavailable"})
	}

	blacklisted, err := h.service.IsBlacklisted(user.Email, user.Id)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "access policy unavailable"})
	}
	if blacklisted {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead && c.Method() != fiber.MethodOptions {
		token := c.Get("X-CSRF-Token")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "invalid CSRF token"})
		}
	}

	c.Cookie(&fiber.Cookie{
		Name:     h.CookieName(),
		Value:    id,
		Path:     "/",
		MaxAge:   7 * 24 * 3600,
		HTTPOnly: true,
		Secure:   h.production,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
	c.Locals("user", user)
	c.Locals("session_id", id)
	c.Locals("csrf_token", session.CSRFToken)

	return c.Next()
}

func (h *MiddlewareHttpHandler) AdminMiddleware(c *fiber.Ctx) error {
	user := utils.GetUserProfileFromCtx(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	if !security.IsAdminRole(user.RoleId) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "admin access required"})
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

func clearBrowserSessionCookie(c *fiber.Ctx, production bool) {
	c.Cookie(&fiber.Cookie{
		Name:     utils.SessionCookieName(production),
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   production,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}
