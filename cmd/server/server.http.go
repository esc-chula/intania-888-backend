package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/apierror"

	"github.com/esc-chula/intania-888-backend/pkg/config"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"go.uber.org/zap"
)

const (
	shutdownTimeout  = 10 * time.Second
	readinessTimeout = time.Second
)

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
	s.registerRootHTTPFeatures()

	router := s.app.Group("/api/v1")
	s.registerAPIMiddleware(router)
	s.registerAPIRoutes(router)

	return router
}

func (s *FiberHTTPServer) registerRootHTTPFeatures() {
	s.app.Use(apierror.RequestID())

	// Register health/readiness Checks
	s.app.Get("/healthz", s.healthcheckHandler())
	s.app.Get("/readyz", s.readinessHandler())

	s.registerSwagger()
}

func (s *FiberHTTPServer) registerAPIMiddleware(router fiber.Router) {
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

	registerRateLimiters(router)
}

func (s *FiberHTTPServer) registerAPIRoutes(router fiber.Router) {
	router.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("server is running !")
	})
}

func (s *FiberHTTPServer) healthcheckHandler() fiber.Handler {
	return func(c *fiber.Ctx) error { return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"}) }
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
