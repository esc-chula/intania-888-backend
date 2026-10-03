package server

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

// OriginGuard enforces exact configured browser origins and permits safe reads without Origin.
// External routes and the OAuth callback are exempt from this browser origin policy.
func (s *FiberHTTPServer) OriginGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if isExternalPath(c.Path()) || c.Path() == "/api/v1/auth/callback" ||
			(c.Method() == fiber.MethodPost && (c.Path() == "/api/v1/auth/token" || c.Path() == "/api/v1/auth/revoke")) {
			return c.Next()
		}

		origin := strings.TrimSpace(c.Get(fiber.HeaderOrigin))
		if origin == "" && (c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead) {
			return c.Next()
		}
		if !s.isAllowedOrigin(origin) {
			return apierror.New(fiber.StatusForbidden, apierror.CodeForbidden, "Origin is not allowed")
		}

		return c.Next()
	}
}

func (s *FiberHTTPServer) isAllowedOrigin(origin string) bool {
	canonical, err := canonicalOrigin(origin)
	if err != nil {
		return false
	}
	_, ok := s.allowedOrigins[canonical]

	return ok
}

func isExternalPath(path string) bool {
	return path == "/api/v1/external" || strings.HasPrefix(path, "/api/v1/external/")
}

func parseAllowedOrigins(rawOrigins string) (map[string]struct{}, error) {
	allowedOrigins := make(map[string]struct{})
	for _, rawOrigin := range strings.Split(rawOrigins, ",") {
		rawOrigin = strings.TrimSpace(rawOrigin)
		if rawOrigin == "" {
			continue
		}
		if rawOrigin == "*" {
			return nil, fmt.Errorf("CORS_ALLOW_ORIGINS cannot use wildcard origins with credentials")
		}
		origin, err := canonicalOrigin(rawOrigin)
		if err != nil {
			return nil, fmt.Errorf("invalid configured CORS origin %q", rawOrigin)
		}
		allowedOrigins[origin] = struct{}{}
	}

	return allowedOrigins, nil
}

func canonicalOrigin(rawOrigin string) (string, error) {
	parsed, err := url.Parse(rawOrigin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("origin must contain only scheme, host, and port")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("origin scheme must be http or https")
	}

	return scheme + "://" + strings.ToLower(parsed.Host), nil
}
