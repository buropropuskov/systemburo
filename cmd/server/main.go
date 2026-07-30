package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	// В alpine-образе нет системной tzdata, а бинарь собран CGO_ENABLED=0 -
	// без этого импорта time.LoadLocation("Europe/Moscow") падает в рантайме и
	// планировщик 06:00 МСК уезжает на UTC (= 09:00 МСК). Вшивает зоны в бинарь.
	_ "time/tzdata"

	_ "systemburo/docs"
	"systemburo/internal/api"
	"systemburo/internal/config"
	"systemburo/internal/crypto"
	"systemburo/internal/database"
	"systemburo/internal/handlers"
	mw "systemburo/internal/middleware"
	"systemburo/internal/models"
	"systemburo/internal/realtime"
	"systemburo/internal/router"
	"systemburo/internal/services"
	appvalidator "systemburo/internal/validator"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"
	"gopkg.in/natefinch/lumberjack.v2"
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
	// Writer для логов: stdout всегда (для docker logs), плюс ротируемый файл,
	// если задан LOG_FILE_PATH. lumberjack ротирует по размеру и удаляет файлы
	// старше MaxAge дней (по умолчанию 30 - месячная ротация).
	var logOut io.Writer = os.Stdout
	if cfg.LogFilePath != "" {
		logOut = io.MultiWriter(os.Stdout, &lumberjack.Logger{
			Filename:   cfg.LogFilePath,
			MaxSize:    cfg.LogMaxSizeMB,
			MaxAge:     cfg.LogMaxAgeDays,
			MaxBackups: cfg.LogMaxBackups,
			Compress:   cfg.LogCompress,
		})
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(logOut, &slog.HandlerOptions{Level: logLevel})))

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
	// Все autoCreateTime/autoUpdateTime поля заполняются в UTC, чтобы избежать
	// расхождений между локальной зоной хост-системы и timestamptz-столбцами
	// в Postgres. Issue #184.
	dsnWithTZ := database.EnsureUTCTimezone(cfg.DatabaseURL)
	db, err := gorm.Open(postgres.Open(dsnWithTZ), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
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
	e.Use(echomw.LoggerWithConfig(echomw.LoggerConfig{Output: logOut}))
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
	// eventsHub создаётся до notificationService (#840 V1): паблишер real-time
	// сигнала "notification.new" передаётся в конструктор опцией, поэтому хаб
	// должен существовать раньше. Конструктор Hub ни от чего не зависит.
	eventsHub := realtime.NewHub()
	notificationServiceEarly := services.NewNotificationService(db, services.WithNotificationRealtimePublisher(eventsHub))
	// Справочники создаются после уведомлений (#1437): разбор записи «на проверке»
	// сообщает инициатору наименования, чем он кончился.
	organizationService := services.NewOrganizationService(db, services.WithOrganizationNotifications(notificationServiceEarly))
	companyService := services.NewCompanyService(db, services.WithCompanyNotifications(notificationServiceEarly))
	userService := services.NewUserService(db, notificationServiceEarly)
	onboardingService := services.NewOnboardingService(db)
	themeService := services.NewThemeService(db)
	unloadPlaceService := services.NewUnloadPlaceService(db)
	bureauService := services.NewBureauService(db)
	auditRecorder := services.NewAuditRecorder(db)
	permissionResolver := services.NewPermissionResolver(db)
	permissionResolver.SetRealtimePublisher(eventsHub) // #840: смена роли/группы/override -> user.permissions
	// Продюсер real-time сигналов обновления таблиц проходной (#840 V2.2/V2.3):
	// аудитория считается по праву table.<name>.view (нужен резолвер), сигнал шлётся
	// через хаб. Инжектится в car/employee/application-сервисы - точки, где строки
	// таблиц меняются (въезд/выезд, принятие заявки).
	tablesRefreshProducer := services.NewTablesRefreshPublisher(db, permissionResolver, eventsHub)
	availableRefreshProducer := services.NewAvailableRefreshPublisher(db, permissionResolver, eventsHub)
	carService := services.NewCarService(db, auditRecorder, services.WithCarTablesProducer(tablesRefreshProducer))
	employeeService := services.NewEmployeeService(db, auditRecorder, services.WithEmployeeTablesProducer(tablesRefreshProducer))
	manualAttachService := services.NewManualAttachService(db, auditRecorder, tablesRefreshProducer, availableRefreshProducer)
	permissionService := services.NewPermissionService(db)
	permissionGroupService := services.NewPermissionGroupService(db, permissionResolver)
	roleService := services.NewRoleService(db, permissionResolver)
	accessDenialService := services.NewAccessDenialService(db)
	banCheckService := services.NewBanCheckService(db, 30*time.Second)
	userService.SetBanCache(banCheckService) // архив/restore мгновенно сбрасывают кэш блокировок
	userBanService := services.NewUserBanService(db, permissionResolver, banCheckService, auditRecorder, services.WithBanRealtimePublisher(eventsHub))
	systemTableService := services.NewSystemTableService(db, cfg.UploadPath, cfg.UploadMaxFileSize, permissionService, services.WithSystemTableRealtimePublisher(eventsHub))
	if err := systemTableService.SeedMissingFields(context.Background()); err != nil {
		slog.Error("не удалось досидить отсутствующие поля таблиц (#345)", "error", err)
	}
	// Догенерировать недостающие права таблиц - в т.ч. новый глагол versions (#980)
	// для таблиц, созданных до его появления. Идемпотентно, не блокирует старт.
	if err := permissionService.ReconcileAllTablePermissions(context.Background()); err != nil {
		slog.Error("не удалось догенерировать права таблиц", "error", err)
	}
	workModesService := services.NewWorkModesService(unloadPlaceService, systemTableService, bureauService)
	uniqueCarService := services.NewUniqueCarService(db)
	uniqueEmployeeService := services.NewUniqueEmployeeService(db)
	feedbackService := services.NewFeedbackService(db, services.WithFeedbackRealtimePublisher(eventsHub))
	// news.refresh (#840): та же аудитория, что и news/announcement-эндпоинты -
	// все активные юзера (страница видна всем авторизованным, без гейта прав).
	newsService := services.NewNewsService(db, services.WithNewsRealtimePublisher(eventsHub))
	notificationService := notificationServiceEarly
	requestLogsService := services.NewRequestLogsService(db)
	employeesHistoryService := services.NewEmployeesHistoryService(db)
	tableSnapshotService := services.NewTableSnapshotService(db, carService, employeeService, employeesHistoryService)
	dailyPassReportService := services.NewDailyPassReportService(db)
	approverService := services.NewApproverService(db)
	consentService := services.NewConsentService(db)
	settingsService := services.NewSettingsService(db, cfg)
	// Гейт согласия на обработку ПД (#1567): TTL кэша как у BanCheckService - сервис
	// будет опрашиваться на каждом protected-запросе.
	pdConsentGateService := services.NewPDConsentGateService(consentService, settingsService, 30*time.Second)
	pdConsentStatsService := services.NewPDConsentStatsService(db, settingsService)
	userService.SetPasswordPolicyProvider(settingsService) // политика паролей при создании/смене
	reminderService := services.NewReminderService(db, notificationService, settingsService)
	telegramService := services.NewTelegramService(cfg.TelegramBotToken, cfg.TelegramChatID)
	bugReportService := services.NewBugReportService(db, telegramService)
	maintenanceService := services.NewMaintenanceService(db)
	markService := services.NewMarkService(db)
	blacklistAuditRecorder := services.NewAuditRecorder(db)
	vehicleBlacklistService := services.NewVehicleBlacklistService(db, blacklistAuditRecorder)
	personBlacklistService := services.NewPersonBlacklistService(db, blacklistAuditRecorder)
	applicationService := services.NewApplicationService(db, permissionService, notificationService, vehicleBlacklistService, personBlacklistService, auditRecorder, services.WithRealtimePublisher(eventsHub), services.WithApplicationTablesProducer(tablesRefreshProducer), services.WithApplicationAvailableProducer(availableRefreshProducer), services.WithApplicationPermissionResolver(permissionResolver))
	attachmentTemplateService := services.NewAttachmentTemplateService(db, cfg.UploadPath)
	attachmentFieldConfigService := services.NewAttachmentFieldConfigService(db)
	attachmentBlankService := services.NewAttachmentBlankService(db)
	trashService := services.NewTrashService(db, auditRecorder)
	trashDBRef := services.NewTrashDBRef(db)
	documentFileService := services.NewDocumentFileService(cfg.UploadPath)
	guideFileService := services.NewDocumentFileServiceIn(cfg.UploadPath, "guide")
	documentGroupService := services.NewDocumentGroupService(db)
	documentService := services.NewDocumentService(db, documentFileService, settingsService)
	guideService := services.NewGuideService(db, permissionResolver)
	statisticsService := services.NewStatisticsService(db, time.Duration(cfg.AnalyticsCacheRefreshSec)*time.Second)

	// Handlers
	authHandler := handlers.NewAuthHandler(authService, maintenanceService, cfg.CookieSecure, cfg.JWTRefreshTTL)
	userTypesHandler := handlers.NewUserTypesHandler(userTypeService)
	lpfHandler := handlers.NewLicensePlateFormatHandler(lpfService)
	attachmentHandler := handlers.NewAttachmentHandler(attachmentService)
	manualAttachHandler := handlers.NewManualAttachHandler(manualAttachService)
	citizenshipHandler := handlers.NewCitizenshipHandler(citizenshipService)
	organizationHandler := handlers.NewOrganizationHandler(organizationService, db)
	companyHandler := handlers.NewCompanyHandler(companyService)
	usersHandler := handlers.NewUsersHandler(userService)
	onboardingHandler := handlers.NewOnboardingHandler(onboardingService)
	themeHandler := handlers.NewThemeHandler(themeService)
	unloadPlaceHandler := handlers.NewUnloadPlaceHandler(unloadPlaceService, cfg.UploadMaxFileSize, cfg.UploadPath)
	bureauHandler := handlers.NewBureauHandler(bureauService)
	workModesHandler := handlers.NewWorkModesHandler(workModesService)
	carHandler := handlers.NewCarHandler(carService)
	employeeHandler := handlers.NewEmployeeHandler(employeeService)
	systemTableHandler := handlers.NewSystemTableHandler(systemTableService, auditRecorder, cfg.UploadMaxFileSize, cfg.UploadPath)
	tableSnapshotHandler := handlers.NewTableSnapshotHandler(tableSnapshotService)
	passReportHandler := handlers.NewPassReportHandler(dailyPassReportService, permissionResolver)
	uniqueCarHandler := handlers.NewUniqueCarHandler(uniqueCarService)
	uniqueEmployeeHandler := handlers.NewUniqueEmployeeHandler(uniqueEmployeeService)
	feedbackHandler := handlers.NewFeedbackHandler(feedbackService)
	newsHandler := handlers.NewNewsHandler(newsService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)
	requestLogsHandler := handlers.NewRequestLogsHandler(requestLogsService)
	employeesHistoryHandler := handlers.NewEmployeesHistoryHandler(employeesHistoryService)
	applicationHandler := handlers.NewApplicationHandler(applicationService, permissionResolver)
	approverHandler := handlers.NewApproverHandler(approverService)
	permissionHandler := handlers.NewPermissionHandler(permissionService, permissionResolver)
	permissionGroupHandler := handlers.NewPermissionGroupHandler(permissionGroupService)
	roleHandler := handlers.NewRoleHandler(roleService)
	accessDenialHandler := handlers.NewAccessDenialHandler(accessDenialService)
	userBanHandler := handlers.NewUserBanHandler(userBanService)
	consentHandler := handlers.NewConsentHandler(consentService, pdConsentGateService, settingsService, db)
	settingsHandler := handlers.NewSettingsHandler(settingsService, documentFileService, cfg.UploadMaxFileSize, pdConsentGateService, pdConsentStatsService)
	bugReportHandler := handlers.NewBugReportHandler(bugReportService)
	maintenanceHandler := handlers.NewMaintenanceHandler(maintenanceService)
	markHandler := handlers.NewMarkHandler(markService)
	vehicleBlacklistHandler := handlers.NewVehicleBlacklistHandler(vehicleBlacklistService)
	personBlacklistHandler := handlers.NewPersonBlacklistHandler(personBlacklistService)
	attachmentTemplateHandler := handlers.NewAttachmentTemplateHandler(attachmentTemplateService, attachmentFieldConfigService)
	attachmentBlankHandler := handlers.NewAttachmentBlankHandler(attachmentBlankService, applicationService, permissionResolver)
	trashHandler := handlers.NewTrashHandler(trashService, trashDBRef)
	documentGroupHandler := handlers.NewDocumentGroupHandler(documentGroupService)
	documentHandler := handlers.NewDocumentHandler(documentService, documentFileService)
	guideHandler := handlers.NewGuideHandler(guideService, guideFileService, cfg.UploadMaxFileSize)
	statisticsHandler := handlers.NewStatisticsHandler(statisticsService)
	reminderHandler := handlers.NewReminderHandler(reminderService)
	auditHandler := handlers.NewAuditHandler(services.NewAuditReader(db))
	authEventHandler := handlers.NewAuthEventHandler(services.NewAuthEventReader(db))

	// Swagger UI: http://localhost:8090/swagger/index.html
	if cfg.SwaggerEnabled {
		e.GET("/swagger/*", echoSwagger.WrapHandler)
	}

	api.SetMaxLimit(cfg.PaginationMaxLimit)

	// Rate-limit входа теперь ведёт единый per-IP счётчик в authService.loginGuard:
	// он показывает остаток попыток для любого логина и блокирует на минуту от момента
	// исчерпания. Отдельный middleware больше не нужен (иначе его sliding-остаток
	// перехватывал бы таймер guard). Оставляем nil - router просто не навесит его.
	var loginLimiter echo.MiddlewareFunc
	maintenanceBlock := mw.MaintenanceBlock(maintenanceService)
	banCheck := mw.BanCheck(banCheckService)
	// Гейт согласия на обработку ПД (#1567). Пока тумблер выключен или текст пуст,
	// пропускает всех - включается настройкой, а не деплоем.
	consentGate := mw.PDConsentGate(pdConsentGateService)
	lastSeen := mw.LastSeen(db)

	// Routes
	eventsTickets := realtime.NewTicketStore(60 * time.Second)
	eventsHandler := handlers.NewEventsHandler(eventsHub, eventsTickets)

	router.Setup(e, router.Dependencies{
		Auth:                authHandler,
		UserTypes:           userTypesHandler,
		Attachments:         attachmentHandler,
		ManualAttach:        manualAttachHandler,
		LPF:                 lpfHandler,
		Citizenship:         citizenshipHandler,
		Organization:        organizationHandler,
		Company:             companyHandler,
		Users:               usersHandler,
		UnloadPlace:         unloadPlaceHandler,
		Cars:                carHandler,
		Employees:           employeeHandler,
		SystemTable:         systemTableHandler,
		TableSnapshot:       tableSnapshotHandler,
		PassReport:          passReportHandler,
		UniqueCar:           uniqueCarHandler,
		UniqueEmployee:      uniqueEmployeeHandler,
		Feedback:            feedbackHandler,
		Application:         applicationHandler,
		Approver:            approverHandler,
		Permissions:         permissionHandler,
		PermGroups:          permissionGroupHandler,
		Roles:               roleHandler,
		AccessDenials:       accessDenialHandler,
		PDAudit:             handlers.NewPDAuditHandler(services.NewPDAuditService(db)),
		UserBan:             userBanHandler,
		Consent:             consentHandler,
		Settings:            settingsHandler,
		News:                newsHandler,
		Notifications:       notificationHandler,
		RequestLogs:         requestLogsHandler,
		EmployeesHistory:    employeesHistoryHandler,
		BugReport:           bugReportHandler,
		Maintenance:         maintenanceHandler,
		Marks:               markHandler,
		VehicleBlacklist:    vehicleBlacklistHandler,
		PersonBlacklist:     personBlacklistHandler,
		AttachmentTemplates: attachmentTemplateHandler,
		AttachmentBlanks:    attachmentBlankHandler,
		Trash:               trashHandler,
		DocumentGroups:      documentGroupHandler,
		Documents:           documentHandler,
		Guide:               guideHandler,
		Bureau:              bureauHandler,
		WorkModes:           workModesHandler,
		Statistics:          statisticsHandler,
		Reminder:            reminderHandler,
		Onboarding:          onboardingHandler,
		Theme:               themeHandler,
		Audit:               auditHandler,
		AuthEvents:          authEventHandler,
		Events:              eventsHandler,
		PermResolver:        permissionResolver,
		DenialLog:           accessDenialService,
		MaintenanceBlock:    maintenanceBlock,
		BanCheck:            banCheck,
		ConsentGate:         consentGate,
		LoginLimiter:        loginLimiter,
		LastSeen:            lastSeen,
		TableReportGate:     mw.RequireTableVerb(db, permissionResolver, accessDenialService, "report"),
		TableVersionsGate:   mw.RequireTableVerb(db, permissionResolver, accessDenialService, "versions"),
		TableTrashGate:      mw.RequireTableVerb(db, permissionResolver, accessDenialService, "trash"),
		TablePassGate:       mw.RequireTablePassVerb(db, permissionResolver, accessDenialService),
		JWTSecret:           []byte(cfg.JWTSecret),
		UploadPath:          cfg.UploadPath,
	})

	// Общий ctx для фоновых задач и graceful shutdown. Отменяется по SIGINT/SIGTERM.
	ctxSig, stopSig := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSig()

	// Периодическая проверка истёкших вложений заявок: деактивирует вложения
	// с прошедшим entry_date_to/entry_time_to, завершает заявку когда все
	// вложения неактивны. См. ApplicationWorkflowService.CheckExpiredAttachments.
	go startExpiryScheduler(ctxSig, applicationService, 15*time.Minute)

	// Напоминания зависшим согласующим (#1315): раз в час отбирает pending-строки
	// application_responsible_users, молчащие дольше настроенного срока, и шлёт
	// уведомление. См. ReminderService.SendPendingReminders.
	go startReminderScheduler(ctxSig, reminderService, time.Hour)

	// Архив access_denials: 3 мес retention, цикл раз в сутки.
	go startAccessDenialsArchiver(ctxSig, accessDenialService, 90*24*time.Hour, 24*time.Hour)

	// Ежедневный сброс территориальных статусов "Покинул/Выехал" -> "Не входил/Не въезжал" в 06:00.
	resetLoc, err := time.LoadLocation(cfg.ResetTimezone)
	if err != nil {
		slog.Warn("неверный RESET_TIMEZONE, используем UTC", "timezone", cfg.ResetTimezone, "error", err)
		resetLoc = time.UTC
	}
	resetService := services.NewTerritoryResetService(db)
	go startDailyStatusReset(ctxSig, tableSnapshotService, resetService, resetLoc)

	// Суточные отчёты охранника по проходам: фиксация в 21:30 + catch-up на
	// старте (пропущенные за даунтайм дни; первый запуск - полный backfill).
	go startDailyPassReportSaver(ctxSig, dailyPassReportService, resetLoc)

	// Снимок дневного пика онлайна (#632): раз в минуту фиксирует текущий онлайн
	// как пик за сегодня (peak_count = MAX(...)). Останавливается по ctxSig.
	go startOnlinePeakSnapshotter(ctxSig, statisticsService, time.Minute)

	// Тёплый кэш аналитики: прогрев из БД при старте + фоновое обновление, чтобы
	// дашборд/insights не считались с нуля после рестарта или деплоя.
	statisticsService.WarmCache(ctxSig)
	go statisticsService.StartCacheRefresh(ctxSig)

	// Обслуживание партиций request_logs: раз в сутки создаёт партиции вперёд,
	// сворачивает партиции старше RequestLogDetailDays в агрегаты и дропает.
	go startLogPartitionWorker(ctxSig, db, cfg.RequestLogDetailDays, cfg.RequestLogPartitionPrecreateDays, cfg.PdAuditRetentionMonths, 24*time.Hour)

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

// startLogPartitionWorker раз в interval обслуживает партиции request_logs:
// создаёт будущие, сворачивает старые в дневные агрегаты и дропает. Первый прогон сразу.
func startLogPartitionWorker(ctx context.Context, db *gorm.DB, detailDays, precreateDays, pdRetentionMonths int, interval time.Duration) {
	run := func() {
		if err := database.MaintainLogPartitions(db, detailDays, precreateDays, pdRetentionMonths); err != nil {
			slog.Error("log partition maintenance failed", "error", err)
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("log partition worker stopped")
			return
		case <-ticker.C:
			run()
		}
	}
}

// startAccessDenialsArchiver запускает периодический архив записей старше retention.
// retention -- срок хранения в активной таблице (по умолчанию 3 месяца).
// interval -- частота запуска (по умолчанию раз в сутки).
func startAccessDenialsArchiver(ctx context.Context, svc *services.AccessDenialService, retention, interval time.Duration) {
	archive := func() {
		cutoff := time.Now().Add(-retention)
		moved, err := svc.ArchiveOlderThan(ctx, cutoff)
		if err != nil {
			slog.Error("access denials archive failed", "error", err)
			return
		}
		if moved > 0 {
			slog.Info("access denials archived", "count", moved, "cutoff", cutoff)
		}
	}
	archive()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("access denials archiver stopped")
			return
		case <-ticker.C:
			archive()
		}
	}
}

// startDailyStatusReset ежедневно в 06:00 по location снимает дневные слепки всех
// активных таблиц, затем сбрасывает территориальные статусы "Покинул/Выехал" (2) ->
// "Не входил/Не въезжал" (0). Статус "На территории" (1) не затрагивается.
// Ошибки логируются, паники нет.
func startDailyStatusReset(ctx context.Context, snapSvc services.TableSnapshotService, svc services.TerritoryResetService, location *time.Location) {
	now := time.Now().In(location)
	next := time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, location)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	timer := time.NewTimer(time.Until(next))
	defer timer.Stop()

	slog.Info("планировщик сброса статусов запущен", "next_reset", next.Format(time.RFC3339))

	select {
	case <-ctx.Done():
		slog.Info("планировщик сброса статусов остановлен до первого срабатывания")
		return
	case <-timer.C:
	}

	snapshotThenReset(ctx, snapSvc, svc)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("планировщик сброса статусов остановлен")
			return
		case <-ticker.C:
			snapshotThenReset(ctx, snapSvc, svc)
		}
	}
}

// snapshotThenReset снимает дневные слепки всех активных таблиц ПЕРЕД сбросом
// территориальных статусов, чтобы сохранить суточное состояние до обнуления.
// Провал снимка логируется, но не отменяет сброс - сброс важнее и должен пройти.
func snapshotThenReset(ctx context.Context, snapSvc services.TableSnapshotService, svc services.TerritoryResetService) {
	created, failed, err := snapSvc.SnapshotAllActiveTables(ctx, models.SnapshotReasonScheduled)
	if err != nil {
		slog.Error("дневной снимок таблиц не выполнен, продолжаем сброс", "error", err)
	} else {
		slog.Info("дневной снимок таблиц выполнен", "created", created, "failed", failed)
	}

	emp, cars, err := svc.ResetExitedStatuses(ctx)
	if err != nil {
		slog.Error("сброс территориальных статусов завершился ошибкой", "error", err)
	} else {
		slog.Info("сброс территориальных статусов выполнен", "employees", emp, "cars", cars)
	}
}

// startDailyPassReportSaver ежедневно в 21:30 по location фиксирует суточные
// отчёты охранников по проходам (агрегаты audit_log за окно [21:30, 21:30) МСК)
// в daily_pass_reports. На старте - разовый CatchUp: дозаписывает дни,
// пропущенные из-за даунтайма (первый запуск делает полный backfill истории).
// CatchUp вместо SaveDailyReports и в тике: покрывает и только что закрытое
// окно, и возможные пропуски. Ошибки логируются, паники нет.
func startDailyPassReportSaver(ctx context.Context, svc services.DailyPassReportService, location *time.Location) {
	// Таймер взводится ДО стартового CatchUp: если долгий catch-up/backfill
	// пересечёт границу 21:30, уже взведённый таймер сработает сразу после него
	// и зафиксирует только что закрывшееся окно - иначе оно ждало бы сутки.
	now := time.Now().In(location)
	next := time.Date(now.Year(), now.Month(), now.Day(), 21, 30, 0, 0, location)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	timer := time.NewTimer(time.Until(next))
	defer timer.Stop()

	if err := svc.CatchUp(ctx); err != nil {
		slog.Error("суточные отчёты проходов: catch-up на старте", "error", err)
	}

	slog.Info("планировщик суточных отчётов проходов запущен", "next_run", next.Format(time.RFC3339))

	select {
	case <-ctx.Done():
		slog.Info("планировщик суточных отчётов проходов остановлен до первого срабатывания")
		return
	case <-timer.C:
	}

	if err := svc.CatchUp(ctx); err != nil {
		slog.Error("суточные отчёты проходов: сохранение", "error", err)
	}

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("планировщик суточных отчётов проходов остановлен")
			return
		case <-ticker.C:
			if err := svc.CatchUp(ctx); err != nil {
				slog.Error("суточные отчёты проходов: сохранение", "error", err)
			}
		}
	}
}

// startOnlinePeakSnapshotter раз в interval фиксирует текущий онлайн как дневной
// пик (#632). Останавливается по отмене ctx (graceful shutdown), горутина не течёт.
func startOnlinePeakSnapshotter(ctx context.Context, svc services.StatisticsService, interval time.Duration) {
	snapshot := func() {
		if err := svc.SnapshotOnlinePeak(ctx); err != nil {
			slog.Error("online peak snapshot failed", "error", err)
		}
	}
	snapshot()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("online peak snapshotter stopped")
			return
		case <-ticker.C:
			snapshot()
		}
	}
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

// startReminderScheduler запускает периодический прогон напоминаний зависшим
// согласующим (#1315). Первый прогон — сразу при старте; далее — каждый interval,
// пока ctx не отменён.
func startReminderScheduler(ctx context.Context, svc services.ReminderService, interval time.Duration) {
	if err := svc.SendPendingReminders(ctx); err != nil {
		slog.Error("initial reminder run failed", "error", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("reminder scheduler stopped")
			return
		case <-ticker.C:
			if err := svc.SendPendingReminders(ctx); err != nil {
				slog.Error("reminder run failed", "error", err)
			}
		}
	}
}
