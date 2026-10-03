package server

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	swagger "github.com/arsmn/fiber-swagger/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"gopkg.in/yaml.v3"

	"github.com/esc-chula/intania-888-backend/docs"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

//go:embed swagger-session.js
var swaggerSessionScript []byte

func configureOpenAPIDocument(cfg config.Config) ([]byte, string, string, error) {
	if !cfg.GetSwagger().Enabled {
		return nil, "", "", nil
	}

	rawURL := strings.TrimSpace(cfg.GetServer().URL)
	if rawURL == "" {
		return nil, "", "", fmt.Errorf("SERVER_URL is required when Swagger is enabled")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, "", "", fmt.Errorf("SERVER_URL must be a full URL with scheme and host")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, "", "", fmt.Errorf("SERVER_URL must use http or https")
	}
	if parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return nil, "", "", fmt.Errorf("SERVER_URL must not contain credentials, query parameters, or fragments")
	}

	document, err := docs.ReadOpenAPI()
	if err != nil {
		return nil, "", "", fmt.Errorf("read OpenAPI document: %w", err)
	}
	var specification map[string]any
	if err := yaml.Unmarshal(document, &specification); err != nil {
		return nil, "", "", fmt.Errorf("parse OpenAPI document: %w", err)
	}

	basePath := strings.TrimRight(parsedURL.EscapedPath(), "/")
	if basePath == "" {
		basePath = "/api/v1"
	}
	serverURL := parsedURL.Scheme + "://" + parsedURL.Host + basePath
	specification["servers"] = []any{
		map[string]any{"url": serverURL},
	}
	serverOrigin, err := canonicalOrigin(parsedURL.Scheme + "://" + parsedURL.Host)
	if err != nil {
		return nil, "", "", fmt.Errorf("parse Swagger API origin: %w", err)
	}

	document, err = yaml.Marshal(specification)
	if err != nil {
		return nil, "", "", fmt.Errorf("render OpenAPI document: %w", err)
	}

	return document, serverURL, serverOrigin, nil
}

func (s *FiberHTTPServer) registerSwagger() {
	swaggerConfig := s.cfg.GetSwagger()
	if !swaggerConfig.Enabled {
		return
	}

	if swaggerConfig.RequireAuth {
		s.app.Use("/swagger/*", basicauth.New(basicauth.Config{
			Users: map[string]string{
				swaggerConfig.Username: swaggerConfig.Password,
			},
			Unauthorized: func(c *fiber.Ctx) error {
				c.Set(fiber.HeaderWWWAuthenticate, `Basic realm="Restricted"`)

				return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
			},
		}))
	}

	s.app.Get("/swagger/openapi.yaml", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "application/yaml; charset=utf-8")
		c.Set(fiber.HeaderCacheControl, "no-store")

		return c.Send(s.openAPIDocument)
	})
	s.app.Get("/swagger/doc.json", func(c *fiber.Ctx) error {
		return c.Redirect("/swagger/openapi.yaml", fiber.StatusMovedPermanently)
	})
	s.app.Get("/swagger/swagger-session.js", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "application/javascript; charset=utf-8")
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.Send(swaggerSessionScript)
	})

	swaggerHandler := swagger.New(swagger.Config{
		URL:                    "/swagger/openapi.yaml",
		Title:                  "Intania 888 API",
		WithCredentials:        true,
		TryItOutEnabled:        true,
		DisplayRequestDuration: true,
		RequestInterceptor:     template.JS("window.IntaniaSwaggerSession.requestInterceptor"),
		ResponseInterceptor:    template.JS("window.IntaniaSwaggerSession.responseInterceptor"),
		OnComplete:             template.JS("window.IntaniaSwaggerSession.onComplete"),
	})
	s.app.Get("/swagger/*", func(c *fiber.Ctx) error {
		if c.Path() != "/swagger/index.html" {
			return swaggerHandler(c)
		}

		if err := swaggerHandler(c); err != nil {
			return err
		}

		responseBody := c.Response().Body()
		// Load the embedded helper after Swagger's bundles and before initialization.
		marker := []byte("    <script>\n    window.onload = function() {")
		loginClientID := s.cfg.GetOAuth().SwaggerApplicationID(s.swaggerAPIOrigin)

		injectedScript := fmt.Sprintf(
			"    <script src=\"/swagger/swagger-session.js\" data-api-base=\"%s\" data-login-client-id=\"%s\"></script>\n%s",
			template.HTMLEscapeString(s.swaggerAPIBaseURL),
			template.HTMLEscapeString(loginClientID),
			marker,
		)
		updatedBody := strings.Replace(
			string(responseBody),
			string(marker),
			injectedScript,
			1,
		)
		if updatedBody == string(responseBody) {
			return fmt.Errorf("inject Swagger session script: index template marker not found")
		}

		return c.SendString(updatedBody)
	})
}
