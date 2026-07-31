package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"systemburo/internal/apperr"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// SearchHandler -- сквозной поиск по разделам системы.
type SearchHandler struct {
	service services.SearchService
}

// NewSearchHandler создаёт обработчик сквозного поиска.
func NewSearchHandler(service services.SearchService) *SearchHandler {
	return &SearchHandler{service: service}
}

// Search godoc
// @Summary      Сквозной поиск по системе
// @Description  Ищет по всем разделам, доступным пользователю, и возвращает результаты
// @Description  сгруппированными по сущностям. Раздел, на который у пользователя нет
// @Description  права, в выдачу не попадает; внутри раздела строки сужаются так же, как
// @Description  в его листинге. Ответ отдаётся даже если часть разделов не ответила --
// @Description  их коды перечислены в degraded.
// @Tags         search
// @Produce      json
// @Security     BearerAuth
// @Param        q     query    string true  "Поисковый запрос, от 3 до 100 символов"
// @Param        types query    string false "Коды разделов через запятую; пусто -- все доступные"
// @Param        limit query    int    false "Результатов на раздел, 1..20 (по умолчанию 5)"
// @Success      200   {object} services.SearchResponse
// @Failure      400   {object} models.HTTPError "Слишком короткий или длинный запрос, неизвестный раздел"
// @Failure      401   {object} models.HTTPError
// @Failure      403   {object} models.HTTPError "Учётная запись заблокирована"
// @Failure      503   {object} models.HTTPError "Ни один раздел не ответил"
// @Router       /search [get]
func (h *SearchHandler) Search(c echo.Context) error {
	userID := GetUserID(c)
	if userID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
	}

	types, err := parseSearchTypes(c.QueryParam("types"))
	if err != nil {
		return err
	}

	limit := 0
	if raw := c.QueryParam("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			return apperr.Validation("Параметр limit должен быть числом")
		}
	}

	resp, err := h.service.Search(c.Request().Context(), userID, c.QueryParam("q"), types, limit)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// parseSearchTypes разбирает фильтр разделов. Неизвестный код -- ошибка, а не молчаливый
// пропуск: опечатка в коде раздела иначе выглядела бы как "в этом разделе ничего нет".
func parseSearchTypes(raw string) ([]services.SearchEntityType, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	types := make([]services.SearchEntityType, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		t := services.SearchEntityType(p)
		if !services.IsKnownSearchType(t) {
			return nil, apperr.Validation("Неизвестный раздел поиска: " + p)
		}
		types = append(types, t)
	}
	return types, nil
}
