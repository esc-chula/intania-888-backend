package middleware

import (
	"errors"
	"strings"

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
	// Check if the request is coming from Postman, Proxyman, curl, or other HTTP clients
	userAgent := c.Get("User-Agent")
	if isNonBrowserRequest(userAgent) {
		h.log.Named("AuthMiddleware").Error("Request from non-browser blocked", zap.String("User-Agent", userAgent))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Requests from non-browser tools are not allowed",
		})
	}

	// Check for browser-specific headers
	if !isBrowserHeadersValid(c) {
		h.log.Named("AuthMiddleware").Error("Invalid browser headers")
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Invalid browser headers",
		})
	}

	// Extract token from the header
	header := c.Get("Authorization")
	if header == "" {
		h.log.Named("AuthMiddleware").Error("Get header: ", zap.Error(errors.New("missing authorization header")))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing authorization header",
		})
	}

	token := strings.Split(header, " ")
	if len(token) != 2 || token[0] != "Bearer" || token[1] == "" {
		h.log.Named("AuthMiddleware").Error("split Bearer and access_token: ", zap.Error(errors.New("error while split header")))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing authorization header",
		})
	}

	// Verify the token
	userId, err := h.service.VerifyToken(token[1])
	if err != nil {
		h.log.Named("AuthMiddleware").Error("Token verification failed", zap.Error(err))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid or expired token",
		})
	}

	// Get the user profile
	userDto, err := h.service.GetMe(*userId)
	if err != nil {
		h.log.Named("AuthMiddleware").Error("User not found", zap.Error(err))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	// Store user in context for downstream handlers
	c.Locals("user", userDto)

	return c.Next()
}

// Helper function to check if the request is from non-browser clients
func isNonBrowserRequest(userAgent string) bool {
	nonBrowserAgents := []string{"postman", "proxyman", "curl", "httpie", "insomnia"}
	userAgent = strings.ToLower(userAgent)

	for _, agent := range nonBrowserAgents {
		if strings.Contains(userAgent, agent) {
			return true
		}
	}
	return false
}

// Helper function to check browser-specific headers
func isBrowserHeadersValid(c *fiber.Ctx) bool {
	acceptLanguage := c.Get("Accept-Language")
	acceptEncoding := c.Get("Accept-Encoding")
	secFetchMode := c.Get("Sec-Fetch-Mode")

	// Check if common browser headers are present
	return acceptLanguage != "" && acceptEncoding != "" && secFetchMode != ""
}
