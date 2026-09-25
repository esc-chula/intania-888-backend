package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/esc-chula/intania-888-backend/docs"

	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"go.uber.org/zap"

	swagger "github.com/arsmn/fiber-swagger/v2"
)

type FiberHttpServer struct {
	app            *fiber.App
	cfg            config.Config
	logger         *zap.Logger
	allowedOrigins map[string]struct{}
}

func NewFiberHttpServer(cfg config.Config, logger *zap.Logger) (*FiberHttpServer, error) {
	allowedOrigins, err := parseAllowedOrigins(cfg.GetCors().AllowOrigins)
	if err != nil {
		return nil, err
	}
	if err := configureSwaggerInfo(cfg); err != nil {
		return nil, err
	}

	if cfg.GetSwagger().Enabled && cfg.GetSwagger().RequireAuth && (strings.TrimSpace(cfg.GetSwagger().Username) == "" || strings.TrimSpace(cfg.GetSwagger().Password) == "") {
		return nil, fmt.Errorf("swagger Basic Auth requires both SWAGGER_USERNAME and SWAGGER_PASSWORD")
	}

	return &FiberHttpServer{
		app:            fiber.New(),
		cfg:            cfg,
		logger:         logger,
		allowedOrigins: allowedOrigins,
	}, nil
}

func configureSwaggerInfo(cfg config.Config) error {
	if !cfg.GetSwagger().Enabled {
		return nil
	}

	rawURL := strings.TrimSpace(cfg.GetServer().Url)
	if rawURL == "" {
		return fmt.Errorf("SERVER_URL is required when Swagger is enabled")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("SERVER_URL must be a full URL with scheme and host")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("SERVER_URL must use http or https")
	}
	if parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return fmt.Errorf("SERVER_URL must not contain credentials, query parameters, or fragments")
	}

	docs.SwaggerInfo.Host = parsedURL.Host
	docs.SwaggerInfo.Schemes = []string{parsedURL.Scheme}
	basePath := strings.TrimRight(parsedURL.EscapedPath(), "/")
	if basePath == "" {
		basePath = "/api/v1"
	}
	docs.SwaggerInfo.BasePath = basePath

	return nil
}

func (s *FiberHttpServer) Start() {
	url := fmt.Sprintf("%v:%d", s.cfg.GetServer().Host, s.cfg.GetServer().Port)

	// init modules

	// Setup signal capturing for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Run the server in a goroutine so it doesn't block
	go func() {
		s.logger.Sugar().Infof("SUCU Backend is starting on %v", url)
		if err := s.app.Listen(url); err != nil {
			s.logger.Sugar().Fatalf("Error while starting server: %v", err)
		}
	}()

	// Wait for a termination signal
	<-quit
	s.logger.Sugar().Info("Gracefully shutting down server...")

	// Create a deadline for shutdown
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shut down the server
	if err := s.app.Shutdown(); err != nil {
		s.logger.Sugar().Fatalf("Error during server shutdown: %v", err)
	}

	s.logger.Sugar().Info("Server shutdown complete.")
}

func (s *FiberHttpServer) InitHttpServer() fiber.Router {
	s.registerSwagger()

	// set global prefix
	router := s.app.Group("/api/v1")

	// apply origin guard
	router.Use(s.OriginGuard())

	// enable cors
	router.Use(cors.New(cors.Config{
		AllowOrigins:     s.cfg.GetCors().AllowOrigins,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders:     "Origin,X-PINGOTHER,Accept,Authorization,Content-Type,X-CSRF-Token",
		ExposeHeaders:    "Link",
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// init logger
	router.Use(logger.New(logger.Config{
		Format:     "${time} ${status} - ${method} ${path}\n",
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
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"message": "Too many requests, please try again later.",
			})
		},
	}))

	router.Use(s.CSRFGuard())

	// healthcheck
	router.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("server is running !")
	})

	return router
}

func (s *FiberHttpServer) registerSwagger() {
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

	s.app.Get("/swagger/*", swagger.HandlerDefault)
}

func (s *FiberHttpServer) OriginGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if isExternalPath(c.Path()) || c.Path() == "/api/v1/auth/callback" {
			return c.Next()
		}

		origin := strings.TrimSpace(c.Get(fiber.HeaderOrigin))
		if origin == "" && (c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead) {
			return c.Next()
		}
		if !s.isAllowedOrigin(origin) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "origin is not allowed",
			})
		}

		return c.Next()
	}
}

func (s *FiberHttpServer) CSRFGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if isExternalPath(c.Path()) || c.Path() == "/api/v1/auth/callback" || c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead || c.Method() == fiber.MethodOptions {
			return c.Next()
		}

		cookieToken := c.Cookies(utils.CSRFTokenCookieName)
		headerToken := c.Get("X-CSRF-Token")
		if cookieToken == "" || headerToken == "" || subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "invalid CSRF token",
			})
		}

		return c.Next()
	}
}

func (s *FiberHttpServer) isAllowedOrigin(origin string) bool {
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
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("origin must contain only scheme, host, and port")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("origin scheme must be http or https")
	}
	return scheme + "://" + strings.ToLower(parsed.Host), nil
}
