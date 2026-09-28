package main

import (
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

// @title Intania888 Backend - API
// @version 1.0.0-breaking
// @description Breaking backend release: all Money fields are fixed two-decimal strings and the current frontend is incompatible until migrated.
// @description All failed /api/v1 requests use the apierror.Response schema with stable codes and X-Request-ID. See docs/README.md for the code catalog and frontend migration handoff.

// @host      localhost:8080
// @schemes   http
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and the token
//
// @securityDefinitions.apikey CookieSession
// @in header
// @name Cookie
// @description Browser cookie session (production uses __Host-session). Mutations also require X-CSRF-Token and an allowed Origin. Swagger 2.0 has no native cookie authentication; use an authenticated browser session.
func main() {
	// config setup
	cfg := config.GetConfig()
	if err := config.ValidateSecurity(cfg); err != nil {
		panic("invalid security configuration: " + err.Error())
	}
	defaultDailyReward, err := value.ParseMoney(cfg.GetDailyReward().DefaultAmount)
	if err != nil {
		panic("invalid DAILY_REWARD_DEFAULT_AMOUNT: " + err.Error())
	}

	isProduction := strings.EqualFold(strings.TrimSpace(cfg.GetServer().Env), "production")
	db := database.NewGORMDatabase(cfg)
	cache := cache.NewRedisClient(cfg)
	logger := logger.NewLogger(cfg)
	oauthConfig := oauth.LoadOAuthConfig(cfg)

	// init all layers
	userRepo := user.NewGORMRepository(db)
	userSvc := user.NewService(userRepo, logger.Named("UserSvc"))
	userHTTP := user.NewHTTPHandler(userSvc)
	policyRepo := policy.NewGORMRepository(db)
	policySvc := policy.NewService(policyRepo, policy.NewRedisSnapshotCache(cache), logger.Named("PolicySvc"))
	policyHTTP := policy.NewHTTPHandler(policySvc)

	authRepo := auth.NewRedisRepository(cache)
	authSvc := auth.NewService(
		authRepo,
		userRepo,
		cfg,
		oauth.NewGoogleOAuthClient(oauthConfig, logger),
		policySvc,
	)

	midRepo := middleware.NewGORMRepository(db)
	midSvc := middleware.NewService(midRepo, middleware.NewRedisSessionStore(cache), cfg, policySvc)
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
	httpServer, err := server.NewFiberHTTPServer(cfg, logger)
	if err != nil {
		logger.Fatal("invalid Swagger configuration", zap.Error(err))
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
	sportTypeHTTP.RegisterRoutes(router, midHTTP.AuthMiddleware)

	// Register external API routes. Deprecated: retain them while their original purpose and
	// consumers are investigated. Do not add new integrations to these routes.
	externalRouter := router.Group("/external")
	//nolint:staticcheck // Keep the deprecated route available while its consumers are investigated.
	userHTTP.RegisterExternalRoutes(externalRouter, midHTTP.ExternalAPIMiddleware)
	//nolint:staticcheck // Keep the deprecated route available while its consumers are investigated.
	authHTTP.RegisterExternalRoutes(externalRouter, midHTTP.ExternalAPIMiddleware)

	// start server
	httpServer.Start()
}
