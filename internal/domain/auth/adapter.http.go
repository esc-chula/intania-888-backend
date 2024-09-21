package auth

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/wiraphatys/intania888/internal/domain/middleware"
	"github.com/wiraphatys/intania888/internal/model"
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
	router.Post("/login/callback", h.LoginCallback)
	router.Post("/refresh", h.RefreshToken)
	router.Get("/me", mid.AuthMiddleware, h.GetMe)
}

// @Summary Login URL
// @Description Retrieves the OAuth login URL
// @Tags Auth
// @Produce  json
// @Success 200 {object} map[string]string "url"
// @Failure 500 {object} map[string]string "internal server error"
// @Router  /auth/login [get]
func (h *AuthHttpHandler) Login(c *fiber.Ctx) error {
	url, err := h.service.GetOAuthUrl()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"url": url})
}

// @Summary OAuth Login Callback
// @Description Verifies the OAuth login and returns credentials
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param   oauthCode  body      model.OAuthCodeDto  true  "OAuth Code"
// @Success 200 {object} map[string]interface{} "credential"
// @Failure 400 {object} map[string]string "cannot parse body"
// @Failure 500 {object} map[string]string "internal server error"
// @Router  /auth/login/callback [post]
func (h *AuthHttpHandler) LoginCallback(c *fiber.Ctx) error {
	var oauthCodeDto model.OAuthCodeDto
	if err := c.BodyParser(&oauthCodeDto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	credential, err := h.service.VerifyOAuthLogin(oauthCodeDto.Code)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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
