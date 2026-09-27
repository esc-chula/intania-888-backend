package middleware

import (
	"errors"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeMiddlewareService struct {
	session                 *model.SessionRecord
	user                    *model.UserDto
	sessionErr, errorPolicy error
	blacklisted             bool
}

func (s fakeMiddlewareService) GetSession(string) (*model.SessionRecord, error) {
	return s.session, s.sessionErr
}
func (s fakeMiddlewareService) VerifyExternalToken(string) (string, error) { return "user-id", nil }
func (s fakeMiddlewareService) GetMe(string) (*model.UserDto, error)       { return s.user, nil }
func (s fakeMiddlewareService) IsBlacklisted(string, string) (bool, error) {
	return s.blacklisted, s.errorPolicy
}
func TestBrowserSessionAndCSRF(t *testing.T) {
	service := fakeMiddlewareService{session: &model.SessionRecord{UserId: "user-id", CSRFToken: "secret"}, user: &model.UserDto{Id: "user-id", Email: "u@example.test"}}
	mid := NewMiddlewareHttpHandler(service, zap.NewNop(), false, config.DefaultSessionIdleTTLSeconds)
	app := fiber.New()
	app.Post("/update", mid.AuthMiddleware, func(c *fiber.Ctx) error { return c.SendStatus(204) })
	req := httptest.NewRequest(http.MethodPost, "/update", nil)
	req.Header.Set("Authorization", "Bearer token")
	response, _ := app.Test(req)
	if response.StatusCode != 401 {
		t.Fatal(response.StatusCode)
	}
	req = httptest.NewRequest(http.MethodPost, "/update", nil)
	req.Header.Set("Cookie", utils.LocalSessionCookieName+"=opaque")
	response, _ = app.Test(req)
	if response.StatusCode != 403 {
		t.Fatal(response.StatusCode)
	}
	req.Header.Set("X-CSRF-Token", "secret")
	response, _ = app.Test(req)
	if response.StatusCode != 204 {
		t.Fatal(response.StatusCode)
	}
	service.sessionErr = errors.New("redis down")
	mid = NewMiddlewareHttpHandler(service, zap.NewNop(), false, config.DefaultSessionIdleTTLSeconds)
	app = fiber.New()
	app.Get("/read", mid.AuthMiddleware, func(c *fiber.Ctx) error { return c.SendStatus(204) })
	req = httptest.NewRequest(http.MethodGet, "/read", nil)
	req.Header.Set("Cookie", "session=opaque")
	response, _ = app.Test(req)
	if response.StatusCode != 503 || len(response.Header.Values("Set-Cookie")) != 0 {
		t.Fatal("uncertain session should preserve cookie")
	}
}
func TestExternalIgnoresBrowserCookie(t *testing.T) {
	service := fakeMiddlewareService{user: &model.UserDto{Id: "user-id", Email: "u@example.test"}}
	mid := NewMiddlewareHttpHandler(service, zap.NewNop(), false, config.DefaultSessionIdleTTLSeconds)
	app := fiber.New()
	app.Get("/external", mid.ExternalAPIMiddleware, func(c *fiber.Ctx) error { return c.SendStatus(204) })
	req := httptest.NewRequest(http.MethodGet, "/external", nil)
	req.Header.Set("Cookie", "session=opaque")
	response, _ := app.Test(req)
	if response.StatusCode != 401 {
		t.Fatal(response.StatusCode)
	}
	req.Header.Set("Authorization", "Bearer external")
	response, _ = app.Test(req)
	if response.StatusCode != 204 {
		t.Fatal(response.StatusCode)
	}
}

func TestMissingStoredSessionClearsCookie(t *testing.T) {
	service := fakeMiddlewareService{sessionErr: ErrSessionMissing}
	mid := NewMiddlewareHttpHandler(service, zap.NewNop(), false, config.DefaultSessionIdleTTLSeconds)
	app := fiber.New()
	app.Get("/read", mid.AuthMiddleware, func(c *fiber.Ctx) error { return c.SendStatus(204) })
	req := httptest.NewRequest(http.MethodGet, "/read", nil)
	req.Header.Set("Cookie", "session=opaque")
	response, _ := app.Test(req)
	if response.StatusCode != 401 || len(response.Header.Values("Set-Cookie")) == 0 {
		t.Fatal("stale session did not return 401 and clear cookie")
	}
}
