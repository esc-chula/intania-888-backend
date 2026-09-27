package auth

import (
	"crypto/subtle"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
)

type AuthHttpHandler struct {
	service    AuthService
	cfg        config.Config
	mid        *middleware.MiddlewareHttpHandler
	production bool
}

func NewAuthHttpHandler(service AuthService, cfg config.Config, production bool) *AuthHttpHandler {
	return &AuthHttpHandler{service: service, cfg: cfg, production: production}
}

func (h *AuthHttpHandler) sessionName() string {
	return utils.SessionCookieName(h.production)
}

func (h *AuthHttpHandler) oauthName() string {
	return utils.OAuthCookieName(h.production)
}

func (h *AuthHttpHandler) RegisterRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	h.mid = mid
	router = router.Group("/auth")
	router.Get("/login", h.Login)
	router.Get("/callback", h.OAuthCallback)
	router.Post("/logout", h.Logout)
	router.Get("/me", mid.AuthMiddleware, h.GetMe)
	// Deprecated: external-token management remains available only while the
	// external integration and its consumers are investigated.
	router.Post("/external-tokens", mid.AuthMiddleware, mid.AdminMiddleware, h.IssueExternalToken)
	router.Delete("/external-tokens/:id", mid.AuthMiddleware, mid.AdminMiddleware, h.RevokeExternalToken)
}

// Deprecated: external API consumers have not been confirmed; keep this route
// available until the original integration is understood.
func (h *AuthHttpHandler) RegisterExternalRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router.Get("/me", mid.ExternalAPIMiddleware, h.GetExternalMe)
}

// @Summary Start Google OAuth login
// @Description Retrieves a Google OAuth login URL and binds it to a short-lived browser cookie.
// @Tags Auth
// @Produce json
// @Success 200 {object} map[string]string "authorization URL"
// @Failure 400 {object} map[string]string "redirect_to is not supported"
// @Failure 503 {object} map[string]string "OAuth login unavailable"
// @Router /auth/login [get]
func (h *AuthHttpHandler) Login(c *fiber.Ctx) error {
	setNoStoreHeaders(c)
	h.clearLegacyCookies(c)
	if _, ok := c.Queries()["redirect_to"]; ok {
		return c.Status(400).JSON(fiber.Map{"error": "redirect_to is not supported"})
	}
	login, err := h.service.StartOAuthLogin()
	if err != nil || login == nil || login.URL == "" || login.State == "" {
		return c.Status(503).JSON(fiber.Map{"error": "unable to start OAuth login"})
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
	return c.JSON(fiber.Map{"url": login.URL})
}

// @Summary Complete Google OAuth login
// @Description Exchanges a state-bound Google authorization code and establishes a cookie session.
// @Tags Auth
// @Produce json
// @Param code query string true "OAuth authorization code"
// @Param state query string true "OAuth state nonce"
// @Success 302 {string} string "fixed frontend redirect"
// @Failure 400 {object} map[string]string "invalid OAuth request"
// @Failure 403 {object} map[string]string "email is not allowed"
// @Failure 500 {object} map[string]string "post-login redirect is not configured"
// @Failure 503 {object} map[string]string "OAuth login or access policy unavailable"
// @Router /auth/callback [get]
func (h *AuthHttpHandler) OAuthCallback(c *fiber.Ctx) error {
	setNoStoreHeaders(c)
	credentials, err := h.service.VerifyOAuthLogin(
		c.Query("code"),
		c.Query("state"),
		c.Cookies(h.oauthName()),
		c.Cookies(h.sessionName()),
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidOAuthState):
			return c.Status(400).JSON(fiber.Map{"error": "invalid or expired OAuth state"})
		case errors.Is(err, ErrUnverifiedEmail), errors.Is(err, ErrEmailNotAllowed):
			return c.Status(403).JSON(fiber.Map{"error": "email is not allowed"})
		case errors.Is(err, security.ErrPolicyUnavailable):
			return c.Status(503).JSON(fiber.Map{"error": "access policy unavailable"})
		default:
			return c.Status(503).JSON(fiber.Map{"error": "unable to complete OAuth login"})
		}
	}
	if credentials == nil || credentials.SessionID == "" {
		return c.Status(503).JSON(fiber.Map{"error": "unable to establish session"})
	}
	redirect, err := h.postLoginRedirect(credentials.IsNewUser)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "post-login redirect is not configured"})
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
	h.clearLegacyCookies(c)
	return c.Redirect(redirect)
}

// @Summary Log out of browser session
// @Description Revokes the active server-side session and clears its browser cookie. An absent or expired session is already logged out.
// @Tags Auth
// @Success 204 "logged out"
// @Failure 403 {object} map[string]string "invalid CSRF token"
// @Failure 503 {object} map[string]string "session store or revocation unavailable"
// @Router /auth/logout [post]
func (h *AuthHttpHandler) Logout(c *fiber.Ctx) error {
	setNoStoreHeaders(c)
	id := c.Cookies(h.sessionName())
	if id == "" {
		h.clearCookie(c, h.sessionName(), true)
		return c.SendStatus(204)
	}
	if h.mid == nil {
		return c.Status(503).JSON(fiber.Map{"error": "session service unavailable"})
	}
	session, err := h.mid.Session(id)
	if err != nil {
		if errors.Is(err, middleware.ErrSessionMissing) {
			h.clearCookie(c, h.sessionName(), true)
			return c.SendStatus(204)
		}
		return c.Status(503).JSON(fiber.Map{"error": "session store unavailable"})
	}
	token := c.Get("X-CSRF-Token")
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
		return c.Status(403).JSON(fiber.Map{"error": "invalid CSRF token"})
	}
	if err := h.service.Logout(id); err != nil {
		return c.Status(503).JSON(fiber.Map{"error": "unable to revoke session"})
	}
	h.clearCookie(c, h.sessionName(), true)
	return c.SendStatus(204)
}

type MeResponse struct {
	Profile   *model.UserDto `json:"profile"`
	CSRFToken string         `json:"csrf_token"`
}

// @Summary Browser profile and CSRF token
// @Description Retrieves the profile and CSRF token associated with the browser session cookie.
// @Tags Auth
// @Produce json
// @Success 200 {object} MeResponse
// @Failure 401 {object} map[string]string "unauthorized"
// @Failure 503 {object} map[string]string "session or policy unavailable"
// @Router /auth/me [get]
func (h *AuthHttpHandler) GetMe(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	profile, _ := c.Locals("user").(*model.UserDto)
	if profile == nil {
		return c.SendStatus(401)
	}

	csrfToken, _ := c.Locals("csrf_token").(string)
	if csrfToken == "" {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}

	return c.JSON(MeResponse{Profile: profile, CSRFToken: csrfToken})
}

// @Summary External client profile
// @Description Retrieves user profile data through Bearer authentication.
// @Tags External
// @Deprecated
// @Produce json
// @Success 200 {object} map[string]interface{} "profile"
// @Failure 401 {object} map[string]string "missing or invalid authorization"
// @Failure 503 {object} map[string]string "token, user, or policy status unavailable"
// @Security BearerAuth
// @Router /external/me [get]
func (h *AuthHttpHandler) GetExternalMe(c *fiber.Ctx) error {
	profile, _ := c.Locals("user").(*model.UserDto)
	if profile == nil {
		return c.SendStatus(401)
	}

	return c.JSON(fiber.Map{"profile": profile})
}

type ExternalTokenResponse struct {
	Token     string `json:"token"`
	ID        string `json:"id"`
	ExpiresIn int    `json:"expires_in"`
}

type externalTokenRequest struct {
	UserID string `json:"user_id"`
}

// @Summary Issue a one-hour external JWT
// @Tags Auth
// @Deprecated
// @Param request body externalTokenRequest true "existing user"
// @Success 201 {object} ExternalTokenResponse
// @Router /auth/external-tokens [post]
func (h *AuthHttpHandler) IssueExternalToken(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	var req externalTokenRequest
	if err := c.BodyParser(&req); err != nil || req.UserID == "" {
		return c.SendStatus(400)
	}

	issuer := utils.GetUserProfileFromCtx(c)
	if issuer == nil {
		return c.SendStatus(401)
	}

	token, jti, err := h.service.IssueExternalToken(req.UserID)
	if err != nil {
		return c.Status(503).JSON(fiber.Map{"error": "unable to issue token"})
	}

	return c.Status(201).JSON(ExternalTokenResponse{Token: token, ID: jti, ExpiresIn: 3600})
}

// @Summary Revoke an external JWT
// @Tags Auth
// @Deprecated
// @Param id path string true "token ID"
// @Success 204 "revoked"
// @Router /auth/external-tokens/{id} [delete]
func (h *AuthHttpHandler) RevokeExternalToken(c *fiber.Ctx) error {
	issuer := utils.GetUserProfileFromCtx(c)
	if issuer == nil {
		return c.SendStatus(401)
	}

	if err := h.service.RevokeExternalToken(c.Params("id")); err != nil {
		return c.Status(503).JSON(fiber.Map{"error": "unable to revoke token"})
	}

	return c.SendStatus(204)
}

func (h *AuthHttpHandler) clearCookie(c *fiber.Ctx, name string, httpOnly bool) {
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

func (h *AuthHttpHandler) clearLegacyCookies(c *fiber.Ctx) {
	for _, name := range []string{"access_token", "refresh_token", "csrf_token", "oauth_state"} {
		h.clearCookie(c, name, name != "csrf_token")
	}
}

func (h *AuthHttpHandler) postLoginRedirect(isNew bool) (string, error) {
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
