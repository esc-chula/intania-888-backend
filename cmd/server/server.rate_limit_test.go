package server

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

type externalLimiterConfig struct{ swaggerTestConfig }

func (externalLimiterConfig) GetJWT() config.JWT { return config.JWT{AccessTokenSecret: "secret"} }

func TestExternalClientQuotaSharesRoutesAndRejectsInvalidCredentialsSeparately(t *testing.T) {
	policies := config.DefaultRateLimits()
	policies.ExternalClient = config.RatePolicy{PerMinute: 1, Burst: 1}
	policies.InvalidExternal = config.RatePolicy{PerMinute: 1, Burst: 1}
	cfg := externalLimiterConfig{swaggerTestConfig{rateLimits: &policies, server: config.Server{Name: "888"}}}
	limits := NewExternalRateLimiters(cfg, zap.NewNop())
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
	router := app.Group("/external", limits.Client)
	next := func(c *fiber.Ctx) error { return c.SendStatus(204) }
	router.Get("/one", next)
	router.Get("/two", next)
	tokens := make(map[string]string)
	for _, client := range []string{"A", "B"} {
		token, err := security.SignDelegatedToken(security.Delegation{
			ID: "grant", UserID: "user", ClientID: client, ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}, "secret", "888", 60)
		if err != nil {
			t.Fatal(err)
		}
		tokens[client] = token
	}
	for _, tc := range []struct {
		path, token string
		want        int
	}{
		{"/one", "invalid", 401}, {"/two", "invalid", 429},
		{"/one", tokens["A"], 204}, {"/two", tokens["A"], 429}, {"/two", tokens["B"], 204},
	} {
		request := httptest.NewRequest("GET", "/external"+tc.path, nil)
		request.Header.Set("Authorization", "Bearer "+tc.token)
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != tc.want {
			t.Fatalf("%s: %d, want %d", tc.path, response.StatusCode, tc.want)
		}
	}
}

func TestSharedIPGuardIncludesExternalRequests(t *testing.T) {
	policies := config.DefaultRateLimits()
	policies.Shared = config.RatePolicy{PerMinute: 1, Burst: 1}
	cfg := swaggerTestConfig{rateLimits: &policies}
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
	router := app.Group("/api/v1")
	registerRateLimiters(router, cfg, nil)
	router.Get("/external/test", func(c *fiber.Ctx) error { return c.SendStatus(204) })
	router.Get("/public", func(c *fiber.Ctx) error { return c.SendStatus(204) })
	for i, path := range []string{"/external/test", "/public"} {
		response, err := app.Test(httptest.NewRequest("GET", "/api/v1"+path, nil))
		if err != nil {
			t.Fatal(err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
		want := 204
		if i == 1 {
			want = 429
		}
		if response.StatusCode != want {
			t.Fatalf("%s: %d, want %d", path, response.StatusCode, want)
		}
	}
}
