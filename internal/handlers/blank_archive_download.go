package handlers

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/download"
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// ArchiveDownloadHandler - скачивание из файлового архива бланков (#1615, срез B3):
// потоковый ZIP за период (билет + публичный редемпшен, EventSource-подобная схема
// SSE-билетов), ZIP заявки для обычных участников и охраны, отдельный файл и список
// реестра для раздела администрирования.
type ArchiveDownloadHandler struct {
	downloads *services.ArchiveDownloadService
	// access/resolver - тот же гейт, что и у прямого скачивания одного бланка
	// (attachment_blank.go, canDownloadBlank): ArchiveDownloadHandler не держит
	// ссылку на AttachmentBlankHandler, а получает те же зависимости напрямую,
	// чтобы не заводить связь handler->handler.
	access   blankAccessService
	resolver *services.PermissionResolver
}

// NewArchiveDownloadHandler создаёт хендлер скачивания. downloads может быть nil -
// каталог архива не настроен, раздел всё равно должен открываться (тот же принцип,
// что у BlankArchiveHandler.Reexport).
func NewArchiveDownloadHandler(downloads *services.ArchiveDownloadService, access blankAccessService, resolver *services.PermissionResolver) *ArchiveDownloadHandler {
	return &ArchiveDownloadHandler{downloads: downloads, access: access, resolver: resolver}
}

// archiveUnavailable - единый ответ на неподнятый файловый архив: раздел настроек
// открывается и без него, но скачивать пока нечего.
func archiveUnavailable() error {
	return echo.NewHTTPError(http.StatusServiceUnavailable, "Файловый архив недоступен: каталог не настроен")
}

// archiveDownloadRangeError переводит доменную ошибку периода в понятный 400.
// requireBlankDocumentsExport закрывает выдачу сохранённых файлов тому, кому не
// положены документы участников. Раздел файлового архива и так админский, но право на
// выгрузку файлов (action.download.file_archive) выдаётся и точечно, а внутри каждого
// файла лежат паспорта - без этой проверки закрытый бланк забирался бы через раздел.
func requireBlankDocumentsExport(c echo.Context, resolver *services.PermissionResolver) error {
	allowed, err := canExportBlankDocuments(c, resolver)
	if err != nil {
		return err
	}
	if !allowed {
		return echo.NewHTTPError(http.StatusForbidden,
			"Сохранённые бланки содержат документы участников - выгрузка закрыта")
	}
	return nil
}

func archiveDownloadRangeError(err error) error {
	if errors.Is(err, services.ErrArchiveDownloadRangeInvalid) {
		return apperr.Validation("Некорректный период: укажите даты в формате ГГГГ-ММ-ДД, «с» не позже «по»")
	}
	return err
}

// EstimateDownload godoc
// @Summary      Оценить объём выгрузки файлового архива за период
// @Tags         file-archive
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.ArchiveDownloadRequest true "Период"
// @Success      200 {object} Response
// @Router       /file-archive/estimate [post]
func (h *ArchiveDownloadHandler) EstimateDownload(c echo.Context) error {
	if h.downloads == nil {
		return archiveUnavailable()
	}
	var req models.ArchiveDownloadRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	estimate, err := h.downloads.Estimate(c.Request().Context(), req.DateFrom, req.DateTo)
	if err != nil {
		return archiveDownloadRangeError(err)
	}
	return RespondSuccess(c, estimate)
}

// IssueDownloadTicket godoc
// @Summary      Выдать билет на потоковый ZIP файлового архива за период
// @Description  Билет одноразовый, живёт секунды и привязан к границам периода - GET /file-archive/download их не принимает заново.
// @Tags         file-archive
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.ArchiveDownloadRequest true "Период"
// @Success      200 {object} Response
// @Failure      403 {object} models.HTTPError
// @Failure      413 {object} models.HTTPError
// @Router       /file-archive/download-ticket [post]
func (h *ArchiveDownloadHandler) IssueDownloadTicket(c echo.Context) error {
	if h.downloads == nil {
		return archiveUnavailable()
	}
	var req models.ArchiveDownloadRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	// Билет проверяется здесь, а не на самой выдаче ZIP: GET /file-archive/download
	// ходит без Authorization (билет вместо заголовка), и права там уже не спросить.
	if err := requireBlankDocumentsExport(c, h.resolver); err != nil {
		return err
	}

	ticket, err := h.downloads.IssueTicket(c.Request().Context(), GetUserID(c), req.DateFrom, req.DateTo)
	switch {
	case errors.Is(err, services.ErrArchiveDownloadTooLarge):
		return apperr.New(http.StatusRequestEntityTooLarge,
			"Выгрузка за этот период больше допустимого предела - сузьте диапазон дат")
	case errors.Is(err, services.ErrArchiveDownloadRangeInvalid):
		return archiveDownloadRangeError(err)
	case err != nil:
		return err
	}
	return RespondSuccess(c, models.ArchiveDownloadTicketResponse{Ticket: ticket})
}

// Download godoc
// @Summary      Скачать потоковый ZIP файлового архива за период
// @Description  Публичный роут (билет вместо Authorization) - по образцу /events, EventSource/прямая ссылка не шлёт заголовок Bearer.
// @Tags         file-archive
// @Produce      application/zip
// @Param        ticket query string true "Билет из POST /file-archive/download-ticket"
// @Success      200
// @Failure      401 {object} models.HTTPError
// @Router       /file-archive/download [get]
func (h *ArchiveDownloadHandler) Download(c echo.Context) error {
	if h.downloads == nil {
		return archiveUnavailable()
	}
	ticket := c.QueryParam("ticket")
	if ticket == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "missing ticket")
	}

	entries, name, userID, err := h.downloads.ConsumeAndCollect(c.Request().Context(), ticket, time.Now())
	if err != nil {
		return err
	}
	// Роут публичный, JWT-middleware сюда контекст не наполняет - без этой строки
	// журнал персональных данных записал бы самый массовый вынос бланков в системе
	// обезличенно. Владельца знает билет, ставим до отдачи байтов: PDAudit читает
	// контекст после обработчика.
	c.Set("user_id", userID)
	return download.StreamZip(c, name, entries)
}

// Archive godoc
// @Summary      Скачать сохранённые бланки заявки единым ZIP
// @Description  Доступ - как у скачивания одного бланка (участник заявки либо охрана/носитель page.available по своему вложению).
// @Description  Дополнительно требуются права detail.documents и detail.documents.export: в ZIP уезжают сохранённые копии с документами участников, обезличить их при отдаче нечем.
// @Tags         file-archive
// @Produce      application/zip
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /applications/{id}/archive [get]
func (h *ArchiveDownloadHandler) Archive(c echo.Context) error {
	if h.downloads == nil {
		return archiveUnavailable()
	}
	appID, err := ParseID(c, "id")
	if err != nil {
		return err
	}

	ctx := c.Request().Context()
	fullAccess := false
	if username, ok := c.Get("username").(string); ok && username != "" {
		fullAccess = h.access.CanAccessApplication(ctx, appID, username, IsSuperAdmin(c))
	}

	entries, err := h.downloads.ArchiveApplicationEntries(ctx, appID, fullAccess, func(attID int) (bool, error) {
		return canDownloadBlank(c, h.access, h.resolver, appID, attID)
	})
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "В архиве нет доступных файлов этой заявки")
	}

	// Гейт документов стоит после проверки доступа к заявке, а не до неё: посторонний
	// должен по-прежнему получать 404 и не узнавать по коду ответа, что заявка с таким
	// номером есть. В ZIP уезжают те же сохранённые копии, что и по одному через
	// ?source=archive, поэтому право требуется то же - закрытое поштучно не должно
	// забираться архивом целиком.
	if err := requireBlankDocumentsExport(c, h.resolver); err != nil {
		return err
	}
	return download.StreamZip(c, "application_"+strconv.Itoa(appID)+".zip", entries)
}

// DownloadFile godoc
// @Summary      Скачать один файл из реестра файлового архива
// @Tags         file-archive
// @Produce      application/octet-stream
// @Security     BearerAuth
// @Param        id path int true "ID строки реестра (blank_exports.id)"
// @Success      200
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /file-archive/files/{id} [get]
func (h *ArchiveDownloadHandler) DownloadFile(c echo.Context) error {
	if h.downloads == nil {
		return archiveUnavailable()
	}
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	if err := requireBlankDocumentsExport(c, h.resolver); err != nil {
		return err
	}

	row, err := h.downloads.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}
	file, err := h.downloads.FileForDownload(row)
	if err != nil {
		return err
	}
	return download.Serve(c, file)
}

// ListItems godoc
// @Summary      Список реестра файлового архива
// @Description  Фильтры: status (один из известных статусов), application_id. Пагинация page/per_page.
// @Tags         file-archive
// @Produce      json
// @Security     BearerAuth
// @Param        status         query int    false "Статус строки реестра"
// @Param        application_id query int    false "ID заявки"
// @Param        page           query int    false "Страница"
// @Param        per_page       query int    false "Размер страницы (<=100)"
// @Success      200 {object} Response
// @Router       /file-archive/items [get]
func (h *ArchiveDownloadHandler) ListItems(c echo.Context) error {
	if h.downloads == nil {
		return archiveUnavailable()
	}

	q := services.ArchiveItemsQuery{Status: c.QueryParam("status")}
	if q.Status != "" && !slices.Contains(models.AllBlankExportStatuses, q.Status) {
		return apperr.Validation("Некорректный статус")
	}
	if v := c.QueryParam("application_id"); v != "" {
		id, err := strconv.Atoi(v)
		if err != nil || id <= 0 {
			return apperr.Validation("Некорректный application_id")
		}
		q.ApplicationID = id
	}

	var p models.PaginationParams
	if err := c.Bind(&p); err != nil {
		p = models.PaginationParams{}
	}
	p.Normalize()
	q.Page, q.PerPage = p.Page, p.PerPage

	items, total, err := h.downloads.ListItems(c.Request().Context(), q)
	if err != nil {
		return err
	}
	return RespondPaginated(c, items, models.PaginationMeta{Total: total, Page: p.Page, PerPage: p.PerPage})
}
