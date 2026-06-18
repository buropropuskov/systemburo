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
	Statistics          *handlers.StatisticsHandler

	// Services (для middleware и audit)
	PermResolver *services.PermissionResolver
	DenialLog    *services.AccessDenialService

	// Middleware - все опциональны (nil в тестах допустим)
	MaintenanceBlock echo.MiddlewareFunc
	BanCheck         echo.MiddlewareFunc
	LoginLimiter     echo.MiddlewareFunc

	// Misc
	JWTSecret []byte
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
	statistics := d.Statistics
	permResolver := d.PermResolver
	denialLog := d.DenialLog
	maintenanceBlock := d.MaintenanceBlock
	banCheck := d.BanCheck
	loginLimiter := d.LoginLimiter
	jwtSecret := d.JWTSecret
	// Health check — вне /api, для мониторинга и readiness-проб.
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Все API-роуты под префиксом /api — разделяет API и SPA-роуты (/news, /center
	// и т.д. в Vue router). Nginx проксирует /api/ на backend, остальное — на frontend.
	api := e.Group("/api")

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

	protected.POST("/logout", auth.Logout)
	protected.POST("/logout-all", auth.LogoutAll)
	protected.GET("/user-data", auth.GetUserData)
	protected.GET("/users/me", auth.GetCurrentUser)
	protected.GET("/users/current", auth.GetCurrentUser)

	// Онбординг-тур (#657) - self-service статус: любой авторизованный читает и
	// помечает прохождение ДЛЯ СЕБЯ (userID из JWT). Не admin-only.
	protected.GET("/onboarding", onboarding.GetStatus)
	protected.POST("/onboarding/complete", onboarding.MarkComplete)

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

	// Управление типами пользователей (admin-only)
	utm := protected.Group("/user-types-management")
	utm.GET("", userTypes.GetAll)
	utm.POST("", userTypes.Create)
	utm.PUT("/:id", userTypes.Update)
	utm.DELETE("/:id", userTypes.Delete)
	utm.GET("/:id/history", userTypes.GetHistory)

	// Гражданства
	csg := protected.Group("/citizenships")
	csg.GET("", cs.GetAll)
	csg.POST("", cs.Create)
	csg.PUT("/:id", cs.Update)
	csg.DELETE("/:id", cs.Delete)
	csg.POST("/:id/restore", cs.Restore)
	csg.GET("/:id/history", cs.GetHistory)
	csg.POST("/clear-default", cs.ClearDefaults)

	// Форматы номерных знаков
	lpfGroup := protected.Group("/license-plate-formats")
	lpfGroup.GET("", lpf.GetAll)
	lpfGroup.POST("", lpf.Create)
	lpfGroup.PUT("/:id", lpf.Update)
	lpfGroup.DELETE("/:id", lpf.Delete)
	lpfGroup.POST("/:id/restore", lpf.Restore)
	lpfGroup.GET("/:id/history", lpf.GetHistory)

	// Марки автомобилей (#185) - справочник с историчностью.
	marksGroup := protected.Group("/marks")
	marksGroup.GET("", marks.GetAll)
	marksGroup.POST("", marks.Create)
	marksGroup.PUT("/:id", marks.Update)
	marksGroup.POST("/:id/archive", marks.Archive)
	marksGroup.POST("/:id/restore", marks.Restore)
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

	// Организации
	orgg := protected.Group("/organizations")
	orgg.GET("", org.GetAll)
	orgg.POST("", org.Create)
	orgg.PUT("/:id", org.Update)
	orgg.DELETE("/:id", org.Delete)
	orgg.POST("/:id/restore", org.Restore)
	orgg.GET("/:id/history", org.GetHistory)
	orgg.GET("/with-users", org.GetWithUsers)
	orgg.GET("/with-users-extended", org.GetWithUsersExtended)
	orgg.GET("/:id/users", org.GetOrganizationUsers)
	orgg.PUT("/:id/users", org.UpdateOrganizationUsers)
	orgg.GET("/:id/tables", org.GetOrganizationTables)
	orgg.PUT("/:id/tables", org.UpdateOrganizationTables)
	orgg.GET("/:id/unload-places", org.GetOrganizationUnloadPlaces)
	orgg.PUT("/:id/unload-places", org.UpdateOrganizationUnloadPlaces)
	protected.GET("/get-organization", org.GetMyOrganization)

	// Компании
	cg := protected.Group("/companies")
	cg.GET("", comp.GetAll)
	cg.POST("", comp.Create)
	cg.PUT("/:id", comp.Update)
	cg.DELETE("/:id", comp.Delete)
	cg.POST("/:id/restore", comp.Restore)
	cg.GET("/:id/history", comp.GetHistory)
	cg.GET("/with-users", comp.GetWithUsers)
	cg.GET("/with-users-extended", comp.GetWithUsersExtended)
	cg.GET("/:id/users", comp.GetUsers)
	cg.PUT("/:id/users", comp.UpdateUsers)
	cg.GET("/:id/tables", comp.GetTables)
	cg.PUT("/:id/tables", comp.UpdateTables)
	cg.GET("/:id/unload-places", comp.GetUnloadPlaces)
	cg.PUT("/:id/unload-places", comp.UpdateUnloadPlaces)

	// Места разгрузки
	upg := protected.Group("/unload-places")
	upg.GET("", up.GetAll)
	upg.POST("", up.Create)
	upg.GET("/:id", up.GetByID)
	upg.PUT("/:id", up.Update)
	upg.DELETE("/:id", up.Delete)
	upg.POST("/:id/restore", up.Restore)
	upg.GET("/:id/history", up.GetHistory)
	upg.GET("/:id/time-slots", up.GetTimeSlots)
	upg.POST("/:id/time-slots", up.AddTimeSlot)
	upg.PUT("/:place_id/time-slots/:slot_id", up.UpdateTimeSlot)
	upg.DELETE("/:place_id/time-slots/:slot_id", up.DeleteTimeSlot)
	upg.POST("/:id/photos", up.UploadPhoto)
	upg.DELETE("/:place_id/photos/:photo_id", up.DeletePhoto)
	upg.POST("/:place_id/photos/:photo_id/main", up.SetMainPhoto)

	// Управление пользователями (admin-only)
	protected.POST("/users", users.Create)
	protected.GET("/users/all", users.GetAll)
	protected.PUT("/users/:username/type", users.UpdateType)
	protected.PUT("/users/:username/password", users.UpdatePassword)
	protected.PUT("/users/:username/info", users.UpdateInfo)
	protected.PUT("/users/:username/organization", users.UpdateOrganization)
	protected.PUT("/users/:username/company", users.UpdateCompany)
	protected.DELETE("/users/:username", users.Delete)
	protected.POST("/users/:username/restore", users.Restore)
	protected.GET("/users/:username/history", users.GetHistory)

	// Машины (в заявках)
	carsGroup := protected.Group("/cars")
	carsGroup.GET("/active-for-tables", cars.GetActiveCarsForTables)
	carsGroup.GET("/fact-for-tables", cars.GetFactCarsForTables)
	carsGroup.GET("/unload-places", cars.GetCarUnloadPlaces)
	carsGroup.GET("/fact-unload-places", cars.GetFactCarUnloadPlaces)
	carsGroup.GET("/check-active", cars.CheckActiveCar)
	carsGroup.GET("/:id/history", cars.GetCarHistory)
	carsGroup.POST("/:id/history", cars.AddCarHistoryEntry)
	carsGroup.GET("/history/all", cars.GetAllCarsHistory)
	carsGroup.GET("/history/current-status", cars.GetCarsCurrentStatus)
	carsGroup.PUT("/:id/territory-status", cars.UpdateCarTerritoryStatus)
	carsGroup.PUT("/:id/deactivate", cars.DeactivateCar)
	carsGroup.PUT("/:id/activate", cars.ActivateCar)
	carsGroup.GET("/history/unified", cars.GetUnifiedCarHistory)
	carsGroup.PUT("/:id/restore", cars.RestoreCar)

	// Сотрудники (в заявках)
	empGroup := protected.Group("/employees")
	empGroup.POST("", employees.CreateEmployee)
	empGroup.GET("/active-for-table/:table_id", employees.GetActiveEmployeesForTable)
	empGroup.PUT("/:id/territory-status", employees.UpdateEmployeeTerritoryStatus)
	empGroup.PUT("/:id/deactivate", employees.DeactivateEmployee)
	empGroup.PUT("/:id/activate", employees.ActivateEmployee)
	empGroup.PUT("/:id/restore", employees.RestoreEmployee)
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
	stg.GET("/:id/history", st.GetHistory)
	stg.GET("/name/:name", st.GetByName)
	stg.GET("/:id/time-slots", st.GetTimeSlots)
	stg.POST("/:id/time-slots", st.AddTimeSlot)
	stg.PUT("/:table_id/time-slots/:slot_id", st.UpdateTimeSlot)
	stg.DELETE("/:table_id/time-slots/:slot_id", st.DeleteTimeSlot)
	stg.POST("/:id/photos", st.UploadPhoto)
	stg.DELETE("/:table_id/photos/:photo_id", st.DeletePhoto)
	stg.POST("/:table_id/photos/:photo_id/main", st.SetMainPhoto)

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

	// Обратная связь
	fbg := protected.Group("/feedback")
	fbg.POST("", fb.Create)
	fbg.GET("/all", fb.GetAll)
	fbg.GET("/stats", fb.GetStats)
	fbg.GET("/my", fb.GetMy)
	fbg.PUT("/:id/status", fb.UpdateStatus)
	fbg.PUT("/:id/read", fb.MarkAsRead)

	// Заявки
	apg := protected.Group("/applications")
	apg.GET("", app.GetApplications)
	apg.POST("", app.CreateApplication)
	apg.POST("/submit-complete-application", app.SubmitCompleteApplication)
	apg.GET("/user", app.GetUserApplications)
	apg.GET("/unread-count", app.GetUnreadCount)
	apg.GET("/:id", app.GetApplicationByID)
	apg.PUT("/:id", app.UpdateApplication)
	apg.GET("/:id/responsible-users", app.GetApplicationResponsibleUsers)
	apg.GET("/:id/details", app.GetApplicationDetails)
	apg.GET("/:id/attachments", app.GetApplicationAttachments)
	apg.GET("/:id/blank", attachmentBlanks.Download) // #183 - скачать заполненный .xlsx
	apg.POST("/:id/update-items-status", app.UpdateApplicationItemsStatus)
	apg.POST("/:id/forward", app.ForwardApplication)
	apg.POST("/:id/approve", app.ApproveApplicationByUser)
	apg.POST("/:id/blacklist-overrides", app.OverrideBlacklistFlag)     // #481 - "всё равно пропустить"
	apg.DELETE("/:id/blacklist-overrides", app.DeleteBlacklistOverride) // #481 - отмена подтверждения (срез C)
	apg.GET("/:id/check-approval-status", app.CheckApprovalStatus)
	apg.POST("/:id/take-to-work", app.TakeApplicationToWork)
	apg.POST("/:id/revoke-from-work", app.RevokeApplicationFromWork)
	apg.POST("/:id/restore-to-work", app.RestoreApplicationToWork)
	apg.GET("/:id/history", app.GetApplicationHistory)
	apg.POST("/:id/revoke-approval", app.RevokeApproval)
	apg.POST("/history", app.AddHistoryEntry)
	apg.GET("/:id/viewers", app.GetApplicationViewers)
	apg.POST("/:id/read", app.MarkAsRead)
	apg.GET("/:id/reads", app.GetReads)

	// Вложения заявок (cars/employees/items внутри вложений)
	att.GET("/:id/cars", app.GetAttachmentCars)
	att.GET("/:id/employees", app.GetAttachmentEmployees)
	att.GET("/:id/items", app.GetAttachmentItems)

	// Утверждающие заявок
	aag := protected.Group("/application-approvers")
	aag.GET("", approvers.GetAll)
	aag.GET("/available-users", approvers.GetAvailableUsers)
	aag.GET("/history", approvers.GetHistory)
	aag.POST("", approvers.Create)
	aag.DELETE("/:id", approvers.Delete)

	// Разрешения
	permGroup := protected.Group("/permissions")
	permGroup.GET("/my", permissions.GetMyPermissions)
	permGroup.GET("/user/:id", permissions.GetUserPermissions)
	permGroup.PUT("/user/:id", permissions.UpdateUserPermissions)
	permGroup.GET("/tree", permissions.GetPermissionTree)
	permGroup.POST("/auto-generate", permissions.AutoGenerate)

	// permission.audit.manage = управление системой прав
	// (роли, группы, назначения, журнал отказов).
	// GET-эндпоинты остаются открытыми для любых авторизованных, т.к. они
	// нужны UI UserPermissionsModal для отображения списков (не разглашают
	// ничего секретного - только публичные метаданные ролей/групп).
	auditRead := mw.RequirePermissionV2(permResolver, denialLog, services.KeyAuditRead)
	auditManage := mw.RequirePermissionV2(permResolver, denialLog, services.KeyAuditManage)

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

	// Роли (#187a). CRUD защищён permission.audit.manage.
	rolesGroup := protected.Group("/roles")
	rolesGroup.GET("", roles.List)
	rolesGroup.POST("", roles.Create, auditManage)
	rolesGroup.PUT("/:id", roles.Update, auditManage)
	rolesGroup.DELETE("/:id", roles.Delete, auditManage)
	rolesGroup.PUT("/:id/default-groups", roles.SetDefaultGroups, auditManage)

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
	protected.PUT("/settings/:key", settings.Update)

	// Новости
	ng := protected.Group("/news")
	ng.GET("", news.GetActiveNews)
	ng.GET("/all", news.GetAllNews)
	ng.POST("", news.CreateNews)
	ng.PUT("/:id", news.UpdateNews)
	ng.DELETE("/:id", news.DeleteNews)

	// Объявления
	ag := protected.Group("/announcements")
	ag.GET("/active", news.GetActiveAnnouncement)
	ag.GET("/all", news.GetAllAnnouncements)
	ag.POST("", news.CreateAnnouncement)
	ag.POST("/set-active", news.SetActiveAnnouncement)
	ag.POST("/:id/hide", news.HideAnnouncement)
	ag.PUT("/:id", news.UpdateAnnouncement)
	ag.DELETE("/:id", news.DeleteAnnouncement)

	// Уведомления
	notif := protected.Group("/notifications")
	notif.GET("", notifications.GetNotifications)
	notif.POST("", notifications.Create)
	notif.PUT("/:id/read", notifications.MarkRead)
	notif.DELETE("/:id", notifications.Delete)
	notif.DELETE("", notifications.DeleteAll)

	// Логи запросов
	rlg := protected.Group("/request-logs")
	rlg.GET("", requestLogs.GetLogs)
	rlg.GET("/users", requestLogs.GetUsers)
	rlg.GET("/stats", requestLogs.GetStats)
	rlg.GET("/realtime", requestLogs.GetRealtime)
	rlg.GET("/timeline", requestLogs.GetTimeline)
	rlg.GET("/export", requestLogs.Export)

	// Bug-report - юзер отправляет со страницы Error500 (POST /api/bug-report)
	protected.POST("/bug-report", bugReport.Submit)

	// Админский toggle maintenance-режима (только type_id=6).
	adminMaint := protected.Group("/admin")
	adminMaint.GET("/maintenance", maintenance.GetAdminStatus)
	adminMaint.PUT("/maintenance", maintenance.ToggleMaintenance)

	// Документы (#39). Admin-операции под page.admin; скачивание и публичный список -- под auth.
	requireAdmin := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageAdmin)

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

	// Статистика дашборда (#632). Доступ ограничен page.statistics.
	if statistics != nil {
		requireStats := mw.RequirePermissionV2(permResolver, denialLog, services.KeyPageStatistics)
		statsGroup := protected.Group("/statistics")
		statsGroup.GET("/summary", statistics.GetSummary, requireStats)
		statsGroup.GET("/timeline", statistics.GetTimeline, requireStats)
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
