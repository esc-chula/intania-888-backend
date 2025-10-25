package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/esc-chula/intania-888-backend/docs"

	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"go.uber.org/zap"

	swagger "github.com/arsmn/fiber-swagger/v2"
)

type FiberHttpServer struct {
	app    *fiber.App
	cfg    config.Config
	logger *zap.Logger
}

func NewFiberHttpServer(cfg config.Config, logger *zap.Logger) *FiberHttpServer {
	return &FiberHttpServer{
		app:    fiber.New(),
		cfg:    cfg,
		logger: logger,
	}
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

	// basic authentication for swagger
	router.Use("/swagger/*", basicauth.New(basicauth.Config{
		Users: map[string]string{
			s.cfg.GetSwagger().Username: s.cfg.GetSwagger().Password,
		},
		Unauthorized: func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderWWWAuthenticate, `Basic realm="Restricted"`)
			return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
		},
	}))

	// swagger
	router.Get("/swagger/*", swagger.HandlerDefault)

	// healthcheck
	router.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("server is running !")
	})

	return router
}

func (s *FiberHttpServer) OriginGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api/v1/external/") {
			return c.Next()
		}

		if c.Path() == "/api/v1/auth/callback" {
			return c.Next()
		}

		origin := c.Get("Origin")
		s.logger.Info("OriginGuard", zap.String("origin", origin))

		//todo: remove this redundanct shit, have fun
		allowedOrigins := strings.Split(s.cfg.GetCors().AllowOrigins, ",")
		isAllowed := false
		for _, allowed := range allowedOrigins {
			if origin == strings.TrimSpace(allowed) {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized",
			})
		}

		return c.Next()
	}
}
