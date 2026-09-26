package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type fakeMiddlewareService struct {
	claims      *utils.AccessTokenClaims
	user        *model.UserDto
	blacklisted bool
	policyErr   error
}

func (s fakeMiddlewareService) VerifyToken(string) (*utils.AccessTokenClaims, error) {
	return s.claims, nil
}

func (s fakeMiddlewareService) GetMe(string) (*model.UserDto, error) { return s.user, nil }

func (s fakeMiddlewareService) IsBlacklisted(string, string) (bool, error) {
	return s.blacklisted, s.policyErr
}

func TestAuthMiddlewareUsesAccessCookieAndIgnoresBearerHeader(t *testing.T) {
	service := fakeMiddlewareService{
		claims: &utils.AccessTokenClaims{UserId: "user-id", SessionId: "session-id"},
		user:   &model.UserDto{Id: "user-id", Email: "user@example.test"},
	}
	middlewareHandler := NewMiddlewareHttpHandler(service, zap.NewNop())
	app := fiber.New()
	app.Get("/protected", middlewareHandler.AuthMiddleware, func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer bearer-value")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("bearer-only request error = %v", err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bearer-only status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}

	request = httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Cookie", utils.AccessTokenCookieName+"=cookie-value")
	response, err = app.Test(request)
	if err != nil {
		t.Fatalf("cookie request error = %v", err)
	}
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("cookie status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
}

func TestExternalMiddlewareRequiresBearerAndIgnoresCookies(t *testing.T) {
	service := fakeMiddlewareService{
		claims: &utils.AccessTokenClaims{UserId: "user-id", SessionId: "session-id"},
		user:   &model.UserDto{Id: "user-id", Email: "user@example.test"},
	}
	middlewareHandler := NewMiddlewareHttpHandler(service, zap.NewNop())
	app := fiber.New()
	app.Get("/external", middlewareHandler.ExternalAPIMiddleware, func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/external", nil)
	request.Header.Set("Cookie", utils.AccessTokenCookieName+"=cookie-value")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("cookie-only external request error = %v", err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("cookie-only external status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}

	request = httptest.NewRequest(http.MethodGet, "/external", nil)
	request.Header.Set("Authorization", "Bearer external-token")
	request.Header.Set("Cookie", utils.AccessTokenCookieName+"=cookie-value")
	response, err = app.Test(request)
	if err != nil {
		t.Fatalf("bearer external request error = %v", err)
	}
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("bearer external status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
}

func TestAuthMiddlewareFailsClosedWhenPolicyIsUnavailable(t *testing.T) {
	service := fakeMiddlewareService{
		claims:    &utils.AccessTokenClaims{UserId: "user-id", SessionId: "session-id"},
		user:      &model.UserDto{Id: "user-id", Email: "user@example.test"},
		policyErr: errors.New("policy backend unavailable"),
	}
	middlewareHandler := NewMiddlewareHttpHandler(service, zap.NewNop())
	app := fiber.New()
	app.Get("/protected", middlewareHandler.AuthMiddleware, func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Cookie", utils.AccessTokenCookieName+"=cookie-value")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("policy failure request error = %v", err)
	}
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("policy failure status = %d, want %d", response.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestAdminMiddlewareUsesOnlyDatabaseRole(t *testing.T) {
	service := fakeMiddlewareService{}
	middlewareHandler := NewMiddlewareHttpHandler(service, zap.NewNop())
	app := fiber.New()
	app.Get("/admin", middlewareHandler.AdminMiddleware, func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request = request.WithContext(request.Context())
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("missing-profile request error = %v", err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing-profile status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}

	app = fiber.New()
	app.Get("/admin", func(c *fiber.Ctx) error {
		c.Locals("user", &model.UserDto{Id: "admin", Email: "admin@example.test", RoleId: "ADMIN"})
		return middlewareHandler.AdminMiddleware(c)
	}, func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })
	response, err = app.Test(httptest.NewRequest(http.MethodGet, "/admin", nil))
	if err != nil {
		t.Fatalf("admin-role request error = %v", err)
	}
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("admin-role status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
}
