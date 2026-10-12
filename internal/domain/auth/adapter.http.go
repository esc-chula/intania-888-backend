package auth

import (
	"crypto/subtle"
	"errors"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/httpcookie"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
	"github.com/esc-chula/intania-888-backend/internal/httplimit"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

// HTTPHandler adapts registered login, browser sessions, and delegated profiles.
type HTTPHandler struct {
	service      ServicePort
	cfg          config.Config
	sessions     SessionReader
	production   bool
	applications *ApplicationHTTPHandler
}

// NewHTTPHandler binds authentication and session services to the configured browser cookie policy.
func NewHTTPHandler(service ServicePort, sessions SessionReader, cfg config.Config, production bool) *HTTPHandler {
	return &HTTPHandler{
		service:    service,
		sessions:   sessions,
		cfg:        cfg,
		production: production,
	}
}

func (h *HTTPHandler) sessionName() string {
	return security.SessionCookieName(h.production)
}

// RegisterRoutes registers login, callback, logout, and authenticated profile routes.
func (h *HTTPHandler) RegisterRoutes(router fiber.Router, authenticate fiber.Handler) {
	router = router.Group("/auth")
	if h.applications == nil {
		panic("application authentication must be configured before registering routes")
	}

	router.Get("/login", authRateMiddleware(h.applications.limits.Login), h.applications.Login)
	router.Get("/callback", authRateMiddleware(h.applications.limits.Callback), h.applications.Callback)
	router.Get("/authorize", authRateMiddleware(h.applications.limits.Login), h.applications.Authorize)
	router.Post("/token", h.applications.Token)
	router.Post("/revoke", h.applications.Revoke)
	router.Post("/logout", h.Logout)
	router.Get("/me", authenticate, h.GetMe)
}

// RegisterExternalRoutes registers the profile route with scoped authentication and its user limit.
func (h *HTTPHandler) RegisterExternalRoutes(
	router fiber.Router,
	requireScope func(string, ...httplimit.Check) fiber.Handler,
	profileRateLimit httplimit.Check,
) {
	router.Get("/me", requireScope(config.ScopeProfileRead, profileRateLimit), h.GetExternalMe)
}

// Logout checks CSRF for an active browser session, revokes it, and clears its cookie.
// Missing or expired sessions succeed; backend revocation failures remain retryable.
func (h *HTTPHandler) Logout(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	id := c.Cookies(h.sessionName())
	if id == "" {
		h.clearCookie(c, h.sessionName(), true)

		return c.SendStatus(fiber.StatusNoContent)
	}

	if h.sessions == nil {
		return apierror.New(fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "Session service is unavailable")
	}

	session, err := h.sessions.Session(c.UserContext(), id)
	if err != nil {
		if errors.Is(err, middleware.ErrSessionMissing) {
			h.clearCookie(c, h.sessionName(), true)

			return c.SendStatus(fiber.StatusNoContent)
		}

		return apierror.Wrap(err, fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "Session service is unavailable")
	}

	token := c.Get("X-CSRF-Token")
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
		return apierror.New(fiber.StatusForbidden, apierror.CodeForbidden, "Invalid CSRF token")
	}

	if err := h.service.Logout(c.UserContext(), id); err != nil {
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "Session service is unavailable")
	}

	h.clearCookie(c, h.sessionName(), true)

	return c.SendStatus(fiber.StatusNoContent)
}

// GetMe returns the authenticated browser profile and its session-bound CSRF token.
func (h *HTTPHandler) GetMe(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	profile := httpidentity.GetProfile(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
	}

	csrfToken := httpidentity.CSRFToken(c)
	if csrfToken == "" {
		return apierror.New(fiber.StatusServiceUnavailable, apierror.CodeDependencyUnavailable, "Session service is unavailable")
	}

	return c.JSON(MeResponse{
		Profile:   httpidentity.Response(profile),
		CSRFToken: csrfToken,
	})
}

// GetExternalMe returns the bearer-authenticated profile without exposing browser CSRF state.
func (h *HTTPHandler) GetExternalMe(c *fiber.Ctx) error {
	profile := httpidentity.GetProfile(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, apierror.CodeUnauthorized, "Authentication required")
	}

	return c.JSON(ExternalMeResponse{Profile: httpidentity.Response(profile)})
}

func (h *HTTPHandler) clearCookie(c *fiber.Ctx, name string, httpOnly bool) {
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HTTPOnly: httpOnly,
		Secure:   true,
		SameSite: httpcookie.SameSite(h.production),
	})
}

func setNoStoreHeaders(c *fiber.Ctx) {
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderPragma, "no-cache")
	c.Set("Referrer-Policy", "no-referrer")
}
