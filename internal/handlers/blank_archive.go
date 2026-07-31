package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"systemburo/internal/blankpath"
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// BlankArchiveHandler - настройки файлового архива бланков, живое превью раскладки и
// ручное пересоздание файлов заявки (#1615). Статистика и скачивание приезжают
// следующими срезами.
type BlankArchiveHandler struct {
	settings services.SettingsService
	paths    *services.ArchivePathService
	exports  *services.BlankExportService
	recorder services.AuditRecorder
}

// NewBlankArchiveHandler создаёт хендлер раздела «Файловый архив». Сервис выгрузки
// приходит отдельно и может отсутствовать: без настроенного каталога архива запись
// не поднимается, а раздел настроек обязан открываться и в этом состоянии.
func NewBlankArchiveHandler(
	settings services.SettingsService,
	paths *services.ArchivePathService,
	exports *services.BlankExportService,
	recorder services.AuditRecorder,
) *BlankArchiveHandler {
	return &BlankArchiveHandler{settings: settings, paths: paths, exports: exports, recorder: recorder}
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

// UpdateSettings сохраняет присланные поля настроек и пишет изменение в общий журнал.
// Запись аудита идёт после успешного сохранения и через Log: не сложившийся журнал не
// должен отменять настройку, которую администратор уже применил.
func (h *BlankArchiveHandler) UpdateSettings(c echo.Context) error {
	var req models.UpdateArchiveSettingsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	ctx := c.Request().Context()

	before, err := h.settings.GetArchiveSettings(ctx)
	if err != nil {
		return err
	}
	after, err := h.settings.UpdateArchiveSettings(ctx, req)
	if err != nil {
		return err
	}

	if details := archiveSettingsDiff(before, after); len(details) > 0 {
		userID := GetUserID(c)
		h.recorder.Log(ctx, nil, models.AuditEntityArchiveSettings, nil, models.ArchiveSettingsActionUpdated, &userID, details)
	}
	return RespondSuccess(c, after)
}

// Preview показывает, как шаблоны разложатся в путь. Шаблоны берутся из запроса (в
// конструкторе они ещё не сохранены), а пустое поле подставляется из текущих настроек:
// администратор правит одно поле, а видеть должен путь целиком.
func (h *BlankArchiveHandler) Preview(c echo.Context) error {
	var req models.ArchivePreviewRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	ctx := c.Request().Context()

	if req.DirTemplate == "" || req.FileTemplate == "" {
		current, err := h.settings.GetArchiveSettings(ctx)
		if err != nil {
			return err
		}
		if req.DirTemplate == "" {
			req.DirTemplate = current.DirTemplate
		}
		if req.FileTemplate == "" {
			req.FileTemplate = current.FileTemplate
		}
	}

	preview, err := h.paths.Preview(ctx, req.DirTemplate, req.FileTemplate, req.ApplicationID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, preview)
}

// GetTokens отдаёт реестр плейсхолдеров для палитры конструктора: ключ, подпись,
// группу, пример и то, где плейсхолдер разрешён. Фронт не должен держать свою копию
// списка - она разъедется с сервером на первом же новом токене.
func (h *BlankArchiveHandler) GetTokens(c echo.Context) error {
	return RespondSuccess(c, blankpath.Tokens())
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

// archiveSettingsDiff собирает изменившиеся настройки как {old, new}. Сравнивается
// состояние до и после записи, а не присланные поля: запрос может прислать значение,
// равное текущему, и запись «изменено» о нём была бы враньём.
func archiveSettingsDiff(before, after *models.ArchiveSettings) map[string]any {
	details := map[string]any{}
	if before.Enabled != after.Enabled {
		details["enabled"] = map[string]any{"old": before.Enabled, "new": after.Enabled}
	}
	if before.DirTemplate != after.DirTemplate {
		details["dir_template"] = map[string]any{"old": before.DirTemplate, "new": after.DirTemplate}
	}
	if before.FileTemplate != after.FileTemplate {
		details["file_template"] = map[string]any{"old": before.FileTemplate, "new": after.FileTemplate}
	}
	if before.QuotaBytes != after.QuotaBytes {
		details["quota_bytes"] = map[string]any{"old": before.QuotaBytes, "new": after.QuotaBytes}
	}
	if before.MinFreeBytes != after.MinFreeBytes {
		details["min_free_bytes"] = map[string]any{"old": before.MinFreeBytes, "new": after.MinFreeBytes}
	}
	if before.WarnPercent != after.WarnPercent {
		details["warn_percent"] = map[string]any{"old": before.WarnPercent, "new": after.WarnPercent}
	}
	if before.RecheckDays != after.RecheckDays {
		details["recheck_days"] = map[string]any{"old": before.RecheckDays, "new": after.RecheckDays}
	}
	if before.FreezeAfterDays != after.FreezeAfterDays {
		details["freeze_after_days"] = map[string]any{"old": before.FreezeAfterDays, "new": after.FreezeAfterDays}
	}
	if before.ZipMaxBytes != after.ZipMaxBytes {
		details["zip_max_bytes"] = map[string]any{"old": before.ZipMaxBytes, "new": after.ZipMaxBytes}
	}
	return details
}
