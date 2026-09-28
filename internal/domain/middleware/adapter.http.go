package middleware

import (
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type MiddlewareHttpHandler struct {
	service        MiddlewareService
	log            *zap.Logger
	production     bool
	sessionIdleTTL int
}

func NewMiddlewareHttpHandler(
	service MiddlewareService,
	log *zap.Logger,
	production bool,
	sessionIdleTTL int,
) *MiddlewareHttpHandler {
	return &MiddlewareHttpHandler{
		service:        service,
		log:            log,
		production:     production,
		sessionIdleTTL: sessionIdleTTL,
	}
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
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	session, err := h.service.GetSession(id)
	if err != nil {
		if errors.Is(err, ErrSessionMissing) {
			clearBrowserSessionCookie(c, h.production)
			return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		}
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Session service is unavailable")
	}

	user, err := h.service.GetMe(session.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			clearBrowserSessionCookie(c, h.production)
			return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		}
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "User status is unavailable")
	}

	blacklisted, err := h.service.IsBlacklisted(user.Email, user.Id)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Access policy is unavailable")
	}
	if blacklisted {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead && c.Method() != fiber.MethodOptions {
		token := c.Get("X-CSRF-Token")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
			return apierror.New(fiber.StatusForbidden, "FORBIDDEN", "Invalid CSRF token")
		}
	}

	c.Cookie(&fiber.Cookie{
		Name:     h.CookieName(),
		Value:    id,
		Path:     "/",
		MaxAge:   h.sessionIdleTTL,
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
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	if !security.IsAdminRole(user.RoleId) {
		return apierror.New(fiber.StatusForbidden, "FORBIDDEN", "Administrator permission required")
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
