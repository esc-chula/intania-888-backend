package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-888-backend/docs"
	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type swaggerTestConfig struct {
	server  config.Server
	swagger config.Swagger
	cors    config.Cors
}

func (c swaggerTestConfig) GetServer() config.Server   { return c.server }
func (c swaggerTestConfig) GetSwagger() config.Swagger { return c.swagger }
func (c swaggerTestConfig) GetCors() config.Cors       { return c.cors }
func (c swaggerTestConfig) GetDb() config.Db           { return config.Db{} }
func (c swaggerTestConfig) GetCache() config.Cache     { return config.Cache{} }
func (c swaggerTestConfig) GetJwt() config.Jwt         { return config.Jwt{} }
func (c swaggerTestConfig) GetOAuth() config.OAuth     { return config.OAuth{} }
func (c swaggerTestConfig) GetSession() config.Session { return config.Session{} }
func (c swaggerTestConfig) GetDailyReward() config.DailyReward {
	return config.DailyReward{}
}

func newSwaggerTestServer(t *testing.T, swaggerConfig config.Swagger) *FiberHttpServer {
	t.Helper()

	httpServer, err := NewFiberHttpServer(swaggerTestConfig{
		server:  config.Server{Url: "http://localhost:8080/api/v1"},
		swagger: swaggerConfig,
		cors:    config.Cors{AllowOrigins: "http://localhost:3000"},
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewFiberHttpServer() error = %v", err)
	}

	httpServer.InitHttpServer()
	return httpServer
}

func TestSwaggerDevelopmentDoesNotRequireAuthenticationOrOrigin(t *testing.T) {
	httpServer := newSwaggerTestServer(t, config.Swagger{
		Enabled: true,
	})

	response, err := httpServer.app.Test(httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Swagger status = %d, want %d", response.StatusCode, http.StatusOK)
	}
}

func TestSwaggerProductionRequiresBasicAuthentication(t *testing.T) {
	httpServer := newSwaggerTestServer(t, config.Swagger{
		Enabled:     true,
		RequireAuth: true,
		Username:    "swagger-user",
		Password:    "swagger-password",
	})

	unauthenticated, err := httpServer.app.Test(httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
	if err != nil {
		t.Fatalf("unauthenticated app.Test() error = %v", err)
	}
	if unauthenticated.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated Swagger status = %d, want %d", unauthenticated.StatusCode, http.StatusUnauthorized)
	}
	if got := unauthenticated.Header.Get("WWW-Authenticate"); got != `Basic realm="Restricted"` {
		t.Fatalf("WWW-Authenticate = %q, want Basic challenge", got)
	}

	wrongCredentials := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	wrongCredentials.SetBasicAuth("swagger-user", "wrong-password")
	wrongResponse, err := httpServer.app.Test(wrongCredentials)
	if err != nil {
		t.Fatalf("wrong-credentials app.Test() error = %v", err)
	}
	if wrongResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong-credentials Swagger status = %d, want %d", wrongResponse.StatusCode, http.StatusUnauthorized)
	}

	correctCredentials := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	correctCredentials.SetBasicAuth("swagger-user", "swagger-password")
	correctResponse, err := httpServer.app.Test(correctCredentials)
	if err != nil {
		t.Fatalf("correct-credentials app.Test() error = %v", err)
	}
	if correctResponse.StatusCode != http.StatusOK {
		t.Fatalf("correct-credentials Swagger status = %d, want %d", correctResponse.StatusCode, http.StatusOK)
	}
}

func TestSwaggerIsNotServedUnderAPIPrefix(t *testing.T) {
	httpServer := newSwaggerTestServer(t, config.Swagger{Enabled: true})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/swagger/index.html", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	response, err := httpServer.app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("legacy Swagger status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
}

func TestSwaggerCanBeDisabled(t *testing.T) {
	httpServer, err := NewFiberHttpServer(swaggerTestConfig{
		server:  config.Server{},
		swagger: config.Swagger{Enabled: false},
		cors:    config.Cors{AllowOrigins: "http://localhost:3000"},
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewFiberHttpServer() error = %v", err)
	}
	httpServer.InitHttpServer()

	response, err := httpServer.app.Test(httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("disabled Swagger status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
}

func TestSwaggerUsesConfiguredServerURL(t *testing.T) {
	_, err := NewFiberHttpServer(swaggerTestConfig{
		server:  config.Server{Url: "https://api.example.test/api/v1"},
		swagger: config.Swagger{Enabled: true},
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewFiberHttpServer() error = %v", err)
	}

	if docs.SwaggerInfo.Host != "api.example.test" {
		t.Fatalf("Swagger host = %q, want %q", docs.SwaggerInfo.Host, "api.example.test")
	}
	if len(docs.SwaggerInfo.Schemes) != 1 || docs.SwaggerInfo.Schemes[0] != "https" {
		t.Fatalf("Swagger schemes = %v, want [https]", docs.SwaggerInfo.Schemes)
	}
	if docs.SwaggerInfo.BasePath != "/api/v1" {
		t.Fatalf("Swagger base path = %q, want %q", docs.SwaggerInfo.BasePath, "/api/v1")
	}
}

func TestSwaggerConfigurationRequiresURLAndCredentialsWhenEnabled(t *testing.T) {
	_, err := NewFiberHttpServer(swaggerTestConfig{
		swagger: config.Swagger{Enabled: true},
	}, zap.NewNop())
	if err == nil {
		t.Fatal("NewFiberHttpServer() error = nil, want missing SERVER_URL error")
	}

	_, err = NewFiberHttpServer(swaggerTestConfig{
		server: config.Server{Url: "http://localhost:8080/api/v1"},
		swagger: config.Swagger{
			Enabled:     true,
			RequireAuth: true,
		},
	}, zap.NewNop())
	if err == nil {
		t.Fatal("NewFiberHttpServer() error = nil, want missing credential error")
	}
}

func newOriginGuardTestServer(t *testing.T) (*FiberHttpServer, fiber.Router) {
	t.Helper()

	httpServer, err := NewFiberHttpServer(swaggerTestConfig{
		server: config.Server{Env: "development"},
		cors:   config.Cors{AllowOrigins: "https://frontend.example.test:8443,http://localhost:3000"},
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewFiberHttpServer() error = %v", err)
	}
	return httpServer, httpServer.InitHttpServer()
}

func TestOriginGuardUsesExactConfiguredOriginsAndAllowsSafeReadsWithoutOrigin(t *testing.T) {
	httpServer, router := newOriginGuardTestServer(t)
	router.Get("/safe", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })
	router.Post("/mutate", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })

	withoutOrigin, err := httpServer.app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/safe", nil))
	if err != nil {
		t.Fatalf("safe request error = %v", err)
	}
	if withoutOrigin.StatusCode != http.StatusNoContent {
		t.Fatalf("safe request status = %d, want %d", withoutOrigin.StatusCode, http.StatusNoContent)
	}

	exactOrigin := httptest.NewRequest(http.MethodGet, "/api/v1/safe", nil)
	exactOrigin.Header.Set("Origin", "https://frontend.example.test:8443")
	response, err := httpServer.app.Test(exactOrigin)
	if err != nil {
		t.Fatalf("exact-origin request error = %v", err)
	}
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("exact-origin status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}

	if got := response.Header.Get("Access-Control-Expose-Headers"); !strings.Contains(got, "X-Request-ID") {
		t.Fatalf("exposed headers = %q, want X-Request-ID", got)
	}

	unknownOrigin := httptest.NewRequest(http.MethodGet, "/api/v1/safe", nil)
	unknownOrigin.Header.Set("Origin", "https://frontend.example.test")
	response, err = httpServer.app.Test(unknownOrigin)
	if err != nil {
		t.Fatalf("unknown-origin request error = %v", err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("unknown-origin status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
	if response.Header.Get("X-Request-ID") == "" {
		t.Fatal("origin rejection did not include X-Request-ID")
	}
	var errorBody map[string]string
	if err := json.NewDecoder(response.Body).Decode(&errorBody); err != nil {
		t.Fatalf("decode origin error body: %v", err)
	}
	if errorBody["code"] != "FORBIDDEN" || errorBody["request_id"] != response.Header.Get("X-Request-ID") {
		t.Fatalf("origin error contract = %#v", errorBody)
	}

	missingOrigin := httptest.NewRequest(http.MethodPost, "/api/v1/mutate", nil)
	response, err = httpServer.app.Test(missingOrigin)
	if err != nil {
		t.Fatalf("missing-origin request error = %v", err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("missing-origin status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
}

func TestUnmatchedAPIPathUsesSharedErrorContract(t *testing.T) {
	httpServer, _ := newOriginGuardTestServer(t)
	response, err := httpServer.app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/not-registered", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	var body apierror.Response
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "RESOURCE_NOT_FOUND" || body.RequestID == "" || body.RequestID != response.Header.Get(apierror.RequestIDHeader) {
		t.Fatalf("unmatched route error = %#v; header=%q", body, response.Header.Get(apierror.RequestIDHeader))
	}
}

func TestExternalAndOAuthCallbackPathsAreOriginAndCSRFExempt(t *testing.T) {
	httpServer, router := newOriginGuardTestServer(t)
	router.Post("/external/test", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })
	router.Get("/auth/callback", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })

	external, err := httpServer.app.Test(httptest.NewRequest(http.MethodPost, "/api/v1/external/test", nil))
	if err != nil {
		t.Fatalf("external request error = %v", err)
	}
	if external.StatusCode != http.StatusNoContent {
		t.Fatalf("external status = %d, want %d", external.StatusCode, http.StatusNoContent)
	}

	callback, err := httpServer.app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/auth/callback", nil))
	if err != nil {
		t.Fatalf("callback request error = %v", err)
	}
	if callback.StatusCode != http.StatusNoContent {
		t.Fatalf("callback status = %d, want %d", callback.StatusCode, http.StatusNoContent)
	}
}

func TestWildcardCredentialOriginsAreRejected(t *testing.T) {
	_, err := NewFiberHttpServer(swaggerTestConfig{
		cors: config.Cors{AllowOrigins: "*"},
	}, zap.NewNop())
	if err == nil {
		t.Fatal("wildcard CORS origin was accepted with credentials enabled")
	}
}
