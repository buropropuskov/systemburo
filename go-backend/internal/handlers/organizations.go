package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// OrganizationHandler содержит HTTP-обработчики для управления организациями.
type OrganizationHandler struct {
	service services.OrganizationService
	db      *gorm.DB
}

// NewOrganizationHandler создаёт новый экземпляр обработчика организаций.
func NewOrganizationHandler(service services.OrganizationService, db *gorm.DB) *OrganizationHandler {
	return &OrganizationHandler{service: service, db: db}
}

// GetAll обрабатывает GET /organizations.
// Возвращает список всех организаций (id, name).
func (h *OrganizationHandler) GetAll(c echo.Context) error {
	orgs, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, orgs)
}

// Create обрабатывает POST /organizations.
// Создаёт новую организацию. Требует права buropropuskov.
func (h *OrganizationHandler) Create(c echo.Context) error {
	username := c.Get("username").(string)
	if err := services.CheckAdminPermissions(h.db, c.Request().Context(), username); err != nil {
		return err
	}

	var req services.CreateOrganizationRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	org, err := h.service.Create(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, org)
}

// Update обрабатывает PUT /organizations/:id.
// Обновляет название организации. Требует права buropropuskov.
func (h *OrganizationHandler) Update(c echo.Context) error {
	username := c.Get("username").(string)
	if err := services.CheckAdminPermissions(h.db, c.Request().Context(), username); err != nil {
		return err
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	var req services.CreateOrganizationRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	org, err := h.service.Update(c.Request().Context(), id, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, org)
}

// Delete обрабатывает DELETE /organizations/:id.
// Удаляет организацию. Нельзя удалить если есть привязанные пользователи.
func (h *OrganizationHandler) Delete(c echo.Context) error {
	username := c.Get("username").(string)
	if err := services.CheckAdminPermissions(h.db, c.Request().Context(), username); err != nil {
		return err
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "Organization deleted"})
}

// GetWithUsers обрабатывает GET /organizations/with-users.
// Возвращает организации с количеством привязанных пользователей.
func (h *OrganizationHandler) GetWithUsers(c echo.Context) error {
	orgs, err := h.service.GetWithUsers(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, orgs)
}

// GetWithUsersExtended обрабатывает GET /organizations/with-users-extended.
// Возвращает организации с количеством пользователей и местами разгрузки.
func (h *OrganizationHandler) GetWithUsersExtended(c echo.Context) error {
	orgs, err := h.service.GetWithUsersExtended(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, orgs)
}

// GetMyOrganization обрабатывает GET /get-organization.
// Возвращает организацию текущего пользователя.
func (h *OrganizationHandler) GetMyOrganization(c echo.Context) error {
	username := c.Get("username").(string)
	resp, err := h.service.GetMyOrganization(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}

// GetOrganizationUsers обрабатывает GET /organizations/:id/users.
// Возвращает ответственных пользователей организации.
func (h *OrganizationHandler) GetOrganizationUsers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	users, err := h.service.GetOrganizationUsers(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, users)
}

// UpdateOrganizationUsers обрабатывает PUT /organizations/:id/users.
// Заменяет список ответственных пользователей организации.
func (h *OrganizationHandler) UpdateOrganizationUsers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	var req services.UpdateOrganizationUsersRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.service.UpdateOrganizationUsers(c.Request().Context(), id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "Organization users updated successfully"})
}

// GetOrganizationTables обрабатывает GET /organizations/:id/tables.
// Возвращает таблицы, привязанные к организации.
func (h *OrganizationHandler) GetOrganizationTables(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	tables, err := h.service.GetOrganizationTables(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, tables)
}

// UpdateOrganizationTables обрабатывает PUT /organizations/:id/tables.
// Заменяет привязку таблиц к организации. Требует права buropropuskov.
func (h *OrganizationHandler) UpdateOrganizationTables(c echo.Context) error {
	username := c.Get("username").(string)
	if err := services.CheckAdminPermissions(h.db, c.Request().Context(), username); err != nil {
		return err
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	var req services.UpdateOrganizationTablesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.service.UpdateOrganizationTables(c.Request().Context(), id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "Organization tables updated successfully"})
}

// GetOrganizationUnloadPlaces обрабатывает GET /organizations/:id/unload-places.
// Возвращает места разгрузки организации.
func (h *OrganizationHandler) GetOrganizationUnloadPlaces(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	places, err := h.service.GetOrganizationUnloadPlaces(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, places)
}

// UpdateOrganizationUnloadPlaces обрабатывает PUT /organizations/:id/unload-places.
// Заменяет привязку мест разгрузки к организации. Требует права buropropuskov.
func (h *OrganizationHandler) UpdateOrganizationUnloadPlaces(c echo.Context) error {
	username := c.Get("username").(string)
	if err := services.CheckAdminPermissions(h.db, c.Request().Context(), username); err != nil {
		return err
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	var req services.UpdateOrganizationUnloadPlacesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.service.UpdateOrganizationUnloadPlaces(c.Request().Context(), id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "Unload places updated successfully"})
}
