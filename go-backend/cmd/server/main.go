package main

import (
	"fmt"
	"log/slog"
	"os"

	_ "systemburo/docs"
	"systemburo/internal/config"
	"systemburo/internal/database"
	"systemburo/internal/handlers"
	mw "systemburo/internal/middleware"
	"systemburo/internal/router"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// @title           Systemburo API
// @version         1.0
// @description     API системы управления пропусками (Бюро пропусков)
// @host            localhost:8090
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT токен в формате: Bearer {token}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Setup logging
	var logLevel slog.Level
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	// Connect to database
	gormLogLevel := logger.Silent
	if cfg.LogLevel == "debug" {
		gormLogLevel = logger.Info
	}
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	})
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to database")

	// AutoMigrate all tables (like Laravel Schema::create)
	if err := database.AutoMigrate(db); err != nil {
		slog.Error("AutoMigrate failed", "error", err)
		os.Exit(1)
	}

	// Seed initial data
	if err := database.Seed(db); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}

	// Setup Echo
	e := echo.New()
	e.HideBanner = true

	// Global middleware
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	e.Use(mw.CORS())
	e.Use(mw.RateLimit(200, 60))

	// Services
	authService := services.NewAuthService(db, cfg.JWTSecret, cfg.JWTRefreshSecret)
	userTypeService := services.NewUserTypeService(db)
	lpfService := services.NewLicensePlateFormatService(db)
	attachmentService := services.NewAttachmentService(db)
	citizenshipService := services.NewCitizenshipService(db)
	organizationService := services.NewOrganizationService(db)
	companyService := services.NewCompanyService(db)
	userService := services.NewUserService(db)
	unloadPlaceService := services.NewUnloadPlaceService(db)
	carService := services.NewCarService(db)
	employeeService := services.NewEmployeeService(db)
	systemTableService := services.NewSystemTableService(db)
	uniqueCarService := services.NewUniqueCarService(db)
	uniqueEmployeeService := services.NewUniqueEmployeeService(db)
	feedbackService := services.NewFeedbackService(db)
	applicationService := services.NewApplicationService(db)
	approverService := services.NewApproverService(db)

	// Handlers
	authHandler := handlers.NewAuthHandler(authService)
	userTypesHandler := handlers.NewUserTypesHandler(userTypeService)
	lpfHandler := handlers.NewLicensePlateFormatHandler(lpfService)
	attachmentHandler := handlers.NewAttachmentHandler(attachmentService)
	citizenshipHandler := handlers.NewCitizenshipHandler(citizenshipService)
	organizationHandler := handlers.NewOrganizationHandler(organizationService, db)
	companyHandler := handlers.NewCompanyHandler(companyService)
	usersHandler := handlers.NewUsersHandler(userService)
	unloadPlaceHandler := handlers.NewUnloadPlaceHandler(unloadPlaceService)
	carHandler := handlers.NewCarHandler(carService)
	employeeHandler := handlers.NewEmployeeHandler(employeeService)
	systemTableHandler := handlers.NewSystemTableHandler(systemTableService)
	uniqueCarHandler := handlers.NewUniqueCarHandler(uniqueCarService)
	uniqueEmployeeHandler := handlers.NewUniqueEmployeeHandler(uniqueEmployeeService)
	feedbackHandler := handlers.NewFeedbackHandler(feedbackService)
	applicationHandler := handlers.NewApplicationHandler(applicationService)
	approverHandler := handlers.NewApproverHandler(approverService)

	// Swagger UI: http://localhost:8090/swagger/index.html
	e.GET("/swagger/*", echoSwagger.WrapHandler)

	// Routes
	router.Setup(e, authHandler, userTypesHandler, attachmentHandler, lpfHandler, citizenshipHandler, organizationHandler, companyHandler, usersHandler, unloadPlaceHandler, carHandler, employeeHandler, systemTableHandler, uniqueCarHandler, uniqueEmployeeHandler, feedbackHandler, applicationHandler, approverHandler, []byte(cfg.JWTSecret))

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.BindHost, cfg.BindPort)
	slog.Info("starting server", "addr", addr)
	if err := e.Start(addr); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
