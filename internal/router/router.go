package router

import (
	"systemburo/internal/handlers"
	mw "systemburo/internal/middleware"

	"github.com/labstack/echo/v4"
)

func Setup(e *echo.Echo, auth *handlers.AuthHandler, userTypes *handlers.UserTypesHandler, attachments *handlers.AttachmentHandler, lpf *handlers.LicensePlateFormatHandler, cs *handlers.CitizenshipHandler, org *handlers.OrganizationHandler, comp *handlers.CompanyHandler, users *handlers.UsersHandler, up *handlers.UnloadPlaceHandler, cars *handlers.CarHandler, employees *handlers.EmployeeHandler, st *handlers.SystemTableHandler, uc *handlers.UniqueCarHandler, ue *handlers.UniqueEmployeeHandler, fb *handlers.FeedbackHandler, app *handlers.ApplicationHandler, approvers *handlers.ApproverHandler, permissions *handlers.PermissionHandler, consent *handlers.ConsentHandler, settings *handlers.SettingsHandler, news *handlers.NewsHandler, notifications *handlers.NotificationHandler, requestLogs *handlers.RequestLogsHandler, employeesHistory *handlers.EmployeesHistoryHandler, jwtSecret []byte) {
	// Health check — вне /api, для мониторинга и readiness-проб.
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Все API-роуты под префиксом /api — разделяет API и SPA-роуты (/news, /center
	// и т.д. в Vue router). Nginx проксирует /api/ на backend, остальное — на frontend.
	api := e.Group("/api")

	// Public routes
	api.POST("/login", auth.Login)
	api.POST("/refresh-token", auth.RefreshToken)
	api.GET("/user-types", auth.GetUserTypes)

	// Protected routes
	protected := api.Group("")
	protected.Use(mw.JWTAuth(jwtSecret))

	protected.POST("/logout", auth.Logout)
	protected.GET("/user-data", auth.GetUserData)
	protected.GET("/users/me", auth.GetCurrentUser)
	protected.GET("/users/current", auth.GetCurrentUser)

	// Шаблоны вложений (unique_attachments)
	att := protected.Group("/attachments")
	att.GET("", attachments.GetActive)
	att.GET("/all", attachments.GetAll)
	att.POST("", attachments.Create)
	att.PUT("/:id", attachments.Update)
	att.DELETE("/:id", attachments.Delete)
	att.PUT("/:id/restore", attachments.Restore)
	att.GET("/:id", attachments.GetByID)

	// Управление типами пользователей (admin-only)
	utm := protected.Group("/user-types-management")
	utm.GET("", userTypes.GetAll)
	utm.POST("", userTypes.Create)
	utm.PUT("/:id", userTypes.Update)
	utm.DELETE("/:id", userTypes.Delete)

	// Гражданства
	csg := protected.Group("/citizenships")
	csg.GET("", cs.GetAll)
	csg.POST("", cs.Create)
	csg.PUT("/:id", cs.Update)
	csg.DELETE("/:id", cs.Delete)
	csg.POST("/clear-default", cs.ClearDefaults)

	// Форматы номерных знаков
	lpfGroup := protected.Group("/license-plate-formats")
	lpfGroup.GET("", lpf.GetAll)
	lpfGroup.POST("", lpf.Create)
	lpfGroup.PUT("/:id", lpf.Update)
	lpfGroup.DELETE("/:id", lpf.Delete)

	// Организации
	orgg := protected.Group("/organizations")
	orgg.GET("", org.GetAll)
	orgg.POST("", org.Create)
	orgg.PUT("/:id", org.Update)
	orgg.DELETE("/:id", org.Delete)
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
	stg.GET("/name/:name", st.GetByName)
	stg.GET("/:id/time-slots", st.GetTimeSlots)
	stg.POST("/:id/time-slots", st.AddTimeSlot)
	stg.PUT("/:table_id/time-slots/:slot_id", st.UpdateTimeSlot)
	stg.DELETE("/:table_id/time-slots/:slot_id", st.DeleteTimeSlot)
	stg.POST("/:id/photos", st.UploadPhoto)
	stg.DELETE("/:table_id/photos/:photo_id", st.DeletePhoto)
	stg.POST("/:table_id/photos/:photo_id/main", st.SetMainPhoto)

	// Реестр автомобилей (unique_cars)
	ucg := protected.Group("/unique-cars")
	ucg.GET("", uc.GetAll)
	ucg.POST("", uc.Create)
	ucg.POST("/batch", uc.CreateBatch)
	ucg.PUT("/:id", uc.Update)
	ucg.PUT("/by-number", uc.UpdateByNumber)
	ucg.DELETE("/:id", uc.Delete)
	ucg.GET("/ownership-info", uc.GetOwnershipInfo)

	// Реестр сотрудников (unique_employees)
	ueg := protected.Group("/unique-employees")
	ueg.GET("", ue.GetAll)
	ueg.POST("", ue.Create)
	ueg.PUT("/:id", ue.Update)
	ueg.DELETE("/:id", ue.Delete)
	ueg.GET("/ownership-info", ue.GetOwnershipInfo)

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
	apg.POST("/:id/update-items-status", app.UpdateApplicationItemsStatus)
	apg.POST("/:id/forward", app.ForwardApplication)
	apg.POST("/:id/approve", app.ApproveApplicationByUser)
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
	aag.POST("", approvers.Create)
	aag.DELETE("/:id", approvers.Delete)

	// Разрешения
	permGroup := protected.Group("/permissions")
	permGroup.GET("/my", permissions.GetMyPermissions)
	permGroup.GET("/user/:id", permissions.GetUserPermissions)
	permGroup.PUT("/user/:id", permissions.UpdateUserPermissions)
	permGroup.GET("/tree", permissions.GetPermissionTree)
	permGroup.POST("/auto-generate", permissions.AutoGenerate)

	// Согласие на обработку ПД (152-ФЗ)
	consents := protected.Group("/consents")
	consents.POST("", consent.Grant)
	consents.DELETE("/:type", consent.Revoke)
	consents.GET("", consent.List)
	consents.GET("/check/:type", consent.Check)

	// Настройки системы
	protected.GET("/settings", settings.GetAll)
	protected.GET("/settings/upload", settings.GetUploadSettings)
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
}
