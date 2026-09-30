package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"gopkg.in/yaml.v3"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

type swaggerTestConfig struct {
	server  config.Server
	swagger config.Swagger
	cors    config.CORS
}

func (c swaggerTestConfig) GetServer() config.Server {
	return c.server
}

func (c swaggerTestConfig) GetSwagger() config.Swagger {
	return c.swagger
}

func (c swaggerTestConfig) GetCORS() config.CORS {
	return c.cors
}

func (c swaggerTestConfig) GetDB() config.DB {
	return config.DB{}
}

func (c swaggerTestConfig) GetCache() config.Cache {
	return config.Cache{}
}

func (c swaggerTestConfig) GetJWT() config.JWT {
	return config.JWT{}
}

func (c swaggerTestConfig) GetOAuth() config.OAuth {
	return config.OAuth{}
}

func (c swaggerTestConfig) GetSession() config.Session {
	return config.Session{}
}

func (c swaggerTestConfig) GetDailyReward() config.DailyReward {
	return config.DailyReward{}
}

func newSwaggerTestServer(t *testing.T, swaggerConfig config.Swagger) *FiberHTTPServer {
	t.Helper()

	httpServer, err := NewFiberHTTPServer(swaggerTestConfig{
		server:  config.Server{URL: "http://localhost:8080/api/v1"},
		swagger: swaggerConfig,
		cors:    config.CORS{AllowOrigins: "http://localhost:3000"},
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewFiberHTTPServer() error = %v", err)
	}

	httpServer.InitHTTPServer()

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
	indexBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read Swagger index: %v", err)
	}
	if !strings.Contains(string(indexBody), `src="/swagger/swagger-session.js"`) {
		t.Fatal("Swagger index does not load the embedded session helper")
	}
	if !strings.Contains(string(indexBody), `data-api-base="http://localhost:8080/api/v1"`) {
		t.Fatal("Swagger index does not include the configured API base URL")
	}

	scriptResponse, err := httpServer.app.Test(
		httptest.NewRequest(http.MethodGet, "/swagger/swagger-session.js", nil),
	)
	if err != nil {
		t.Fatalf("Swagger session script request error = %v", err)
	}
	if scriptResponse.StatusCode != http.StatusOK {
		t.Fatalf("Swagger session script status = %d, want %d", scriptResponse.StatusCode, http.StatusOK)
	}
	scriptBody, err := io.ReadAll(scriptResponse.Body)
	if err != nil {
		t.Fatalf("read Swagger session script: %v", err)
	}
	if !strings.Contains(string(scriptBody), "window.IntaniaSwaggerSession") {
		t.Fatal("served Swagger helper is missing its UI callbacks")
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

	for _, path := range []string{"/swagger/openapi.yaml", "/swagger/doc.json"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)

		response, err := httpServer.app.Test(request)
		if err != nil {
			t.Fatalf("unauthenticated %s request error = %v", path, err)
		}
		if response.StatusCode != http.StatusUnauthorized {
			t.Errorf("unauthenticated %s status = %d, want %d", path, response.StatusCode, http.StatusUnauthorized)
		}
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
	httpServer, err := NewFiberHTTPServer(swaggerTestConfig{
		server:  config.Server{},
		swagger: config.Swagger{Enabled: false},
		cors:    config.CORS{AllowOrigins: "http://localhost:3000"},
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewFiberHTTPServer() error = %v", err)
	}
	httpServer.InitHTTPServer()

	response, err := httpServer.app.Test(httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("disabled Swagger status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}

	specResponse, err := httpServer.app.Test(httptest.NewRequest(http.MethodGet, "/swagger/openapi.yaml", nil))
	if err != nil {
		t.Fatalf("disabled OpenAPI app.Test() error = %v", err)
	}
	if specResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("disabled OpenAPI status = %d, want %d", specResponse.StatusCode, http.StatusNotFound)
	}
}

func TestSwaggerUsesConfiguredServerURL(t *testing.T) {
	httpServer, err := NewFiberHTTPServer(swaggerTestConfig{
		server:  config.Server{URL: "https://api.example.test/gateway/api/v1"},
		swagger: config.Swagger{Enabled: true},
		cors:    config.CORS{AllowOrigins: "http://localhost:3000"},
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewFiberHTTPServer() error = %v", err)
	}
	httpServer.InitHTTPServer()

	response, err := httpServer.app.Test(httptest.NewRequest(http.MethodGet, "/swagger/openapi.yaml", nil))
	if err != nil {
		t.Fatalf("OpenAPI request error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("OpenAPI status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if got := response.Header.Get(fiber.HeaderContentType); !strings.HasPrefix(got, "application/yaml") {
		t.Fatalf("OpenAPI content type = %q, want application/yaml", got)
	}

	var document map[string]any
	if err := yaml.NewDecoder(response.Body).Decode(&document); err != nil {
		t.Fatalf("decode OpenAPI response: %v", err)
	}
	servers, ok := document["servers"].([]any)
	if !ok || len(servers) != 1 {
		t.Fatalf("OpenAPI servers = %v, want one configured server", document["servers"])
	}
	server, ok := servers[0].(map[string]any)
	if !ok || server["url"] != "https://api.example.test/gateway/api/v1" {
		t.Errorf("OpenAPI server URL = %v, want %q", servers[0], "https://api.example.test/gateway/api/v1")
	}

	uiResponse, err := httpServer.app.Test(httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
	if err != nil {
		t.Fatalf("Swagger UI request error = %v", err)
	}

	uiContents, err := io.ReadAll(uiResponse.Body)
	if err != nil {
		t.Fatalf("read Swagger UI response: %v", err)
	}
	if !strings.Contains(string(uiContents), "/swagger/openapi.yaml") {
		t.Error("Swagger UI does not point to /swagger/openapi.yaml")
	}

	redirect, err := httpServer.app.Test(httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil))
	if err != nil {
		t.Fatalf("legacy Swagger document request error = %v", err)
	}
	if redirect.StatusCode != http.StatusMovedPermanently ||
		redirect.Header.Get("Location") != "/swagger/openapi.yaml" {
		t.Errorf(
			"legacy document response = %d %q, want redirect to /swagger/openapi.yaml",
			redirect.StatusCode,
			redirect.Header.Get("Location"),
		)
	}
}

func TestSwaggerConfigurationRequiresURLAndCredentialsWhenEnabled(t *testing.T) {
	_, err := NewFiberHTTPServer(swaggerTestConfig{
		swagger: config.Swagger{Enabled: true},
	}, zap.NewNop())
	if err == nil {
		t.Fatal("NewFiberHTTPServer() error = nil, want missing SERVER_URL error")
	}

	_, err = NewFiberHTTPServer(swaggerTestConfig{
		server: config.Server{URL: "http://localhost:8080/api/v1"},
		swagger: config.Swagger{
			Enabled:     true,
			RequireAuth: true,
		},
	}, zap.NewNop())
	if err == nil {
		t.Fatal("NewFiberHTTPServer() error = nil, want missing credential error")
	}
}

func newOriginGuardTestServer(t *testing.T) (*FiberHTTPServer, fiber.Router) {
	t.Helper()

	httpServer, err := NewFiberHTTPServer(swaggerTestConfig{
		server: config.Server{Env: "development"},
		cors:   config.CORS{AllowOrigins: "https://frontend.example.test:8443,http://localhost:3000"},
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewFiberHTTPServer() error = %v", err)
	}

	return httpServer, httpServer.InitHTTPServer()
}

func TestOriginGuardUsesExactConfiguredOriginsAndAllowsSafeReadsWithoutOrigin(t *testing.T) {
	httpServer, router := newOriginGuardTestServer(t)
	router.Get("/safe", func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})
	router.Post("/mutate", func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})

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

func TestRateLimitAppliesToPublicAPIReads(t *testing.T) {
	httpServer, router := newOriginGuardTestServer(t)
	router.Get("/public-read", func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})

	for requestNumber := 1; requestNumber <= 201; requestNumber++ {
		response, err := httpServer.app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/public-read", nil))
		if err != nil {
			t.Fatalf("request %d error = %v", requestNumber, err)
		}
		want := http.StatusNoContent
		if requestNumber == 201 {
			want = http.StatusTooManyRequests
		}
		if response.StatusCode != want {
			if err := response.Body.Close(); err != nil {
				t.Errorf("close response for request %d: %v", requestNumber, err)
			}
			t.Fatalf("request %d status = %d, want %d", requestNumber, response.StatusCode, want)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatalf("close response for request %d: %v", requestNumber, err)
		}
	}
}

func TestUnmatchedAPIPathUsesSharedErrorContract(t *testing.T) {
	httpServer, _ := newOriginGuardTestServer(t)
	response, err := httpServer.app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/not-registered", nil))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	var body apierror.Response
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "RESOURCE_NOT_FOUND" ||
		body.RequestID == "" ||
		body.RequestID != response.Header.Get(apierror.RequestIDHeader) {
		t.Fatalf("unmatched route error = %#v; header=%q", body, response.Header.Get(apierror.RequestIDHeader))
	}
}

func TestExternalAndOAuthCallbackPathsAreOriginAndCSRFExempt(t *testing.T) {
	httpServer, router := newOriginGuardTestServer(t)
	router.Post("/external/test", func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})
	router.Get("/auth/callback", func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})

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
	_, err := NewFiberHTTPServer(swaggerTestConfig{
		cors: config.CORS{AllowOrigins: "*"},
	}, zap.NewNop())
	if err == nil {
		t.Fatal("wildcard CORS origin was accepted with credentials enabled")
	}
}
