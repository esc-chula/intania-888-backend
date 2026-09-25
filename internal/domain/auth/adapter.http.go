package auth

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
)

const defaultOAuthStateCookieMaxAge = 10 * 60

type AuthHttpHandler struct {
	service AuthService
	cfg     config.Config
}

func NewAuthHttpHandler(service AuthService, cfg ...config.Config) *AuthHttpHandler {
	var handlerConfig config.Config
	if len(cfg) > 0 {
		handlerConfig = cfg[0]
	}
	return &AuthHttpHandler{service: service, cfg: handlerConfig}
}

func (h *AuthHttpHandler) RegisterRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router = router.Group("/auth")

	router.Get("/login", h.Login)
	router.Get("/callback", h.OAuthCallback)
	router.Post("/refresh", h.RefreshToken)
	router.Post("/logout", mid.AuthMiddleware, h.Logout)
	router.Get("/me", mid.AuthMiddleware, h.GetMe)
}

func (h *AuthHttpHandler) RegisterExternalRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router.Get("/me", mid.ExternalAPIMiddleware, h.GetExternalMe)
}

// @Summary Login URL
// @Description Retrieves a Google OAuth login URL and binds it to a short-lived browser cookie.
// @Tags Auth
// @Produce json
// @Success 200 {object} map[string]string "authorization URL"
// @Failure 400 {object} map[string]string "redirect_to is not supported"
// @Failure 500 {object} map[string]string "internal server error"
// @Router /auth/login [get]
func (h *AuthHttpHandler) Login(c *fiber.Ctx) error {
	if _, supplied := c.Queries()["redirect_to"]; supplied {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "redirect_to is not supported",
		})
	}

	login, err := h.service.StartOAuthLogin()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to start OAuth login",
		})
	}

	secure, sameSite := h.cookiePolicy()
	c.Cookie(&fiber.Cookie{
		Name:     utils.OAuthStateCookieName,
		Value:    login.State,
		Path:     "/",
		MaxAge:   h.oauthStateMaxAge(),
		HTTPOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})

	return c.JSON(fiber.Map{"url": login.URL})
}

// @Summary OAuth Callback
// @Description Exchanges a state-bound Google authorization code and establishes a cookie session.
// @Tags Auth
// @Produce json
// @Param code query string true "OAuth authorization code"
// @Param state query string true "OAuth state nonce"
// @Success 302 {string} string "fixed frontend redirect"
// @Failure 400 {object} map[string]string "invalid OAuth request"
// @Failure 403 {object} map[string]string "email is not allowed"
// @Failure 500 {object} map[string]string "internal server error"
// @Router /auth/callback [get]
func (h *AuthHttpHandler) OAuthCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")
	cookieState := c.Cookies(utils.OAuthStateCookieName)
	if code == "" || state == "" || cookieState == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid OAuth callback",
		})
	}

	credentials, err := h.service.VerifyOAuthLogin(code, state, cookieState)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidOAuthState):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid or expired OAuth state",
			})
		case errors.Is(err, ErrUnverifiedEmail), errors.Is(err, ErrEmailNotAllowed):
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "email is not allowed",
			})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "unable to complete OAuth login",
			})
		}
	}

	if err := h.setSessionCookies(c, credentials); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to establish session",
		})
	}
	h.clearCookie(c, utils.OAuthStateCookieName, true)

	redirectURL, err := h.postLoginRedirect(credentials.IsNewUser)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "post-login redirect is not configured",
		})
	}
	return c.Redirect(redirectURL)
}

// @Summary Refresh Session
// @Description Rotates the refresh token from the HttpOnly cookie and replaces the session cookies.
// @Tags Auth
// @Success 204 "session refreshed"
// @Failure 401 {object} map[string]string "invalid or replayed refresh token"
// @Router /auth/refresh [post]
func (h *AuthHttpHandler) RefreshToken(c *fiber.Ctx) error {
	refreshToken := c.Cookies(utils.RefreshTokenCookieName)
	if refreshToken == "" {
		h.clearAuthenticationCookies(c)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing refresh cookie",
		})
	}

	credentials, err := h.service.RefreshToken(refreshToken)
	if err != nil {
		h.clearAuthenticationCookies(c)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid or replayed refresh token",
		})
	}
	if err := h.setSessionCookies(c, credentials); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to establish session",
		})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Logout
// @Description Revokes the active server-side session and clears authentication cookies.
// @Tags Auth
// @Success 204 "logged out"
// @Failure 500 {object} map[string]string "internal server error"
// @Router /auth/logout [post]
func (h *AuthHttpHandler) Logout(c *fiber.Ctx) error {
	sessionID, _ := c.Locals("session_id").(string)
	err := h.service.Logout(sessionID)
	h.clearAuthenticationCookies(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to revoke session",
		})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary GetMe
// @Description Retrieves the profile associated with the access cookie.
// @Tags Auth
// @Produce json
// @Success 200 {object} map[string]string "profile"
// @Failure 401 {object} map[string]string "unauthorized"
// @Router /auth/me [get]
func (h *AuthHttpHandler) GetMe(c *fiber.Ctx) error {
	userDto, ok := c.Locals("user").(*model.UserDto)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user profile not found"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"profile": userDto})
}

// @Summary Get profile (External API)
// @Description Retrieves user profile data through Bearer authentication.
// @Tags External
// @Produce json
// @Success 200 {object} map[string]interface{} "profile"
// @Failure 401 {object} map[string]string "missing or invalid authorization"
// @Router /external/me [get]
// @Security BearerAuth
func (h *AuthHttpHandler) GetExternalMe(c *fiber.Ctx) error {
	return h.GetMe(c)
}

func (h *AuthHttpHandler) setSessionCookies(c *fiber.Ctx, credentials *SessionCredentials) error {
	if credentials == nil || credentials.AccessToken == "" || credentials.RefreshToken == "" {
		return errors.New("empty session credentials")
	}
	csrfToken, err := utils.NewOpaqueToken(32)
	if err != nil {
		return err
	}
	secure, sameSite := h.cookiePolicy()
	c.Cookie(&fiber.Cookie{
		Name:     utils.AccessTokenCookieName,
		Value:    credentials.AccessToken,
		Path:     "/",
		MaxAge:   int(credentials.ExpiresIn),
		HTTPOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
	c.Cookie(&fiber.Cookie{
		Name:     utils.RefreshTokenCookieName,
		Value:    credentials.RefreshToken,
		Path:     "/",
		MaxAge:   h.refreshTokenMaxAge(),
		HTTPOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
	c.Cookie(&fiber.Cookie{
		Name:     utils.CSRFTokenCookieName,
		Value:    csrfToken,
		Path:     "/",
		MaxAge:   h.refreshTokenMaxAge(),
		Secure:   secure,
		SameSite: sameSite,
	})
	return nil
}

func (h *AuthHttpHandler) clearAuthenticationCookies(c *fiber.Ctx) {
	h.clearCookie(c, utils.AccessTokenCookieName, true)
	h.clearCookie(c, utils.RefreshTokenCookieName, true)
	h.clearCookie(c, utils.CSRFTokenCookieName, false)
	h.clearCookie(c, utils.OAuthStateCookieName, true)
}

func (h *AuthHttpHandler) clearCookie(c *fiber.Ctx, name string, httpOnly bool) {
	secure, sameSite := h.cookiePolicy()
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HTTPOnly: httpOnly,
		Secure:   secure,
		SameSite: sameSite,
	})
}

func (h *AuthHttpHandler) postLoginRedirect(isNewUser bool) (string, error) {
	redirectURL := strings.TrimSpace(h.service.GetPostLoginRedirectURL())
	parsedURL, err := url.Parse(redirectURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" || parsedURL.User != nil || parsedURL.Fragment != "" {
		return "", errors.New("invalid post-login redirect URL")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", errors.New("invalid post-login redirect scheme")
	}
	query := parsedURL.Query()
	query.Del("is_new_user")
	if isNewUser {
		query.Set("is_new_user", "true")
	}
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func (h *AuthHttpHandler) cookiePolicy() (bool, string) {
	secure := false
	sameSite := fiber.CookieSameSiteLaxMode
	if h.cfg != nil {
		oauthConfig := h.cfg.GetOAuth()
		secure = oauthConfig.CookieSecure
		serverEnv := strings.ToLower(strings.TrimSpace(h.cfg.GetServer().Env))
		secure = secure || serverEnv == "prod" || serverEnv == "production"
		if strings.EqualFold(oauthConfig.CookieSameSite, fiber.CookieSameSiteNoneMode) {
			sameSite = fiber.CookieSameSiteNoneMode
		} else if strings.EqualFold(oauthConfig.CookieSameSite, fiber.CookieSameSiteStrictMode) {
			sameSite = fiber.CookieSameSiteStrictMode
		}
	}
	if sameSite == fiber.CookieSameSiteNoneMode && !secure {
		sameSite = fiber.CookieSameSiteLaxMode
	}
	return secure, sameSite
}

func (h *AuthHttpHandler) oauthStateMaxAge() int {
	if h.cfg != nil && h.cfg.GetOAuth().StateExpiration > 0 {
		return h.cfg.GetOAuth().StateExpiration
	}
	return defaultOAuthStateCookieMaxAge
}

func (h *AuthHttpHandler) refreshTokenMaxAge() int {
	if h.cfg != nil && h.cfg.GetJwt().RefreshTokenExpiration > 0 {
		return h.cfg.GetJwt().RefreshTokenExpiration
	}
	return 24 * 60 * 60
}
