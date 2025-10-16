package main

import (
	"github.com/esc-chula/intania-888-backend/cmd/server"
	"github.com/esc-chula/intania-888-backend/internal/domain/auth"
	"github.com/esc-chula/intania-888-backend/internal/domain/bill"
	"github.com/esc-chula/intania-888-backend/internal/domain/color"
	"github.com/esc-chula/intania-888-backend/internal/domain/event"
	"github.com/esc-chula/intania-888-backend/internal/domain/match"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/domain/sporttype"
	"github.com/esc-chula/intania-888-backend/internal/domain/stakemine"
	"github.com/esc-chula/intania-888-backend/internal/domain/user"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/pkg/database"
	"github.com/esc-chula/intania-888-backend/pkg/logger"
	"github.com/esc-chula/intania-888-backend/pkg/oauth"
)

// @title Intania888 Backend - API
// @version 0.0.0
// @description  This is an Intania888 Backend API in Intania888 project.

// @host      https://888api.chula.engineering
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and the token
func main() {
	// config setup
	cfg := config.GetConfig()
	db := database.NewGormDatabase(cfg)
	cache := cache.NewRedisClient(cfg)
	logger := logger.NewLogger(cfg)
	oauthConfig := oauth.LoadOAuthConfig(cfg)

	// init all layers
	userRepo := user.NewUserRepository(db)
	userSvc := user.NewUserService(userRepo, db, logger.Named("UserSvc"))
	userHttp := user.NewUserHttpHandler(userSvc)

	authRepo := auth.NewAuthRepository(*cache)
	authSvc := auth.NewAuthService(authRepo, userRepo, cfg, logger.Named("AuthSvc"), oauth.NewGoogleOAuthClient(oauthConfig, logger))
	authHttp := auth.NewAuthHttpHandler(authSvc)

	midRepo := middleware.NewMiddlewareRepository(db)
	midSvc := middleware.NewMiddlewareService(midRepo, cache, logger.Named("MiddlewareSvc"), cfg)
	midHttp := middleware.NewMiddlewareHttpHandler(midSvc, logger)

	billRepo := bill.NewBillRepository(db)
	billSvc := bill.NewBillService(billRepo, userRepo, db, logger.Named("BillSvc"))
	billHttp := bill.NewBillHttpHandler(billSvc)

	matchRepo := match.NewMatchRepository(db)
	matchSvc := match.NewMatchService(matchRepo, logger.Named("MatchSvc"))
	matchHttp := match.NewMatchHttpHandler(matchSvc)

	colorRepo := color.NewColorRepository(db)
	colorSvc := color.NewColorService(colorRepo, logger.Named("ColorSvc"))
	colorHttp := color.NewColorHttpHandler(colorSvc)

	eventRepo := event.NewEventRepository(db, *cache)
	eventSvc := event.NewEventService(eventRepo, userRepo, cfg, logger)
	eventHttp := event.NewEventHttpHandler(eventSvc)

	stakeMineRepo := stakemine.NewStakeMineRepository(db)
	stakeMineSvc := stakemine.NewStakeMineService(stakeMineRepo, db, logger.Named("StakeMineSvc"))
	stakeMineHttp := stakemine.NewStakeMineHttpHandler(stakeMineSvc)
	sportTypeRepo := sporttype.NewSportTypeRepository(db)
	sportTypeSvc := sporttype.NewSportTypeService(sportTypeRepo, logger.Named("SportTypeSvc"))
	sportTypeHttp := sporttype.NewSportTypeHttpHandler(sportTypeSvc)

	// init router
	server := server.NewFiberHttpServer(cfg, logger)
	router := server.InitHttpServer()

	// register routes
	userHttp.RegisterRoutes(router, midHttp)
	authHttp.RegisterRoutes(router, midHttp)
	billHttp.RegisterRoutes(router, midHttp)
	matchHttp.RegisterRoutes(router, midHttp)
	colorHttp.RegisterRoutes(router, midHttp)
	eventHttp.RegisterRoutes(router, midHttp)
	stakeMineHttp.RegisterRoutes(router, midHttp)
	sportTypeHttp.RegisterRoutes(router, midHttp)

	// register external API routes
	externalRouter := router.Group("/external")
	userHttp.RegisterExternalRoutes(externalRouter, midHttp)

	// start server
	server.Start()
}
