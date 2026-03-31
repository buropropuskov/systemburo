package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// CompanyHandler HTTP-обработчики для работы с компаниями.
type CompanyHandler struct {
	service services.CompanyService
}

// NewCompanyHandler создаёт экземпляр обработчика компаний.
func NewCompanyHandler(service services.CompanyService) *CompanyHandler {
	return &CompanyHandler{service: service}
}

// GetAll godoc
// @Summary      Получить все компании
// @Description  Возвращает список всех компаний, отсортированных по имени
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.Company
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /companies [get]
func (h *CompanyHandler) GetAll(c echo.Context) error {
	companies, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, companies)
}

// GetWithUsers godoc
// @Summary      Получить компании с количеством пользователей
// @Description  Возвращает список компаний с количеством привязанных пользователей
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} services.CompanyWithUsersResponse
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /companies/with-users [get]
func (h *CompanyHandler) GetWithUsers(c echo.Context) error {
	companies, err := h.service.GetWithUsers(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, companies)
}

// GetWithUsersExtended godoc
// @Summary      Получить компании с расширенной информацией
// @Description  Возвращает компании с количеством пользователей и местами разгрузки
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} services.CompanyWithUsersExtendedResponse
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /companies/with-users-extended [get]
func (h *CompanyHandler) GetWithUsersExtended(c echo.Context) error {
	companies, err := h.service.GetWithUsersExtended(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, companies)
}

// Create godoc
// @Summary      Создать компанию
// @Description  Создаёт новую компанию. Требуются права buropropuskov
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.CreateCompanyRequest true "Данные новой компании"
// @Success      200 {object} models.Company
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies [post]
func (h *CompanyHandler) Create(c echo.Context) error {
	username := c.Get("username").(string)

	var req services.CreateCompanyRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	company, err := h.service.Create(c.Request().Context(), username, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, company)
}

// Update godoc
// @Summary      Обновить компанию
// @Description  Обновляет название компании по ID. Требуются права buropropuskov
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Param        request body services.CreateCompanyRequest true "Обновлённые данные компании"
// @Success      200 {object} models.Company
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/{id} [put]
func (h *CompanyHandler) Update(c echo.Context) error {
	username := c.Get("username").(string)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	var req services.CreateCompanyRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	company, err := h.service.Update(c.Request().Context(), username, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, company)
}

// Delete godoc
// @Summary      Удалить компанию
// @Description  Удаляет компанию по ID. Нельзя удалить если есть привязанные пользователи. Требуются права buropropuskov
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Router       /companies/{id} [delete]
func (h *CompanyHandler) Delete(c echo.Context) error {
	username := c.Get("username").(string)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	if err := h.service.Delete(c.Request().Context(), username, id); err != nil {
		return err
	}
	return RespondMessage(c, "Company deleted")
}

// GetUsers godoc
// @Summary      Получить пользователей компании
// @Description  Возвращает список ответственных пользователей компании
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Success      200 {array} services.CompanyUserResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /companies/{id}/users [get]
func (h *CompanyHandler) GetUsers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	users, err := h.service.GetUsers(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, users)
}

// UpdateUsers godoc
// @Summary      Обновить пользователей компании
// @Description  Заменяет список ответственных пользователей компании с поддержкой обязательного согласования
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Param        request body services.UpdateCompanyUsersRequest true "Список ответственных пользователей"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /companies/{id}/users [put]
func (h *CompanyHandler) UpdateUsers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	var req services.UpdateCompanyUsersRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.UpdateUsers(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Company users updated successfully")
}

// GetUnloadPlaces godoc
// @Summary      Получить места разгрузки компании
// @Description  Возвращает список активных мест разгрузки, привязанных к компании
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Success      200 {array} services.CompanyUnloadPlaceResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /companies/{id}/unload-places [get]
func (h *CompanyHandler) GetUnloadPlaces(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	places, err := h.service.GetUnloadPlaces(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, places)
}

// UpdateUnloadPlaces godoc
// @Summary      Обновить места разгрузки компании
// @Description  Обновляет привязку мест разгрузки к компании. Требуются права buropropuskov
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Param        request body services.UpdateCompanyUnloadPlacesRequest true "Список ID мест разгрузки"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/{id}/unload-places [put]
func (h *CompanyHandler) UpdateUnloadPlaces(c echo.Context) error {
	username := c.Get("username").(string)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	var req services.UpdateCompanyUnloadPlacesRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.UpdateUnloadPlaces(c.Request().Context(), username, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Unload places updated successfully")
}

// GetTables godoc
// @Summary      Получить таблицы компании
// @Description  Возвращает список активных таблиц, привязанных к компании
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Success      200 {array} services.CompanyTableResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /companies/{id}/tables [get]
func (h *CompanyHandler) GetTables(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	tables, err := h.service.GetTables(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, tables)
}

// UpdateTables godoc
// @Summary      Обновить таблицы компании
// @Description  Обновляет привязку таблиц к компании. Требуются права buropropuskov
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Param        request body services.UpdateCompanyTablesRequest true "Список ID таблиц"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/{id}/tables [put]
func (h *CompanyHandler) UpdateTables(c echo.Context) error {
	username := c.Get("username").(string)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	var req services.UpdateCompanyTablesRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.UpdateTables(c.Request().Context(), username, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Company tables updated successfully")
}
