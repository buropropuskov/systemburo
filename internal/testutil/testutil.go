package testutil

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/crypto"
	"systemburo/internal/database"
	"systemburo/internal/handlers"
	mw "systemburo/internal/middleware"
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
	"request_log", "request_logs", "notifications", "news", "announcements",
	"feedback", "application_items", "items",
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
	"application_status_history", "applications",
	"bureau_time_slots",
	"companies_unload_places", "organization_unload_places",
	"unload_place_time_slots", "unload_place_photos", "unload_places",
	"table_fields", "table_field_facts", "companies_tables", "organization_tables",
	"system_table_time_slots", "system_table_photos", "system_tables",
	"license_plate_format_cells", "license_plate_formats",
	"citizenships",
	"companies_users", "organization_users",
	"auth_events", "refresh_tokens", "users",
	"roles",
	"companies", "organizations", "user_types",
}

// SetupTestApp creates a fully wired Echo app with real DB, identical to production.
// AutoMigrate runs once per test binary via sync.Once; each test still uses CleanDB for isolation.
func SetupTestApp(t *testing.T) (*echo.Echo, *gorm.DB, func()) {
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
	consentHandler := handlers.NewConsentHandler(consentService, db)
	settingsHandler := handlers.NewSettingsHandler(settingsService, documentFileService, 10*1024*1024)
	telegramService := services.NewTelegramService("", "")
	bugReportService := services.NewBugReportService(db, telegramService)
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
	guideHandler := handlers.NewGuideHandler(guideService, guideFileService, 10*1024*1024)
	auditHandler := handlers.NewAuditHandler(services.NewAuditReader(db))
	authEventHandler := handlers.NewAuthEventHandler(services.NewAuthEventReader(db))

	// Setup Echo with routes (no rate limiter, no logger — clean for tests)
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = handlers.CustomHTTPErrorHandler
	e.Validator = appvalidator.New()
	// Журнал доступа к персональным данным (152-ФЗ) висит глобально и в проде, и здесь:
	// без него сверка путей проверялась бы только юнитом, а именно она молча разъехалась
	// с реальными адресами (#1472).
	e.Use(mw.PDAudit(db))
	// nil loginLimiter - в тестах rate-limit на /login не применяется,
	// т.к. тесты делают много логинов подряд. Отдельный Test* покрывает сам лимитер.
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

	return e, db, cleanup
}

// CleanDB deletes all test data and re-seeds reference tables.
// Uses DELETE (faster than TRUNCATE for mostly-empty tables: no ACCESS EXCLUSIVE locks).
// Sequences for reference tables are reset so Seed produces deterministic IDs.
func CleanDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	for _, table := range tables {
		if err := db.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatalf("CleanDB delete %s: %v", table, err)
		}
	}

	// Reset sequences for tables that Seed re-populates (IDs must be deterministic).
	db.Exec("ALTER SEQUENCE user_types_id_seq RESTART WITH 1")
	db.Exec("ALTER SEQUENCE permissions_id_seq RESTART WITH 1")

	if err := database.Seed(db); err != nil {
		t.Fatalf("CleanDB seed failed: %v", err)
	}
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
