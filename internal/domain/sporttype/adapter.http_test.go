package sporttype

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/cmd/server"
	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

type routeConfig struct{ config.Config }

func (routeConfig) GetServer() config.Server {
	return config.Server{Env: "development"}
}

func (routeConfig) GetSwagger() config.Swagger {
	return config.Swagger{}
}

func (routeConfig) GetCORS() config.CORS {
	return config.CORS{AllowOrigins: "http://localhost:3000"}
}

type routeAuthService struct {
	role        string
	blacklisted bool
}

func (routeAuthService) GetSession(_ context.Context, id string) (*security.Session, error) {
	if id != "opaque" {
		return nil, middleware.ErrSessionMissing
	}

	return &security.Session{
		UserID:    "user",
		CSRFToken: "csrf",
	}, nil
}

func (s routeAuthService) GetMe(context.Context, string) (*identity.Profile, error) {
	return &identity.Profile{
		ID:     "user",
		Email:  "user@example.test",
		RoleID: s.role,
	}, nil
}

func (routeAuthService) VerifyExternalGrant(context.Context, string, string) (string, error) {
	return "", errors.New("external credentials cannot access browser routes")
}

func (s routeAuthService) IsBlacklisted(context.Context, string, string) (bool, error) {
	return s.blacklisted, nil
}

func sportTypeTestApp(t *testing.T, role string, repo *memoryRepository) *fiber.App {
	return sportTypeTestAppWithAuth(t, routeAuthService{role: role}, repo)
}

func sportTypeTestAppWithAuth(t *testing.T, authService routeAuthService, repo *memoryRepository) *fiber.App {
	t.Helper()
	guard, err := server.NewFiberHTTPServer(routeConfig{}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
	app.Use(apierror.RequestID())
	router := app.Group("/api/v1", guard.OriginGuard())
	mid := middleware.NewHTTPHandler(authService, false, config.DefaultSessionIdleTTLSeconds)
	NewHTTPHandler(NewService(repo, zap.NewNop())).RegisterRoutes(router, mid.AuthMiddleware, mid.AdminMiddleware)

	return app
}

func sportTypeRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Cookie", "session=opaque")
	request.Header.Set("X-CSRF-Token", "csrf")

	return request
}

func sportTypeResponse(t *testing.T, app *fiber.App, request *http.Request) (int, []byte) {
	t.Helper()
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	return response.StatusCode, body
}

func TestSportTypeHTTPCRUD(t *testing.T) {
	repo := newMemoryRepository()
	app := sportTypeTestApp(t, security.RoleAdmin, repo)
	tests := []struct {
		method, path, body string
		status             int
		want               string
	}{
		{"POST", "/sport-types/admin", `{"id":"new_ID-1","title":" Sport "}`, 201, `{"id":"new_ID-1","title":"Sport"}`},
		{"GET", "/sport-types/new_ID-1", "", 200, `{"id":"new_ID-1","title":"Sport"}`},
		{"PATCH", "/sport-types/admin/new_ID-1", `{"title":" Renamed "}`, 200, `{"id":"new_ID-1","title":"Renamed"}`},
		{"DELETE", "/sport-types/admin/new_ID-1", "", 204, ""},
		{"GET", "/sport-types/new_ID-1", "", 404, "RESOURCE_NOT_FOUND"},
		{"PATCH", "/sport-types/admin/new_ID-1", `{"title":"Sport"}`, 404, "RESOURCE_NOT_FOUND"},
		{"DELETE", "/sport-types/admin/new_ID-1", "", 404, "RESOURCE_NOT_FOUND"},
		{"POST", "/sport-types/admin", `{"id":"S","title":"Other"}`, 409, "CONFLICT"},
		{"DELETE", "/sport-types/admin/USED", "", 409, "SPORT_TYPE_IN_USE"},
	}
	for _, test := range tests {
		t.Run(test.method+test.path, func(t *testing.T) {
			status, body := sportTypeResponse(t, app, sportTypeRequest(test.method, test.path, test.body))
			if status != test.status {
				t.Fatalf("status = %d; want %d; body=%s", status, test.status, body)
			}
			if status < 400 {
				if string(body) != test.want {
					t.Fatalf("body = %s; want %s", body, test.want)
				}

				return
			}
			var failure apierror.Response
			if err := json.Unmarshal(body, &failure); err != nil {
				t.Fatal(err)
			}
			if failure.Code != test.want || failure.RequestID == "" {
				t.Fatalf("failure = %+v", failure)
			}
		})
	}
	if _, exists := repo.rows["USED"]; !exists {
		t.Fatal("referenced sport was removed")
	}
}

func TestSportTypeHTTPRejectsInvalidRequests(t *testing.T) {
	tests := []struct{ method, path, body string }{
		{"POST", "/sport-types/admin", `{}`},
		{"POST", "/sport-types/admin", `{"id":null,"title":"Sport"}`},
		{"POST", "/sport-types/admin", `{"id":"กีฬา","title":"Sport"}`},
		{"POST", "/sport-types/admin", `{"id":" S ","title":"Valid title"}`},
		{"POST", "/sport-types/admin", `{"id":"a/b","title":"Valid title"}`},
		{"POST", "/sport-types/admin", `{"id":"a?b","title":"Valid title"}`},
		{"POST", "/sport-types/admin", `{"id":"a%b","title":"Valid title"}`},
		{"POST", "/sport-types/admin", `{"id":"a.b","title":"Valid title"}`},
		{"POST", "/sport-types/admin", `{"id":"a:b","title":"Valid title"}`},
		{"POST", "/sport-types/admin", `{"id":"a\u0000b","title":"Valid title"}`},
		{"POST", "/sport-types/admin", `{"id":"a\tb","title":"Valid title"}`},
		{"POST", "/sport-types/admin", `{"id":"a\u007fb","title":"Valid title"}`},
		{"POST", "/sport-types/admin", `{"id":"S2","title":null}`},
		{"POST", "/sport-types/admin", `{"id":"S2","title":"Sport","extra":true}`},
		{"POST", "/sport-types/admin", `[]`},
		{"POST", "/sport-types/admin", `{"id":"S2","title":"Sport"} {}`},
		{"PATCH", "/sport-types/admin/S", `{}`},
		{"PATCH", "/sport-types/admin/S", `{"title":" \t "}`},
		{"PATCH", "/sport-types/admin/S", `{"title":123}`},
		{"PATCH", "/sport-types/admin/S", `{"title":"Sport","id":"S2"}`},
		{"PATCH", "/sport-types/admin/S", `{"title":"` + strings.Repeat("ก", 101) + `"}`},
		{"GET", "/sport-types/กีฬา", ""},
		{"GET", "/sport-types/%20S%20", ""},
		{"PATCH", "/sport-types/admin/%20S%20", `{"title":"Valid title"}`},
		{"DELETE", "/sport-types/admin/%20S%20", ""},
		{"GET", "/sport-types/a%2Fb", ""},
		{"PATCH", "/sport-types/admin/a%2Fb", `{"title":"Valid title"}`},
		{"DELETE", "/sport-types/admin/a%2Fb", ""},
		{"GET", "/sport-types/a%3Fb", ""},
		{"PATCH", "/sport-types/admin/a%3Fb", `{"title":"Valid title"}`},
		{"DELETE", "/sport-types/admin/a%3Fb", ""},
		{"GET", "/sport-types/a%25b", ""},
		{"PATCH", "/sport-types/admin/a%25b", `{"title":"Valid title"}`},
		{"DELETE", "/sport-types/admin/a%25b", ""},
		{"GET", "/sport-types/a%00b", ""},
		{"PATCH", "/sport-types/admin/a%00b", `{"title":"Valid title"}`},
		{"DELETE", "/sport-types/admin/a%00b", ""},
		{"DELETE", "/sport-types/admin/กีฬา", ""},
	}
	for _, test := range tests {
		t.Run(test.method+test.body, func(t *testing.T) {
			repo := newMemoryRepository()
			app := sportTypeTestApp(t, security.RoleAdmin, repo)
			status, body := sportTypeResponse(t, app, sportTypeRequest(test.method, test.path, test.body))
			if status != 400 || !strings.Contains(string(body), `"code":"INVALID_REQUEST"`) || repo.calls != 0 {
				t.Fatalf("invalid request = %d %s; storage calls=%d", status, body, repo.calls)
			}
		})
	}
}

func TestSportTypeHTTPAuthenticationAdminOriginAndCSRF(t *testing.T) {
	mutations := []struct{ method, path, body string }{
		{"POST", "/sport-types/admin", `{"id":"new_ID-1","title":"Sport"}`},
		{"PATCH", "/sport-types/admin/S", `{"title":"Renamed"}`},
		{"DELETE", "/sport-types/admin/S", ""},
	}
	checks := []struct {
		name, role, header, value string
		status                    int
	}{
		{"no session", security.RoleAdmin, "Cookie", "", 401},
		{"invalid session", security.RoleAdmin, "Cookie", "session=invalid", 401},
		{"non admin", security.RoleUser, "", "", 403},
		{"missing CSRF", security.RoleAdmin, "X-CSRF-Token", "", 403},
		{"wrong CSRF", security.RoleAdmin, "X-CSRF-Token", "wrong", 403},
		{"missing Origin", security.RoleAdmin, "Origin", "", 403},
		{"wrong Origin", security.RoleAdmin, "Origin", "https://hostile.test", 403},
	}
	for _, mutation := range mutations {
		for _, check := range checks {
			t.Run(mutation.method+check.name, func(t *testing.T) {
				repo := newMemoryRepository()
				app := sportTypeTestApp(t, check.role, repo)
				request := sportTypeRequest(mutation.method, mutation.path, mutation.body)
				if check.header != "" {
					request.Header.Set(check.header, check.value)
				}

				status, body := sportTypeResponse(t, app, request)
				if status != check.status || repo.calls != 0 {
					t.Fatalf("protected route = %d %s; calls=%d", status, body, repo.calls)
				}
			})
		}
	}
}

func TestSportTypeCatalogueReadsArePublicAndSkipAccountBlacklist(t *testing.T) {
	for _, path := range []string{"/sport-types", "/sport-types/S", "/sport-types/admin"} {
		t.Run(path, func(t *testing.T) {
			app := sportTypeTestAppWithAuth(t, routeAuthService{
				role:        security.RoleUser,
				blacklisted: true,
			}, newMemoryRepository())
			request := sportTypeRequest(http.MethodGet, path, "")
			request.Header.Del("X-CSRF-Token")

			if status, body := sportTypeResponse(t, app, request); status != http.StatusOK {
				t.Fatalf("public catalogue read with blocked-account cookie = %d %s", status, body)
			}

			request.Header.Del("Cookie")

			if status, body := sportTypeResponse(t, app, request); status != http.StatusOK {
				t.Fatalf("public catalogue read without a session = %d %s", status, body)
			}

			request.Header.Set("Origin", "https://unlisted.example.test")

			if status, body := sportTypeResponse(t, app, request); status != http.StatusForbidden {
				t.Fatalf("public catalogue read from an unlisted origin = %d %s", status, body)
			}

			request.Header.Del("Origin")

			if status, body := sportTypeResponse(t, app, request); status != http.StatusOK {
				t.Fatalf("public catalogue read without an Origin = %d %s", status, body)
			}
		})
	}
}

func (routeConfig) GetRateLimits() config.RateLimits {
	return config.DefaultRateLimits()
}

func (s routeAuthService) GetExternalProfile(ctx context.Context, id string) (*identity.Profile, error) {
	return s.GetMe(ctx, id)
}
