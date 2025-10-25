package auth

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/gofiber/fiber/v2"
)

type AuthHttpHandler struct {
	service AuthService
}

func NewAuthHttpHandler(service AuthService) *AuthHttpHandler {
	return &AuthHttpHandler{service: service}
}

func (h *AuthHttpHandler) RegisterRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router = router.Group("/auth")

	router.Get("/login", h.Login)
	router.Get("/callback", h.OAuthCallback)        // Google redirects
	router.Post("/login/callback", h.LoginCallback) // Legacy: Frontend calls
	router.Post("/refresh", h.RefreshToken)
	router.Get("/me", mid.AuthMiddleware, h.GetMe)
}

func (h *AuthHttpHandler) RegisterExternalRoutes(router fiber.Router, mid *middleware.MiddlewareHttpHandler) {
	router.Get("/me", mid.ExternalAPIMiddleware, h.GetMe)
}

// @Summary Login URL
// @Description Retrieves the OAuth login URL
// @Tags Auth
// @Produce  json
// @Param   redirect_to  query  string  false  "URL to redirect to after successful authentication"
// @Success 200 {object} map[string]string "url"
// @Failure 400 {object} map[string]string "invalid redirect URL"
// @Failure 500 {object} map[string]string "internal server error"
// @Router  /auth/login [get]
func (h *AuthHttpHandler) Login(c *fiber.Ctx) error {
	redirectTo := c.Query("redirect_to")

	// Validate redirect URL if provided
	if redirectTo != "" && !h.service.IsAllowedRedirect(redirectTo) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid or unauthorized redirect URL"})
	}

	url, err := h.service.GetOAuthUrl(redirectTo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"url": url})
}

// @Summary OAuth Callback (Google redirects here)
// @Description Handles OAuth callback from Google, exchanges code for tokens, and redirects appropriately
// @Tags Auth
// @Param   code   query  string  true   "OAuth authorization code from Google"
// @Param   state  query  string  false  "State parameter with redirect URL for third parties"
// @Success 302 {string} string "redirect to frontend or third-party site"
// @Failure 400 {object} map[string]string "missing code"
// @Failure 500 {object} map[string]string "internal server error"
// @Router  /auth/callback [get]
func (h *AuthHttpHandler) OAuthCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing authorization code"})
	}

	credential, err := h.service.VerifyOAuthLogin(code)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if state != "" && h.service.IsAllowedRedirect(state) {
		redirectUrl := state
		if strings.Contains(state, "?") {
			redirectUrl += "&"
		} else {
			redirectUrl += "?"
		}
		redirectUrl += "access_token=" + url.QueryEscape(credential.AccessToken)
		redirectUrl += "&refresh_token=" + url.QueryEscape(credential.RefreshToken)
		redirectUrl += "&expires_in=" + url.QueryEscape(fmt.Sprintf("%d", credential.ExpiresIn))
		redirectUrl += "&is_new_user=" + url.QueryEscape(fmt.Sprintf("%t", credential.IsNewUser))

		return c.Redirect(redirectUrl)
	}

	frontendUrl := h.service.GetFrontendUrl()
	if frontendUrl == "" {
		frontendUrl = "http://localhost:3000"
	}

	redirectUrl := frontendUrl + "?access_token=" + url.QueryEscape(credential.AccessToken)
	redirectUrl += "&refresh_token=" + url.QueryEscape(credential.RefreshToken)
	redirectUrl += "&expires_in=" + url.QueryEscape(fmt.Sprintf("%d", credential.ExpiresIn))
	redirectUrl += "&is_new_user=" + url.QueryEscape(fmt.Sprintf("%t", credential.IsNewUser))

	return c.Redirect(redirectUrl)
}

// @Summary OAuth Login Callback
// @Description Verifies the OAuth login and returns credentials or redirects to third-party site
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param   oauthCode  body      model.OAuthCodeDto  true  "OAuth Code"
// @Param   state      query     string              false "State parameter with redirect URL"
// @Success 200 {object} map[string]interface{} "credential"
// @Success 302 {string} string "redirect to third-party site with credentials"
// @Failure 400 {object} map[string]string "cannot parse body"
// @Failure 500 {object} map[string]string "internal server error"
// @Router  /auth/login/callback [post]
func (h *AuthHttpHandler) LoginCallback(c *fiber.Ctx) error {
	var oauthCodeDto model.OAuthCodeDto
	if err := c.BodyParser(&oauthCodeDto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	state := c.Query("state")

	credential, err := h.service.VerifyOAuthLogin(oauthCodeDto.Code)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if state != "" && h.service.IsAllowedRedirect(state) {
		redirectUrl := state
		if strings.Contains(state, "?") {
			redirectUrl += "&"
		} else {
			redirectUrl += "?"
		}
		redirectUrl += "access_token=" + url.QueryEscape(credential.AccessToken)
		redirectUrl += "&refresh_token=" + url.QueryEscape(credential.RefreshToken)
		redirectUrl += "&expires_in=" + url.QueryEscape(fmt.Sprintf("%d", credential.ExpiresIn))
		redirectUrl += "&is_new_user=" + url.QueryEscape(fmt.Sprintf("%t", credential.IsNewUser))

		return c.Redirect(redirectUrl)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"credential": credential,
	})
}

// @Summary Refresh Token
// @Description Refreshes the access token using the refresh token
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param   refreshToken  body      model.RefreshTokenDto  true  "Refresh Token"
// @Success 200 {object} map[string]interface{} "credential"
// @Failure 400 {object} map[string]string "cannot parse body"
// @Failure 500 {object} map[string]string "internal server error"
// @Router  /auth/refresh [post]
func (h *AuthHttpHandler) RefreshToken(c *fiber.Ctx) error {
	var refreshTokenDto model.RefreshTokenDto
	if err := c.BodyParser(&refreshTokenDto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	credential, err := h.service.RefreshToken(refreshTokenDto.RefreshToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"credential": credential,
	})
}

// @Summary GetMe
// @Description Retrieves user profile data
// @Tags Auth
// @Produce  json
// @Success 200 {object} map[string]string "profile"
// @Failure 400 {object} map[string]string "bad request error"
// @Router  /auth/me [get]
func (h *AuthHttpHandler) GetMe(c *fiber.Ctx) error {
	userDto, ok := c.Locals("user").(*model.UserDto)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errors.New("not found user profile in context").Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"profile": userDto,
	})
}
