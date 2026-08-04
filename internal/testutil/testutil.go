package testutil

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	dbOnce   sync.Once
	cachedDB *gorm.DB
	// cachedResolver -- последний созданный в SetupTestApp resolver прав. Нужен
	// GrantTableVerb, чтобы сбросить кэш прав юзера после выдачи override (иначе
	// grant, сделанный после первого резолва юзера, не подхватится в том же тесте).
	cachedResolver *services.PermissionResolver
)

const (
	TestJWTSecret        = "test-jwt-secret"
	TestJWTRefreshSecret = "test-jwt-refresh-secret"
)

// tables lists all tables in FK-safe deletion order (dependents first).
var tables = []string{
	"audit_log",
	// blank_exports живёт без внешних ключей, поэтому чистка заявок её строк не
	// снимает: прошлый прогон оставлял заявку с тем же идентификатором уже
	// выгруженной и замороженной, и следующий видел чужое состояние (#1615).
	"blank_exports",
	// Партии вымышленных данных стенда (#1682): внешних ключей у них нет намеренно -
	// перечень созданного должен пережить удаление самих записей, иначе удалять партию
	// будет не по чему. Значит и чистка соседних таблиц их не снимает.
	"fake_batch_items", "fake_batches",
	"daily_pass_reports",
	"system_settings",
	"user_online_peaks",
	"report_templates",
	"pd_audit_logs", "pd_consents",
	"access_denials", "access_denial_archives",
	"user_permission_overrides", "user_groups", "permission_group_grants",
	"role_default_groups", "permission_groups",
	"user_permissions", "permissions",
	"bug_reports",
	"guide_sections",
	"documents", "document_groups",
	// request_logs_daily - суточный агрегат запросов, наполняется через ON CONFLICT
	// DO UPDATE с прибавлением счётчика. Внешних ключей у него нет, поэтому чистка
	// журнала его не касается: значение накапливалось от прогона к прогону, и
	// TestLogPartitionMaintenance ждал единицу, а получал столько, сколько раз
	// запускали тесты на этой базе.
	"request_logs_daily", "request_log", "request_logs", "notifications", "news", "announcements",
	// analytics_cache и user_ban_histories тоже живут без внешних ключей: кэш отдаёт
	// тесту цифры прошлого прогона, история банов копится вечно.
	"analytics_cache", "user_ban_histories",
	// feedback_reads тоже без внешних ключей: строки прошлого прогона переживают и
	// чистку, и первичный TRUNCATE, а идентификаторы обращений и пользователей
	// начинаются заново - рано или поздно пара совпадает, и обращение приходит в
	// тест уже прочитанным (поймано на TestFeedback_ReadIsPerUser).
	"feedback_reads", "feedback", "application_items", "items",
	"application_blacklist_overrides", "application_blacklist_flags",
	"employee_target_tables", "employee_files", "application_employees", "employees",
	"car_unload_places", "car_target_tables", "cars",
	"vehicle_blacklists",
	"person_blacklists",
	"attachment_template_mappings", "attachment_templates",
	"attachment_custom_values", "attachment_custom_fields", "attachment_field_configs",
	"attachments",
	"unique_employees", "unique_cars", "unique_attachments",
	"application_answers", "application_question_attachments", "application_question_views", "application_question_reads", "application_questions",
	"application_status_views", "application_reads", "application_viewers", "application_approvers", "application_responsible_users",
	"application_supplement_approvals", "application_supplements",
	"application_status_history", "applications",
	"bureau_time_slots",
	"companies_unload_places", "organization_unload_places",
	"unload_place_time_slots", "unload_place_photos", "unload_places",
	"table_fields", "table_field_facts", "companies_tables", "organization_tables",
	// Окна предупреждений ссылаются на таблицу поста без каскада (NO ACTION), то есть
	// непустая таблица окон заблокировала бы удаление самих постов.
	"system_table_warning_windows",
	"system_table_time_slots", "system_table_photos", "system_tables",
	"license_plate_format_cells", "license_plate_formats",
	"citizenships",
	"companies_users", "organization_users",
	"auth_events", "refresh_tokens", "users",
	"roles",
	"companies", "organizations", "user_types",
}

// Ptr - указатель на значение: поля запроса настроек указательные, чтобы
// отсутствующий ключ означал «не трогать».
func Ptr[T any](v T) *T { return &v }

// SetArchiveSettings задаёт настройки файлового архива в тесте. Раньше тесты
// дёргали PUT /file-archive/settings, но правка настроек ушла из веба в консольную
// команду (#1615): раскладку каталогов и пороги места задаёт тот, кто разворачивает
// систему. Тестам нужен тот же путь, что у команды - сервис напрямую.
func SetArchiveSettings(t *testing.T, db *gorm.DB, req models.UpdateArchiveSettingsRequest) {
	t.Helper()
	svc := services.NewSettingsService(db, &config.Config{PaginationMaxLimit: 100})
	if _, err := svc.UpdateArchiveSettings(context.Background(), req); err != nil {
		t.Fatalf("не удалось задать настройки архива: %v", err)
	}
}

// TryArchiveSettings пробует сохранить настройки и возвращает ошибку проверки:
// тестам границ значений нужен отказ, а не падение.
func TryArchiveSettings(db *gorm.DB, req models.UpdateArchiveSettingsRequest) error {
	svc := services.NewSettingsService(db, &config.Config{PaginationMaxLimit: 100})
	_, err := svc.UpdateArchiveSettings(context.Background(), req)
	return err
}

// ArchiveEnabled - короткая форма для самого частого случая: включить или выключить
// выгрузку бланков.
func ArchiveEnabled(t *testing.T, db *gorm.DB, on bool) {
	t.Helper()
	SetArchiveSettings(t, db, models.UpdateArchiveSettingsRequest{Enabled: &on})
}

// CleanupTables отдаёт перечень таблиц, которые чистятся между тестами. Нужен
// гвард-тесту: он сверяет список с фактическим составом базы и не даёт новой
// таблице тихо остаться вне чистки.
func CleanupTables() []string {
	return append([]string(nil), tables...)
}

// CleanupExempt - таблицы, намеренно оставленные вне чистки, с причиной. Причина
// обязательна: без неё исключение через полгода читается как забытая строка.
var CleanupExempt = map[string]string{
	"attachment_unload_places":     "каскад от attachments",
	"forward_attachments":          "каскад от attachments",
	"security_user_tables":         "каскад от users",
	"security_user_unload_places":  "каскад от users",
	"table_snapshots":              "каскад от system_tables",
	"unload_place_warning_windows": "каскад от unload_places",
	"marks":                        "справочник, наполняется Seed",
	"role_permission_grants":       "выдачи прав ролям, наполняются Seed",
}

// SetupTestApp creates a fully wired Echo app with real DB, identical to production.
// AutoMigrate runs once per test binary via sync.Once; each test still uses CleanDB for isolation.
func SetupTestApp(t *testing.T) (*echo.Echo, *gorm.DB, func()) {
	t.Helper()
	e, db, _, cleanup := setupTestApp(t, false)
	return e, db, cleanup
}

// SetupTestAppWithArchive - то же приложение плюс путь к корню файлового архива
// (#1615). Отдельной функцией: корень нужен единицам тестов, которые проверяют
// файлы на диске, а менять сигнатуру SetupTestApp ради них - трогать три сотни
// чужих вызовов.
func SetupTestAppWithArchive(t *testing.T) (*echo.Echo, *gorm.DB, string, func()) {
	t.Helper()
	return setupTestApp(t, false)
}

// SetupTestAppWithConsentGate поднимает приложение с навешенным middleware гейта
// согласия на обработку ПД (#1567).
//
// Отдельной функцией, а не флагом по умолчанию: гейт закрывает protected-API, и
// включи мы его для всех, три сотни существующих тестов (в них согласия нет)
// начали бы получать 403 вместо своих ответов.
func SetupTestAppWithConsentGate(t *testing.T) (*echo.Echo, *gorm.DB, func()) {
	t.Helper()
	e, db, _, cleanup := setupTestApp(t, true)
	return e, db, cleanup
}

func setupTestApp(t *testing.T, withConsentGate bool) (*echo.Echo, *gorm.DB, string, func()) {
	t.Helper()

	crypto.SetGlobalKey(nil) // passthrough in tests

	dbOnce.Do(func() {
		db := initTestDB()
		if err := database.AutoMigrate(db); err != nil {
			log.Fatalf("AutoMigrate failed: %v", err)
		}
		// One-time TRUNCATE to clean leftover data from previous runs.
		query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(tables, ", "))
		if err := db.Exec(query).Error; err != nil {
			log.Fatalf("initial truncate failed: %v", err)
		}
		if err := database.Seed(db); err != nil {
			log.Fatalf("Seed failed: %v", err)
		}
		if err := captureSeedSnapshot(db); err != nil {
			log.Fatalf("seed snapshot failed: %v", err)
		}
		cachedDB = db
	})

	db := cachedDB

	// Create all services (same wiring as cmd/server/main.go)
	authService := services.NewAuthService(db, TestJWTSecret, TestJWTRefreshSecret, 15*time.Minute, 168*time.Hour)
	userTypeService := services.NewUserTypeService(db)
	lpfService := services.NewLicensePlateFormatService(db)
	attachmentService := services.NewAttachmentService(db)
	citizenshipService := services.NewCitizenshipService(db)
	notificationServiceEarly := services.NewNotificationService(db)
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
	carService := services.NewCarService(db, auditRecorder)
	employeeService := services.NewEmployeeService(db, auditRecorder)
	manualAttachService := services.NewManualAttachService(db, auditRecorder, nil, nil)
	permissionService := services.NewPermissionService(db)
	permissionResolver := services.NewPermissionResolver(db)
	cachedResolver = permissionResolver
	permissionGroupService := services.NewPermissionGroupService(db, permissionResolver)
	roleService := services.NewRoleService(db, permissionResolver)
	accessDenialService := services.NewAccessDenialService(db)
	userBanService := services.NewUserBanService(db, permissionResolver, nil, auditRecorder)
	systemTableService := services.NewSystemTableService(db, "./uploads", 10*1024*1024, permissionService)
	workModesService := services.NewWorkModesService(unloadPlaceService, systemTableService, bureauService)
	uniqueCarService := services.NewUniqueCarService(db)
	uniqueEmployeeService := services.NewUniqueEmployeeService(db)
	feedbackService := services.NewFeedbackService(db)
	newsService := services.NewNewsService(db)
	notificationService := notificationServiceEarly
	requestLogsService := services.NewRequestLogsService(db)
	employeesHistoryService := services.NewEmployeesHistoryService(db)
	approverService := services.NewApproverService(db)
	consentService := services.NewConsentService(db)
	settingsService := services.NewSettingsService(db, &config.Config{
		UploadMaxFileSize:       10 * 1024 * 1024,
		UploadAllowedImageTypes: []string{"image/jpeg", "image/png", "image/webp"},
		UploadAllowedDocTypes:   []string{"application/pdf"},
		PaginationMaxLimit:      100,
	})
	userService.SetPasswordPolicyProvider(settingsService)
	// Гейт согласия на обработку ПД (#1567). TTL нулевой: в тестах кэш только мешал
	// бы - настройки меняются прямо в ходе теста и должны читаться сразу.
	pdConsentGateService := services.NewPDConsentGateService(consentService, settingsService, 0)
	pdConsentStatsService := services.NewPDConsentStatsService(db, pdConsentGateService)

	// Create maintenance service early so authHandler can get it.
	maintenanceService := services.NewMaintenanceService(db)
	markService := services.NewMarkService(db)
	blacklistAuditRecorder := services.NewAuditRecorder(db)
	vehicleBlacklistService := services.NewVehicleBlacklistService(db, blacklistAuditRecorder)
	personBlacklistService := services.NewPersonBlacklistService(db, blacklistAuditRecorder)
	applicationService := services.NewApplicationService(db, permissionService, notificationService, vehicleBlacklistService, personBlacklistService, auditRecorder, services.WithApplicationPermissionResolver(permissionResolver))
	attachmentTemplateService := services.NewAttachmentTemplateService(db, "./uploads")
	attachmentFieldConfigService := services.NewAttachmentFieldConfigService(db)
	attachmentBlankService := services.NewAttachmentBlankService(db)
	trashService := services.NewTrashService(db, auditRecorder)
	trashDBRef := services.NewTrashDBRef(db)
	documentFileService := services.NewDocumentFileService("./uploads")
	guideFileService := services.NewDocumentFileServiceIn("./uploads", "guide")
	documentGroupService := services.NewDocumentGroupService(db)
	documentService := services.NewDocumentService(db, documentFileService, settingsService)
	guideService := services.NewGuideService(db, permissionResolver)

	// Create all handlers
	authHandler := handlers.NewAuthHandler(authService, maintenanceService, false, 168*time.Hour)
	userTypesHandler := handlers.NewUserTypesHandler(userTypeService)
	lpfHandler := handlers.NewLicensePlateFormatHandler(lpfService)
	attachmentHandler := handlers.NewAttachmentHandler(attachmentService)
	citizenshipHandler := handlers.NewCitizenshipHandler(citizenshipService)
	organizationHandler := handlers.NewOrganizationHandler(organizationService, db)
	companyHandler := handlers.NewCompanyHandler(companyService)
	usersHandler := handlers.NewUsersHandler(userService)
	onboardingHandler := handlers.NewOnboardingHandler(onboardingService)
	themeHandler := handlers.NewThemeHandler(themeService)
	uploadDir := t.TempDir()
	unloadPlaceHandler := handlers.NewUnloadPlaceHandler(unloadPlaceService, 10*1024*1024, uploadDir)
	bureauHandler := handlers.NewBureauHandler(bureauService)
	workModesHandler := handlers.NewWorkModesHandler(workModesService)
	carHandler := handlers.NewCarHandler(carService)
	employeeHandler := handlers.NewEmployeeHandler(employeeService)
	manualAttachHandler := handlers.NewManualAttachHandler(manualAttachService)
	systemTableHandler := handlers.NewSystemTableHandler(systemTableService, auditRecorder, 10*1024*1024, uploadDir)
	tableSnapshotHandler := handlers.NewTableSnapshotHandler(services.NewTableSnapshotService(db, carService, employeeService, employeesHistoryService))
	passReportHandler := handlers.NewPassReportHandler(services.NewDailyPassReportService(db), permissionResolver)
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
	settingsHandler := handlers.NewSettingsHandler(settingsService, documentFileService, 10*1024*1024, pdConsentGateService, pdConsentStatsService)
	// Файловый архив поднимается и в тестах: без него роуты /file-archive не
	// существуют, и гвард прав сверялся бы с роутером, где их просто нет. Корень
	// архива - временный каталог теста: запись проверяется на настоящем диске,
	// подменять писатель заглушкой смысла нет, он ровно про диск и есть.
	archiveDir := filepath.Join(t.TempDir(), "archive")
	archiveWriter, err := services.NewArchiveWriter(archiveDir)
	if err != nil {
		t.Fatalf("archive writer: %v", err)
	}
	archivePathService := services.NewArchivePathService(db, time.UTC)
	// Место и квота (#1615, срез B2): та же пара каталогов, что видит настоящий
	// процесс (архив/загрузки), логи в тестах не пишутся в файл. Как и в cmd/server,
	// сторож поднимается раньше сервиса выгрузки - фоновый прогон спрашивает пороги
	// перед записью и получает сторожа конструктором.
	blankExportQuotaService := services.NewBlankExportQuotaService(
		db, settingsService, notificationService, permissionResolver, auditRecorder,
		archiveDir, uploadDir, "")
	blankArchiveHandler := handlers.NewBlankArchiveHandler(settingsService,
		services.NewBlankExportService(db, attachmentBlankService, archivePathService,
			archiveWriter, settingsService, blankExportQuotaService))
	blankArchiveStatsHandler := handlers.NewBlankArchiveStatsHandler(blankExportQuotaService)
	// Скачивание из файлового архива (#1615, срез B3) - тот же писатель и корень, что
	// у сервиса выгрузки выше; access/resolver повторяют пару, которой пользуется
	// attachmentBlankHandler ниже, чтобы гейт ZIP заявки и гейт одного бланка не разъехались.
	archiveDownloadService := services.NewArchiveDownloadService(db, archiveWriter, settingsService)
	archiveDownloadHandler := handlers.NewArchiveDownloadHandler(
		archiveDownloadService, applicationService, permissionResolver)
	telegramService := services.NewTelegramService("", "")
	bugReportService := services.NewBugReportService(db, telegramService)
	bugReportHandler := handlers.NewBugReportHandler(bugReportService)
	maintenanceHandler := handlers.NewMaintenanceHandler(maintenanceService)
	markHandler := handlers.NewMarkHandler(markService)
	vehicleBlacklistHandler := handlers.NewVehicleBlacklistHandler(vehicleBlacklistService)
	personBlacklistHandler := handlers.NewPersonBlacklistHandler(personBlacklistService)
	attachmentTemplateHandler := handlers.NewAttachmentTemplateHandler(attachmentTemplateService, attachmentFieldConfigService)
	attachmentBlankHandler := handlers.NewAttachmentBlankHandler(attachmentBlankService, applicationService, permissionResolver, archiveDownloadService)
	trashHandler := handlers.NewTrashHandler(trashService, trashDBRef)
	documentGroupHandler := handlers.NewDocumentGroupHandler(documentGroupService)
	documentHandler := handlers.NewDocumentHandler(documentService, documentFileService)
	guideHandler := handlers.NewGuideHandler(guideService, guideFileService, 10*1024*1024)
	auditHandler := handlers.NewAuditHandler(services.NewAuditReader(db))
	authEventHandler := handlers.NewAuthEventHandler(services.NewAuthEventReader(db))
	// Real-time хендлер поднимаем и в тестах: хаб и хранилище билетов - обычные
	// структуры в памяти без горутин, зато роуты /events и /events/ticket начинают
	// существовать. Без них гвард-тест белого списка гейта согласия сверялся бы с
	// роутером, где нужного роута нет вовсе.
	eventsHandler := handlers.NewEventsHandler(realtime.NewHub(), realtime.NewTicketStore(60*time.Second))

	// Сквозной поиск поднимается и здесь: гвард-тесты его прав сверяются с роутером
	// тестового приложения, и без регистрации они молча проверяли бы роутер без поиска.
	searchService, err := services.NewSearchService(db, permissionResolver)
	if err != nil {
		t.Fatalf("search service: %v", err)
	}
	searchHandler := handlers.NewSearchHandler(searchService)

	// Setup Echo with routes (no rate limiter, no logger — clean for tests)
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = handlers.CustomHTTPErrorHandler
	e.Validator = appvalidator.New()
	// Журнал доступа к персональным данным (152-ФЗ) висит глобально и в проде, и здесь:
	// без него сверка путей проверялась бы только юнитом, а именно она молча разъехалась
	// с реальными адресами (#1472).
	e.Use(mw.PDAudit(db))
	// Гейт согласия идёт в паре с проверкой блокировки: их порядок - часть контракта
	// (забаненный не может дать согласие, поэтому ему показывается блокировка, а не
	// требование). В обычном тестовом приложении BanCheck не навешивается, чтобы не
	// менять поведение существующих тестов, поэтому пара поднимается только здесь.
	var consentGate, banCheck echo.MiddlewareFunc
	if withConsentGate {
		consentGate = mw.PDConsentGate(pdConsentGateService)
		banCheck = mw.BanCheck(services.NewBanCheckService(db, 0))
	}
	// nil loginLimiter - в тестах rate-limit на /login не применяется,
	// т.к. тесты делают много логинов подряд. Отдельный Test* покрывает сам лимитер.
	router.Setup(e, router.Dependencies{
		ConsentGate:         consentGate,
		BanCheck:            banCheck,
		Auth:                authHandler,
		UserTypes:           userTypesHandler,
		Attachments:         attachmentHandler,
		ManualAttach:        manualAttachHandler,
		LPF:                 lpfHandler,
		Citizenship:         citizenshipHandler,
		Organization:        organizationHandler,
		Company:             companyHandler,
		Users:               usersHandler,
		Onboarding:          onboardingHandler,
		Theme:               themeHandler,
		UnloadPlace:         unloadPlaceHandler,
		Bureau:              bureauHandler,
		WorkModes:           workModesHandler,
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
		Audit:               auditHandler,
		AuthEvents:          authEventHandler,
		Events:              eventsHandler,
		Search:              searchHandler,
		BlankArchive:        blankArchiveHandler,
		BlankArchiveStats:   blankArchiveStatsHandler,
		ArchiveDownload:     archiveDownloadHandler,
		PermResolver:        permissionResolver,
		DenialLog:           accessDenialService,
		TableReportGate:     mw.RequireTableVerb(db, permissionResolver, accessDenialService, "report"),
		TableVersionsGate:   mw.RequireTableVerb(db, permissionResolver, accessDenialService, "versions"),
		TableTrashGate:      mw.RequireTableVerb(db, permissionResolver, accessDenialService, "trash"),
		TablePassGate:       mw.RequireTablePassVerb(db, permissionResolver, accessDenialService),
		JWTSecret:           []byte(TestJWTSecret),
		UploadPath:          uploadDir,
	})

	// No-op cleanup: shared DB stays open for the test binary lifetime.
	cleanup := func() {}

	return e, db, archiveDir, cleanup
}

// CleanDB deletes all test data and restores reference tables from the snapshot
// taken right after the one-time Seed.
//
// Чистка уходит в базу одним пакетом инструкций, а не запросом на таблицу: на
// девяноста таблицах доминировали не сами удаления, а обмен с базой - 26 мс против
// 8.6 мс на тех же DELETE, отправленных вместе.
//
// TRUNCATE здесь заведомо хуже, хотя и выглядит уместнее: на этом списке таблиц он
// стоит 618 мс против 8.6 мс у пакета DELETE - на каждую таблицу приходится своя
// блокировка и сброс на диск, а таблицы почти всегда пустые.
func CleanDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec(cleanupSQL()).Error; err != nil {
		t.Fatalf("CleanDB cleanup failed: %v", err)
	}

	if err := restoreSeedSnapshot(db); err != nil {
		t.Fatalf("CleanDB restore failed: %v", err)
	}
}

// seedSnapshotSchema держит копии справочных таблиц отдельно от public: гвард
// изоляции перечисляет таблицы public и потребовал бы объяснения на каждую копию.
const seedSnapshotSchema = "testsnap"

// seedSnapshot - одна таблица, наполненная Seed, и её счётчик id (пустой, если
// счётчика нет: у связочных таблиц ключ составной).
type seedSnapshot struct {
	table string
	seq   string
}

var (
	seedSnapshots []seedSnapshot

	cleanupOnce sync.Once
	cleanupStmt string
)

// cleanupSQL - все удаления одним пакетом инструкций. Порядок тот же, что в tables:
// дочерние строки уходят раньше родительских.
func cleanupSQL() string {
	cleanupOnce.Do(func() {
		stmts := make([]string, 0, len(tables))
		for _, table := range tables {
			stmts = append(stmts, "DELETE FROM "+table)
		}
		cleanupStmt = strings.Join(stmts, "; ")
	})
	return cleanupStmt
}

// captureSeedSnapshot запоминает состояние справочников сразу после разового Seed,
// чтобы между тестами их восстанавливала вставка из копии.
//
// Seed идемпотентен и вызывать его повторно безопасно, но за прогон пакета handlers
// это происходит больше семисот раз, а один вызов - это десятки запросов, пять пар
// DROP/CREATE INDEX и обход разовых бэкфиллов: 65 мс против 5 мс у вставки из копии.
func captureSeedSnapshot(db *gorm.DB) error {
	if err := db.Exec("CREATE SCHEMA IF NOT EXISTS " + seedSnapshotSchema).Error; err != nil {
		return fmt.Errorf("create snapshot schema: %w", err)
	}

	// Восстанавливать нужно в порядке, обратном удалению: родители раньше детей.
	// Таблицы вне чистки идут следом - часть из них чистка всё же опустошает
	// каскадом от родителей (выдачи прав роли уходят вместе с ролями).
	candidates := make([]string, 0, len(tables)+len(CleanupExempt))
	for i := len(tables) - 1; i >= 0; i-- {
		candidates = append(candidates, tables[i])
	}
	exempt := make([]string, 0, len(CleanupExempt))
	for table := range CleanupExempt {
		exempt = append(exempt, table)
	}
	sort.Strings(exempt)
	candidates = append(candidates, exempt...)

	filled := make([]seedSnapshot, 0, len(candidates))
	for _, table := range candidates {
		var rows int64
		if err := db.Raw("SELECT count(*) FROM " + table).Scan(&rows).Error; err != nil {
			return fmt.Errorf("count %s: %w", table, err)
		}
		if rows == 0 {
			continue
		}

		snap := seedSnapshotSchema + "." + table
		if err := db.Exec("DROP TABLE IF EXISTS " + snap).Error; err != nil {
			return fmt.Errorf("drop snapshot %s: %w", snap, err)
		}
		if err := db.Exec("CREATE UNLOGGED TABLE " + snap + " AS SELECT * FROM " + table).Error; err != nil {
			return fmt.Errorf("create snapshot %s: %w", snap, err)
		}

		var seq string
		if err := db.Raw(`
			SELECT COALESCE(pg_get_serial_sequence(?, 'id'), '')
			WHERE EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = ? AND column_name = 'id'
			)`, table, table).Scan(&seq).Error; err != nil {
			return fmt.Errorf("serial sequence of %s: %w", table, err)
		}

		filled = append(filled, seedSnapshot{table: table, seq: seq})
	}

	// Какие из снятых таблиц чистка реально опустошает - выясняем пробой, а не
	// рассуждением о внешних ключах: часть таблиц уходит каскадом от родителей, а
	// часть (справочник марок) чистка не трогает вовсе, и вставка копии поверх
	// уцелевших строк ловит дубль по первичному ключу.
	if err := db.Exec(cleanupSQL()).Error; err != nil {
		return fmt.Errorf("probe cleanup: %w", err)
	}
	seedSnapshots = seedSnapshots[:0]
	for _, snap := range filled {
		var rows int64
		if err := db.Raw("SELECT count(*) FROM " + snap.table).Scan(&rows).Error; err != nil {
			return fmt.Errorf("probe count %s: %w", snap.table, err)
		}
		if rows > 0 {
			if err := db.Exec("DROP TABLE IF EXISTS " + seedSnapshotSchema + "." + snap.table).Error; err != nil {
				return fmt.Errorf("drop unused snapshot %s: %w", snap.table, err)
			}
			continue
		}
		seedSnapshots = append(seedSnapshots, snap)
	}

	// Проба оставила базу пустой - возвращаем состояние, которое дал Seed.
	return restoreSeedSnapshot(db)
}

// restoreSeedSnapshot возвращает справочники в состояние сразу после Seed.
func restoreSeedSnapshot(db *gorm.DB) error {
	if len(seedSnapshots) == 0 {
		return nil
	}

	stmts := make([]string, 0, len(seedSnapshots)*2)
	for _, snap := range seedSnapshots {
		stmts = append(stmts, "INSERT INTO "+snap.table+
			" SELECT * FROM "+seedSnapshotSchema+"."+snap.table)
		if snap.seq == "" {
			continue
		}
		// Счётчик двигаем следом за вставкой: идентификаторы пришли из копии готовыми,
		// сам он о них не знает, и следующая запись в тесте столкнулась бы с занятым id.
		stmts = append(stmts, "SELECT setval('"+snap.seq+"', COALESCE((SELECT max(id) FROM "+snap.table+"), 1))")
	}
	return db.Exec(strings.Join(stmts, "; ")).Error
}

func initTestDB() *gorm.DB {
	dsn := getTestDSN()
	ensureTestDatabase(dsn)
	db, err := gorm.Open(postgres.Open(database.EnsureUTCTimezone(dsn)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		log.Fatalf("failed to connect to test database: %v", err)
	}
	return db
}

func getTestDSN() string {
	if dsn := os.Getenv("DATABASE_URL_TEST"); dsn != "" {
		return dsn
	}
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		base = "postgres://postgres:123@db/auto_registry"
	}
	return strings.Replace(base, "/auto_registry", "/auto_registry_test", 1)
}

func ensureTestDatabase(testDSN string) {
	adminDSN := strings.Replace(testDSN, "/auto_registry_test", "/postgres", 1)
	adminDB, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("failed to connect to admin database: %v", err)
	}
	sqlDB, _ := adminDB.DB()
	defer sqlDB.Close()
	adminDB.Exec("CREATE DATABASE auto_registry_test")
}
