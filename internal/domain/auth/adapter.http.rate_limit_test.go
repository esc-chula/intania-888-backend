package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

func TestAuthenticationQuotasSeparateClientsFailuresAndRevocation(t *testing.T) {
	policies := config.DefaultRateLimits()
	policies.Token = config.RatePolicy{PerMinute: 1, Burst: 1}
	policies.InvalidClient = config.RatePolicy{PerMinute: 1, Burst: 1}
	h := &ApplicationHTTPHandler{
		limits: NewRateLimiters(policies, nil),
		registry: &config.AuthRegistry{Applications: []config.AuthApplication{
			{ID: "A", Mode: config.CodeApplication, Secret: "secret"},
			{ID: "B", Mode: config.CodeApplication, Secret: "secret"},
		}},
	}
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
	app.Post("/token", h.Token)
	app.Post("/revoke", h.Revoke)
	for _, tc := range []struct {
		path, client, secret string
		want                 int
	}{
		{"/token", "A", "wrong", 401},
		{"/revoke", "A", "wrong", 429},
		{"/token", "A", "secret", 400},
		{"/token", "A", "secret", 429},
		{"/token", "B", "secret", 400},
		{"/revoke", "A", "secret", 400},
	} {
		request := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader("grant_type=unsupported"))
		request.Header.Set("Content-Type", fiber.MIMEApplicationForm)
		if tc.path == "/revoke" && tc.secret == "secret" {
			request.Header.Set("Content-Type", fiber.MIMEApplicationJSON)
		}
		request.SetBasicAuth(tc.client, tc.secret)
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != tc.want {
			t.Fatalf("%s/%s: %d, want %d", tc.path, tc.client, response.StatusCode, tc.want)
		}
		if tc.want == 429 && response.Header.Get("Retry-After") == "" {
			t.Fatal("missing Retry-After")
		}
	}
}

func TestInitiationQuotaDoesNotConsumeCallbackCapacity(t *testing.T) {
	policies := config.DefaultRateLimits()
	policies.Login = config.RatePolicy{PerMinute: 1, Burst: 1}
	policies.Callback = config.RatePolicy{PerMinute: 1, Burst: 1}
	h := &HTTPHandler{applications: &ApplicationHTTPHandler{
		registry: &config.AuthRegistry{}, limits: NewRateLimiters(policies, nil),
	}}
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
	h.RegisterRoutes(app, func(c *fiber.Ctx) error { return c.Next() })
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/auth/login", 400}, {"/auth/authorize", 429}, {"/auth/callback", 400}, {"/auth/callback", 429},
	} {
		response, err := app.Test(httptest.NewRequest("GET", tc.path, nil))
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
