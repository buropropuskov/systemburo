package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// BlankArchiveHandler - показ настроек файлового архива, ручное пересоздание файлов
// заявки и донаполнение за прошлый период (#1615). Правка самих настроек живёт в
// консольной команде server archive: раскладка каталогов и пороги места - дело того,
// кто разворачивает систему, а не бюро пропусков.
type BlankArchiveHandler struct {
	settings services.SettingsService
	exports  *services.BlankExportService
}

// NewBlankArchiveHandler создаёт хендлер раздела «Файловый архив». Сервис выгрузки
// приходит отдельно и может отсутствовать: без настроенного каталога архива запись
// не поднимается, а раздел обязан открываться и в этом состоянии.
func NewBlankArchiveHandler(settings services.SettingsService, exports *services.BlankExportService) *BlankArchiveHandler {
	return &BlankArchiveHandler{settings: settings, exports: exports}
}

// GetSettings отдаёт настройки файлового архива. На базе, где их ни разу не сохраняли,
// возвращаются значения по умолчанию - раздел должен открываться до первой настройки.
func (h *BlankArchiveHandler) GetSettings(c echo.Context) error {
	settings, err := h.settings.GetArchiveSettings(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, settings)
}

// Reexport пересоздаёт файлы заявки в архиве по текущим данным и настройкам.
//
// Ручка нужна там, где ждать фоновый разбор очереди нечестно: администратор поправил
// шаблон раскладки или починил бланк и должен увидеть результат на этой же заявке.
// Право то же, что на настройки: пересоздание переписывает файлы на диске и по весу
// ближе к настройке, чем к просмотру.
func (h *BlankArchiveHandler) Reexport(c echo.Context) error {
	if h.exports == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Файловый архив недоступен: каталог не настроен")
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	result, err := h.exports.ExportApplication(c.Request().Context(), id, services.BlankExportReasonReexport)
	switch {
	case errors.Is(err, services.ErrArchiveDisabled):
		return echo.NewHTTPError(http.StatusConflict, "Выгрузка бланков выключена в настройках файлового архива")
	case err != nil:
		return err
	}
	return RespondSuccess(c, result)
}

// Backfill ставит в очередь на выгрузку все заявки периода, по желанию суженные
// типом вложения. Ответ асинхронный (202): разбор идёт фоновым воркером (B1), а не в
// рамках этого запроса - широкий диапазон иначе держал бы администратора часами.
//
// Тот же запрос обслуживает и «пересоздать бланки этого типа» после правки
// маппингов шаблона (unique_attachment_id без сужения периода до нужного дня):
// auto-enqueue на каждую правку поставил бы в очередь десятки тысяч файлов, поэтому
// пересборка - осознанное действие администратора, а не следствие сохранения формы.
func (h *BlankArchiveHandler) Backfill(c echo.Context) error {
	if h.exports == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Файловый архив недоступен: каталог не настроен")
	}
	var req models.ArchiveBackfillRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	// Границы считает сервис: он знает рабочую таймзону раскладки, а разбор дат в
	// UTC отрезал бы не тот кусок суток (см. ParsePeriod).
	from, toExclusive, err := h.exports.ParsePeriod(req.DateFrom, req.DateTo)
	if err != nil {
		return err
	}

	queued, err := h.exports.Backfill(c.Request().Context(), from, toExclusive, req.UniqueAttachmentID)
	switch {
	case errors.Is(err, services.ErrArchiveDisabled):
		return echo.NewHTTPError(http.StatusConflict, "Выгрузка бланков выключена в настройках файлового архива")
	case err != nil:
		return err
	}
	return RespondAccepted(c, models.ArchiveBackfillResponse{Queued: queued})
}
