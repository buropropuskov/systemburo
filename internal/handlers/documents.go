package handlers

import (
	"net/http"
	"path/filepath"
	"strconv"

	"systemburo/internal/download"
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// DocumentHandler -- HTTP-обработчики документов.
type DocumentHandler struct {
	service services.DocumentService
	fileSvc services.DocumentFileService
}

// NewDocumentHandler создаёт новый DocumentHandler.
func NewDocumentHandler(service services.DocumentService, fileSvc services.DocumentFileService) *DocumentHandler {
	return &DocumentHandler{service: service, fileSvc: fileSvc}
}

// List godoc
// @Summary      Список документов для админки
// @Tags         documents
// @Produce      json
// @Security     BearerAuth
// @Param        group_id query int false "Фильтр по группе"
// @Param        include_hidden query int false "Включить скрытые (1=да)"
// @Success      200 {array} models.DocumentListItem
// @Failure      401 {object} models.HTTPError
// @Router       /documents [get]
func (h *DocumentHandler) List(c echo.Context) error {
	var groupID *int
	if raw := c.QueryParam("group_id"); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil || id <= 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
		}
		groupID = &id
	}
	includeHidden := c.QueryParam("include_hidden") == "1"

	items, err := h.service.List(c.Request().Context(), groupID, includeHidden)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// Upload godoc
// @Summary      Загрузка документа (multipart)
// @Tags         documents
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        file formData file true "Файл документа"
// @Param        title formData string true "Название"
// @Param        description formData string false "Описание"
// @Param        group_id formData int false "ID группы"
// @Param        published_at formData string false "Дата публикации (RFC3339)"
// @Param        sort_order formData int false "Порядок"
// @Success      201 {object} models.DocumentListItem
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /documents [post]
func (h *DocumentHandler) Upload(c echo.Context) error {
	userID := GetUserID(c)

	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Файл не передан")
	}

	req, err := bindUploadRequest(c)
	if err != nil {
		return err
	}

	item, err := h.service.Upload(c.Request().Context(), userID, req, file)
	if err != nil {
		return err
	}
	return RespondCreated(c, item)
}

// UpdateMeta godoc
// @Summary      Обновление метаданных документа
// @Tags         documents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID документа"
// @Param        request body models.UpdateDocumentMetaRequest true "Данные для обновления"
// @Success      200 {object} models.DocumentListItem
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /documents/{id} [put]
func (h *DocumentHandler) UpdateMeta(c echo.Context) error {
	userID := GetUserID(c)
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	var req models.UpdateDocumentMetaRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	item, err := h.service.UpdateMeta(c.Request().Context(), userID, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, item)
}

// ReplaceFile godoc
// @Summary      Замена файла документа
// @Tags         documents
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID документа"
// @Param        file formData file true "Новый файл"
// @Success      200 {object} models.DocumentListItem
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /documents/{id}/file [put]
func (h *DocumentHandler) ReplaceFile(c echo.Context) error {
	userID := GetUserID(c)
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Файл не передан")
	}
	item, err := h.service.ReplaceFile(c.Request().Context(), userID, id, file)
	if err != nil {
		return err
	}
	return RespondSuccess(c, item)
}

// Delete godoc
// @Summary      Удаление документа (файл + запись в БД)
// @Tags         documents
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID документа"
// @Success      200 {string} string "Документ удалён"
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /documents/{id} [delete]
func (h *DocumentHandler) Delete(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	return RespondMessage(c, "Документ удалён")
}

// Reorder godoc
// @Summary      Изменение порядка документов внутри группы
// @Tags         documents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.ReorderDocumentsRequest true "group_id + массив ID"
// @Success      200 {string} string "Порядок обновлён"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /documents/reorder [put]
func (h *DocumentHandler) Reorder(c echo.Context) error {
	var req models.ReorderDocumentsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Reorder(c.Request().Context(), req); err != nil {
		return err
	}
	return RespondMessage(c, "Порядок документов обновлён")
}

// GetPublic godoc
// @Summary      Публичный список документов (только видимые, сгруппированы)
// @Tags         documents
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.PublicDocumentGroup
// @Failure      401 {object} models.HTTPError
// @Router       /public/documents [get]
func (h *DocumentHandler) GetPublic(c echo.Context) error {
	groups, err := h.service.GetPublic(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, groups)
}

// Download godoc
// @Summary      Скачивание файла документа
// @Tags         documents
// @Produce      application/octet-stream
// @Security     BearerAuth
// @Param        id path int true "ID документа"
// @Success      200 {file} binary
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /documents/{id}/download [get]
func (h *DocumentHandler) Download(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}

	doc, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return err
	}

	filePath := filepath.Join(h.fileSvc.UploadDir(), doc.StoredName)
	return download.Serve(c, download.File{Path: filePath, Name: doc.FileName, Mime: doc.MimeType})
}

// bindUploadRequest парсит поля multipart-формы для Upload.
func bindUploadRequest(c echo.Context) (models.UploadDocumentRequest, error) {
	req := models.UploadDocumentRequest{}
	req.Title = c.FormValue("title")

	if desc := c.FormValue("description"); desc != "" {
		req.Description = &desc
	}

	if gidRaw := c.FormValue("group_id"); gidRaw != "" {
		gid, err := strconv.Atoi(gidRaw)
		if err != nil || gid <= 0 {
			return req, echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
		}
		req.GroupID = &gid
	}

	if pat := c.FormValue("published_at"); pat != "" {
		req.PublishedAt = &pat
	}

	if soRaw := c.FormValue("sort_order"); soRaw != "" {
		so, err := strconv.Atoi(soRaw)
		if err == nil {
			req.SortOrder = so
		}
	}

	// Валидация: title обязателен
	if req.Title == "" {
		return req, echo.NewHTTPError(http.StatusBadRequest, "Поле title обязательно")
	}

	return req, nil
}
