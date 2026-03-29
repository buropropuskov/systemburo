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

// GetAll godoc
// @Summary      Получить все организации
// @Description  Возвращает список всех организаций (id, name)
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} services.OrganizationInfoResponse
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /organizations [get]
func (h *OrganizationHandler) GetAll(c echo.Context) error {
	orgs, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, orgs)
}

// Create godoc
// @Summary      Создать организацию
// @Description  Создаёт новую организацию. Требует права buropropuskov
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.CreateOrganizationRequest true "Данные новой организации"
// @Success      200 {object} services.OrganizationInfoResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations [post]
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

// Update godoc
// @Summary      Обновить организацию
// @Description  Обновляет название организации по ID. Требует права buropropuskov
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Param        request body services.CreateOrganizationRequest true "Обновлённые данные организации"
// @Success      200 {object} services.OrganizationInfoResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations/{id} [put]
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

// Delete godoc
// @Summary      Удалить организацию
// @Description  Удаляет организацию по ID. Нельзя удалить если есть привязанные пользователи. Требует права buropropuskov
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Router       /organizations/{id} [delete]
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

// GetWithUsers godoc
// @Summary      Получить организации с количеством пользователей
// @Description  Возвращает список организаций с количеством привязанных пользователей
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} services.OrganizationWithUsersResponse
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /organizations/with-users [get]
func (h *OrganizationHandler) GetWithUsers(c echo.Context) error {
	orgs, err := h.service.GetWithUsers(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, orgs)
}

// GetWithUsersExtended godoc
// @Summary      Получить организации с расширенной информацией
// @Description  Возвращает организации с количеством пользователей и местами разгрузки
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} map[string]interface{}
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /organizations/with-users-extended [get]
func (h *OrganizationHandler) GetWithUsersExtended(c echo.Context) error {
	orgs, err := h.service.GetWithUsersExtended(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, orgs)
}

// GetMyOrganization godoc
// @Summary      Получить организацию текущего пользователя
// @Description  Возвращает организацию, к которой привязан текущий авторизованный пользователь
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} services.MyOrganizationResponse
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /get-organization [get]
func (h *OrganizationHandler) GetMyOrganization(c echo.Context) error {
	username := c.Get("username").(string)
	resp, err := h.service.GetMyOrganization(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}

// GetOrganizationUsers godoc
// @Summary      Получить пользователей организации
// @Description  Возвращает список ответственных пользователей организации
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Success      200 {array} services.OrganizationUserResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /organizations/{id}/users [get]
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

// UpdateOrganizationUsers godoc
// @Summary      Обновить пользователей организации
// @Description  Заменяет список ответственных пользователей организации (replace-стратегия)
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Param        request body services.UpdateOrganizationUsersRequest true "Список ответственных пользователей"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /organizations/{id}/users [put]
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

// GetOrganizationTables godoc
// @Summary      Получить таблицы организации
// @Description  Возвращает список таблиц, привязанных к организации
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Success      200 {array} services.OrganizationTableResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /organizations/{id}/tables [get]
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

// UpdateOrganizationTables godoc
// @Summary      Обновить таблицы организации
// @Description  Заменяет привязку таблиц к организации. Требует права buropropuskov
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Param        request body services.UpdateOrganizationTablesRequest true "Список ID таблиц"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations/{id}/tables [put]
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

// GetOrganizationUnloadPlaces godoc
// @Summary      Получить места разгрузки организации
// @Description  Возвращает список мест разгрузки, привязанных к организации
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Success      200 {array} services.OrganizationUnloadPlaceResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /organizations/{id}/unload-places [get]
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

// UpdateOrganizationUnloadPlaces godoc
// @Summary      Обновить места разгрузки организации
// @Description  Заменяет привязку мест разгрузки к организации. Требует права buropropuskov
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Param        request body services.UpdateOrganizationUnloadPlacesRequest true "Список ID мест разгрузки"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations/{id}/unload-places [put]
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
