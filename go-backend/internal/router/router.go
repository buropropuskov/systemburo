package router

import (
	"systemburo/internal/handlers"
	mw "systemburo/internal/middleware"

	"github.com/labstack/echo/v4"
)

func Setup(e *echo.Echo, auth *handlers.AuthHandler, userTypes *handlers.UserTypesHandler, attachments *handlers.AttachmentHandler, lpf *handlers.LicensePlateFormatHandler, cs *handlers.CitizenshipHandler, org *handlers.OrganizationHandler, comp *handlers.CompanyHandler, jwtSecret []byte) {
	// Public routes
	e.POST("/register", auth.Register)
	e.POST("/login", auth.Login)
	e.POST("/refresh-token", auth.RefreshToken)
	e.GET("/user-types", auth.GetUserTypes)

	// Protected routes
	protected := e.Group("")
	protected.Use(mw.JWTAuth(jwtSecret))

	protected.POST("/logout", auth.Logout)
	protected.GET("/user-data", auth.GetUserData)
	protected.GET("/users/me", auth.GetCurrentUser)

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
}
