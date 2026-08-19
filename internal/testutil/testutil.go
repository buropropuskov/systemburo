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
	"request_logs_daily", "request_log", "request_logs", "notifications", "user_notification_preferences",
	// Подписки Web Push (#974): такая же прямая FK на users, что и у настроек уведомлений.
	"push_subscriptions",
	// Очередь исходящих писем (#1906). Внешний ключ на users стоит с ON DELETE SET
	// NULL, поэтому чистка пользователей строки не уносит: письмо прошлого прогона
	// оставалось в очереди, и следующий тест находил по адресу чужую строку - уже
	// отправленную. Ровно тот же класс, что blank_exports и fake_batches выше.
	"email_messages",
	"news", "announcements",
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
	// application_files чистится явно, а не каскадом от applications: черновики
	// лежат с application_id NULL и каскад их не достаёт.
	"application_files",
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
	// marks удаляются после cars: у машины есть ссылка на марку. Прежде таблица стояла
	// в CleanupExempt с причиной «наполняется Seed», но Seed марок не заводит - ни одна
	// не появлялась, пока их не начал создавать наливщик стенда, и тогда они стали
	// копиться между прогонами (#1682).
	"marks",
	"citizenships",
	"companies_users", "organization_users",
	"user_onboarding_progress",
	"auth_events", "refresh_tokens", "used_passwords", "users",
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
	"role_permission_grants":       "выдачи прав ролям, наполняются Seed",
}

// SetupTestApp creates a fully wired Echo app with real DB, identical to production.
// AutoMigrate runs once per test binary via sync.Once; each test still uses CleanDB for isolation.
func SetupTestApp(t *testing.T) (*echo.Echo, *gorm.DB, func()) {
	t.Helper()
	e, db, _, cleanup := SetupTestAppWithUploads(t)
	return e, db, cleanup
}

// SetupTestAppWithUploads -- то же приложение плюс каталог загрузок: тестам файлов
// заявки нужно видеть, что именно легло на диск. Отдельной функцией по той же
// причине, что и вариант с архивом, - чтобы не трогать три сотни чужих вызовов.
func SetupTestAppWithUploads(t *testing.T) (*echo.Echo, *gorm.DB, string, func()) {
	t.Helper()
	e, db, _, uploadDir, cleanup := setupTestApp(t, false, false)
	return e, db, uploadDir, cleanup
}

// SetupTestAppWithArchive - то же приложение плюс путь к корню файлового архива
// (#1615). Отдельной функцией: корень нужен единицам тестов, которые проверяют
// файлы на диске, а менять сигнатуру SetupTestApp ради них - трогать три сотни
// чужих вызовов.
func SetupTestAppWithArchive(t *testing.T) (*echo.Echo, *gorm.DB, string, func()) {
	t.Helper()
	e, db, archiveDir, _, cleanup := setupTestApp(t, false, false)
	return e, db, archiveDir, cleanup
}

// SetupTestAppWithConsentGate поднимает приложение с навешенным middleware гейта
// согласия на обработку ПД (#1567).
//
// Отдельной функцией, а не флагом по умолчанию: гейт закрывает protected-API, и
// включи мы его для всех, три сотни существующих тестов (в них согласия нет)
// начали бы получать 403 вместо своих ответов.
func SetupTestAppWithConsentGate(t *testing.T) (*echo.Echo, *gorm.DB, func()) {
	t.Helper()
	e, db, _, _, cleanup := setupTestApp(t, true, false)
	return e, db, cleanup
}

// SetupTestAppWithPasswordGate поднимает приложение с навешенным middleware
// обязательной смены пароля (#1911). Отдельной функцией по той же причине, что и
// вариант с гейтом согласия: включённый для всех гейт отдавал бы 403 каждому тесту,
// где у пользователя поднят флаг.
func SetupTestAppWithPasswordGate(t *testing.T) (*echo.Echo, *gorm.DB, func()) {
	t.Helper()
	e, db, _, _, cleanup := setupTestApp(t, false, true)
	return e, db, cleanup
}

// SetupTestAppWithBothGates поднимает приложение с ОБОИМИ гейтами сразу - согласия
// на обработку ПД и обязательной смены пароля. Именно так система стоит на
// установке, где согласие запрошено: новый работник упирается в оба одновременно.
//
// Связки не было, и это оказалось слепым пятном: каждый гейт проверялся в
// одиночку, а то, что они закрывают друг другу выход и запирают работника
// снаружи, не ловил ни один тест.
func SetupTestAppWithBothGates(t *testing.T) (*echo.Echo, *gorm.DB, func()) {
	t.Helper()
	e, db, _, _, cleanup := setupTestApp(t, true, true)
	return e, db, cleanup
}

func setupTestApp(t *testing.T, withConsentGate, withPasswordGate bool) (*echo.Echo, *gorm.DB, string, string, func()) {
	t.Helper()

	crypto.SetGlobalKey(nil) // passthrough in tests

	dbOnce.Do(func() {
		db := initTestDB()
		// sync.Once защищает только свой процесс, а `go test ./...` запускает
		// пакеты параллельными бинарями по одной базе. Двое одновременных
		// CREATE TABLE по одной таблице падают конфликтом системного индекса
		// каталога PostgreSQL (#1974), и то же касается сидера: два пакета
		// вставляли выдачи прав роли и ловили duplicate key. Замок делает
		// подготовку базы тем, чем она и должна быть - разовой, а не гонкой.
		if err := withPreparationLock(db, func() error {
			if err := database.AutoMigrate(db); err != nil {
				return fmt.Errorf("AutoMigrate failed: %w", err)
			}
			// One-time TRUNCATE to clean leftover data from previous runs.
			query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(tables, ", "))
			if err := db.Exec(query).Error; err != nil {
				return fmt.Errorf("initial truncate failed: %w", err)
			}
			if err := database.Seed(db); err != nil {
				return fmt.Errorf("Seed failed: %w", err)
			}
			return nil
		}); err != nil {
			log.Fatal(err)
		}
		if err := captureSeedSnapshot(db); err != nil {
			log.Fatalf("seed snapshot failed: %v", err)
		}
		cachedDB = db
	})

	db := cachedDB

	// Настройки возвращаются к посеянным ДО сборки сервисов. Служба настроек
	// читает их в кэш процесса один раз, при создании, а CleanDB тест зовёт уже
	// после SetupTestApp - и новая служба успевала прочитать то, что оставил
	// предыдущий тест. Так проверка политики паролей, включённая в тесте
	// настроек, роняла следующий тест на создании пользователя.
	if err := restoreSeedTable(db, "system_settings"); err != nil {
		t.Fatalf("восстановление настроек перед сборкой служб: %v", err)
	}

	// Create all services (same wiring as cmd/server/main.go)
	userTypeService := services.NewUserTypeService(db)
	lpfService := services.NewLicensePlateFormatService(db)
	attachmentService := services.NewAttachmentService(db)
	citizenshipService := services.NewCitizenshipService(db)
	permissionResolver := services.NewPermissionResolver(db)
	// pushService поднимается и в тестах (#974), пустыми VAPID-ключами: Configured()
	// false, Send() - no-op. Подключается той же опцией конструктора, что и в
	// cmd/server/main.go, - иначе прод и тесты разошлись бы в wiring и push молчал бы
	// в проде при зелёных тестах (уже бывало с другой опцией в этом файле).
	pushService := services.NewPushService(db, "", "", "")
	notificationServiceEarly := services.NewNotificationService(db,
		services.WithNotificationPermissionResolver(permissionResolver),
		services.WithNotificationPushSender(pushService))
	authService := services.NewAuthService(db, TestJWTSecret, TestJWTRefreshSecret, 15*time.Minute, 168*time.Hour, services.WithAuthNotifications(notificationServiceEarly))
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
	cachedResolver = permissionResolver
	permissionGroupService := services.NewPermissionGroupService(db, permissionResolver)
	roleService := services.NewRoleService(db, permissionResolver)
	accessDenialService := services.NewAccessDenialService(db)
	userBanService := services.NewUserBanService(db, permissionResolver, nil, auditRecorder, services.WithBanNotifications(notificationServiceEarly))
	systemTableService := services.NewSystemTableService(db, "./uploads", 10*1024*1024, permissionService)
	workModesService := services.NewWorkModesService(unloadPlaceService, systemTableService, bureauService)
	uniqueCarService := services.NewUniqueCarService(db)
	uniqueEmployeeService := services.NewUniqueEmployeeService(db)
	// #1748: уведомления новостей/документов/обратной связи/техработ/корзины -
	// notificationServiceEarly и permissionResolver уже подняты выше, значит и
	// сервисам ниже есть чем реально слать (без wiring уведомления/тесты просто
	// молчали бы, а не падали - но тогда TestFeedback_Notify_*/TestNews_Notify_*/
	// TestTrash_Notify_* не смогли бы проверить реальное поведение).
	feedbackService := services.NewFeedbackService(db,
		services.WithFeedbackNotifications(notificationServiceEarly),
		services.WithFeedbackPermissionResolver(permissionResolver))
	newsService := services.NewNewsService(db, services.WithNewsNotifications(notificationServiceEarly))
	notificationService := notificationServiceEarly
	// Снимок показателей включён и в тестах: в проде он стоит между обработчиком и
	// базой, и собирать приложение без него значило бы проверять другую цепочку.
	// Секунда - чтобы соседние обращения внутри одного теста не читали вчерашнее.
	requestLogsService := services.NewRequestLogsService(db, services.WithRequestLogsStatsCache(time.Second))
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
	maintenanceService := services.NewMaintenanceService(db, services.WithMaintenanceNotifications(notificationServiceEarly))
	markService := services.NewMarkService(db)
	blacklistAuditRecorder := services.NewAuditRecorder(db)
	vehicleBlacklistService := services.NewVehicleBlacklistService(db, blacklistAuditRecorder)
	personBlacklistService := services.NewPersonBlacklistService(db, blacklistAuditRecorder)
	uploadDir := t.TempDir()
	applicationFileService := services.NewApplicationFileService(db, uploadDir, auditRecorder)
	applicationService := services.NewApplicationService(db, permissionService, notificationService, vehicleBlacklistService, personBlacklistService, auditRecorder, services.WithApplicationPermissionResolver(permissionResolver), services.WithApplicationFiles(applicationFileService, 30, 100*1024*1024))
	attachmentTemplateService := services.NewAttachmentTemplateService(db, "./uploads")
	attachmentFieldConfigService := services.NewAttachmentFieldConfigService(db)
	attachmentBlankService := services.NewAttachmentBlankService(db)
	attachmentImportService := services.NewAttachmentImportService(db, auditRecorder, uploadDir)
	trashService := services.NewTrashService(db, auditRecorder, services.WithTrashNotifications(notificationServiceEarly))
	trashDBRef := services.NewTrashDBRef(db)
	documentFileService := services.NewDocumentFileService("./uploads")
	guideFileService := services.NewDocumentFileServiceIn("./uploads", "guide")
	documentGroupService := services.NewDocumentGroupService(db)
	documentService := services.NewDocumentService(db, documentFileService, settingsService, services.WithDocumentNotifications(notificationServiceEarly))
	guideService := services.NewGuideService(db, permissionResolver)

	// Create all handlers
	authHandler := handlers.NewAuthHandler(authService, maintenanceService, false, 168*time.Hour)
	userTypesHandler := handlers.NewUserTypesHandler(userTypeService)
	lpfHandler := handlers.NewLicensePlateFormatHandler(lpfService)
	attachmentHandler := handlers.NewAttachmentHandler(attachmentService)
	citizenshipHandler := handlers.NewCitizenshipHandler(citizenshipService)
	organizationHandler := handlers.NewOrganizationHandler(organizationService, db, permissionResolver)
	companyHandler := handlers.NewCompanyHandler(companyService, db, permissionResolver)
	usersHandler := handlers.NewUsersHandler(userService)
	onboardingHandler := handlers.NewOnboardingHandler(onboardingService)
	themeHandler := handlers.NewThemeHandler(themeService)
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
	pushHandler := handlers.NewPushHandler(pushService)
	requestLogsHandler := handlers.NewRequestLogsHandler(requestLogsService, auditRecorder)
	employeesHistoryHandler := handlers.NewEmployeesHistoryHandler(employeesHistoryService)
	applicationHandler := handlers.NewApplicationHandler(applicationService, permissionResolver)
	applicationFileHandler := handlers.NewApplicationFileHandler(
		applicationFileService, applicationService,
		10*1024*1024, 10, 30*1024*1024,
		[]string{"image/jpeg", "image/png", "image/webp", "application/pdf"},
		2000, 82,
	)
	approverHandler := handlers.NewApproverHandler(approverService)
	permissionHandler := handlers.NewPermissionHandler(permissionService, permissionResolver)
	permissionGroupHandler := handlers.NewPermissionGroupHandler(permissionGroupService)
	roleHandler := handlers.NewRoleHandler(roleService)
	accessDenialHandler := handlers.NewAccessDenialHandler(accessDenialService)
	userBanHandler := handlers.NewUserBanHandler(userBanService)
	consentHandler := handlers.NewConsentHandler(consentService, pdConsentGateService, settingsService, db)
	settingsHandler := handlers.NewSettingsHandler(settingsService, documentFileService, 10*1024*1024, pdConsentGateService, pdConsentStatsService)
	// Состояние плановой смены паролей (#1909) - как в бою. Почтовый сервис не
	// подключаем: в тестах почта не настроена, и ручка обязана честно об этом
	// сообщать, а не отвечать «сервис недоступен».
	settingsHandler.SetRotationStatusService(
		services.NewPasswordRotationStatusService(db, settingsService, nil, time.UTC))
	// Файловый архив поднимается и в тестах: без него роуты /file-archive не
	// существуют, и гвард прав сверялся бы с роутером, где их просто нет. Корень
	// архива - временный каталог теста: запись проверяется на настоящем диске,
	// подменять писатель заглушкой смысла нет, он ровно про диск и есть.
	archiveDir := filepath.Join(t.TempDir(), "archive")
	archiveWriter, err := services.NewArchiveWriter(archiveDir)
	if err != nil {
		t.Fatalf("archive writer: %v", err)
	}
	// Шифрование архива включается теми же переменными, что в рабочем процессе.
	// Без этого тест не отличил бы закрытый файл от открытого, а расходятся эти
	// пути именно на площадке с ключами: ZIP расшифровывал, поштучное скачивание
	// отдавало шифротекст. Пустые переменные оставляют прежнее поведение.
	archiveCrypto, err := services.NewArchiveCrypto(os.Getenv("ARCHIVE_AGE_RECIPIENT"), os.Getenv("ARCHIVE_AGE_IDENTITY"))
	if err != nil {
		t.Fatalf("archive crypto: %v", err)
	}
	archiveWriter.SetCrypto(archiveCrypto)
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
	attachmentImportHandler := handlers.NewAttachmentImportHandler(attachmentImportService)
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
	// Нулевой TTL: тест поднимает и снимает флаг прямо в базе, ждать протухания
	// кэша ему нечем.
	var mustChangePassword echo.MiddlewareFunc
	if withPasswordGate {
		mustChangePassword = mw.MustChangePassword(services.NewPasswordChangeGateService(db, 0))
	}
	// nil loginLimiter - в тестах rate-limit на /login не применяется,
	// т.к. тесты делают много логинов подряд. Отдельный Test* покрывает сам лимитер.
	router.Setup(e, router.Dependencies{
		ConsentGate:         consentGate,
		MustChangePassword:  mustChangePassword,
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
		Impersonation:       handlers.NewImpersonationHandler(services.NewImpersonationService(db, TestJWTSecret, permissionResolver, auditRecorder)),
		JWTSecret:           []byte(TestJWTSecret),
		JWTRefreshSecret:    []byte(TestJWTRefreshSecret),
		UploadPath:          uploadDir,
	})

	// No-op cleanup: shared DB stays open for the test binary lifetime.
	cleanup := func() {}

	return e, db, archiveDir, uploadDir, cleanup
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
	// часть чистка не трогает вовсе (см. CleanupExempt), и вставка копии поверх
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
// preparationLockKey - произвольное, но постоянное число: замок общий для всех
// тестовых бинарей, и совпадать он обязан только сам с собой.
const preparationLockKey int64 = 774219740001

// withPreparationLock выполняет fn под межпроцессным замком базы.
//
// Замок берётся на ОДНОМ соединении: pg_advisory_lock живёт в сеансе, а gorm
// раздаёт соединения из пула, и освобождение с другого соединения молча не
// сработало бы - замок висел бы до конца сеанса, а следующий бинарь ждал бы его
// впустую. Отсюда явный conn вместо db.Exec.
//
// Ожидание не ограничено по времени намеренно: под замком идут миграция и сидер,
// это секунды, а тайм-аут превратил бы редкую гонку в редкое падение с менее
// понятной причиной.
func withPreparationLock(db *gorm.DB, fn func() error) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("подключение для замка подготовки: %w", err)
	}
	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("соединение для замка подготовки: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", preparationLockKey); err != nil {
		return fmt.Errorf("взятие замка подготовки: %w", err)
	}
	defer func() {
		if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", preparationLockKey); err != nil {
			log.Printf("не удалось снять замок подготовки: %v", err)
		}
	}()

	return fn()
}

// restoreSeedTable возвращает одну таблицу к состоянию после Seed. Нужна там, где
// полное восстановление снимка избыточно и небезопасно: оно чистит всё подряд, а
// здесь требуется вернуть только настройки.
func restoreSeedTable(db *gorm.DB, table string) error {
	for _, snap := range seedSnapshots {
		if snap.table != table {
			continue
		}
		stmt := "DELETE FROM " + snap.table +
			"; INSERT INTO " + snap.table + " SELECT * FROM " + seedSnapshotSchema + "." + snap.table
		if snap.seq != "" {
			stmt += "; SELECT setval('" + snap.seq + "', COALESCE((SELECT max(id) FROM " + snap.table + "), 1))"
		}
		return db.Exec(stmt).Error
	}
	return nil
}

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
