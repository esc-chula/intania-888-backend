package auth

import (
	"crypto/subtle"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

// HTTPHandler adapts browser OAuth/session requests and administrator external-token operations.
type HTTPHandler struct {
	service    ServicePort
	cfg        config.Config
	sessions   SessionReader
	production bool
}

// NewHTTPHandler binds authentication and session services to the configured browser cookie policy.
func NewHTTPHandler(service ServicePort, sessions SessionReader, cfg config.Config, production bool) *HTTPHandler {
	return &HTTPHandler{service: service, sessions: sessions, cfg: cfg, production: production}
}

func (h *HTTPHandler) sessionName() string {
	return security.SessionCookieName(h.production)
}

func (h *HTTPHandler) oauthName() string {
	return security.OAuthCookieName(h.production)
}

// RegisterRoutes registers login, callback, logout, and authenticated profile routes.
// External-token administration requires both authentication and administrator middleware.
func (h *HTTPHandler) RegisterRoutes(router fiber.Router, authenticate, admin fiber.Handler) {
	router = router.Group("/auth")
	router.Get("/login", h.Login)
	router.Get("/callback", h.OAuthCallback)
	router.Post("/logout", h.Logout)
	router.Get("/me", authenticate, h.GetMe)

	// Deprecated: external-token management remains available only while the
	// external integration and its consumers are investigated.
	router.Post("/external-tokens", authenticate, admin, h.IssueExternalToken)
	router.Delete("/external-tokens/:id", authenticate, admin, h.RevokeExternalToken)
}

// RegisterExternalRoutes registers the legacy profile route behind external bearer authentication.
//
// Deprecated: external API consumers have not been confirmed; keep this route
// available until the original integration is understood.
func (h *HTTPHandler) RegisterExternalRoutes(router fiber.Router, authenticate fiber.Handler) {
	router.Get("/me", authenticate, h.GetExternalMe)
}

// Login returns the authorization URL and binds its state to a short-lived HttpOnly cookie.
// @Summary Start Google OAuth login
// @Description Retrieves a Google OAuth login URL and binds it to a short-lived browser cookie.
// @Tags Auth
// @Produce json
// @Success 200 {object} map[string]string "authorization URL"
// @Failure 400 {object} apierror.Response "redirect_to is not supported"
// @Failure 503 {object} apierror.Response "OAuth login unavailable"
// @Router /auth/login [get]
func (h *HTTPHandler) Login(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	if _, ok := c.Queries()["redirect_to"]; ok {
		return apierror.New(fiber.StatusBadRequest, "INVALID_REQUEST", "redirect_to is not supported")
	}

	login, err := h.service.StartOAuthLogin(c.UserContext())
	if err != nil || login == nil || login.URL == "" || login.State == "" {
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "OAuth login is unavailable")
	}

	ttl := 600
	if h.cfg != nil && h.cfg.GetOAuth().StateExpiration > 0 {
		ttl = h.cfg.GetOAuth().StateExpiration
	}

	c.Cookie(&fiber.Cookie{
		Name:     h.oauthName(),
		Value:    login.State,
		Path:     "/",
		MaxAge:   ttl,
		HTTPOnly: true,
		Secure:   h.production,
		SameSite: fiber.CookieSameSiteLaxMode,
	})

	return c.JSON(LoginResponse{URL: login.URL})
}

// OAuthCallback exchanges the browser-bound login request, sets the session cookie, and redirects to the fixed frontend URL.
// @Summary Complete Google OAuth login
// @Description Exchanges a state-bound Google authorization code and establishes a cookie session.
// @Tags Auth
// @Produce json
// @Param code query string true "OAuth authorization code"
// @Param state query string true "OAuth state nonce"
// @Success 302 {string} string "fixed frontend redirect"
// @Failure 400 {object} apierror.Response "invalid OAuth request"
// @Failure 403 {object} apierror.Response "email is not allowed"
// @Failure 500 {object} apierror.Response "post-login redirect is not configured"
// @Failure 503 {object} apierror.Response "OAuth login or access policy unavailable"
// @Router /auth/callback [get]
func (h *HTTPHandler) OAuthCallback(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	credentials, err := h.service.VerifyOAuthLogin(
		c.UserContext(),
		c.Query("code"),
		c.Query("state"),
		c.Cookies(h.oauthName()),
		c.Cookies(h.sessionName()),
	)
	if err != nil {
		return mapAuthError(err)
	}

	if credentials == nil || credentials.SessionID == "" {
		return apierror.New(fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Session service is unavailable")
	}

	redirect, err := h.postLoginRedirect(credentials.IsNewUser)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusInternalServerError, "INTERNAL_ERROR", "Post-login redirect is not configured")
	}

	c.Cookie(&fiber.Cookie{
		Name:     h.sessionName(),
		Value:    credentials.SessionID,
		Path:     "/",
		MaxAge:   h.cfg.GetSession().IdleTTLSeconds,
		HTTPOnly: true,
		Secure:   h.production,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
	h.clearCookie(c, h.oauthName(), true)

	return c.Redirect(redirect)
}

// Logout checks CSRF for an active browser session, revokes it, and clears its cookie.
// Missing or expired sessions succeed; backend revocation failures remain retryable.
// @Summary Log out of browser session
// @Description Revokes the active server-side session and clears its browser cookie. An absent or expired session is already logged out.
// @Tags Auth
// @Success 204 "logged out"
// @Failure 403 {object} apierror.Response "invalid CSRF token"
// @Failure 503 {object} apierror.Response "session store or revocation unavailable"
// @Router /auth/logout [post]
// @Security CookieSession
func (h *HTTPHandler) Logout(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	id := c.Cookies(h.sessionName())
	if id == "" {
		h.clearCookie(c, h.sessionName(), true)
		return c.SendStatus(204)
	}

	if h.sessions == nil {
		return apierror.New(fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Session service is unavailable")
	}

	session, err := h.sessions.Session(c.UserContext(), id)
	if err != nil {
		if errors.Is(err, middleware.ErrSessionMissing) {
			h.clearCookie(c, h.sessionName(), true)

			return c.SendStatus(204)
		}

		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Session service is unavailable")
	}

	token := c.Get("X-CSRF-Token")
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
		return apierror.New(fiber.StatusForbidden, "FORBIDDEN", "Invalid CSRF token")
	}

	if err := h.service.Logout(c.UserContext(), id); err != nil {
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Session service is unavailable")
	}

	h.clearCookie(c, h.sessionName(), true)

	return c.SendStatus(204)
}

// GetMe returns the authenticated browser profile and its session-bound CSRF token.
// @Summary Browser profile and CSRF token
// @Description Retrieves the profile and CSRF token associated with the browser session cookie.
// @Tags Auth
// @Produce json
// @Success 200 {object} MeResponse
// @Failure 401 {object} apierror.Response "unauthorized"
// @Failure 503 {object} apierror.Response "session or policy unavailable"
// @Router /auth/me [get]
// @Security CookieSession
func (h *HTTPHandler) GetMe(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	profile := httpidentity.GetProfile(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	csrfToken := httpidentity.CSRFToken(c)
	if csrfToken == "" {
		return apierror.New(fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Session service is unavailable")
	}

	return c.JSON(MeResponse{Profile: httpidentity.Response(profile), CSRFToken: csrfToken})
}

// GetExternalMe returns the bearer-authenticated profile without exposing browser CSRF state.
// @Summary External client profile
// @Description Retrieves user profile data through Bearer authentication.
// @Tags External
// @Deprecated
// @Produce json
// @Success 200 {object} map[string]interface{} "profile"
// @Failure 401 {object} apierror.Response "missing or invalid authorization"
// @Failure 503 {object} apierror.Response "token, user, or policy status unavailable"
// @Security BearerAuth
// @Router /external/me [get]
func (h *HTTPHandler) GetExternalMe(c *fiber.Ctx) error {
	profile := httpidentity.GetProfile(c)
	if profile == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	return c.JSON(ExternalMeResponse{Profile: httpidentity.Response(profile)})
}

// IssueExternalToken validates the selected account and returns its one-hour external JWT and revocation ID.
// @Summary Issue a one-hour external JWT
// @Tags Auth
// @Deprecated
// @Param request body externalTokenRequest true "existing user"
// @Success 201 {object} ExternalTokenResponse
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 503 {object} apierror.Response
// @Router /auth/external-tokens [post]
// @Security CookieSession
func (h *HTTPHandler) IssueExternalToken(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	var req externalTokenRequest
	if err := apierror.BindJSON(c, &req); err != nil {
		return err
	}

	issuer := httpidentity.GetProfile(c)
	if issuer == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	token, jti, err := h.service.IssueExternalToken(c.UserContext(), req.UserID)
	if err != nil {
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Token service is unavailable")
	}

	return c.Status(201).JSON(ExternalTokenResponse{Token: token, ID: jti, ExpiresIn: 3600})
}

// RevokeExternalToken revokes the selected external JWT identifier and returns an empty success response.
// @Summary Revoke an external JWT
// @Tags Auth
// @Deprecated
// @Param id path string true "token ID"
// @Success 204 "revoked"
// @Failure 401 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 503 {object} apierror.Response
// @Router /auth/external-tokens/{id} [delete]
// @Security CookieSession
func (h *HTTPHandler) RevokeExternalToken(c *fiber.Ctx) error {
	issuer := httpidentity.GetProfile(c)
	if issuer == nil {
		return apierror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}

	if strings.TrimSpace(c.Params("id")) == "" {
		return apierror.Invalid(map[string]string{"id": "is required"})
	}
	if err := h.service.RevokeExternalToken(c.UserContext(), c.Params("id")); err != nil {
		return apierror.Wrap(err, fiber.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Token service is unavailable")
	}

	return c.SendStatus(204)
}

func (h *HTTPHandler) clearCookie(c *fiber.Ctx, name string, httpOnly bool) {
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HTTPOnly: httpOnly,
		Secure:   strings.HasPrefix(name, "__Host-"),
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}

func (h *HTTPHandler) postLoginRedirect(isNew bool) (string, error) {
	raw := strings.TrimSpace(h.service.GetPostLoginRedirectURL())
	parsed, err := url.Parse(raw)

	if err != nil || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.Fragment != "" {
		return "", errors.New("invalid post-login redirect")
	}

	q := parsed.Query()
	q.Del("is_new_user")
	if isNew {
		q.Set("is_new_user", "true")
	}

	parsed.RawQuery = q.Encode()

	return parsed.String(), nil
}

func setNoStoreHeaders(c *fiber.Ctx) {
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderPragma, "no-cache")
	c.Set("Referrer-Policy", "no-referrer")
}
