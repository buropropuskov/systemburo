package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// PersonBlacklistHandler - HTTP-обработчики чёрного списка людей (#443).
type PersonBlacklistHandler struct {
	service services.PersonBlacklistService
}

// NewPersonBlacklistHandler создаёт обработчик.
func NewPersonBlacklistHandler(service services.PersonBlacklistService) *PersonBlacklistHandler {
	return &PersonBlacklistHandler{service: service}
}

// GetAll godoc
// @Summary      Список чёрного списка людей
// @Tags         person-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        include_archived query bool false "Включать снятые записи"
// @Success      200 {array} models.PersonBlacklist
// @Router       /person-blacklist [get]
func (h *PersonBlacklistHandler) GetAll(c echo.Context) error {
	includeArchived := c.QueryParam("include_archived") == "true"
	items, err := h.service.GetAll(c.Request().Context(), includeArchived)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// Create godoc
// @Summary      Добавить человека в чёрный список
// @Tags         person-blacklist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreatePersonBlacklistRequest true "Данные записи"
// @Success      201 {object} models.PersonBlacklist
// @Router       /person-blacklist [post]
func (h *PersonBlacklistHandler) Create(c echo.Context) error {
	var req models.CreatePersonBlacklistRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	userID, _ := c.Get("user_id").(int)
	entry, err := h.service.Create(c.Request().Context(), req, userID)
	if err != nil {
		return err
	}
	return RespondCreated(c, entry)
}

// Update godoc
// @Summary      Редактировать запись чёрного списка (ФИО, причина)
// @Tags         person-blacklist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID записи"
// @Param        request body models.UpdatePersonBlacklistRequest true "ФИО, причина"
// @Success      200 {object} models.PersonBlacklist
// @Router       /person-blacklist/{id} [put]
func (h *PersonBlacklistHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.UpdatePersonBlacklistRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	userID, _ := c.Get("user_id").(int)
	entry, err := h.service.Update(c.Request().Context(), id, req, userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, entry)
}

// Delete godoc
// @Summary      Снять человека с чёрного списка (архивация)
// @Tags         person-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID записи"
// @Success      200 {string} string "Человек снят с чёрного списка"
// @Router       /person-blacklist/{id} [delete]
func (h *PersonBlacklistHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Archive(c.Request().Context(), id, userID); err != nil {
		return err
	}
	return RespondMessage(c, "Человек снят с чёрного списка")
}

// Restore godoc
// @Summary      Вернуть человека в чёрный список
// @Tags         person-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID записи"
// @Success      200 {string} string "Человек возвращён в чёрный список"
// @Router       /person-blacklist/{id}/restore [post]
func (h *PersonBlacklistHandler) Restore(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Restore(c.Request().Context(), id, userID); err != nil {
		return err
	}
	return RespondMessage(c, "Человек возвращён в чёрный список")
}

// Check godoc
// @Summary      Проверить, в чёрном ли списке человек
// @Tags         person-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        last_name query string true "Фамилия"
// @Param        first_name query string true "Имя"
// @Param        middle_name query string false "Отчество"
// @Success      200 {object} models.PersonBlacklistCheckResult
// @Router       /person-blacklist/check [get]
func (h *PersonBlacklistHandler) Check(c echo.Context) error {
	lastName := c.QueryParam("last_name")
	firstName := c.QueryParam("first_name")
	if lastName == "" || firstName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "last_name и first_name обязательны")
	}
	res, err := h.service.Check(c.Request().Context(), lastName, firstName, c.QueryParam("middle_name"))
	if err != nil {
		return err
	}
	return RespondSuccess(c, res)
}

// Impact godoc
// @Summary      Предпросмотр последствий внесения человека в чёрный список
// @Description  Где человек сейчас фигурирует: сколько активных строк перестанет действовать, из каких таблиц постов они уйдут, в каких заявках есть. Ничего не меняет.
// @Tags         person-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        last_name   query string true  "Фамилия"
// @Param        first_name  query string true  "Имя"
// @Param        middle_name query string false "Отчество"
// @Success      200 {object} map[string]interface{} "success + данные предпросмотра"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /person-blacklist/impact [get]
func (h *PersonBlacklistHandler) Impact(c echo.Context) error {
	lastName := c.QueryParam("last_name")
	firstName := c.QueryParam("first_name")
	if lastName == "" || firstName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "last_name и first_name обязательны")
	}
	impact, err := h.service.Impact(c.Request().Context(), lastName, firstName, c.QueryParam("middle_name"))
	if err != nil {
		return err
	}
	return RespondSuccess(c, impact)
}

// GetHistory godoc
// @Summary      История записи чёрного списка людей
// @Tags         person-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID записи"
// @Success      200 {array} models.PersonBlacklistHistoryItem
// @Router       /person-blacklist/{id}/history [get]
func (h *PersonBlacklistHandler) GetHistory(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	history, err := h.service.GetHistory(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, history)
}

// GetAllHistory godoc
// @Summary      Весь журнал чёрного списка людей
// @Tags         person-blacklist
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.PersonBlacklistHistoryItem
// @Router       /person-blacklist/history [get]
func (h *PersonBlacklistHandler) GetAllHistory(c echo.Context) error {
	history, err := h.service.GetAllHistory(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, history)
}

// BulkArchive godoc
// @Summary      Групповое снятие людей с чёрного списка
// @Tags         person-blacklist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID записей"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /person-blacklist/bulk/archive [post]
func (h *PersonBlacklistHandler) BulkArchive(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны записи")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkArchive(c.Request().Context(), req.IDs, userID)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkRestore godoc
// @Summary      Групповое восстановление людей в чёрный список
// @Tags         person-blacklist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID записей"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /person-blacklist/bulk/restore [post]
func (h *PersonBlacklistHandler) BulkRestore(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны записи")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkRestore(c.Request().Context(), req.IDs, userID)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// Purge godoc
// @Summary      Удалить запись чёрного списка людей навсегда
// @Tags         person-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID записи"
// @Success      200 {string} string "Запись удалена навсегда"
// @Router       /person-blacklist/{id}/purge [delete]
func (h *PersonBlacklistHandler) Purge(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Purge(c.Request().Context(), id, userID); err != nil {
		return err
	}
	return RespondMessage(c, "Запись удалена навсегда")
}
