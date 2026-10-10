package middleware

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/httpcookie"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
	"github.com/esc-chula/intania-888-backend/internal/httplimit"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
)

// HTTPHandler authenticates browser sessions and enforces administrator permissions.
type HTTPHandler struct {
	browserLimit         httplimit.Check
	externalClientLimit  httplimit.Check
	invalidExternalLimit httplimit.Check
	service              ServicePort
	production           bool
	sessionIdleTTL       int
}

// NewHTTPHandler binds identity checks to the browser cookie environment and idle lifetime in seconds.
func NewHTTPHandler(
	service ServicePort,
	production bool,
	sessionIdleTTL int,
) *HTTPHandler {
	return &HTTPHandler{
		service:        service,
		production:     production,
		sessionIdleTTL: sessionIdleTTL,
	}
}

// CookieName returns the environment-specific browser session cookie name.
func (h *HTTPHandler) CookieName() string {
	return security.SessionCookieName(h.production)
}

// Session loads and renews the session needed by the authentication logout adapter.
func (h *HTTPHandler) Session(ctx context.Context, sessionID string) (*security.Session, error) {
	return h.service.GetSession(ctx, sessionID)
}

// AuthMiddleware checks session, current account, blacklist status, and CSRF on mutations.
// It renews the browser cookie and attaches the authenticated profile and session context before continuing.
func (h *HTTPHandler) AuthMiddleware(c *fiber.Ctx) error {
	id := c.Cookies(h.CookieName())
	if id == "" {
		clearBrowserSessionCookie(c, h.production)

		return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
	}

	session, err := h.service.GetSession(c.UserContext(), id)
	if err != nil {
		if errors.Is(err, ErrSessionMissing) {
			clearBrowserSessionCookie(c, h.production)

			return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
		}

		return apierror.Wrap(err, fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "Session service is unavailable")
	}

	if h.browserLimit != nil {
		if err := h.browserLimit(c, session.UserID); err != nil {
			return err
		}
	}

	user, err := h.service.GetMe(c.UserContext(), session.UserID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			clearBrowserSessionCookie(c, h.production)

			return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
		}

		return apierror.Wrap(err, fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "User status is unavailable")
	}

	blacklisted, err := h.service.IsBlacklisted(c.UserContext(), user.Email, user.ID)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "Access policy is unavailable")
	}
	if blacklisted {
		return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
	}

	if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead && c.Method() != fiber.MethodOptions {
		token := c.Get("X-CSRF-Token")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
			return apierror.New(fiber.StatusForbidden, apierror.CodeForbidden, "Invalid CSRF token")
		}
	}

	c.Cookie(&fiber.Cookie{
		Name:     h.CookieName(),
		Value:    id,
		Path:     "/",
		MaxAge:   h.sessionIdleTTL,
		HTTPOnly: true,
		Secure:   true,
		SameSite: httpcookie.SameSite(h.production),
	})
	httpidentity.SetProfile(c, user)
	httpidentity.SetSession(c, id, session.CSRFToken)

	return c.Next()
}

// AdminMiddleware requires an authenticated administrator profile set by preceding authentication middleware.
func (h *HTTPHandler) AdminMiddleware(c *fiber.Ctx) error {
	user := httpidentity.GetProfile(c)
	if user == nil {
		return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
	}

	if !security.IsAdminRole(user.RoleID) {
		return apierror.New(fiber.StatusForbidden, apierror.CodeForbidden, "Administrator permission required")
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
		Name:     security.SessionCookieName(production),
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HTTPOnly: true,
		Secure:   true,
		SameSite: httpcookie.SameSite(production),
	})
}

// ConfigureRateLimits installs checks at session, signed-client and rejected-credential boundaries.
func (h *HTTPHandler) ConfigureRateLimits(browser, externalClient, invalidExternal httplimit.Check) {
	h.browserLimit = browser
	h.externalClientLimit = externalClient
	h.invalidExternalLimit = invalidExternal
}
