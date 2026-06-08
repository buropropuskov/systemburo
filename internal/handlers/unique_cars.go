package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// UniqueCarHandler -- HTTP-обработчики уникальных машин.
type UniqueCarHandler struct {
	service services.UniqueCarService
}

// NewUniqueCarHandler создаёт новый экземпляр обработчика уникальных машин.
func NewUniqueCarHandler(service services.UniqueCarService) *UniqueCarHandler {
	return &UniqueCarHandler{service: service}
}

// GetAll godoc
// @Summary      Получение уникальных машин
// @Description  Возвращает список уникальных машин с фильтрацией по владельцу
// @Tags         unique-cars
// @Produce      json
// @Security     BearerAuth
// @Param        filter_type query string false "Тип фильтра: user, organization, company, all, all_system"
// @Success      200 {array} services.UniqueCarWithRelations
// @Failure      401 {object} models.HTTPError
// @Router       /unique-cars [get]
func (h *UniqueCarHandler) GetAll(c echo.Context) error {
	username := c.Get("username").(string)
	filterType := c.QueryParam("filter_type")
	if filterType == "" {
		filterType = "user"
	}

	cars, err := h.service.GetAll(c.Request().Context(), username, filterType)
	if err != nil {
		return err
	}
	return RespondSuccess(c, cars)
}

// Create godoc
// @Summary      Создание уникальной машины
// @Description  Создаёт новую уникальную машину с проверкой уникальности
// @Tags         unique-cars
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.NewUniqueCarRequest true "Данные машины"
// @Success      200 {object} services.UniqueCarResponse
// @Failure      400 {object} models.HTTPError "Дубликат"
// @Failure      401 {object} models.HTTPError
// @Router       /unique-cars [post]
func (h *UniqueCarHandler) Create(c echo.Context) error {
	username := c.Get("username").(string)
	var req services.NewUniqueCarRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	car, err := h.service.Create(c.Request().Context(), username, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, car)
}

// CreateBatch godoc
// @Summary      Пакетное создание уникальных машин
// @Description  Создаёт несколько уникальных машин, пропуская дубликаты
// @Tags         unique-cars
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body []services.NewUniqueCarRequest true "Массив данных машин"
// @Success      200 {object} services.BatchCreateCarsResponse
// @Success      207 {object} services.BatchCreateCarsResponse "Частичный успех"
// @Failure      401 {object} models.HTTPError
// @Router       /unique-cars/batch [post]
func (h *UniqueCarHandler) CreateBatch(c echo.Context) error {
	username := c.Get("username").(string)
	var reqs []services.NewUniqueCarRequest
	if err := c.Bind(&reqs); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	resp, httpStatus, err := h.service.CreateBatch(c.Request().Context(), username, reqs)
	if err != nil {
		return err
	}
	return c.JSON(httpStatus, Response{Success: true, Data: resp})
}

// Update godoc
// @Summary      Обновление уникальной машины
// @Description  Обновляет данные уникальной машины по ID
// @Tags         unique-cars
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID машины"
// @Param        request body services.NewUniqueCarRequest true "Данные машины"
// @Success      200 {object} services.UniqueCarResponse
// @Failure      400 {object} models.HTTPError "Дубликат"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError "Нет прав"
// @Failure      404 {object} models.HTTPError "Не найдена"
// @Router       /unique-cars/{id} [put]
func (h *UniqueCarHandler) Update(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	var req services.NewUniqueCarRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	car, err := h.service.Update(c.Request().Context(), username, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, car)
}

// UpdateByNumber godoc
// @Summary      Обновление уникальной машины по номеру
// @Description  Находит машину по номеру и марке, обновляет данные
// @Tags         unique-cars
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.UpdateCarByNumberRequest true "Номер, марка и новые данные"
// @Success      200 {object} services.UniqueCarResponse
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError "Нет прав"
// @Failure      404 {object} models.HTTPError "Не найдена"
// @Router       /unique-cars/by-number [put]
func (h *UniqueCarHandler) UpdateByNumber(c echo.Context) error {
	username := c.Get("username").(string)
	var req services.UpdateCarByNumberRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	car, err := h.service.UpdateByNumber(c.Request().Context(), username, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, car)
}

// Delete godoc
// @Summary      Удаление уникальной машины
// @Description  Удаляет уникальную машину по ID с проверкой прав
// @Tags         unique-cars
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID машины"
// @Success      200 {object} map[string]string "message: Car deleted successfully"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError "Нет прав"
// @Failure      404 {object} models.HTTPError "Не найдена"
// @Router       /unique-cars/{id} [delete]
func (h *UniqueCarHandler) Delete(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	if err := h.service.Delete(c.Request().Context(), username, id); err != nil {
		return err
	}
	return RespondMessage(c, "Car deleted successfully")
}

// GetHistory godoc
// @Summary      История изменений мастер-машины
// @Description  Возвращает аудит изменений мастер-записи машины (data_changed)
// @Tags         unique-cars
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID машины"
// @Success      200 {array} services.UniqueCarHistoryItem
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError "Нет прав"
// @Failure      404 {object} models.HTTPError "Не найдена"
// @Router       /unique-cars/{id}/history [get]
func (h *UniqueCarHandler) GetHistory(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	items, err := h.service.GetHistory(c.Request().Context(), username, id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// Lookup godoc
// @Summary      Найти машину по номеру и марке
// @Description  Поиск машины (LOWER/TRIM) для открытия карточки со страницы ЧС. 404 если нет.
// @Tags         unique-cars
// @Produce      json
// @Security     BearerAuth
// @Param        number query string true "Номер машины"
// @Param        mark query string false "Марка"
// @Success      200 {object} services.UniqueCarWithRelations
// @Failure      404 {object} models.HTTPError
// @Router       /unique-cars/lookup [get]
func (h *UniqueCarHandler) Lookup(c echo.Context) error {
	number := c.QueryParam("number")
	if number == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "number обязателен")
	}
	car, err := h.service.LookupByNumberMark(c.Request().Context(), number, c.QueryParam("mark"))
	if err != nil {
		return err
	}
	if car == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Машина не найдена")
	}
	return RespondSuccess(c, car)
}

// GetOwnershipInfo godoc
// @Summary      Информация о владельце для машин
// @Description  Возвращает данные о привязке пользователя к организации/компании
// @Tags         unique-cars
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} services.CarOwnerInfo
// @Failure      401 {object} models.HTTPError
// @Router       /unique-cars/ownership-info [get]
func (h *UniqueCarHandler) GetOwnershipInfo(c echo.Context) error {
	username := c.Get("username").(string)
	info, err := h.service.GetOwnerInfo(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, info)
}
