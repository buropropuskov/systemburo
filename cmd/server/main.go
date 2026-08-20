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
	"systemburo/internal/httpx"
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

// shutdownGrace - сколько main ждёт горутину остановки после того, как HTTP-сервер
// закрылся. С запасом над её собственными окнами: 10 секунд на закрытие соединений,
// затем по 5 на дожатие push-рассылки и запись журнала обращений.
const shutdownGrace = 25 * time.Second

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
	// Подкоманды обслуживания живут в этом же бинаре: в рабочем образе есть только
	// собранные server и seed, компилятора там нет, и отдельный инструмент пришлось
	// бы вносить в сборку образа.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "cleanup":
			os.Exit(runCleanup(os.Args[2:]))
		case "storage":
			os.Exit(runStorage(os.Args[2:]))
		case "archive":
			os.Exit(runArchive(os.Args[2:]))
		case "entity":
			os.Exit(runEntity(os.Args[2:]))
		case "fake":
			os.Exit(runFake(os.Args[2:]))
		case "vapid":
			os.Exit(runVAPID(os.Args[2:]))
		}
	}

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

	// Пул настраивается сразу после подключения, до миграций и сидов: они тоже ходят
	// в базу, и делать это на драйверных умолчаниях незачем.
	if err := database.ConfigureConnectionPool(db, database.PoolConfig{
		MaxOpenConns:    cfg.DBMaxOpenConns,
		MaxIdleConns:    cfg.DBMaxIdleConns,
		ConnMaxLifetime: cfg.DBConnMaxLifetime,
		ConnMaxIdleTime: cfg.DBConnMaxIdleTime,
	}); err != nil {
		slog.Error("failed to configure DB connection pool", "error", err)
		os.Exit(1)
	}
	slog.Info("db pool configured",
		"max_open", cfg.DBMaxOpenConns,
		"max_idle", cfg.DBMaxIdleConns,
		"conn_max_lifetime", cfg.DBConnMaxLifetime,
		"conn_max_idle_time", cfg.DBConnMaxIdleTime)

	// Предел одновременных проверок пароля ставится до приёма трафика: перенастройка
	// на ходу не видна вычислениям, уже занявшим слот.
	services.SetArgon2Concurrency(cfg.Argon2HashConcurrency)

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

	// Сроки жизни соединения. Длинные обработчики (SSE-поток, потоковые выгрузки
	// файлового архива) снимают срок записи с себя сами - см. httpx.AllowLongResponse.
	httpx.ApplyServerTimeouts(e.Server, httpx.ServerTimeouts{
		ReadHeader: cfg.HTTPReadHeaderTimeout,
		Read:       cfg.HTTPReadTimeout,
		Write:      cfg.HTTPWriteTimeout,
		Idle:       cfg.HTTPIdleTimeout,
	})
	slog.Info("http timeouts configured",
		"read_header", cfg.HTTPReadHeaderTimeout,
		"read", cfg.HTTPReadTimeout,
		"write", cfg.HTTPWriteTimeout,
		"idle", cfg.HTTPIdleTimeout)

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
	// Журнал обращений пишется пачками из фоновой горутины (#2125): обработчик
	// только кладёт запись в очередь. Останавливается в graceful shutdown ниже,
	// иначе остаток очереди уйдёт вместе с процессом.
	requestLogWriter := mw.NewRequestLogWriter(db)
	e.Use(requestLogWriter.Middleware())

	// Services
	userTypeService := services.NewUserTypeService(db)
	lpfService := services.NewLicensePlateFormatService(db)
	attachmentService := services.NewAttachmentService(db)
	citizenshipService := services.NewCitizenshipService(db)
	// eventsHub создаётся до notificationService (#840 V1): паблишер real-time
	// сигнала "notification.new" передаётся в конструктор опцией, поэтому хаб
	// должен существовать раньше. Конструктор Hub ни от чего не зависит.
	eventsHub := realtime.NewHub()
	// Резолвер прав поднимается до уведомлений (#1748): экран настроек прячет типы,
	// которых человек не получит по правам, и резолвер нужен сервису при создании.
	permissionResolver := services.NewPermissionResolver(db)
	permissionResolver.SetRealtimePublisher(eventsHub) // #840: смена роли/группы/override -> user.permissions
	// pushService создаётся до notificationService (#974): рассылка Web Push
	// подключается опцией конструктора, как и real-time паблишер. Пустые
	// VAPID-ключи в параметрах не мешают подняться - Send() тогда молча ничего не
	// отправляет (push выключен).
	pushService := services.NewPushService(db, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject)
	notificationServiceEarly := services.NewNotificationService(db,
		services.WithNotificationRealtimePublisher(eventsHub),
		services.WithNotificationPermissionResolver(permissionResolver),
		services.WithNotificationPushSender(pushService))
	// authService создаётся после notificationService (#1748 S3): уведомление о
	// блокировке входа передаётся в конструктор опцией.
	authService := services.NewAuthService(db, cfg.JWTSecret, cfg.JWTRefreshSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL, services.WithAuthNotifications(notificationServiceEarly))
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
	// Продюсер real-time сигналов обновления таблиц проходной (#840 V2.2/V2.3):
	// аудитория считается по праву table.<name>.view (нужен резолвер), сигнал шлётся
	// через хаб. Инжектится в car/employee/application-сервисы - точки, где строки
	// таблиц меняются (въезд/выезд, принятие заявки).
	tablesRefreshProducer := services.NewTablesRefreshPublisher(db, permissionResolver, eventsHub)
	availableRefreshProducer := services.NewAvailableRefreshPublisher(db, permissionResolver, eventsHub)
	carService := services.NewCarService(db, auditRecorder, services.WithCarTablesProducer(tablesRefreshProducer), services.WithCarNotifications(notificationServiceEarly))
	employeeService := services.NewEmployeeService(db, auditRecorder, services.WithEmployeeTablesProducer(tablesRefreshProducer), services.WithEmployeeNotifications(notificationServiceEarly))
	manualAttachService := services.NewManualAttachService(db, auditRecorder, tablesRefreshProducer, availableRefreshProducer)
	permissionService := services.NewPermissionService(db)
	permissionGroupService := services.NewPermissionGroupService(db, permissionResolver)
	roleService := services.NewRoleService(db, permissionResolver)
	accessDenialService := services.NewAccessDenialService(db)
	banCheckService := services.NewBanCheckService(db, 30*time.Second)
	userService.SetBanCache(banCheckService) // архив/restore мгновенно сбрасывают кэш блокировок
	userBanService := services.NewUserBanService(db, permissionResolver, banCheckService, auditRecorder, services.WithBanRealtimePublisher(eventsHub), services.WithBanNotifications(notificationServiceEarly))
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
	// #1748 S5: уведомления feedback_created/feedback_answered - аудитория первого
	// считается резолвером прав (page.admin.feedback), поэтому сервису нужен и
	// notificationServiceEarly, и permissionResolver (оба уже подняты выше).
	feedbackService := services.NewFeedbackService(db, services.WithFeedbackRealtimePublisher(eventsHub), services.WithFeedbackNotifications(notificationServiceEarly), services.WithFeedbackPermissionResolver(permissionResolver))
	// news.refresh (#840): та же аудитория, что и news/announcement-эндпоинты -
	// все активные юзера (страница видна всем авторизованным, без гейта прав).
	// news_published (#1748 S5) - та же логика, отдельным уведомлением.
	newsService := services.NewNewsService(db, services.WithNewsRealtimePublisher(eventsHub), services.WithNewsNotifications(notificationServiceEarly))
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
	pdConsentStatsService := services.NewPDConsentStatsService(db, pdConsentGateService)
	userService.SetPasswordPolicyProvider(settingsService) // политика паролей при создании/смене
	reminderService := services.NewReminderService(db, notificationService, settingsService)
	// Предупреждение об истекающем завтра пропуске (#1748, S4): раньше об этом
	// узнавали постфактум, когда CheckExpiredAttachments уже деактивировал вложение.
	expiryNotifyService := services.NewExpiryNotifyService(db, notificationService)
	telegramService := services.NewTelegramService(cfg.TelegramBotToken, cfg.TelegramChatID)
	bugReportService := services.NewBugReportService(db, telegramService)
	// maintenance_scheduled (#1748 S5): уведомление активным пользователям при
	// задании окна плановых техработ.
	maintenanceService := services.NewMaintenanceService(db, services.WithMaintenanceNotifications(notificationServiceEarly))
	markService := services.NewMarkService(db)
	blacklistAuditRecorder := services.NewAuditRecorder(db)
	vehicleBlacklistService := services.NewVehicleBlacklistService(db, blacklistAuditRecorder)
	personBlacklistService := services.NewPersonBlacklistService(db, blacklistAuditRecorder)
	applicationFileService := services.NewApplicationFileService(db, cfg.UploadPath, auditRecorder)
	applicationService := services.NewApplicationService(db, permissionService, notificationService, vehicleBlacklistService, personBlacklistService, auditRecorder, services.WithRealtimePublisher(eventsHub), services.WithApplicationTablesProducer(tablesRefreshProducer), services.WithApplicationAvailableProducer(availableRefreshProducer), services.WithApplicationPermissionResolver(permissionResolver), services.WithApplicationFiles(applicationFileService, cfg.ApplicationFileMaxCount, cfg.ApplicationFileMaxTotal))
	attachmentTemplateService := services.NewAttachmentTemplateService(db, cfg.UploadPath)
	attachmentFieldConfigService := services.NewAttachmentFieldConfigService(db)
	attachmentBlankService := services.NewAttachmentBlankService(db)
	attachmentImportService := services.NewAttachmentImportService(db, auditRecorder, cfg.UploadPath)
	// trash_restored (#1748 S5): уведомление автору записи (заявителю) при
	// восстановлении машины/сотрудника из корзины.
	trashService := services.NewTrashService(db, auditRecorder, services.WithTrashNotifications(notificationServiceEarly))
	trashDBRef := services.NewTrashDBRef(db)
	documentFileService := services.NewDocumentFileService(cfg.UploadPath)
	guideFileService := services.NewDocumentFileServiceIn(cfg.UploadPath, "guide")
	documentGroupService := services.NewDocumentGroupService(db)
	// document_published (#1748 S5): уведомление активным пользователям при
	// загрузке документа.
	documentService := services.NewDocumentService(db, documentFileService, settingsService, services.WithDocumentNotifications(notificationServiceEarly))
	guideService := services.NewGuideService(db, permissionResolver)
	statisticsService := services.NewStatisticsService(db, time.Duration(cfg.AnalyticsCacheRefreshSec)*time.Second)

	// Режим «войти как пользователь» (#1912). Тот же секрет подписи, что у обычного
	// входа: проверка маркера в middleware остаётся одна на всех.
	impersonationService := services.NewImpersonationService(db, cfg.JWTSecret, permissionResolver, auditRecorder)

	// Handlers
	authHandler := handlers.NewAuthHandler(authService, maintenanceService, cfg.CookieSecure, cfg.JWTRefreshTTL)
	impersonationHandler := handlers.NewImpersonationHandler(impersonationService)
	userTypesHandler := handlers.NewUserTypesHandler(userTypeService)
	lpfHandler := handlers.NewLicensePlateFormatHandler(lpfService)
	attachmentHandler := handlers.NewAttachmentHandler(attachmentService)
	manualAttachHandler := handlers.NewManualAttachHandler(manualAttachService)
	citizenshipHandler := handlers.NewCitizenshipHandler(citizenshipService)
	organizationHandler := handlers.NewOrganizationHandler(organizationService, db, permissionResolver)
	companyHandler := handlers.NewCompanyHandler(companyService, db, permissionResolver)
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
	pushHandler := handlers.NewPushHandler(pushService)
	requestLogsHandler := handlers.NewRequestLogsHandler(requestLogsService, auditRecorder)
	employeesHistoryHandler := handlers.NewEmployeesHistoryHandler(employeesHistoryService)
	applicationHandler := handlers.NewApplicationHandler(applicationService, permissionResolver)
	// Типы файлов заявки: картинки и документы одним списком. Разделять их незачем -
	// заявитель прикладывает и снимок, и pdf в одно поле.
	applicationFileTypes := append(append([]string{}, cfg.UploadAllowedImageTypes...), cfg.UploadAllowedDocTypes...)
	applicationFileHandler := handlers.NewApplicationFileHandler(
		applicationFileService, applicationService,
		cfg.UploadMaxFileSize, cfg.ApplicationFileMaxCount, cfg.ApplicationFileMaxTotal, applicationFileTypes,
		cfg.ApplicationFileImageMaxSide, cfg.ApplicationFileJPEGQuality,
	)
	approverHandler := handlers.NewApproverHandler(approverService)
	permissionHandler := handlers.NewPermissionHandler(permissionService, permissionResolver)
	// roleNotifier уведомляет владельца учётки о смене роли (#1748 S3), оборачивая
	// permissionGroupService.SetUserRole - его файл off-limits для этого среза.
	permissionGroupHandler := handlers.NewPermissionGroupHandler(permissionGroupService)
	roleHandler := handlers.NewRoleHandler(roleService)
	accessDenialHandler := handlers.NewAccessDenialHandler(accessDenialService)
	userBanHandler := handlers.NewUserBanHandler(userBanService)
	consentHandler := handlers.NewConsentHandler(consentService, pdConsentGateService, settingsService, db)
	settingsHandler := handlers.NewSettingsHandler(settingsService, documentFileService, cfg.UploadMaxFileSize, pdConsentGateService, pdConsentStatsService)
	// Почтовая рассылка (#1906). Сервис создаётся всегда: при пустом SMTP_HOST он
	// отвечает "почта не настроена" и не даёт молча копить недоставленное.
	mailService := services.NewMailService(db, cfg)
	settingsHandler.SetMailSender(mailService)
	// Рабочая таймзона суточных операций: по ней же считается каталог дня в файловом
	// архиве, иначе заявка, поданная поздним вечером, легла бы в папку следующего дня.
	resetLoc, err := time.LoadLocation(cfg.ResetTimezone)
	if err != nil {
		slog.Warn("неверный RESET_TIMEZONE, используем UTC", "timezone", cfg.ResetTimezone, "error", err)
		resetLoc = time.UTC
	}
	// Состояние проверки сроков действия паролей для экрана настроек (#1909):
	// считается по тем же условиям, по которым будет отбирать работников сам прогон.
	settingsHandler.SetRotationStatusService(
		services.NewPasswordRotationStatusService(db, settingsService, mailService, resetLoc))

	// Прогоны по паролям (#1910): плановая проверка сроков и ручное обновление.
	// Базовый адрес системы для писем берём из списка разрешённых источников:
	// отдельного параметра под адрес нет, а ссылка на localhost в письме у
	// получателя всё равно не откроется - сервис такую отбрасывает сам.
	publicBaseURL := ""
	if len(cfg.CORSAllowedOrigins) > 0 {
		publicBaseURL = cfg.CORSAllowedOrigins[0]
	}
	// Учётные данные новому работнику и пароль, заданный администратором, уходят
	// письмом тем же почтовым сервисом и с тем же адресом системы.
	userService.SetMailSender(mailService, publicBaseURL)

	passwordRotationService := services.NewPasswordRotationService(
		db, settingsService, mailService, notificationServiceEarly, permissionResolver, publicBaseURL)
	settingsHandler.SetRotationService(passwordRotationService)
	usersHandler.SetRotationService(passwordRotationService)

	archivePathService := services.NewArchivePathService(db, resetLoc)
	// Место и квота файлового архива (#1615, срез B2): сводка занятого места и
	// порог, останавливающий очередь выгрузки при нехватке места. Поднимается
	// независимо от blankExportService - сводку и статус диска нужно показывать
	// и когда каталог архива ещё не настроен (тот же принцип, что у настроек), -
	// но раньше него: фоновый прогон выгрузки спрашивает пороги перед каждой
	// записью и получает сторожа конструктором.
	blankExportQuotaService := services.NewBlankExportQuotaService(
		db, settingsService, notificationService, permissionResolver, auditRecorder,
		cfg.ArchivePath, cfg.UploadPath, cfg.LogFilePath)
	// Писатель архива поднимается один на процесс. Не сложился корень - раздел
	// настроек всё равно должен открываться, поэтому сервис выгрузки остаётся nil,
	// а ручка пересоздания честно отвечает «архив недоступен».
	var blankExportService *services.BlankExportService
	var archiveDownloadService *services.ArchiveDownloadService
	// Шифрование архива разбирается до писателя: неверная пара ключей должна
	// останавливать старт, а не всплывать при первой выгрузке.
	archiveCrypto, err := services.NewArchiveCrypto(cfg.ArchiveAgeRecipient, cfg.ArchiveAgeIdentity)
	if err != nil {
		slog.Error("не удалось включить шифрование файлового архива", "error", err)
		os.Exit(1)
	}
	// Состояние шифрования печатается при запуске: иначе понять, включилось ли оно,
	// можно только по именам файлов в архиве, а это замечают не сразу.
	if archiveCrypto.Enabled() {
		slog.Info("файловый архив шифруется", "recipient", cfg.ArchiveAgeRecipient)
	} else {
		slog.Warn("файловый архив пишется без шифрования: ключи не заданы")
	}
	if archiveWriter, err := services.NewArchiveWriter(cfg.ArchivePath); err != nil {
		slog.Error("файловый архив не поднят", "path", cfg.ArchivePath, "error", err)
	} else {
		archiveWriter.SetCrypto(archiveCrypto)
		blankExportService = services.NewBlankExportService(
			db, attachmentBlankService, archivePathService, archiveWriter, settingsService,
			blankExportQuotaService)
		// Скачивание (#1615, B3) делит писателя с сервисом выгрузки - оба читают/пишут
		// один и тот же корень архива, заводить второй экземпляр незачем.
		archiveDownloadService = services.NewArchiveDownloadService(db, archiveWriter, settingsService)
	}
	// Точки изменения заявки ставят её в очередь на выгрузку (#1615, B1). Сеттеры,
	// а не конструкторские опции: сервисы выше уже собраны, а blankExportService
	// поднят только сейчас (зависит от attachmentBlankService/archivePathService).
	// blankExportService типизированный nil безопасен - Enqueue* на нём no-op.
	// Приложенные к заявке файлы уезжают в архив тем же путём, что и бланки.
	blankExportService.SetApplicationFiles(applicationFileService)
	applicationService.SetBlankExportEnqueuer(blankExportService)
	organizationService.SetBlankExportEnqueuer(blankExportService)
	companyService.SetBlankExportEnqueuer(blankExportService)
	carService.SetBlankExportEnqueuer(blankExportService)
	employeeService.SetBlankExportEnqueuer(blankExportService)
	blankArchiveHandler := handlers.NewBlankArchiveHandler(settingsService, blankExportService)
	blankArchiveStatsHandler := handlers.NewBlankArchiveStatsHandler(blankExportQuotaService)
	// access/resolver - те же зависимости, что у AttachmentBlankHandler ниже: гейт
	// скачивания одного бланка и ZIP заявки из архива обязаны совпадать (#1615, B3).
	archiveDownloadHandler := handlers.NewArchiveDownloadHandler(archiveDownloadService, applicationService, permissionResolver)
	bugReportHandler := handlers.NewBugReportHandler(bugReportService)
	maintenanceHandler := handlers.NewMaintenanceHandler(maintenanceService)
	markHandler := handlers.NewMarkHandler(markService)
	vehicleBlacklistHandler := handlers.NewVehicleBlacklistHandler(vehicleBlacklistService)
	personBlacklistHandler := handlers.NewPersonBlacklistHandler(personBlacklistService)
	attachmentTemplateHandler := handlers.NewAttachmentTemplateHandler(attachmentTemplateService, attachmentFieldConfigService)
	// archiveDownloadService - тот же источник, что у ZIP заявки (h.Archive выше):
	// скачивание одного бланка с source=archive обязано видеть те же файлы (#1615, C6).
	attachmentBlankHandler := handlers.NewAttachmentBlankHandler(attachmentBlankService, applicationService, permissionResolver, archiveDownloadService)
	attachmentImportHandler := handlers.NewAttachmentImportHandler(attachmentImportService)
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
	// Приём файла массового импорта (blank-import, C1C2) разбирает .xlsx на до 2000
	// строк - дороже обычной ручки, поэтому сверх общего лимита свой, per-user/IP.
	importListLimiter := mw.RateLimit(10, 60)
	// Смена своего пароля (#1915): форма принимает текущий пароль, значит годится
	// для подбора. Лестница блокировки входа сюда не достаёт - она считает неудачные
	// логины, - поэтому свой лимит: пять попыток за пять минут на пользователя.
	selfPasswordLimiter := mw.RateLimit(5, 300)
	maintenanceBlock := mw.MaintenanceBlock(maintenanceService)
	banCheck := mw.BanCheck(banCheckService)
	// Гейт согласия на обработку ПД (#1567). Пока тумблер выключен или текст пуст,
	// пропускает всех - включается настройкой, а не деплоем.
	consentGate := mw.PDConsentGate(pdConsentGateService)
	// Обязательная смена пароля из письма (#1911). Флаг читается из базы, а не из
	// маркера доступа: маркер живёт до 15 минут и версии пароля не несёт.
	mustChangePassword := mw.MustChangePassword(services.NewPasswordChangeGateService(db, 30*time.Second))
	lastSeen := mw.LastSeen(db)

	// Routes
	eventsTickets := realtime.NewTicketStore(60 * time.Second)
	eventsHandler := handlers.NewEventsHandler(eventsHub, eventsTickets)

	// Сквозной поиск. Конструктор проверяет реестр разделов и падает на старте, если
	// раздел объявлен без права: такой раздел в рантайме был бы открыт всем подряд.
	searchService, err := services.NewSearchService(db, permissionResolver)
	if err != nil {
		slog.Error("failed to init search service", "error", err)
		os.Exit(1)
	}
	searchHandler := handlers.NewSearchHandler(searchService)

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
		ApplicationFiles:    applicationFileHandler,
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
		Push:                pushHandler,
		RequestLogs:         requestLogsHandler,
		EmployeesHistory:    employeesHistoryHandler,
		BugReport:           bugReportHandler,
		Maintenance:         maintenanceHandler,
		Marks:               markHandler,
		VehicleBlacklist:    vehicleBlacklistHandler,
		PersonBlacklist:     personBlacklistHandler,
		AttachmentTemplates: attachmentTemplateHandler,
		AttachmentBlanks:    attachmentBlankHandler,
		AttachmentImport:    attachmentImportHandler,
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
		Search:              searchHandler,
		BlankArchive:        blankArchiveHandler,
		BlankArchiveStats:   blankArchiveStatsHandler,
		ArchiveDownload:     archiveDownloadHandler,
		PermResolver:        permissionResolver,
		DenialLog:           accessDenialService,
		MaintenanceBlock:    maintenanceBlock,
		BanCheck:            banCheck,
		ConsentGate:         consentGate,
		MustChangePassword:  mustChangePassword,
		LoginLimiter:        loginLimiter,
		ImportListLimiter:   importListLimiter,
		SelfPasswordLimiter: selfPasswordLimiter,
		LastSeen:            lastSeen,
		TableReportGate:     mw.RequireTableVerb(db, permissionResolver, accessDenialService, "report"),
		TableVersionsGate:   mw.RequireTableVerb(db, permissionResolver, accessDenialService, "versions"),
		TableTrashGate:      mw.RequireTableVerb(db, permissionResolver, accessDenialService, "trash"),
		TablePassGate:       mw.RequireTablePassVerb(db, permissionResolver, accessDenialService),
		Impersonation:       impersonationHandler,
		JWTSecret:           []byte(cfg.JWTSecret),
		JWTRefreshSecret:    []byte(cfg.JWTRefreshSecret),
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

	// Предупреждение об истекающем пропуске (#1748, S4): ежедневно в 09:00 по рабочей
	// зоне отбирает заявки, чей общий срок кончается через три дня или завтра, и шлёт
	// инициатору. См. ExpiryNotifyService.NotifyExpiringSoon. Дедупликация от повторных
	// рестартов - внутри сервиса (по существующему уведомлению за последние сутки).
	go startExpiryNotifyScheduler(ctxSig, expiryNotifyService, resetLoc)

	// Архив access_denials: 3 мес retention, цикл раз в сутки.
	go startAccessDenialsArchiver(ctxSig, accessDenialService, 90*24*time.Hour, 24*time.Hour)

	// Ежедневный сброс территориальных статусов "Покинул/Выехал" -> "Не входил/Не въезжал"
	// в 06:00. resetLoc посчитан выше, вместе с сервисами.
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

	// Суточная уборка технического мусора: недействительные токены сессий,
	// прочитанные уведомления, непрочитанные уведомления (свой, более мягкий срок) и
	// подписки Web Push без единой успешной доставки (#974). Остальные журналы
	// чистятся только вручную подкомандой cleanup - там решение за оператором.
	go startRetentionWorker(ctxSig, db, cfg.RefreshTokenRetentionDays, cfg.ReadNotificationRetentionDays, cfg.NotificationRetentionDays, cfg.PushSubscriptionRetentionDays, 24*time.Hour)

	// Уборка файлов, загруженных к заявке, которую так и не отправили (#1721).
	go startApplicationFileSweeper(ctxSig, applicationFileService, cfg.ApplicationFileDraftTTL, time.Hour)

	// Разбор очереди исходящих писем (#1906). При пустом SMTP_HOST горутина
	// завершается сразу: очередь тогда и не наполняется.
	go startMailWorker(ctxSig, mailService, cfg.MailWorkerTick)

	// Проверка сроков действия паролей (#1910): раз в сутки в 04:00 по рабочей
	// зоне. 03:00 занят сверкой файлового архива, 06:00 - сбросом территориальных
	// статусов.
	go startPasswordRotationScheduler(ctxSig, passwordRotationService, resetLoc)

	// Файловый архив бланков (#1615, B1): разбор очереди enqueue, подметатель
	// повторов и ежесуточная сверка реестра с диском в 03:00 по resetLoc. nil,
	// если каталог архива не поднялся - startFileArchiveWorker сама это проверяет
	// и завершается без цикла.
	go startFileArchiveWorker(ctxSig, blankExportService, cfg.ArchiveWorkerTick, cfg.ArchiveSweepInterval, resetLoc)

	// Graceful shutdown. О завершении сообщаем каналом: e.Start возвращает
	// ErrServerClosed сразу после e.Shutdown, и без ожидания main выходил, пока
	// эта горутина только принималась дожимать фоновые отправки.
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctxSig.Done()
		slog.Info("shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.Shutdown(ctx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
		// Push-рассылка (#974): HTTP-сервер уже остановлен и новых запросов не
		// примет, но фоновые push-горутины, запущенные ДО остановки, могли ещё не
		// закончить отправку. Даём им отдельный, короткий срок - иначе рантайм Go
		// убьёт их вместе с процессом молча, без единой строки в логе.
		pushCtx, pushCancel := context.WithTimeout(context.Background(), services.PushShutdownGrace)
		defer pushCancel()
		pushService.Shutdown(pushCtx)

		// Журнал обращений: в очереди писателя лежат записи уже обслуженных
		// запросов. Без слива они пропадут - в журнале не окажется как раз тех
		// обращений, что шли перед остановкой, а разбирают обычно их.
		logCtx, logCancel := context.WithTimeout(context.Background(), mw.RequestLogShutdownGrace)
		defer logCancel()
		requestLogWriter.Shutdown(logCtx)
	}()

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.BindHost, cfg.BindPort)
	slog.Info("starting server", "addr", addr)
	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
	// Даём горутине остановки доделать своё: дожать push-отправки и записать
	// накопленный журнал обращений. Срок с запасом над их собственными окнами -
	// если он всё же вышел, процесс уходит, но с записью в логе, а не молча.
	select {
	case <-shutdownDone:
	case <-time.After(shutdownGrace):
		slog.Warn("остановка не уложилась в отведённый срок - часть фоновых задач прервана")
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

// startRetentionWorker раз в interval сметает данные, которые обесценились сами:
// недействительные токены сессий, прочитанные и непрочитанные уведомления (два
// разных срока) и подписки Web Push без единой успешной доставки (#974). Первый
// прогон сразу - после долгого простоя мусор копится, ждать сутки незачем.
func startRetentionWorker(ctx context.Context, db *gorm.DB, tokenDays, notificationDays, unreadNotificationDays, pushSubscriptionDays int, interval time.Duration) {
	run := func() {
		database.SweepRoutine(ctx, db, tokenDays, notificationDays, unreadNotificationDays, pushSubscriptionDays)
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("retention worker stopped")
			return
		case <-ticker.C:
			run()
		}
	}
}

// startApplicationFileSweeper убирает файлы, загруженные к заявке, которую так и
// не отправили (#1721): заявитель выбрал документы и закрыл форму. Ходит чаще
// суток, потому что такие файлы занимают место, ни на что не влияя.
// startPasswordRotationScheduler раз в сутки в 04:00 по location проверяет сроки
// действия паролей: сначала предупреждает тех, у кого срок подходит, затем помечает
// истёкшими пароли тех, у кого он вышел. Выключенная настройка делает оба шага
// пустыми - решение сервиса, а не планировщика.
//
// Паролей прогон не придумывает и писем с ними не шлёт: помеченный работник входит
// своим прежним паролем, а дальше формы смены его не пускает гейт. Повторный проход
// того же дня никого не выберет - уже помеченные из отбора исключены.
func startPasswordRotationScheduler(ctx context.Context, svc *services.PasswordRotationService, location *time.Location) {
	if svc == nil {
		return
	}
	now := time.Now().In(location)
	next := time.Date(now.Year(), now.Month(), now.Day(), services.RotationRunHour, 0, 0, 0, location)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	timer := time.NewTimer(time.Until(next))
	defer timer.Stop()
	slog.Info("планировщик сроков действия паролей запущен", "next_run", next.Format(time.RFC3339))

	run := func() {
		svc.NotifyExpiring(ctx)
		svc.RunScheduled(ctx)
	}

	select {
	case <-ctx.Done():
		slog.Info("планировщик сроков действия паролей остановлен до первого срабатывания")
		return
	case <-timer.C:
	}
	run()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("планировщик сроков действия паролей остановлен")
			return
		case <-ticker.C:
			run()
		}
	}
}

// startMailWorker разбирает очередь исходящих писем: раз в tick забирает пачку
// ожидающих отправки и шлёт её одним SMTP-соединением. Ненастроенная почта
// завершает горутину сразу - при пустом SMTP_HOST письма в очередь не попадают.
func startMailWorker(ctx context.Context, svc services.MailSender, tick time.Duration) {
	if svc == nil || !svc.Enabled() {
		slog.Info("почта не настроена, воркер очереди писем не запущен")
		return
	}
	run := func() {
		sent, failed := svc.ProcessQueue(ctx)
		if sent > 0 || failed > 0 {
			slog.Info("почта: очередь разобрана", "sent", sent, "failed", failed)
		}
	}
	run()
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("почта: воркер очереди остановлен")
			return
		case <-ticker.C:
			run()
		}
	}
}

func startApplicationFileSweeper(ctx context.Context, svc services.ApplicationFileService, ttl, interval time.Duration) {
	run := func() {
		removed, err := svc.SweepOrphans(ctx, ttl)
		if err != nil {
			slog.Error("не удалось убрать неприложенные файлы заявок", "error", err)
		} else if removed > 0 {
			slog.Info("убраны неприложенные файлы заявок", "count", removed)
		}
		// Второй проход - по диску: строки уносит каскад от заявки, файлы остаются.
		orphans, err := svc.SweepDiskOrphans(ctx)
		if err != nil {
			slog.Error("не удалось убрать файлы заявок без записей", "error", err)
			return
		}
		if orphans > 0 {
			slog.Info("убраны файлы заявок без записей", "count", orphans)
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("application file sweeper stopped")
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

// startExpiryNotifyScheduler гоняет предупреждения об истекающем пропуске (#1748, S4)
// ежедневно в services.ExpiryNotifyRunHour по location. Прогона при старте нет
// намеренно: раньше задача висела на тикере от момента запуска, и перезапуск бэкенда
// среди ночи переносил рассылку на ночь - человек получал предупреждение в три часа
// вместо утра. От повторов внутри одних суток защищает сам сервис.
func startExpiryNotifyScheduler(ctx context.Context, svc services.ExpiryNotifyService, location *time.Location) {
	next := nextDailyRun(time.Now(), services.ExpiryNotifyRunHour, location)
	timer := time.NewTimer(time.Until(next))
	defer timer.Stop()
	slog.Info("планировщик предупреждений об истечении пропуска запущен", "next_run", next.Format(time.RFC3339))

	select {
	case <-ctx.Done():
		slog.Info("expiry notify scheduler stopped")
		return
	case <-timer.C:
	}

	run := func() {
		if err := svc.NotifyExpiringSoon(ctx); err != nil {
			slog.Error("expiry notify run failed", "error", err)
		}
	}
	run()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("expiry notify scheduler stopped")
			return
		case <-ticker.C:
			run()
		}
	}
}

// nextDailyRun возвращает ближайшее наступление часа hour по location: сегодняшнее,
// если оно ещё впереди, иначе завтрашнее.
func nextDailyRun(now time.Time, hour int, location *time.Location) time.Time {
	local := now.In(location)
	next := time.Date(local.Year(), local.Month(), local.Day(), hour, 0, 0, 0, location)
	if !next.After(local) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

// startFileArchiveWorker обслуживает файловый архив бланков (#1615, B1):
//   - разбирает очередь enqueue по каждому тику ArchiveWorkerTick И по Wake() -
//     точки изменения заявки будят воркер сразу, тик подстраховывает; перед
//     разбором спрашиваются пороги места (B2): узнать про нехватку надо ДО записи,
//     а сработавший жёсткий порог пропускает разбор целиком;
//   - раз в ArchiveSweepInterval подметает заявки, чья прошлая попытка выгрузки
//     провалилась транзиентно и подошёл срок повтора;
//   - раз в сутки в 03:00 по location сверяет реестр с диском в окне recheck_days
//     (ловит заявки, потерявшие очередь вместе с процессом, и заодно докладывает
//     о выявленном расхождении);
//   - убирает временный мусор (.tmp-*) на старте и затем раз в час - недописанный
//     файл после падения процесса иначе остаётся под каталогом заявки навсегда.
//
// svc == nil, если каталог архива не поднялся (Validate прошёл, но запись
// недоступна) - воркеру тогда нечего делать, и горутина завершается сразу.
func startFileArchiveWorker(ctx context.Context, svc *services.BlankExportService, tick, sweepInterval time.Duration, location *time.Location) {
	if svc == nil {
		return
	}
	writer := svc.Writer()

	if removed, err := writer.CleanupTemp(time.Hour); err != nil {
		slog.Error("файловый архив: уборка временных файлов на старте не удалась", "error", err)
	} else if removed > 0 {
		slog.Info("файловый архив: временные файлы убраны на старте", "count", removed)
	}

	workTicker := time.NewTicker(tick)
	defer workTicker.Stop()
	sweepTicker := time.NewTicker(sweepInterval)
	defer sweepTicker.Stop()
	tmpTicker := time.NewTicker(time.Hour)
	defer tmpTicker.Stop()

	now := time.Now().In(location)
	nextRecheck := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, location)
	if !nextRecheck.After(now) {
		nextRecheck = nextRecheck.Add(24 * time.Hour)
	}
	recheckTimer := time.NewTimer(time.Until(nextRecheck))
	defer recheckTimer.Stop()

	slog.Info("файловый архив: воркер запущен",
		"tick", tick, "sweep_interval", sweepInterval, "next_recheck", nextRecheck.Format(time.RFC3339))

	processQueue := func() {
		if processed, failed := svc.ProcessQueue(ctx); processed+failed > 0 {
			slog.Info("файловый архив: очередь разобрана", "processed", processed, "failed", failed)
		}
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("файловый архив: воркер остановлен")
			return
		case <-svc.Wake():
			processQueue()
		case <-workTicker.C:
			processQueue()
		case <-sweepTicker.C:
			if processed, failed := svc.Sweep(ctx); processed+failed > 0 {
				slog.Info("файловый архив: подметены заявки на повтор", "processed", processed, "failed", failed)
			}
		case <-tmpTicker.C:
			if removed, err := writer.CleanupTemp(time.Hour); err != nil {
				slog.Error("файловый архив: уборка временных файлов не удалась", "error", err)
			} else if removed > 0 {
				slog.Info("файловый архив: временные файлы убраны", "count", removed)
			}
		case <-recheckTimer.C:
			processed, failed := svc.Recheck(ctx)
			slog.Info("файловый архив: ночная сверка выполнена", "processed", processed, "failed", failed)
			nextRecheck = nextRecheck.Add(24 * time.Hour)
			recheckTimer.Reset(time.Until(nextRecheck))
		}
	}
}
