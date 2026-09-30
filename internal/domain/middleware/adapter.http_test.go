package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

func newFiberTestApp() *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
	app.Use(apierror.RequestID())
	return app
}

func TestDependencyFailureUsesSharedErrorContractAndRequestID(t *testing.T) {
	service := fakeMiddlewareService{sessionErr: errors.New("redis unavailable")}
	mid := NewHTTPHandler(service, false, config.DefaultSessionIdleTTLSeconds)
	app := newFiberTestApp()
	app.Get("/private", mid.AuthMiddleware, func(c *fiber.Ctx) error { return c.SendStatus(204) })

	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	request.AddCookie(&http.Cookie{Name: mid.CookieName(), Value: "opaque"})
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	if response.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusServiceUnavailable)
	}
	var body apierror.Response
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "DEPENDENCY_UNAVAILABLE" || body.RequestID == "" || body.RequestID != response.Header.Get(apierror.RequestIDHeader) {
		t.Fatalf("dependency error contract = %#v; header=%q", body, response.Header.Get(apierror.RequestIDHeader))
	}
}

type fakeMiddlewareService struct {
	session                 *security.Session
	user                    *identity.Profile
	sessionErr, errorPolicy error
	blacklisted             bool
}

func (s fakeMiddlewareService) GetSession(context.Context, string) (*security.Session, error) {
	return s.session, s.sessionErr
}
func (s fakeMiddlewareService) VerifyScopedExternalToken(context.Context, string, string) (string, error) {
	return "user-id", nil
}
func (s fakeMiddlewareService) GetMe(context.Context, string) (*identity.Profile, error) {
	return s.user, nil
}
func (s fakeMiddlewareService) IsBlacklisted(context.Context, string, string) (bool, error) {
	return s.blacklisted, s.errorPolicy
}
func TestBrowserSessionAndCSRF(t *testing.T) {
	service := fakeMiddlewareService{session: &security.Session{UserID: "user-id", CSRFToken: "secret"}, user: &identity.Profile{ID: "user-id", Email: "u@example.test"}}
	mid := NewHTTPHandler(service, false, config.DefaultSessionIdleTTLSeconds)
	app := newFiberTestApp()
	app.Post("/update", mid.AuthMiddleware, func(c *fiber.Ctx) error { return c.SendStatus(204) })
	req := httptest.NewRequest(http.MethodPost, "/update", nil)
	req.Header.Set("Authorization", "Bearer token")
	response, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 401 {
		t.Fatal(response.StatusCode)
	}
	req = httptest.NewRequest(http.MethodPost, "/update", nil)
	req.Header.Set("Cookie", security.LocalSessionCookieName+"=opaque")
	response, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 403 {
		t.Fatal(response.StatusCode)
	}
	req.Header.Set("X-CSRF-Token", "secret")
	response, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 204 {
		t.Fatal(response.StatusCode)
	}
	service.sessionErr = errors.New("redis down")
	mid = NewHTTPHandler(service, false, config.DefaultSessionIdleTTLSeconds)
	app = newFiberTestApp()
	app.Get("/read", mid.AuthMiddleware, func(c *fiber.Ctx) error { return c.SendStatus(204) })
	req = httptest.NewRequest(http.MethodGet, "/read", nil)
	req.Header.Set("Cookie", "session=opaque")
	response, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 503 || len(response.Header.Values("Set-Cookie")) != 0 {
		t.Fatal("uncertain session should preserve cookie")
	}
}
func TestExternalIgnoresBrowserCookie(t *testing.T) {
	service := fakeMiddlewareService{user: &identity.Profile{ID: "user-id", Email: "u@example.test"}}
	mid := NewHTTPHandler(service, false, config.DefaultSessionIdleTTLSeconds)
	app := newFiberTestApp()
	app.Get("/external", mid.RequireExternalScope(config.ScopeProfileRead), func(c *fiber.Ctx) error { return c.SendStatus(204) })
	req := httptest.NewRequest(http.MethodGet, "/external", nil)
	req.Header.Set("Cookie", "session=opaque")
	response, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 401 {
		t.Fatal(response.StatusCode)
	}
	req.Header.Set("Authorization", "Bearer external")
	response, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 204 {
		t.Fatal(response.StatusCode)
	}
}

func TestMissingStoredSessionClearsCookie(t *testing.T) {
	service := fakeMiddlewareService{sessionErr: ErrSessionMissing}
	mid := NewHTTPHandler(service, false, config.DefaultSessionIdleTTLSeconds)
	app := newFiberTestApp()
	app.Get("/read", mid.AuthMiddleware, func(c *fiber.Ctx) error { return c.SendStatus(204) })
	req := httptest.NewRequest(http.MethodGet, "/read", nil)
	req.Header.Set("Cookie", "session=opaque")
	response, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 401 || len(response.Header.Values("Set-Cookie")) == 0 {
		t.Fatal("stale session did not return 401 and clear cookie")
	}
}
