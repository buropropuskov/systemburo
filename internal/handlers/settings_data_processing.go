package handlers

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"systemburo/internal/download"
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// dataProcessingAllowedExt -- расширения, разрешённые для документа согласия на обработку данных.
// XLSX здесь ради переноса текста согласия в редактор: часть заказчиков ведёт перечень
// обрабатываемых данных таблицей.
var dataProcessingAllowedExt = map[string]bool{".pdf": true, ".doc": true, ".docx": true, ".xlsx": true}

// GetDataProcessingMeta возвращает метаданные документа согласия (или null, если не загружен).
func (h *SettingsHandler) GetDataProcessingMeta(c echo.Context) error {
	meta, err := h.service.GetDataProcessingDoc(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, meta)
}

// ServeDataProcessingDoc отдаёт файл документа согласия: inline для просмотра в браузере,
// attachment при ?download=1 для скачивания.
func (h *SettingsHandler) ServeDataProcessingDoc(c echo.Context) error {
	meta, err := h.service.GetDataProcessingDoc(c.Request().Context())
	if err != nil {
		return err
	}
	if meta == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Документ согласия не загружен")
	}
	filePath := filepath.Join(h.fileSvc.UploadDir(), meta.StoredName)
	return download.Serve(c, download.File{
		Path:   filePath,
		Name:   meta.FileName,
		Mime:   meta.MimeType,
		Inline: c.QueryParam("download") != "1",
	})
}

// UploadDataProcessingDoc сохраняет новый документ согласия, заменяя предыдущий.
func (h *SettingsHandler) UploadDataProcessingDoc(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Файл не передан")
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !dataProcessingAllowedExt[ext] {
		return echo.NewHTTPError(http.StatusBadRequest, "Недопустимый тип файла. Разрешены: pdf, doc, docx, xlsx")
	}

	ctx := c.Request().Context()
	storedName, savedExt, err := h.fileSvc.Save(ctx, file, h.maxFileSize)
	if err != nil {
		return err
	}

	meta := &models.DataProcessingDocument{
		StoredName: storedName,
		FileName:   file.Filename,
		MimeType:   services.DetectMimeType(savedExt),
		Ext:        savedExt,
		UploadedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Старый файл удаляем только после успешной записи метаданных, чтобы при сбое
	// записи не остаться без документа вовсе.
	old, _ := h.service.GetDataProcessingDoc(ctx)
	if err := h.service.SetDataProcessingDoc(ctx, meta); err != nil {
		h.fileSvc.Delete(storedName)
		return err
	}
	if old != nil && old.StoredName != "" && old.StoredName != storedName {
		h.fileSvc.Delete(old.StoredName)
	}
	return RespondSuccess(c, meta)
}

// DeleteDataProcessingDoc удаляет документ согласия и его метаданные.
func (h *SettingsHandler) DeleteDataProcessingDoc(c echo.Context) error {
	ctx := c.Request().Context()
	meta, err := h.service.GetDataProcessingDoc(ctx)
	if err != nil {
		return err
	}
	if meta == nil {
		return RespondSuccess(c, map[string]bool{"deleted": true})
	}
	if err := h.service.ClearDataProcessingDoc(ctx); err != nil {
		return err
	}
	h.fileSvc.Delete(meta.StoredName)
	return RespondSuccess(c, map[string]bool{"deleted": true})
}
