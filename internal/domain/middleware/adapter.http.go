package middleware

import (
	"errors"
	"slices"
	"strings"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/utils"
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
			"error": "Idiot",
		})
	}

	// Check for browser-specific headers
	if !isBrowserHeadersValid(c) {
		h.log.Named("AuthMiddleware").Error("Invalid browser headers")
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Idiot",
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

	if isInBlacklists(userDto) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing authorization header",
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

func (h *MiddlewareHttpHandler) AdminMiddleware(c *fiber.Ctx) error {
	user := utils.GetUserProfileFromCtx(c)
	if user == nil {
		h.log.Named("AdminMiddleware").Error("User not found in context")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// Check if user is hardcoded admin or has ADMIN role
	authorizedAdmins := []string{
		"6633165121@student.chula.ac.th",
		"6738086221@student.chula.ac.th",
		"6633149121@student.chula.ac.th",
	}
	isAuthorizedAdmin := false
	for _, adminEmail := range authorizedAdmins {
		if user.Email == adminEmail {
			isAuthorizedAdmin = true
			break
		}
	}

	if !isAuthorizedAdmin && user.RoleId != "ADMIN" {
		h.log.Named("AdminMiddleware").Warn("Non-admin attempted admin action",
			zap.String("user_id", user.Id),
			zap.String("role", user.RoleId),
			zap.String("email", user.Email),
			zap.String("endpoint", c.Path()))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "admin access required",
		})
	}

	return c.Next()
}

func isInBlacklists(user *model.UserDto) bool {
	blacklistEmail := []string{
		"6530162621@student.chula.ac.th",
		"6633129621@student.chula.ac.th",
		"6733023821@student.chula.ac.th",
		"6630054621@student.chula.ac.th",
		"6538004621@student.chula.ac.th",
		"6733291621@student.chula.ac.th",
		"6430039021@student.chula.ac.th",
	}

	blacklistId := []string{
		"115982048644097094953",
		"101935624102444830754",
	}

	if found := slices.Contains(blacklistEmail, user.Email); found {
		return true
	}

	if found := slices.Contains(blacklistId, user.Id); found {
		return true
	}

	return false
}
