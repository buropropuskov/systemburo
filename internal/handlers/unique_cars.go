package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// UniqueCarHandler -- HTTP-обработчики уникальных машин.
type UniqueCarHandler struct {
	service  services.UniqueCarService
	resolver *services.PermissionResolver
}

// NewUniqueCarHandler создаёт новый экземпляр обработчика уникальных машин.
func NewUniqueCarHandler(service services.UniqueCarService, resolver *services.PermissionResolver) *UniqueCarHandler {
	return &UniqueCarHandler{service: service, resolver: resolver}
}

// GetAll godoc
// @Summary      Получение уникальных машин
// @Description  Возвращает список уникальных машин с фильтрацией по владельцу. Без per_page -
// @Description  полный массив (legacy, для ExistingCarsModal/CreateApplication). С per_page -
// @Description  пагинация + серверный поиск search_query (#1158, срез 2, для CarsView).
// @Tags         unique-cars
// @Produce      json
// @Security     BearerAuth
// @Param        filter_type  query string false "Тип фильтра: user, organization, company, all, all_system"
// @Param        search_query query string false "Поисковый запрос (номер/марка/формат/организация/компания)"
// @Param        page         query int    false "Номер страницы (с per_page)"
// @Param        per_page     query int    false "Размер страницы (<=100); наличие включает пагинацию"
// @Success      200 {array} services.UniqueCarWithRelations
// @Failure      401 {object} models.HTTPError
// @Router       /unique-cars [get]
func (h *UniqueCarHandler) GetAll(c echo.Context) error {
	username := c.Get("username").(string)
	filterType := c.QueryParam("filter_type")
	if filterType == "" {
		filterType = "user"
	}

	// Legacy mode: без per_page отдаём полный массив без поиска, как раньше -
	// ExistingCarsModal/CreateApplication дёргают этот путь без пагинации.
	if c.QueryParam("per_page") == "" {
		cars, err := h.service.GetAll(c.Request().Context(), username, filterType)
		if err != nil {
			return err
		}
		return RespondSuccess(c, cars)
	}

	var params models.PaginationParams
	if err := c.Bind(&params); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid pagination parameters")
	}
	params.Normalize()

	searchQuery := c.QueryParam("search_query")
	cars, total, err := h.service.GetAllPaginated(c.Request().Context(), username, filterType, searchQuery, params.Page, params.PerPage)
	if err != nil {
		return err
	}
	return RespondPaginated(c, cars, models.PaginationMeta{
		Total: total, Page: params.Page, PerPage: params.PerPage,
	})
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
// @Failure      403 {object} models.HTTPError "Чужая организация или компания без права"
// @Router       /unique-cars [post]
func (h *UniqueCarHandler) Create(c echo.Context) error {
	username := c.Get("username").(string)
	var req services.NewUniqueCarRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	canOverride, err := canOverrideOrganization(c, h.resolver)
	if err != nil {
		return err
	}
	car, err := h.service.Create(c.Request().Context(), username, req, canOverride)
	if err != nil {
		return err
	}
	return RespondSuccess(c, car)
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

	canOverride, err := canOverrideOrganization(c, h.resolver)
	if err != nil {
		return err
	}
	car, err := h.service.Update(c.Request().Context(), username, id, req, canOverride)
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

// GetRegistryLog godoc
// @Summary      Журнал реестра машин
// @Description  Все события реестра: создание, правка полей, удаление - с автором и
// @Description  временем. Единственный способ узнать, кем и когда удалена запись: у
// @Description  исчезнувшей строки истории по id больше нет. Доступен администратору.
// @Tags         unique-cars
// @Produce      json
// @Security     BearerAuth
// @Param        limit query int false "Сколько записей вернуть (по умолчанию и максимум 500)"
// @Success      200 {array} services.UniqueCarHistoryItem
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError "Не администратор"
// @Router       /unique-cars/history [get]
func (h *UniqueCarHandler) GetRegistryLog(c echo.Context) error {
	username := c.Get("username").(string)
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	items, err := h.service.GetRegistryLog(c.Request().Context(), username, limit)
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
