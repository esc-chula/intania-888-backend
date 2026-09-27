package main

import (
	"strings"

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
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/pkg/database"
	"github.com/esc-chula/intania-888-backend/pkg/logger"
	"github.com/esc-chula/intania-888-backend/pkg/oauth"
	"go.uber.org/zap"
)

// @title Intania888 Backend - API
// @version 1.0.0-breaking
// @description Breaking backend release: all Money fields are fixed two-decimal strings and the current frontend is incompatible until migrated.

// @host      localhost:8080
// @schemes   http
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and the token
func main() {
	// config setup
	cfg := config.GetConfig()
	if err := config.ValidateSecurity(cfg); err != nil {
		panic("invalid security configuration: " + err.Error())
	}
	defaultDailyReward, err := model.ParseMoney(cfg.GetDailyReward().DefaultAmount)
	if err != nil {
		panic("invalid DAILY_REWARD_DEFAULT_AMOUNT: " + err.Error())
	}

	isProduction := strings.EqualFold(strings.TrimSpace(cfg.GetServer().Env), "production")
	db := database.NewGormDatabase(cfg)
	cache := cache.NewRedisClient(cfg)
	logger := logger.NewLogger(cfg)
	oauthConfig := oauth.LoadOAuthConfig(cfg)

	// init all layers
	userRepo := user.NewUserRepository(db)
	userSvc := user.NewUserService(userRepo, db, logger.Named("UserSvc"))
	userHttp := user.NewUserHttpHandler(userSvc)
	policyRepo := policy.NewRepository(db)
	policySvc := policy.NewService(policyRepo, cache, logger.Named("PolicySvc"))
	policyHttp := policy.NewHttpHandler(policySvc)

	authRepo := auth.NewAuthRepository(*cache)
	authSvc := auth.NewAuthService(
		authRepo,
		userRepo,
		cfg,
		logger.Named("AuthSvc"),
		oauth.NewGoogleOAuthClient(oauthConfig, logger),
		policySvc,
	)
	authHttp := auth.NewAuthHttpHandler(authSvc, cfg, isProduction)

	midRepo := middleware.NewMiddlewareRepository(db)
	midSvc := middleware.NewMiddlewareService(midRepo, cache, logger.Named("MiddlewareSvc"), cfg, policySvc)
	midHttp := middleware.NewMiddlewareHttpHandler(
		midSvc,
		logger,
		isProduction,
		cfg.GetSession().IdleTTLSeconds,
	)

	billRepo := bill.NewBillRepository(db)
	billSvc := bill.NewBillService(billRepo, userRepo, db, logger.Named("BillSvc"))
	billHttp := bill.NewBillHttpHandler(billSvc)

	matchRepo := match.NewMatchRepository(db)
	matchSvc := match.NewMatchService(matchRepo, db, logger.Named("MatchSvc"))
	matchHttp := match.NewMatchHttpHandler(matchSvc)

	colorRepo := color.NewColorRepository(db)
	colorSvc := color.NewColorService(colorRepo, logger.Named("ColorSvc"))
	colorHttp := color.NewColorHttpHandler(colorSvc)

	eventRepo := event.NewEventRepository(db, *cache)
	eventSvc := event.NewEventService(eventRepo, userRepo, defaultDailyReward, logger)
	eventHttp := event.NewEventHttpHandler(eventSvc)

	stakeMineRepo := stakemine.NewStakeMineRepository(db)
	stakeMineSvc := stakemine.NewStakeMineService(stakeMineRepo, db, logger.Named("StakeMineSvc"))
	stakeMineHttp := stakemine.NewStakeMineHttpHandler(stakeMineSvc)

	sportTypeRepo := sporttype.NewSportTypeRepository(db)
	sportTypeSvc := sporttype.NewSportTypeService(sportTypeRepo, logger.Named("SportTypeSvc"))
	sportTypeHttp := sporttype.NewSportTypeHttpHandler(sportTypeSvc)

	// init router
	httpServer, err := server.NewFiberHttpServer(cfg, logger)
	if err != nil {
		logger.Fatal("invalid Swagger configuration", zap.Error(err))
	}

	router := httpServer.InitHttpServer()

	// register routes
	userHttp.RegisterRoutes(router, midHttp)
	authHttp.RegisterRoutes(router, midHttp)
	policyHttp.RegisterRoutes(router, midHttp.AuthMiddleware, midHttp.AdminMiddleware)
	billHttp.RegisterRoutes(router, midHttp)
	matchHttp.RegisterRoutes(router, midHttp)
	colorHttp.RegisterRoutes(router, midHttp)
	eventHttp.RegisterRoutes(router, midHttp)
	stakeMineHttp.RegisterRoutes(router, midHttp)
	sportTypeHttp.RegisterRoutes(router, midHttp)

	// Register external API routes. Deprecated: retain them while their original purpose and
	// consumers are investigated. Do not add new integrations to these routes.
	externalRouter := router.Group("/external")
	//nolint:staticcheck // Keep the deprecated route available while its consumers are investigated.
	userHttp.RegisterExternalRoutes(externalRouter, midHttp)
	//nolint:staticcheck // Keep the deprecated route available while its consumers are investigated.
	authHttp.RegisterExternalRoutes(externalRouter, midHttp)

	// start server
	httpServer.Start()
}
