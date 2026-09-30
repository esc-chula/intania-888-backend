package server

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/esc-chula/intania-888-backend/docs"
	"github.com/esc-chula/intania-888-backend/internal/apierror"

	"github.com/esc-chula/intania-888-backend/pkg/config"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	swagger "github.com/arsmn/fiber-swagger/v2"
)

const (
	shutdownTimeout  = 10 * time.Second
	readinessTimeout = time.Second
)

//go:embed swagger-session.js
var swaggerSessionScript []byte

// ReadinessCheck verifies that one application dependency can serve requests.
type ReadinessCheck func(context.Context) error

// FiberHTTPServer owns the HTTP application, shared middleware, and route composition.
type FiberHTTPServer struct {
	app               *fiber.App
	cfg               config.Config
	logger            *zap.Logger
	allowedOrigins    map[string]struct{}
	readinessChecks   []ReadinessCheck
	openAPIDocument   []byte
	swaggerAPIBaseURL string
	swaggerAPIOrigin  string
}

// NewFiberHTTPServer constructs the HTTP application after validating origins and documentation settings.
func NewFiberHTTPServer(cfg config.Config, logger *zap.Logger, readinessChecks ...ReadinessCheck) (*FiberHTTPServer, error) {
	allowedOrigins, err := parseAllowedOrigins(cfg.GetCORS().AllowOrigins)
	if err != nil {
		return nil, err
	}
	openAPIDocument, swaggerAPIBaseURL, swaggerAPIOrigin, err := configureOpenAPIDocument(cfg)
	if err != nil {
		return nil, err
	}
	if swaggerAPIOrigin != "" {
		allowedOrigins[swaggerAPIOrigin] = struct{}{}
	}

	if cfg.GetSwagger().Enabled && cfg.GetSwagger().RequireAuth &&
		(strings.TrimSpace(cfg.GetSwagger().Username) == "" ||
			strings.TrimSpace(cfg.GetSwagger().Password) == "") {
		return nil, fmt.Errorf("swagger Basic Auth requires both SWAGGER_USERNAME and SWAGGER_PASSWORD")
	}

	return &FiberHTTPServer{
		app: fiber.New(fiber.Config{
			ErrorHandler: apierror.ErrorHandler(logger),
		}),
		cfg:               cfg,
		logger:            logger,
		allowedOrigins:    allowedOrigins,
		readinessChecks:   readinessChecks,
		openAPIDocument:   openAPIDocument,
		swaggerAPIBaseURL: swaggerAPIBaseURL,
		swaggerAPIOrigin:  swaggerAPIOrigin,
	}, nil
}

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

// Start listens on the configured address and blocks until an interrupt or termination signal.
// Listener and shutdown errors are returned so callers can close their dependencies.
func (s *FiberHTTPServer) Start() error {
	url := fmt.Sprintf("%v:%d", s.cfg.GetServer().Host, s.cfg.GetServer().Port)

	// init modules

	// Setup signal capturing for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)
	listenErr := make(chan error, 1)

	// Run the server in a goroutine so it doesn't block
	go func() {
		s.logger.Sugar().Infof("SUCU Backend is starting on %v", url)
		listenErr <- s.app.Listen(url)
	}()

	select {
	case err := <-listenErr:
		if err != nil {
			return fmt.Errorf("listen on %s: %w", url, err)
		}

		return nil
	case <-quit:
	}
	s.logger.Sugar().Info("Gracefully shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shut down the server
	if err := s.shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	s.logger.Sugar().Info("Server shutdown complete.")

	return nil
}

func (s *FiberHTTPServer) shutdown(ctx context.Context) error {
	return s.app.ShutdownWithContext(ctx)
}

// InitHTTPServer installs request IDs, documentation routes, and shared API middleware.
// It returns the /api/v1 router for feature registration and should be called once at startup.
func (s *FiberHTTPServer) InitHTTPServer() fiber.Router {
	s.app.Use(apierror.RequestID())
	s.app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
	})
	s.app.Get("/readyz", s.readinessHandler())
	s.registerSwagger()

	// set global prefix
	router := s.app.Group("/api/v1")

	// apply origin guard
	router.Use(s.OriginGuard())

	// enable cors
	corsAllowOrigins := s.cfg.GetCORS().AllowOrigins
	if s.swaggerAPIOrigin != "" {
		corsAllowOrigins = strings.Trim(strings.Join([]string{corsAllowOrigins, s.swaggerAPIOrigin}, ","), ",")
	}
	router.Use(cors.New(cors.Config{
		AllowOrigins:     corsAllowOrigins,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders:     "Origin,X-PINGOTHER,Accept,Authorization,Content-Type,X-CSRF-Token",
		ExposeHeaders:    "Link,X-Request-ID",
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// init logger
	router.Use(logger.New(logger.Config{
		Format:     "${time} request_id=${locals:request_id} ${status} - ${method} ${path}\n",
		TimeFormat: "2006/01/02 15:04:05",
		TimeZone:   "Asia/Bangkok",
	}))

	router.Use(limiter.New(limiter.Config{
		Max:        200,
		Expiration: 60 * time.Second,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return apierror.New(fiber.StatusTooManyRequests, apierror.CodeTooManyRequests, "Too many requests")
		},
	}))

	// healthcheck
	router.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("server is running !")
	})

	return router
}

func (s *FiberHTTPServer) readinessHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(context.Background(), readinessTimeout)
		defer cancel()

		for _, check := range s.readinessChecks {
			if err := check(ctx); err != nil {
				s.logger.Warn("Readiness check failed", zap.Error(err))

				return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "not_ready"})
			}
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ready"})
	}
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
		injectedScript := fmt.Sprintf(
			"    <script src=\"/swagger/swagger-session.js\" data-api-base=\"%s\" data-login-client-id=\"888-web\"></script>\n%s",
			template.HTMLEscapeString(s.swaggerAPIBaseURL),
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

// OriginGuard enforces exact configured browser origins and permits safe reads without Origin.
// External routes and the OAuth callback are exempt from this browser origin policy.
func (s *FiberHTTPServer) OriginGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if isExternalPath(c.Path()) || c.Path() == "/api/v1/auth/callback" ||
			(c.Method() == fiber.MethodPost && (c.Path() == "/api/v1/auth/token" || c.Path() == "/api/v1/auth/revoke")) {
			return c.Next()
		}

		origin := strings.TrimSpace(c.Get(fiber.HeaderOrigin))
		if origin == "" && (c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead) {
			return c.Next()
		}
		if !s.isAllowedOrigin(origin) {
			return apierror.New(fiber.StatusForbidden, apierror.CodeForbidden, "Origin is not allowed")
		}

		return c.Next()
	}
}

func (s *FiberHTTPServer) isAllowedOrigin(origin string) bool {
	canonical, err := canonicalOrigin(origin)
	if err != nil {
		return false
	}
	_, ok := s.allowedOrigins[canonical]

	return ok
}

func isExternalPath(path string) bool {
	return path == "/api/v1/external" || strings.HasPrefix(path, "/api/v1/external/")
}

func parseAllowedOrigins(rawOrigins string) (map[string]struct{}, error) {
	allowedOrigins := make(map[string]struct{})
	for _, rawOrigin := range strings.Split(rawOrigins, ",") {
		rawOrigin = strings.TrimSpace(rawOrigin)
		if rawOrigin == "" {
			continue
		}
		if rawOrigin == "*" {
			return nil, fmt.Errorf("CORS_ALLOW_ORIGINS cannot use wildcard origins with credentials")
		}
		origin, err := canonicalOrigin(rawOrigin)
		if err != nil {
			return nil, fmt.Errorf("invalid configured CORS origin %q", rawOrigin)
		}
		allowedOrigins[origin] = struct{}{}
	}

	return allowedOrigins, nil
}

func canonicalOrigin(rawOrigin string) (string, error) {
	parsed, err := url.Parse(rawOrigin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("origin must contain only scheme, host, and port")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("origin scheme must be http or https")
	}

	return scheme + "://" + strings.ToLower(parsed.Host), nil
}
