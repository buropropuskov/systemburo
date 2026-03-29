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

// GetAll обрабатывает GET /companies -- список всех компаний.
func (h *CompanyHandler) GetAll(c echo.Context) error {
	companies, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, companies)
}

// GetWithUsers обрабатывает GET /companies/with-users -- компании с количеством пользователей.
func (h *CompanyHandler) GetWithUsers(c echo.Context) error {
	companies, err := h.service.GetWithUsers(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, companies)
}

// GetWithUsersExtended обрабатывает GET /companies/with-users-extended -- расширенная информация.
func (h *CompanyHandler) GetWithUsersExtended(c echo.Context) error {
	companies, err := h.service.GetWithUsersExtended(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, companies)
}

// Create обрабатывает POST /companies -- создание компании (требуются права buropropuskov).
func (h *CompanyHandler) Create(c echo.Context) error {
	username := c.Get("username").(string)

	var req services.CreateCompanyRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	company, err := h.service.Create(c.Request().Context(), username, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, company)
}

// Update обрабатывает PUT /companies/:id -- обновление компании (требуются права buropropuskov).
func (h *CompanyHandler) Update(c echo.Context) error {
	username := c.Get("username").(string)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	var req services.CreateCompanyRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	company, err := h.service.Update(c.Request().Context(), username, id, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, company)
}

// Delete обрабатывает DELETE /companies/:id -- удаление компании (требуются права buropropuskov).
func (h *CompanyHandler) Delete(c echo.Context) error {
	username := c.Get("username").(string)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	if err := h.service.Delete(c.Request().Context(), username, id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "Company deleted"})
}

// GetUsers обрабатывает GET /companies/:id/users -- ответственные пользователи компании.
func (h *CompanyHandler) GetUsers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	users, err := h.service.GetUsers(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, users)
}

// UpdateUsers обрабатывает PUT /companies/:id/users -- обновление ответственных пользователей.
func (h *CompanyHandler) UpdateUsers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	var req services.UpdateCompanyUsersRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.service.UpdateUsers(c.Request().Context(), id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "Company users updated successfully"})
}

// GetUnloadPlaces обрабатывает GET /companies/:id/unload-places -- места разгрузки компании.
func (h *CompanyHandler) GetUnloadPlaces(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	places, err := h.service.GetUnloadPlaces(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, places)
}

// UpdateUnloadPlaces обрабатывает PUT /companies/:id/unload-places -- обновление мест разгрузки (buropropuskov).
func (h *CompanyHandler) UpdateUnloadPlaces(c echo.Context) error {
	username := c.Get("username").(string)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	var req services.UpdateCompanyUnloadPlacesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.service.UpdateUnloadPlaces(c.Request().Context(), username, id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "Unload places updated successfully"})
}

// GetTables обрабатывает GET /companies/:id/tables -- таблицы компании.
func (h *CompanyHandler) GetTables(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	tables, err := h.service.GetTables(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, tables)
}

// UpdateTables обрабатывает PUT /companies/:id/tables -- обновление таблиц компании (buropropuskov).
func (h *CompanyHandler) UpdateTables(c echo.Context) error {
	username := c.Get("username").(string)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	var req services.UpdateCompanyTablesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.service.UpdateTables(c.Request().Context(), username, id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "Company tables updated successfully"})
}
