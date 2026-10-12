package server

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

func TestExternalClientQuotaSharesRoutesAndSeparatesClients(t *testing.T) {
	policies := config.DefaultRateLimits()
	policies.ExternalClient = config.RatePolicy{
		PerMinute: 1,
		Burst:     1,
	}
	cfg := swaggerTestConfig{rateLimits: &policies}
	limits := NewExternalRateLimiters(cfg, zap.NewNop())
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
	router := app.Group("/external", func(c *fiber.Ctx) error {
		if err := limits.Client(c, c.Get("X-Test-Client")); err != nil {
			return err
		}

		return c.Next()
	})
	next := func(c *fiber.Ctx) error {
		return c.SendStatus(204)
	}
	router.Get("/one", next)
	router.Get("/two", next)

	for _, tc := range []struct {
		path, client string
		want         int
	}{
		{"/one", "A", 204},
		{"/two", "A", 429},
		{"/two", "B", 204},
	} {
		request := httptest.NewRequest("GET", "/external"+tc.path, nil)
		request.Header.Set("X-Test-Client", tc.client)
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
