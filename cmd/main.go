package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/cmd/server"
	"github.com/esc-chula/intania-888-backend/internal/domain/auth"
	"github.com/esc-chula/intania-888-backend/internal/domain/bill"
	"github.com/esc-chula/intania-888-backend/internal/domain/color"
	"github.com/esc-chula/intania-888-backend/internal/domain/event"
	"github.com/esc-chula/intania-888-backend/internal/domain/match"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/domain/policy"
	"github.com/esc-chula/intania-888-backend/internal/domain/sporttype"
	"github.com/esc-chula/intania-888-backend/internal/domain/stakemine"
	"github.com/esc-chula/intania-888-backend/internal/domain/user"
	"github.com/esc-chula/intania-888-backend/internal/value"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/pkg/database"
	"github.com/esc-chula/intania-888-backend/pkg/logger"
	"github.com/esc-chula/intania-888-backend/pkg/oauth"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	// config setup
	cfg := config.GetConfig()
	if err := config.ValidateSecurity(cfg); err != nil {
		return fmt.Errorf("invalid security configuration: %w", err)
	}
	defaultDailyReward, err := value.ParseMoney(cfg.GetDailyReward().DefaultAmount)
	if err != nil {
		return fmt.Errorf("invalid DAILY_REWARD_DEFAULT_AMOUNT: %w", err)
	}

	isProduction := strings.EqualFold(strings.TrimSpace(cfg.GetServer().Env), "production")
	logger := logger.NewLogger(cfg)
	if logger == nil {
		return fmt.Errorf("SERVER_ENV must be development or production")
	}
	defer func() {
		_ = logger.Sync() //nolint:errcheck // Zap's standard stream sink may report EINVAL when syncing a pipe.
	}()

	db := database.NewGORMDatabase(cfg)
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get PostgreSQL connection pool: %w", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close PostgreSQL connection pool: %w", err))
		}
	}()

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 5*time.Second)
	if err := sqlDB.PingContext(startupCtx); err != nil {
		cancelStartup()
		return fmt.Errorf("check PostgreSQL at startup: %w", err)
	}
	cancelStartup()

	cacheClient := cache.NewRedisClient(cfg)
	defer func() {
		if err := cacheClient.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close Redis client: %w", err))
		}
	}()
	startupCtx, cancelStartup = context.WithTimeout(context.Background(), 5*time.Second)
	if err := cacheClient.Ping(startupCtx); err != nil {
		cancelStartup()
		return fmt.Errorf("check Redis at startup: %w", err)
	}
	cancelStartup()

	oauthConfig := oauth.LoadOAuthConfig(cfg)

	// init all layers
	userRepo := user.NewGORMRepository(db)
	userSvc := user.NewService(userRepo, logger.Named("UserSvc"))
	userHTTP := user.NewHTTPHandler(userSvc)
	policyRepo := policy.NewGORMRepository(db)
	policySvc := policy.NewService(policyRepo, policy.NewRedisSnapshotCache(cacheClient), logger.Named("PolicySvc"))
	policyHTTP := policy.NewHTTPHandler(policySvc)

	authRepo := auth.NewRedisRepository(cacheClient)
	authSvc := auth.NewService(
		authRepo,
		userRepo,
		cfg,
		oauth.NewGoogleOAuthClient(oauthConfig, logger),
		policySvc,
	)

	midRepo := middleware.NewGORMRepository(db)
	midSvc := middleware.NewService(midRepo, middleware.NewRedisSessionStore(cacheClient), cfg, policySvc)
	midHTTP := middleware.NewHTTPHandler(
		midSvc,
		isProduction,
		cfg.GetSession().IdleTTLSeconds,
	)
	authHTTP := auth.NewHTTPHandler(authSvc, midHTTP, cfg, isProduction)

	billRepo := bill.NewGORMRepository(db)
	billSvc := bill.NewService(billRepo, billRepo, time.Now, uuid.NewString)
	billHTTP := bill.NewHTTPHandler(billSvc)

	matchRepo := match.NewGORMRepository(db)
	matchSvc := match.NewService(matchRepo, matchRepo, time.Now, uuid.NewString)
	matchHTTP := match.NewHTTPHandler(matchSvc)

	colorRepo := color.NewGORMRepository(db)
	colorSvc := color.NewService(colorRepo, logger.Named("ColorSvc"))
	colorHTTP := color.NewHTTPHandler(colorSvc)

	eventRepo := event.NewGORMRepository(db)
	eventSvc := event.NewService(eventRepo, defaultDailyReward, logger)
	eventHTTP := event.NewHTTPHandler(eventSvc)

	stakeMineRepo := stakemine.NewGORMRepository(db)
	stakeMineSvc := stakemine.NewService(stakeMineRepo, logger.Named("StakeMineSvc"))
	stakeMineHTTP := stakemine.NewHTTPHandler(stakeMineSvc)

	sportTypeRepo := sporttype.NewGORMRepository(db)
	sportTypeSvc := sporttype.NewService(sportTypeRepo, logger.Named("SportTypeSvc"))
	sportTypeHTTP := sporttype.NewHTTPHandler(sportTypeSvc)

	// init router
	httpServer, err := server.NewFiberHTTPServer(cfg, logger,
		func(ctx context.Context) error { return sqlDB.PingContext(ctx) },
		cacheClient.Ping,
	)
	if err != nil {
		return fmt.Errorf("invalid Swagger configuration: %w", err)
	}

	router := httpServer.InitHTTPServer()

	// register routes
	userHTTP.RegisterRoutes(router, midHTTP.AuthMiddleware, midHTTP.AdminMiddleware)
	authHTTP.RegisterRoutes(router, midHTTP.AuthMiddleware, midHTTP.AdminMiddleware)
	policyHTTP.RegisterRoutes(router, midHTTP.AuthMiddleware, midHTTP.AdminMiddleware)
	billHTTP.RegisterRoutes(router, midHTTP.AuthMiddleware, midHTTP.AdminMiddleware)
	matchHTTP.RegisterRoutes(router, midHTTP.AuthMiddleware, midHTTP.AdminMiddleware)
	colorHTTP.RegisterRoutes(router, midHTTP.AuthMiddleware)
	eventHTTP.RegisterRoutes(router, midHTTP.AuthMiddleware, midHTTP.AdminMiddleware)
	stakeMineHTTP.RegisterRoutes(router, midHTTP.AuthMiddleware)
	sportTypeHTTP.RegisterRoutes(router, midHTTP.AuthMiddleware, midHTTP.AdminMiddleware)

	// Register external API routes. Deprecated: retain them while their original purpose and
	// consumers are investigated. Do not add new integrations to these routes.
	externalRouter := router.Group("/external")
	//nolint:staticcheck // Keep the deprecated route available while its consumers are investigated.
	userHTTP.RegisterExternalRoutes(externalRouter, midHTTP.ExternalAPIMiddleware)
	//nolint:staticcheck // Keep the deprecated route available while its consumers are investigated.
	authHTTP.RegisterExternalRoutes(externalRouter, midHTTP.ExternalAPIMiddleware)

	// start server
	if err := httpServer.Start(); err != nil {
		logger.Error("HTTP server stopped with an error", zap.Error(err))
		return err
	}
	return nil
}
