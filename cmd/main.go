package main

import (
	"github.com/wiraphatys/intania888/cmd/server"
	"github.com/wiraphatys/intania888/internal/domain/auth"
	"github.com/wiraphatys/intania888/internal/domain/bill"
	"github.com/wiraphatys/intania888/internal/domain/middleware"
	"github.com/wiraphatys/intania888/internal/domain/user"
	"github.com/wiraphatys/intania888/pkg/cache"
	"github.com/wiraphatys/intania888/pkg/config"
	"github.com/wiraphatys/intania888/pkg/database"
	"github.com/wiraphatys/intania888/pkg/logger"
	"github.com/wiraphatys/intania888/pkg/oauth"
)

// @title Intania888 Backend - API
// @version 0.0.0
// @description  This is an Intania888 Backend API in Intania888 project.

// @host      localhost:8080
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
	userSvc := user.NewUserService(userRepo, logger)
	userHttp := user.NewUserHttpHandler(userSvc)

	authRepo := auth.NewAuthRepository(*cache)
	authSvc := auth.NewAuthService(authRepo, userRepo, cfg, logger, oauth.NewGoogleOAuthClient(oauthConfig, logger))
	authHttp := auth.NewAuthHttpHandler(authSvc)

	midRepo := middleware.NewMiddlewareRepository(db)
	midSvc := middleware.NewMiddlewareService(midRepo, cache, logger, cfg)
	midHttp := middleware.NewMiddlewareHttpHandler(midSvc, logger)

	billRepo := bill.NewBillRepository(db)
	billSvc := bill.NewBillService(billRepo, logger)
	billHttp := bill.NewBillHttpHandler(billSvc)

	// init router
	server := server.NewFiberHttpServer(cfg, logger)
	router := server.InitHttpServer()

	// register routes
	userHttp.RegisterRoutes(router, midHttp)
	authHttp.RegisterRoutes(router, midHttp)
	billHttp.RegisterRoutes(router, midHttp)

	// start server
	server.Start()
}
