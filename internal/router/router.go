package router

import (
	"systemburo/internal/handlers"
	mw "systemburo/internal/middleware"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// Dependencies - все хендлеры/сервисы/middleware, нужные для регистрации маршрутов.
// Использование именованных полей вместо длинного списка позиционных параметров
// решает проблему "30+ параметров в Setup": IDE подсказывает имена, диффы при
// добавлении handler-а становятся одной строкой, тесты могут не заполнять
// неиспользуемые поля.
type Dependencies struct {
	// Handlers
	Auth                *handlers.AuthHandler
	UserTypes           *handlers.UserTypesHandler
	Attachments         *handlers.AttachmentHandler
	ManualAttach        *handlers.ManualAttachHandler
	LPF                 *handlers.LicensePlateFormatHandler
	Citizenship         *handlers.CitizenshipHandler
	Organization        *handlers.OrganizationHandler
	Company             *handlers.CompanyHandler
	Users               *handlers.UsersHandler
	Onboarding          *handlers.OnboardingHandler
	UnloadPlace         *handlers.UnloadPlaceHandler
	Cars                *handlers.CarHandler
	Employees           *handlers.EmployeeHandler
	SystemTable         *handlers.SystemTableHandler
	TableSnapshot       *handlers.TableSnapshotHandler
	PassReport          *handlers.PassReportHandler
	UniqueCar           *handlers.UniqueCarHandler
	UniqueEmployee      *handlers.UniqueEmployeeHandler
	Feedback            *handlers.FeedbackHandler
	Application         *handlers.ApplicationHandler
	Approver            *handlers.ApproverHandler
	Permissions         *handlers.PermissionHandler
	PermGroups          *handlers.PermissionGroupHandler
	Roles               *handlers.RoleHandler
	AccessDenials       *handlers.AccessDenialHandler
	UserBan             *handlers.UserBanHandler
	Consent             *handlers.ConsentHandler
	Settings            *handlers.SettingsHandler
	News                *handlers.NewsHandler
	Notifications       *handlers.NotificationHandler
	RequestLogs         *handlers.RequestLogsHandler
	EmployeesHistory    *handlers.EmployeesHistoryHandler
	BugReport           *handlers.BugReportHandler
	Maintenance         *handlers.MaintenanceHandler
	Marks               *handlers.MarkHandler
	VehicleBlacklist    *handlers.VehicleBlacklistHandler
	PersonBlacklist     *handlers.PersonBlacklistHandler
	AttachmentTemplates *handlers.AttachmentTemplateHandler
	AttachmentBlanks    *handlers.AttachmentBlankHandler
	Trash               *handlers.TrashHandler
	DocumentGroups      *handlers.DocumentGroupHandler
	Documents           *handlers.DocumentHandler
	Guide               *handlers.GuideHandler
	Statistics          *handlers.StatisticsHandler
	Reminder            *handlers.ReminderHandler
	Bureau              *handlers.BureauHandler
	WorkModes           *handlers.WorkModesHandler
	Audit               *handlers.AuditHandler
	AuthEvents          *handlers.AuthEventHandler
	Events              *handlers.EventsHandler

	// Services (для middleware и audit)
	PermResolver *services.PermissionResolver
	DenialLog    *services.AccessDenialService

	// Middleware - все опциональны (nil в тестах допустим)
	MaintenanceBlock echo.MiddlewareFunc
	BanCheck         echo.MiddlewareFunc
	LoginLimiter     echo.MiddlewareFunc
	LastSeen         echo.MiddlewareFunc
	// TableReportGate - RequireTableVerb(..., "report"): гейт отчётов по проходам
	// правом table.<name>.report. НЕ опционален для роутов pass-report (main и
	// testutil обязаны заполнять) - без гейта отчёт открылся бы любому залогиненному.
	TableReportGate echo.MiddlewareFunc

	// Misc
	JWTSecret  []byte
	UploadPath string
}

// Setup регистрирует все маршруты. См. Dependencies для описания полей.
func Setup(e *echo.Echo, d Dependencies) {
	auth := d.Auth
	userTypes := d.UserTypes
	attachments := d.Attachments
	lpf := d.LPF
	cs := d.Citizenship
	org := d.Organization
	comp := d.Company
	users := d.Users
	onboarding := d.Onboarding
	up := d.UnloadPlace
	cars := d.Cars
	employees := d.Employees
	st := d.SystemTable
	tsnap := d.TableSnapshot
	passReport := d.PassReport
	uc := d.UniqueCar
	ue := d.UniqueEmployee
	fb := d.Feedback
	app := d.Application
	approvers := d.Approver
	permissions := d.Permissions
	permGroups := d.PermGroups
	roles := d.Roles
	accessDenials := d.AccessDenials
	userBan := d.UserBan
	consent := d.Consent
	settings := d.Settings
	news := d.News
	notifications := d.Notifications
	requestLogs := d.RequestLogs
	employeesHistory := d.EmployeesHistory
	bugReport := d.BugReport
	maintenance := d.Maintenance
	marks := d.Marks
	vehicleBlacklist := d.VehicleBlacklist
	personBlacklist := d.PersonBlacklist
	attachmentTemplates := d.AttachmentTemplates
	attachmentBlanks := d.AttachmentBlanks
	trash := d.Trash
	docGroups := d.DocumentGroups
	docs := d.Documents
	guide := d.Guide
	statistics := d.Statistics
	reminder := d.Reminder
	bureau := d.Bureau
	audit := d.Audit
	authEvents := d.AuthEvents
	events := d.Events
	permResolver := d.PermResolver
	denialLog := d.DenialLog
	// requireAdmin — гейт admin-страниц по page.admin (super/admin проходят,
	// обычные — по гранту). Заменяет легаси type-code проверки в сервисах (Ф5).
	requireAdmin := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageAdmin)
	maintenanceBlock := d.MaintenanceBlock
	banCheck := d.BanCheck
	loginLimiter := d.LoginLimiter
	lastSeen := d.LastSeen
	jwtSecret := d.JWTSecret
	// Health check — вне /api, для мониторинга и readiness-проб.
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Все API-роуты под префиксом /api — разделяет API и SPA-роуты (/news, /center
	// и т.д. в Vue router). Nginx проксирует /api/ на backend, остальное — на frontend.
	api := e.Group("/api")

	// Статика загруженных файлов (фото мест разгрузки и системных таблиц).
	// Публично, без JWT: тег <img> не отправляет Authorization. Под /api, чтобы
	// прод-nginx (проксирует /api на backend) раздавал файлы без отдельного
	// location и правок nginx.
	if d.UploadPath != "" {
		api.Static("/uploads", d.UploadPath)
	}

	// Public routes. /login опционально защищён per-IP rate limiter-ом.
	loginHandlers := []echo.MiddlewareFunc{}
	if loginLimiter != nil {
		loginHandlers = append(loginHandlers, loginLimiter)
	}
	api.POST("/login", auth.Login, loginHandlers...)
	api.POST("/refresh-token", auth.RefreshToken)
	api.GET("/user-types", auth.GetUserTypes)
	// Публичный статус техработ - без JWT, чтобы страница /maintenance и форма /login
	// могли его опросить.
	api.GET("/settings/maintenance", maintenance.GetPublicStatus)
	// Публичные контакты Бюро пропусков - нужны на логине и в плашке блокировки.
	api.GET("/settings/contacts", settings.GetPublicContacts)

	// Real-time SSE-поток (#840). Публичный роут намеренно: EventSource не шлёт
	// заголовок Authorization. Подключение авторизуется одноразовым билетом из query
	// (выдаётся защищённым POST /events/ticket ниже), а не access-токеном - токен в
	// query утёк бы в журналы. Consume билета внутри Stream.
	if events != nil {
		api.GET("/events", events.Stream)
	}

	// Protected routes
	protected := api.Group("")
	protected.Use(mw.JWTAuth(jwtSecret))
	// Maintenance block - после JWTAuth (нужен type_id в context). Super-admin
	// проходит, остальным 503. Передаём nil в тестах чтобы не блочить.
	if maintenanceBlock != nil {
		protected.Use(maintenanceBlock)
	}
	// BanCheck - после JWTAuth (нужен user_id). Забаненный получает 403 даже с
	// валидным access-токеном до истечения exp. Кэш TTL 30s. Инвалидируется при
	// Ban/Unban из UserBanService. nil в тестах чтобы не требовать service.
	if banCheck != nil {
		protected.Use(banCheck)
	}
	// LastSeen - после JWTAuth (нужен user_id). Обновляет users.last_seen для
	// учёта онлайна (#632), с in-memory троттлингом и асинхронной записью.
	// nil в тестах, где БД-запись не нужна.
	if lastSeen != nil {
		protected.Use(lastSeen)
	}

	protected.POST("/logout", auth.Logout)
	protected.POST("/logout-all", auth.LogoutAll)
	protected.GET("/user-data", auth.GetUserData)
	protected.GET("/users/me", auth.GetCurrentUser)
	protected.GET("/users/current", auth.GetCurrentUser)

	// Онбординг-тур (#657) - self-service статус: любой авторизованный читает и
	// помечает прохождение ДЛЯ СЕБЯ (userID из JWT). Не admin-only.
	protected.GET("/onboarding", onboarding.GetStatus)
	protected.POST("/onboarding/complete", onboarding.MarkComplete)

	// Выдача одноразового билета для SSE-потока (#840). Защищён JWTAuth+banCheck:
	// забаненный/разлогиненный билет не получит, значит и поток не переоткроет.
	if events != nil {
		protected.POST("/events/ticket", events.IssueTicket)
	}

	// Шаблоны вложений (unique_attachments)
	att := protected.Group("/attachments")
	att.GET("", attachments.GetActive)
	att.GET("/all", attachments.GetAll)
	att.POST("", attachments.Create)
	att.PUT("/:id", attachments.Update)
	att.DELETE("/:id", attachments.Delete)
	att.PUT("/:id/restore", attachments.Restore)
	att.GET("/:id/history", attachments.GetHistory)
	att.GET("/:id", attachments.GetByID)
	// Привязка ручного вложения-сироты к заявке (#1049 режим-2): только super/admin.
	// Внимание: :id здесь = экземпляр attachments.id (ручная сирота), а НЕ unique_attachment
	// (шаблон), как в CRUD-маршрутах группы выше. Разные таблицы под одним префиксом.
	att.POST("/:id/attach-to-application", d.ManualAttach.AttachToApplication, requireAdmin)

	// Единый журнал аудита (#870): сводный + история одной сущности через фильтры
	// entity_type/entity_id. Admin-only - кросс-сущностный аудит чувствителен.
	protected.GET("/audit", audit.GetAuditLog, requireAdmin)

	// Управление типами пользователей (admin-only, page.admin)
	utm := protected.Group("/user-types-management", requireAdmin)
	utm.GET("", userTypes.GetAll)
	utm.POST("", userTypes.Create)
	utm.PUT("/:id", userTypes.Update)
	utm.DELETE("/:id", userTypes.Delete)
	utm.GET("/:id/history", userTypes.GetHistory)

	// Гражданства. Список и история — для всех авторизованных (дропдаун гражданств
	// в форме заявки); изменяющие операции — page.admin (Ф5, ранее service checkAdmin).
	csg := protected.Group("/citizenships")
	csg.GET("", cs.GetAll)
	csg.POST("", cs.Create, requireAdmin)
	csg.PUT("/:id", cs.Update, requireAdmin)
	csg.DELETE("/:id", cs.Delete, requireAdmin)
	csg.POST("/:id/restore", cs.Restore, requireAdmin)
	// Групповые операции (статический bulk приоритетнее param :id в Echo).
	csg.POST("/bulk/archive", cs.BulkArchive, requireAdmin)
	csg.POST("/bulk/restore", cs.BulkRestore, requireAdmin)
	csg.GET("/:id/history", cs.GetHistory)
	csg.POST("/clear-default", cs.ClearDefaults, requireAdmin)

	// Форматы номерных знаков
	lpfGroup := protected.Group("/license-plate-formats")
	lpfGroup.GET("", lpf.GetAll)
	lpfGroup.POST("", lpf.Create)
	lpfGroup.PUT("/:id", lpf.Update)
	lpfGroup.DELETE("/:id", lpf.Delete)
	lpfGroup.POST("/:id/restore", lpf.Restore)
	// Групповые операции (статический bulk приоритетнее param :id в Echo).
	lpfGroup.POST("/bulk/archive", lpf.BulkArchive)
	lpfGroup.POST("/bulk/restore", lpf.BulkRestore)
	lpfGroup.GET("/:id/history", lpf.GetHistory)

	// Марки автомобилей (#185) - справочник с историчностью.
	marksGroup := protected.Group("/marks")
	marksGroup.GET("", marks.GetAll)
	marksGroup.POST("", marks.Create)
	marksGroup.PUT("/:id", marks.Update)
	marksGroup.POST("/:id/archive", marks.Archive)
	marksGroup.POST("/:id/restore", marks.Restore)
	// Групповые операции (статический bulk приоритетнее param :id в Echo).
	marksGroup.POST("/bulk/archive", marks.BulkArchive)
	marksGroup.POST("/bulk/restore", marks.BulkRestore)
	marksGroup.GET("/:id/history", marks.GetHistory)

	// Чёрный список машин (#443). POST/DELETE/restore защищены page.admin.blacklist.
	// GET списка/истории и /check открыты любым авторизованным: фронт рендерит
	// страницу даже без права, а /check нужен всем при подаче заявки.
	requireBlacklist := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageBlacklist)
	vblGroup := protected.Group("/vehicle-blacklist")
	vblGroup.GET("", vehicleBlacklist.GetAll)
	vblGroup.GET("/check", vehicleBlacklist.Check)
	vblGroup.GET("/history", vehicleBlacklist.GetAllHistory)
	vblGroup.GET("/:id/history", vehicleBlacklist.GetHistory)
	vblGroup.POST("", vehicleBlacklist.Create, requireBlacklist)
	vblGroup.PUT("/:id", vehicleBlacklist.Update, requireBlacklist)
	vblGroup.DELETE("/:id", vehicleBlacklist.Delete, requireBlacklist)
	vblGroup.DELETE("/:id/purge", vehicleBlacklist.Purge, requireBlacklist)
	vblGroup.POST("/:id/restore", vehicleBlacklist.Restore, requireBlacklist)
	// Групповые операции (статический bulk приоритетнее param :id в Echo).
	vblGroup.POST("/bulk/archive", vehicleBlacklist.BulkArchive, requireBlacklist)
	vblGroup.POST("/bulk/restore", vehicleBlacklist.BulkRestore, requireBlacklist)

	// Чёрный список людей (#443). Та же permission page.admin.blacklist (одна страница).
	pblGroup := protected.Group("/person-blacklist")
	pblGroup.GET("", personBlacklist.GetAll)
	pblGroup.GET("/check", personBlacklist.Check)
	pblGroup.GET("/history", personBlacklist.GetAllHistory)
	pblGroup.GET("/:id/history", personBlacklist.GetHistory)
	pblGroup.POST("", personBlacklist.Create, requireBlacklist)
	pblGroup.PUT("/:id", personBlacklist.Update, requireBlacklist)
	pblGroup.DELETE("/:id", personBlacklist.Delete, requireBlacklist)
	pblGroup.DELETE("/:id/purge", personBlacklist.Purge, requireBlacklist)
	pblGroup.POST("/:id/restore", personBlacklist.Restore, requireBlacklist)
	// Групповые операции (статический bulk приоритетнее param :id в Echo).
	pblGroup.POST("/bulk/archive", personBlacklist.BulkArchive, requireBlacklist)
	pblGroup.POST("/bulk/restore", personBlacklist.BulkRestore, requireBlacklist)

	// Attachment Excel-templates (#183) - вложенные ручки под /attachments/:id/...
	attRoot := protected.Group("/attachments")
	attRoot.GET("/:id/template", attachmentTemplates.Get)
	attRoot.GET("/:id/templates", attachmentTemplates.ListTemplates)
	attRoot.POST("/:id/template", attachmentTemplates.Upload)
	attRoot.GET("/:id/template/file", attachmentTemplates.DownloadFile)
	attRoot.GET("/:id/template/:tid/file", attachmentTemplates.DownloadFileByID)
	attRoot.PUT("/:id/template/mappings", attachmentTemplates.UpdateMappings)
	attRoot.PUT("/:id/template/:tid/activate", attachmentTemplates.SetActive)
	attRoot.PUT("/:id/template/deactivate", attachmentTemplates.DeactivateAll)
	attRoot.DELETE("/:id/template", attachmentTemplates.Delete)
	attRoot.DELETE("/:id/template/:tid", attachmentTemplates.DeleteByID)
	attRoot.GET("/:id/template-fields", attachmentTemplates.GetFields)
	attRoot.GET("/:id/custom-fields", attachmentTemplates.ListCustomFields)
	attRoot.POST("/:id/custom-fields", attachmentTemplates.CreateCustomField)
	attRoot.PUT("/custom-fields/:fid", attachmentTemplates.UpdateCustomField)
	attRoot.DELETE("/custom-fields/:fid", attachmentTemplates.DeleteCustomField)
	// Настройка полей вложения (feedback-0608-H / #529): видимость/обязательность
	// базовых полей реестра + кастомные поля одним ответом.
	attRoot.GET("/:id/field-config", attachmentTemplates.GetFieldConfig)
	attRoot.PUT("/:id/field-config", attachmentTemplates.SaveFieldConfig)

	// Организации. Изменяющие операции и история - page.admin (Ф5, ранее handler-level
	// CheckAdminPermissions); списки и привязка пользователей - как было, без гейта.
	orgg := protected.Group("/organizations")
	orgg.GET("", org.GetAll)
	orgg.POST("", org.Create, requireAdmin)
	orgg.PUT("/:id", org.Update, requireAdmin)
	orgg.DELETE("/:id", org.Delete, requireAdmin)
	orgg.POST("/:id/restore", org.Restore, requireAdmin)
	orgg.GET("/:id/history", org.GetHistory, requireAdmin)
	orgg.GET("/with-users", org.GetWithUsers)
	orgg.GET("/with-users-extended", org.GetWithUsersExtended)
	orgg.GET("/:id/users", org.GetOrganizationUsers)
	orgg.PUT("/:id/users", org.UpdateOrganizationUsers)
	orgg.GET("/:id/members", org.GetMembers)
	// Блокеры архивации и перенос всех в другую организацию - гейт как у Delete (page.admin).
	orgg.GET("/:id/blocking-users", org.GetBlockingUsers, requireAdmin)
	orgg.POST("/:id/reassign-users", org.ReassignUsers, requireAdmin)
	orgg.GET("/:id/tables", org.GetOrganizationTables)
	orgg.PUT("/:id/tables", org.UpdateOrganizationTables, requireAdmin)
	orgg.GET("/:id/unload-places", org.GetOrganizationUnloadPlaces)
	orgg.PUT("/:id/unload-places", org.UpdateOrganizationUnloadPlaces, requireAdmin)
	// Групповые операции (bulk). Статический сегмент bulk имеет приоритет над
	// param :id в Echo, поэтому /bulk/restore не конфликтует с /:id/restore.
	orgg.POST("/bulk/type", org.BulkUpdateType, requireAdmin)
	orgg.POST("/bulk/unload-places", org.BulkAssignUnloadPlaces, requireAdmin)
	orgg.POST("/bulk/tables", org.BulkAssignTables, requireAdmin)
	orgg.POST("/bulk/users", org.BulkAssignUsers, requireAdmin)
	orgg.POST("/bulk/archive", org.BulkArchive, requireAdmin)
	orgg.POST("/bulk/restore", org.BulkRestore, requireAdmin)
	protected.GET("/get-organization", org.GetMyOrganization)

	// Компании. Изменяющие операции и история - page.admin (Ф5, ранее service checkAdmin);
	// списки и привязка пользователей (UpdateUsers) - как было, без отдельного гейта.
	cg := protected.Group("/companies")
	cg.GET("", comp.GetAll)
	cg.POST("", comp.Create, requireAdmin)
	cg.PUT("/:id", comp.Update, requireAdmin)
	cg.DELETE("/:id", comp.Delete, requireAdmin)
	cg.POST("/:id/restore", comp.Restore, requireAdmin)
	cg.GET("/:id/history", comp.GetHistory, requireAdmin)
	cg.GET("/with-users", comp.GetWithUsers)
	cg.GET("/with-users-extended", comp.GetWithUsersExtended)
	cg.GET("/:id/users", comp.GetUsers)
	cg.PUT("/:id/users", comp.UpdateUsers)
	cg.GET("/:id/members", comp.GetMembers)
	// Блокеры архивации и перенос всех в другую компанию - гейт как у Delete (page.admin).
	cg.GET("/:id/blocking-users", comp.GetBlockingUsers, requireAdmin)
	cg.POST("/:id/reassign-users", comp.ReassignUsers, requireAdmin)
	cg.GET("/:id/tables", comp.GetTables)
	cg.PUT("/:id/tables", comp.UpdateTables, requireAdmin)
	cg.GET("/:id/unload-places", comp.GetUnloadPlaces)
	cg.PUT("/:id/unload-places", comp.UpdateUnloadPlaces, requireAdmin)
	// Групповые операции (bulk). Статический сегмент bulk имеет приоритет над
	// param :id в Echo, поэтому /bulk/restore не конфликтует с /:id/restore.
	cg.POST("/bulk/type", comp.BulkUpdateType, requireAdmin)
	cg.POST("/bulk/unload-places", comp.BulkAssignUnloadPlaces, requireAdmin)
	cg.POST("/bulk/tables", comp.BulkAssignTables, requireAdmin)
	cg.POST("/bulk/users", comp.BulkAssignUsers, requireAdmin)
	cg.POST("/bulk/archive", comp.BulkArchive, requireAdmin)
	cg.POST("/bulk/restore", comp.BulkRestore, requireAdmin)

	// Места разгрузки
	upg := protected.Group("/unload-places")
	upg.GET("", up.GetAll)
	upg.POST("", up.Create)
	upg.GET("/:id", up.GetByID)
	upg.PUT("/:id", up.Update)
	upg.DELETE("/:id", up.Delete)
	upg.POST("/:id/restore", up.Restore)
	upg.GET("/:id/usage", up.GetUsage)
	upg.POST("/:id/detach-all", up.DetachAll, requireAdmin)
	upg.DELETE("/:id/organizations/:org_id", up.DetachOrganization, requireAdmin)
	upg.DELETE("/:id/companies/:company_id", up.DetachCompany, requireAdmin)
	// Групповые операции (статический bulk приоритетнее param :id в Echo).
	upg.POST("/bulk/archive", up.BulkArchive)
	upg.POST("/bulk/restore", up.BulkRestore)
	upg.GET("/:id/history", up.GetHistory)
	upg.GET("/:id/time-slots", up.GetTimeSlots)
	upg.POST("/:id/time-slots", up.AddTimeSlot)
	upg.PUT("/:place_id/time-slots/:slot_id", up.UpdateTimeSlot)
	upg.DELETE("/:place_id/time-slots/:slot_id", up.DeleteTimeSlot)
	upg.GET("/:id/warning-windows", up.GetWarningWindows)
	upg.POST("/:id/warning-windows", up.AddWarningWindow)
	upg.PUT("/:place_id/warning-windows/:window_id", up.UpdateWarningWindow)
	upg.DELETE("/:place_id/warning-windows/:window_id", up.DeleteWarningWindow)
	upg.POST("/:id/photos", up.UploadPhoto)
	upg.DELETE("/:place_id/photos/:photo_id", up.DeletePhoto)
	upg.POST("/:place_id/photos/:photo_id/main", up.SetMainPhoto)

	// Расписание работы Бюро (single-owner). Чтение -- любой авторизованный
	// (нужно модалке режимов работы), изменения -- админ (раздел «Информация Бюро»).
	bureauGroup := protected.Group("/bureau")
	bureauGroup.GET("/time-slots", bureau.GetTimeSlots)
	bureauGroup.POST("/time-slots", bureau.AddTimeSlot, requireAdmin)
	bureauGroup.PUT("/time-slots/:slot_id", bureau.UpdateTimeSlot, requireAdmin)
	bureauGroup.DELETE("/time-slots/:slot_id", bureau.DeleteTimeSlot, requireAdmin)

	// Режимы работы -- read-only агрегатор расписаний Бюро, мест разгрузки и мест
	// прохода в единой форме слота (для модалки «Режимы работы» в ЛК). Чтение
	// любому авторизованному.
	protected.GET("/work-modes", d.WorkModes.GetWorkModes)

	// Управление пользователями - page.admin.users (Ф5, ранее service checkAdmin
	// по type-коду manager/buropropuskov). Тот же ключ, что и у FE-роута раздела.
	requireUsers := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageAdminUsers)
	protected.POST("/users", users.Create, requireUsers)
	protected.GET("/users/all", users.GetAll, requireUsers)
	protected.PUT("/users/:username/type", users.UpdateType, requireUsers)
	protected.PUT("/users/:username/password", users.UpdatePassword, requireUsers)
	protected.PUT("/users/:username/info", users.UpdateInfo, requireUsers)
	protected.PUT("/users/:username/organization", users.UpdateOrganization, requireUsers)
	protected.PUT("/users/:username/company", users.UpdateCompany, requireUsers)
	protected.DELETE("/users/:username", users.Delete, requireUsers)
	protected.POST("/users/:username/restore", users.Restore, requireUsers)
	protected.GET("/users/:username/history", users.GetHistory, requireUsers)
	// Групповые операции над пользователями (username-keyed). Статические сегменты
	// bulk/* приоритетнее /users/:username в роутинге Echo.
	protected.POST("/users/bulk/archive", users.BulkArchive, requireUsers)
	protected.POST("/users/bulk/restore", users.BulkRestore, requireUsers)
	protected.POST("/users/bulk/type", users.BulkUpdateType, requireUsers)
	protected.POST("/users/bulk/organization", users.BulkAssignOrganization, requireUsers)
	protected.POST("/users/bulk/company", users.BulkAssignCompany, requireUsers)
	// История входов пользователя (auth_events): вход/выход/провал/блокировка/сессия.
	protected.GET("/users/:username/auth-events", authEvents.ListForUser, requireUsers)
	// Привязка мест доступа к охраннику (#706)
	protected.GET("/users/:username/unload-places", users.GetUserUnloadPlaces, requireUsers)
	protected.PUT("/users/:username/unload-places", users.SetUserUnloadPlaces, requireUsers)
	protected.GET("/users/:username/tables", users.GetUserTables, requireUsers)
	protected.PUT("/users/:username/tables", users.SetUserTables, requireUsers)

	// Машины (в заявках)
	carsGroup := protected.Group("/cars")
	carsGroup.GET("/active-for-tables", cars.GetActiveCarsForTables)
	carsGroup.GET("/active-for-table/:table_id", cars.GetActiveCarsForTable)
	// Ручное добавление машин без заявки (#1049): super/admin проходят авто,
	// остальные - по гранту entity.cars.manual_add.
	carsGroup.POST("/manual", cars.CreateManualCars,
		mw.RequirePermissionV2(permResolver, denialLog, services.KeyEntityCarsManualAdd))
	carsGroup.GET("/fact-for-tables", cars.GetFactCarsForTables)
	carsGroup.GET("/fact-for-table/:table_id", cars.GetFactCarsForTable)
	carsGroup.GET("/unload-places", cars.GetCarUnloadPlaces)
	carsGroup.GET("/fact-unload-places", cars.GetFactCarUnloadPlaces)
	carsGroup.GET("/check-active", cars.CheckActiveCar)
	carsGroup.GET("/:id/history", cars.GetCarHistory)
	carsGroup.POST("/:id/history", cars.AddCarHistoryEntry)
	carsGroup.GET("/history/all", cars.GetAllCarsHistory)
	carsGroup.GET("/history/table/:table_id", cars.GetCarsHistoryByTable)
	carsGroup.GET("/history/current-status", cars.GetCarsCurrentStatus)
	carsGroup.PUT("/:id/territory-status", cars.UpdateCarTerritoryStatus)
	carsGroup.PUT("/:id/deactivate", cars.DeactivateCar)
	carsGroup.PUT("/:id/activate", cars.ActivateCar)
	carsGroup.GET("/history/unified", cars.GetUnifiedCarHistory)
	carsGroup.PUT("/:id/restore", cars.RestoreCar)
	// Групповые операции над строками таблицы проходной (#1194): перенос/добавление/
	// снятие набора машин с таблиц «Проезд». Права admin - как остальные bulk-операции.
	carsGroup.POST("/bulk/move-table", cars.BulkMoveTable, requireAdmin)
	carsGroup.POST("/bulk/add-table", cars.BulkAddTable, requireAdmin)
	carsGroup.POST("/bulk/unbind-table", cars.BulkUnbindTable, requireAdmin)

	// Сотрудники (в заявках)
	empGroup := protected.Group("/employees")
	empGroup.POST("", employees.CreateEmployee)
	// Ручное добавление сотрудников без заявки (#1049): super/admin проходят авто,
	// остальные - по гранту entity.employees.manual_add.
	empGroup.POST("/manual", employees.CreateManualEmployees,
		mw.RequirePermissionV2(permResolver, denialLog, services.KeyEntityEmployeesManualAdd))
	empGroup.GET("/active-for-table/:table_id", employees.GetActiveEmployeesForTable)
	empGroup.PUT("/:id/territory-status", employees.UpdateEmployeeTerritoryStatus)
	empGroup.PUT("/:id/deactivate", employees.DeactivateEmployee)
	empGroup.PUT("/:id/activate", employees.ActivateEmployee)
	empGroup.PUT("/:id/restore", employees.RestoreEmployee)
	// Групповые операции над строками таблицы проходной (#1194): статические
	// сегменты bulk/* приоритетнее /:id в роутинге Echo.
	empGroup.POST("/bulk/move-table", employees.BulkMoveTable, requireAdmin)
	empGroup.POST("/bulk/add-table", employees.BulkAddTable, requireAdmin)
	empGroup.POST("/bulk/unbind-table", employees.BulkUnbindTable, requireAdmin)
	empGroup.GET("/:id/history", employeesHistory.GetByEmployee)
	empGroup.GET("/history/unified", employeesHistory.GetUnified)
	empGroup.GET("/history/all", employeesHistory.GetAll)
	empGroup.GET("/history/current-status", employeesHistory.GetCurrentStatus)
	empGroup.GET("/history/table/:table_id", employeesHistory.GetByTable)

	// Системные таблицы (конструктор таблиц)
	stg := protected.Group("/system-tables")
	stg.GET("", st.GetAll)
	stg.POST("", st.Create)
	stg.GET("/:id", st.GetByID)
	stg.PUT("/:id", st.Update)
	stg.DELETE("/:id", st.Delete)
	stg.POST("/:id/restore", st.Restore)
	stg.GET("/:id/usage", st.GetUsage)
	stg.POST("/:id/detach-all", st.DetachAll, requireAdmin)
	stg.DELETE("/:id/organizations/:org_id", st.DetachOrganization, requireAdmin)
	stg.DELETE("/:id/companies/:company_id", st.DetachCompany, requireAdmin)
	// param :id в Echo, поэтому /bulk/archive и /bulk/restore не конфликтуют с /:id/restore.
	stg.POST("/bulk/archive", st.BulkArchive)
	stg.POST("/bulk/restore", st.BulkRestore)
	stg.GET("/:id/history", st.GetHistory)
	stg.GET("/name/:name", st.GetByName)
	stg.GET("/:id/time-slots", st.GetTimeSlots)
	stg.POST("/:id/time-slots", st.AddTimeSlot)
	stg.PUT("/:table_id/time-slots/:slot_id", st.UpdateTimeSlot)
	stg.DELETE("/:table_id/time-slots/:slot_id", st.DeleteTimeSlot)
	stg.GET("/:id/warning-windows", st.GetWarningWindows)
	stg.POST("/:id/warning-windows", st.AddWarningWindow)
	stg.PUT("/:table_id/warning-windows/:window_id", st.UpdateWarningWindow)
	stg.DELETE("/:table_id/warning-windows/:window_id", st.DeleteWarningWindow)
	stg.POST("/:id/photos", st.UploadPhoto)
	stg.DELETE("/:table_id/photos/:photo_id", st.DeletePhoto)
	stg.POST("/:table_id/photos/:photo_id/main", st.SetMainPhoto)

	// Версии (слепки) состояния таблицы (#980). Дневной снимок в 06:00 снимает джоба
	// (см. startDailyStatusReset), ручной - POST. Читалки под общей auth-защитой, как
	// соседние sub-роуты system-tables (trash/history): доступ вкладки гейтит фронт
	// правом table.<slug>.versions. Чистка разрушительна - только admin/super.
	stg.POST("/:id/snapshots", tsnap.Create)
	stg.GET("/:id/snapshots", tsnap.List)
	stg.GET("/:id/snapshots/:sid", tsnap.Get)
	// Экспорт версии/текущего состояния (xlsx|pdf) файлом на скачивание. Читалка -
	// auth-only, как соседи; sid=current экспортирует текущее состояние таблицы.
	stg.GET("/:id/snapshots/:sid/export", tsnap.Export)
	stg.DELETE("/:id/snapshots", tsnap.Cleanup, requireAdmin)

	// Суточный отчёт охранника по проходам: живое окно [посл. 21:30, now) и
	// история дней. В отличие от соседей гейт НЕ на фронте: право
	// table.<name>.report проверяет BE (d.TableReportGate = RequireTableVerb),
	// FE-кнопка проверяет тот же ключ (#976: FE-гейт и BE-гейт - один набор прав).
	stg.GET("/:id/pass-report/live", passReport.Live, d.TableReportGate)
	stg.GET("/:id/pass-reports", passReport.List, d.TableReportGate)

	// Столбцы таблицы (#345)
	stg.PUT("/:id/fields", st.UpdateFields)
	// Столбцы фактовой таблицы (#345)
	stg.PUT("/:id/fact-fields", st.UpdateFactFields)

	// Корзина таблицы (#186) - удалённые элементы с возможностью восстановить
	// или окончательно удалить. Тип элементов определяется по table_type
	// системной таблицы (cars или people).
	stg.GET("/:id/trash", trash.List)
	stg.GET("/:id/trash/history", trash.History)
	stg.POST("/:id/trash/restore", trash.Restore)
	stg.DELETE("/:id/trash/:item_id", trash.PurgeOne)
	stg.DELETE("/:id/trash", trash.ClearAll)

	// Реестр автомобилей (unique_cars)
	ucg := protected.Group("/unique-cars")
	ucg.GET("", uc.GetAll)
	ucg.POST("", uc.Create)
	ucg.POST("/batch", uc.CreateBatch)
	ucg.PUT("/:id", uc.Update)
	ucg.PUT("/by-number", uc.UpdateByNumber)
	ucg.DELETE("/:id", uc.Delete)
	ucg.GET("/ownership-info", uc.GetOwnershipInfo)
	ucg.GET("/lookup", uc.Lookup, requireBlacklist)
	ucg.GET("/:id/history", uc.GetHistory)

	// Реестр сотрудников (unique_employees)
	ueg := protected.Group("/unique-employees")
	ueg.GET("", ue.GetAll)
	ueg.POST("", ue.Create)
	ueg.PUT("/:id", ue.Update)
	ueg.DELETE("/:id", ue.Delete)
	ueg.GET("/ownership-info", ue.GetOwnershipInfo)
	ueg.GET("/lookup", ue.Lookup, requireBlacklist)
	ueg.GET("/:id/history", ue.GetHistory)

	// Обратная связь. Отправка (POST) и свои обращения (GET /my) - любому
	// авторизованному; админ-операции (список/статистика/статус/прочтение) -
	// page.admin.feedback (Ф5, ранее service checkAdmin).
	requireFeedbackAdmin := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageAdminFeedback)
	fbg := protected.Group("/feedback")
	fbg.POST("", fb.Create)
	fbg.GET("/all", fb.GetAll, requireFeedbackAdmin)
	fbg.GET("/stats", fb.GetStats, requireFeedbackAdmin)
	fbg.GET("/my", fb.GetMy)
	fbg.PUT("/:id/status", fb.UpdateStatus, requireFeedbackAdmin)
	fbg.PUT("/:id/read", fb.MarkAsRead, requireFeedbackAdmin)
	fbg.PUT("/:id/flag", fb.SetFlag, requireFeedbackAdmin)

	// Заявки
	apg := protected.Group("/applications")
	apg.GET("", app.GetApplications)
	apg.POST("", app.CreateApplication)
	apg.POST("/submit-complete-application", app.SubmitCompleteApplication)
	apg.GET("/user", app.GetUserApplications)
	apg.GET("/user/status-updates-count", app.GetUserStatusUpdatesCount) // #1349 - счётчик чипа "Обновления" в ЛК
	apg.GET("/unread-count", app.GetUnreadCount)
	apg.GET("/available-attachments", app.GetAvailableAttachments)          // #706 - "Доступные мне" для охранников
	apg.GET("/available-attachments/:id", app.GetAvailableAttachmentDetail) // #706 - деталь вложения
	apg.GET("/attachable", app.GetAttachableApplications, requireAdmin)     // #1049 - заявки для привязки ручного вложения (super/admin)
	apg.GET("/:id", app.GetApplicationByID)
	apg.PUT("/:id", app.UpdateApplication)
	apg.GET("/:id/responsible-users", app.GetApplicationResponsibleUsers)
	apg.GET("/:id/details", app.GetApplicationDetails)
	apg.GET("/:id/attachments", app.GetApplicationAttachments)
	apg.GET("/:id/blank", attachmentBlanks.Download) // #183 - скачать заполненный .xlsx
	apg.POST("/:id/update-items-status", app.UpdateApplicationItemsStatus)
	apg.POST("/:id/forward", app.ForwardApplication)
	apg.GET("/:id/forward-messages", app.GetForwardMessages) // #967 - ветка заявки (пересылки)
	apg.POST("/:id/approve", app.ApproveApplicationByUser)
	apg.POST("/:id/blacklist-overrides", app.OverrideBlacklistFlag)     // #481 - "всё равно пропустить"
	apg.DELETE("/:id/blacklist-overrides", app.DeleteBlacklistOverride) // #481 - отмена подтверждения (срез C)
	apg.GET("/:id/check-approval-status", app.CheckApprovalStatus)
	apg.POST("/:id/take-to-work", app.TakeApplicationToWork)
	// #1393 - принимающий доназначает посты и места элементам заявки
	apg.PUT("/:id/elements/tables", app.AssignElementTables)
	apg.PUT("/:id/elements/unload-places", app.AssignCarUnloadPlaces)
	apg.POST("/:id/revoke-from-work", app.RevokeApplicationFromWork)
	apg.POST("/:id/restore-to-work", app.RestoreApplicationToWork)
	apg.POST("/:id/withdraw", app.WithdrawApplication)
	apg.GET("/:id/history", app.GetApplicationHistory)
	apg.POST("/:id/revoke-approval", app.RevokeApproval)
	apg.POST("/history", app.AddHistoryEntry)
	apg.GET("/:id/viewers", app.GetApplicationViewers)
	apg.POST("/:id/read", app.MarkAsRead)
	apg.GET("/:id/reads", app.GetReads)

	// Вопросы к заявке (#973: Q&A-топики + тред ответов)
	apg.GET("/:id/questions", app.GetApplicationQuestions)
	apg.POST("/:id/questions", app.CreateApplicationQuestion)
	apg.POST("/:id/questions/seen", app.MarkQuestionsSeen)
	apg.POST("/:id/questions/:questionId/answers", app.CreateApplicationAnswer)
	apg.POST("/:id/questions/:questionId/read", app.MarkQuestionRead)

	// Вложения заявок (cars/employees/items внутри вложений)
	att.GET("/:id/cars", app.GetAttachmentCars)
	att.GET("/:id/employees", app.GetAttachmentEmployees)
	att.GET("/:id/items", app.GetAttachmentItems)

	// Утверждающие заявок. Управление - page.admin (Ф5, ранее service checkAdmin);
	// журнал (history) доступен всем авторизованным (как и раньше - без checkAdmin).
	aag := protected.Group("/application-approvers")
	aag.GET("", approvers.GetAll, requireAdmin)
	aag.GET("/available-users", approvers.GetAvailableUsers, requireAdmin)
	aag.GET("/history", approvers.GetHistory)
	aag.POST("", approvers.Create, requireAdmin)
	aag.PATCH("/:id", approvers.Update, requireAdmin)
	aag.DELETE("/:id", approvers.Delete, requireAdmin)

	// permission.audit.manage = управление системой прав (роли, группы, назначения,
	// индивидуальные права пользователей). super + admin проходят (audit.manage не
	// super-only), обычный - по гранту. auditRead - чтение журнала отказов.
	// Публичные GET-списки групп/ролей остаются открытыми любому авторизованному.
	auditRead := mw.RequirePermissionV2(permResolver, denialLog, services.KeyAuditRead)
	auditManage := mw.RequirePermissionV2(permResolver, denialLog, services.KeyAuditManage)
	// Выдача/снятие тумблера "Администратор" -- super-only (ключ action.grant.admin).
	grantAdmin := mw.RequirePermissionV2(permResolver, denialLog, services.KeyActionGrantAdmin)

	// Разрешения. Чтение/запись чужих прав (effective, override) - auditManage
	// (super + admin). Выдача super-only ключей через override не-суперу запрещена
	// в сервисе. Свои права (/my) - любому авторизованному.
	permGroup := protected.Group("/permissions")
	permGroup.GET("/my", permissions.GetMyPermissions)
	permGroup.GET("/user/:id", permissions.GetUserPermissions, auditManage)
	permGroup.GET("/user/:id/effective", permissions.GetUserEffectivePermissions, auditManage)
	permGroup.PUT("/user/:id", permissions.UpdateUserPermissions, auditManage)
	permGroup.GET("/tree", permissions.GetPermissionTree)
	permGroup.GET("/catalog", permissions.GetCatalog)
	permGroup.POST("/auto-generate", permissions.AutoGenerate)

	// Группы прав (#187a). CRUD защищён permission.audit.manage.
	pgGroup := protected.Group("/permission-groups")
	pgGroup.GET("", permGroups.List)
	pgGroup.GET("/:id", permGroups.Get)
	pgGroup.POST("", permGroups.Create, auditManage)
	pgGroup.PUT("/:id", permGroups.Update, auditManage)
	pgGroup.DELETE("/:id", permGroups.Delete, auditManage)
	pgGroup.POST("/merge", permGroups.Merge, auditManage)
	protected.GET("/users/:user_id/permission-groups", permGroups.ListForUser)
	protected.POST("/users/:user_id/permission-groups/:group_id", permGroups.AssignToUser, auditManage)
	protected.DELETE("/users/:user_id/permission-groups/:group_id", permGroups.UnassignFromUser, auditManage)
	protected.PUT("/users/:id/role", permGroups.SetUserRole, auditManage)
	protected.PUT("/users/:id/admin", permGroups.SetUserAdmin, grantAdmin)

	// Роли (#187a). CRUD защищён permission.audit.manage.
	rolesGroup := protected.Group("/roles")
	rolesGroup.GET("", roles.List)
	rolesGroup.POST("", roles.Create, auditManage)
	rolesGroup.PUT("/:id", roles.Update, auditManage)
	rolesGroup.DELETE("/:id", roles.Delete, auditManage)
	rolesGroup.PUT("/:id/default-groups", roles.SetDefaultGroups, auditManage)
	rolesGroup.PUT("/:id/permissions", roles.SetPermissions, auditManage)

	// Журнал отказов в доступе (#230).
	denialsGroup := protected.Group("/access-denials")
	denialsGroup.GET("", accessDenials.List, auditRead)
	denialsGroup.GET("/archive", accessDenials.ListArchive, auditRead)
	denialsGroup.DELETE("", accessDenials.DeleteByFilter, auditManage)
	denialsGroup.POST("/archive", accessDenials.ArchiveOlderThan, auditManage)

	// Бан пользователей (#230). Защищён action.ban.user.
	banUser := mw.RequirePermissionV2(permResolver, denialLog, services.KeyActionBanUser)
	protected.POST("/users/:id/ban", userBan.Ban, banUser)
	protected.POST("/users/:id/unban", userBan.Unban, banUser)
	protected.POST("/users/bulk/ban", userBan.BulkBan, banUser)
	protected.POST("/users/bulk/unban", userBan.BulkUnban, banUser)

	// Согласие на обработку ПД (152-ФЗ)
	consents := protected.Group("/consents")
	consents.POST("", consent.Grant)
	consents.DELETE("/:type", consent.Revoke)
	consents.GET("", consent.List)
	consents.GET("/check/:type", consent.Check)

	// Настройки системы
	protected.GET("/settings", settings.GetAll)
	protected.GET("/settings/upload", settings.GetUploadSettings)
	protected.GET("/settings/notifications", settings.GetNotificationSettings)
	protected.GET("/settings/password-policy", settings.GetPasswordPolicy)
	protected.PUT("/settings/:key", settings.Update)

	// Новости. Активные (GET "") - всем авторизованным; управление - page.admin
	// (Ф5, ранее service checkAdmin).
	ng := protected.Group("/news")
	ng.GET("", news.GetActiveNews)
	ng.GET("/all", news.GetAllNews, requireAdmin)
	ng.POST("", news.CreateNews, requireAdmin)
	ng.PUT("/:id", news.UpdateNews, requireAdmin)
	ng.DELETE("/:id", news.DeleteNews, requireAdmin)

	// Объявления. Активное (GET /active) - всем авторизованным; управление - page.admin.
	ag := protected.Group("/announcements")
	ag.GET("/active", news.GetActiveAnnouncement)
	ag.GET("/all", news.GetAllAnnouncements, requireAdmin)
	ag.POST("", news.CreateAnnouncement, requireAdmin)
	ag.POST("/set-active", news.SetActiveAnnouncement, requireAdmin)
	ag.POST("/:id/hide", news.HideAnnouncement, requireAdmin)
	ag.PUT("/:id", news.UpdateAnnouncement, requireAdmin)
	ag.DELETE("/:id", news.DeleteAnnouncement, requireAdmin)

	// Уведомления. Свои - любому авторизованному; рассылка (Create) - админ
	// (page.admin, Ф5: ранее handler-проверка type_id 5/6 manager/buropropuskov).
	notif := protected.Group("/notifications")
	notif.GET("", notifications.GetNotifications)
	notif.POST("", notifications.Create, requireAdmin)
	notif.PUT("/:id/read", notifications.MarkRead)
	notif.DELETE("/:id", notifications.Delete)
	notif.DELETE("", notifications.DeleteAll)

	// Логи запросов (мониторинг) - целиком admin-only, page.admin (Ф5, ранее service checkAdmin).
	rlg := protected.Group("/request-logs", requireAdmin)
	rlg.GET("", requestLogs.GetLogs)
	rlg.GET("/users", requestLogs.GetUsers)
	rlg.GET("/stats", requestLogs.GetStats)
	rlg.GET("/realtime", requestLogs.GetRealtime)
	rlg.GET("/timeline", requestLogs.GetTimeline)
	rlg.GET("/history", requestLogs.GetHistory)
	rlg.GET("/export", requestLogs.Export)

	// Bug-report - юзер отправляет со страницы Error500 (POST /api/bug-report)
	protected.POST("/bug-report", bugReport.Submit)

	// Админский toggle maintenance-режима (только type_id=6).
	adminMaint := protected.Group("/admin")
	adminMaint.GET("/maintenance", maintenance.GetAdminStatus)
	adminMaint.PUT("/maintenance", maintenance.ToggleMaintenance)

	// Документы (#39). Admin-операции под page.admin (requireAdmin определён выше);
	// скачивание и публичный список -- под auth.

	// Сброс онбординг-тура пользователю - админ-действие (после сброса у юзера
	// снова автозапуск). Под page.admin, в отличие от self-эндпоинтов /onboarding.
	protected.POST("/users/:username/onboarding/reset", onboarding.ResetForUser, requireAdmin)
	if docGroups != nil {
		dgGroup := protected.Group("/document-groups")
		dgGroup.GET("", docGroups.List, requireAdmin)
		dgGroup.POST("", docGroups.Create, requireAdmin)
		dgGroup.PUT("/reorder", docGroups.Reorder, requireAdmin)
		dgGroup.PUT("/:id", docGroups.Update, requireAdmin)
		dgGroup.DELETE("/:id", docGroups.Delete, requireAdmin)
	}
	if docs != nil {
		docsGroup := protected.Group("/documents")
		docsGroup.GET("", docs.List, requireAdmin)
		docsGroup.POST("", docs.Upload, requireAdmin)
		docsGroup.PUT("/reorder", docs.Reorder, requireAdmin)
		docsGroup.PUT("/:id", docs.UpdateMeta, requireAdmin)
		docsGroup.PUT("/:id/file", docs.ReplaceFile, requireAdmin)
		docsGroup.DELETE("/:id", docs.Delete, requireAdmin)
		docsGroup.GET("/:id/download", docs.Download)

		protected.GET("/public/documents", docs.GetPublic)
	}

	// Руководство (B1). Любому авторизованному; список отдаёт только разделы, на
	// которые есть право guide.<role>, скачивание гейтит ту же проверку в хендлере
	// (динамический ключ по :role не выражается статическим middleware).
	if guide != nil {
		guideGroup := protected.Group("/guide")
		guideGroup.GET("/sections", guide.ListSections)
		guideGroup.GET("/sections/:role/download", guide.Download)

		// Админ-управление (B1b): правка текста раздела + загрузка/удаление PDF. Гейт page.admin.
		guideGroup.GET("/admin/sections", guide.AdminListSections, requireAdmin)
		guideGroup.PUT("/admin/sections/:role", guide.UpdateSection, requireAdmin)
		guideGroup.PUT("/admin/sections/:role/file", guide.UploadFile, requireAdmin)
		guideGroup.DELETE("/admin/sections/:role/file", guide.DeleteFile, requireAdmin)
	}

	// Документ согласия на обработку данных. Просмотр/скачивание -- любому авторизованному
	// (виден на странице подачи заявки), управление -- под page.admin.
	if settings != nil {
		dpGroup := protected.Group("/settings/data-processing")
		dpGroup.GET("/document/meta", settings.GetDataProcessingMeta)
		dpGroup.GET("/document", settings.ServeDataProcessingDoc)
		dpGroup.POST("/document", settings.UploadDataProcessingDoc, requireAdmin)
		dpGroup.DELETE("/document", settings.DeleteDataProcessingDoc, requireAdmin)
	}

	// Статистика дашборда (#632). Доступ ограничен page.statistics.
	if statistics != nil {
		requireStats := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageStatistics)
		statsGroup := protected.Group("/statistics")
		statsGroup.GET("/summary", statistics.GetSummary, requireStats)
		statsGroup.GET("/processing-summary", statistics.GetProcessingSummary, requireStats)
		statsGroup.GET("/processing-journal", statistics.GetProcessingJournal, requireStats)
		if reminder != nil {
			statsGroup.GET("/stuck-approvals", reminder.GetStuckApprovals, requireStats)
		}
		statsGroup.GET("/timeline", statistics.GetTimeline, requireStats)
		statsGroup.GET("/online-peaks", statistics.GetOnlinePeaks, requireStats)
		statsGroup.GET("/online-users", statistics.GetOnlineUsers, requireStats)
		statsGroup.GET("/recent-passages", statistics.GetRecentPassages, requireStats)
		statsGroup.GET("/metrics", statistics.GetMetrics, requireStats)
		statsGroup.GET("/insights", statistics.GetInsights, requireStats)
		statsGroup.POST("/report", statistics.RunReport, requireStats)
		statsGroup.GET("/templates", statistics.ListTemplates, requireStats)
		statsGroup.POST("/templates", statistics.CreateTemplate, requireStats)
		statsGroup.PUT("/templates/:id", statistics.UpdateTemplate, requireStats)
		statsGroup.DELETE("/templates/:id", statistics.DeleteTemplate, requireStats)
	}
}
