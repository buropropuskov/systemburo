package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "systemburo/docs"
	"systemburo/internal/api"
	"systemburo/internal/config"
	"systemburo/internal/crypto"
	"systemburo/internal/database"
	"systemburo/internal/handlers"
	mw "systemburo/internal/middleware"
	"systemburo/internal/router"
	"systemburo/internal/services"
	appvalidator "systemburo/internal/validator"

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

	encKey, err := crypto.ParseHexKey(cfg.DataEncryptionKey)
	if err != nil {
		slog.Error("invalid DATA_ENCRYPTION_KEY", "error", err)
		os.Exit(1)
	}
	crypto.SetGlobalKey(encKey)
	if encKey != nil {
		slog.Info("encryption enabled for personal data")
	} else {
		slog.Warn("DATA_ENCRYPTION_KEY not set, personal data stored unencrypted")
	}

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
	e.HTTPErrorHandler = handlers.CustomHTTPErrorHandler
	e.Validator = appvalidator.New()

	// Global middleware
	e.Use(mw.RequestID())
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	// Security headers: HSTS, CSP, X-Frame-Options и т.д. HSTS включается только
	// в режиме CookieSecure (prod/staging) - на http://localhost HSTS бесполезен
	// и даже вреден (браузер запомнит домен как HTTPS-only).
	hstsMaxAge := 0
	if cfg.CookieSecure {
		hstsMaxAge = 63072000 // 2 года - рекомендация MDN/OWASP для production
	}
	e.Use(echomw.SecureWithConfig(echomw.SecureConfig{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		HSTSMaxAge:            hstsMaxAge,
		HSTSPreloadEnabled:    cfg.CookieSecure,
		ContentSecurityPolicy: "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; img-src 'self' data: blob:; connect-src 'self'; font-src 'self' data:; frame-ancestors 'none'",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
	}))
	e.Use(mw.CORS(cfg.CORSAllowedOrigins))
	e.Use(mw.RateLimit(cfg.RateLimitPerMinute, cfg.RateLimitWindowSec))
	e.Use(mw.PDAudit(db))
	e.Use(mw.RequestLogger(db))

	// Services
	authService := services.NewAuthService(db, cfg.JWTSecret, cfg.JWTRefreshSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
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
	permissionService := services.NewPermissionService(db)
	systemTableService := services.NewSystemTableService(db, cfg.UploadPath, cfg.UploadMaxFileSize, permissionService)
	uniqueCarService := services.NewUniqueCarService(db)
	uniqueEmployeeService := services.NewUniqueEmployeeService(db)
	feedbackService := services.NewFeedbackService(db)
	newsService := services.NewNewsService(db)
	notificationService := services.NewNotificationService(db)
	requestLogsService := services.NewRequestLogsService(db)
	employeesHistoryService := services.NewEmployeesHistoryService(db)
	applicationService := services.NewApplicationService(db, permissionService, notificationService)
	approverService := services.NewApproverService(db)
	consentService := services.NewConsentService(db)
	settingsService := services.NewSettingsService(db, cfg)

	// Handlers
	authHandler := handlers.NewAuthHandler(authService, cfg.CookieSecure, cfg.JWTRefreshTTL)
	userTypesHandler := handlers.NewUserTypesHandler(userTypeService)
	lpfHandler := handlers.NewLicensePlateFormatHandler(lpfService)
	attachmentHandler := handlers.NewAttachmentHandler(attachmentService)
	citizenshipHandler := handlers.NewCitizenshipHandler(citizenshipService)
	organizationHandler := handlers.NewOrganizationHandler(organizationService, db)
	companyHandler := handlers.NewCompanyHandler(companyService)
	usersHandler := handlers.NewUsersHandler(userService)
	unloadPlaceHandler := handlers.NewUnloadPlaceHandler(unloadPlaceService, cfg.UploadMaxFileSize, cfg.UploadPath)
	carHandler := handlers.NewCarHandler(carService)
	employeeHandler := handlers.NewEmployeeHandler(employeeService)
	systemTableHandler := handlers.NewSystemTableHandler(systemTableService)
	uniqueCarHandler := handlers.NewUniqueCarHandler(uniqueCarService)
	uniqueEmployeeHandler := handlers.NewUniqueEmployeeHandler(uniqueEmployeeService)
	feedbackHandler := handlers.NewFeedbackHandler(feedbackService)
	newsHandler := handlers.NewNewsHandler(newsService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)
	requestLogsHandler := handlers.NewRequestLogsHandler(requestLogsService)
	employeesHistoryHandler := handlers.NewEmployeesHistoryHandler(employeesHistoryService)
	applicationHandler := handlers.NewApplicationHandler(applicationService)
	approverHandler := handlers.NewApproverHandler(approverService)
	permissionHandler := handlers.NewPermissionHandler(permissionService)
	consentHandler := handlers.NewConsentHandler(consentService, db)
	settingsHandler := handlers.NewSettingsHandler(settingsService)

	// Swagger UI: http://localhost:8090/swagger/index.html
	if cfg.SwaggerEnabled {
		e.GET("/swagger/*", echoSwagger.WrapHandler)
	}

	api.SetMaxLimit(cfg.PaginationMaxLimit)

	// /login - отдельный per-IP rate limiter: 5 попыток / 15 минут.
	// Защита от онлайн brute-force до попадания в Argon2id (который замедляет
	// только офлайн-атаки). Дополняется per-user lockout в authService.Login.
	loginLimiter := mw.LoginRateLimit(5, 15*time.Minute)

	// Routes
	router.Setup(e, authHandler, userTypesHandler, attachmentHandler, lpfHandler, citizenshipHandler, organizationHandler, companyHandler, usersHandler, unloadPlaceHandler, carHandler, employeeHandler, systemTableHandler, uniqueCarHandler, uniqueEmployeeHandler, feedbackHandler, applicationHandler, approverHandler, permissionHandler, consentHandler, settingsHandler, newsHandler, notificationHandler, requestLogsHandler, employeesHistoryHandler, []byte(cfg.JWTSecret), loginLimiter)

	// Общий ctx для фоновых задач и graceful shutdown. Отменяется по SIGINT/SIGTERM.
	ctxSig, stopSig := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSig()

	// Периодическая проверка истёкших вложений заявок: деактивирует вложения
	// с прошедшим entry_date_to/entry_time_to, завершает заявку когда все
	// вложения неактивны. См. ApplicationWorkflowService.CheckExpiredAttachments.
	go startExpiryScheduler(ctxSig, applicationService, 15*time.Minute)

	// Graceful shutdown
	go func() {
		<-ctxSig.Done()
		slog.Info("shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.Shutdown(ctx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	}()

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.BindHost, cfg.BindPort)
	slog.Info("starting server", "addr", addr)
	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

// startExpiryScheduler запускает периодическую проверку истёкших вложений заявок.
// Первая проверка — сразу при старте; далее — каждый interval, пока ctx не отменён.
func startExpiryScheduler(ctx context.Context, svc services.ApplicationService, interval time.Duration) {
	if err := svc.CheckExpiredAttachments(ctx); err != nil {
		slog.Error("initial expiry check failed", "error", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("expiry scheduler stopped")
			return
		case <-ticker.C:
			if err := svc.CheckExpiredAttachments(ctx); err != nil {
				slog.Error("expiry check failed", "error", err)
			}
		}
	}
}
