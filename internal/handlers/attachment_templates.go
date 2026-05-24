package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// AttachmentTemplateHandler - HTTP API настроек Excel-бланков (#183).
type AttachmentTemplateHandler struct {
	service services.AttachmentTemplateService
}

// NewAttachmentTemplateHandler создаёт handler.
func NewAttachmentTemplateHandler(s services.AttachmentTemplateService) *AttachmentTemplateHandler {
	return &AttachmentTemplateHandler{service: s}
}

// GetTemplate godoc
// @Summary      Получить настройки бланка вложения
// @Tags         attachment-templates
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID UniqueAttachment"
// @Success      200 {object} models.AttachmentTemplate
// @Router       /attachments/{id}/template [get]
func (h *AttachmentTemplateHandler) Get(c echo.Context) error {
	uaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	t, err := h.service.Get(c.Request().Context(), uaID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, t)
}

func (h *AttachmentTemplateHandler) ListTemplates(c echo.Context) error {
	uaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	templates, err := h.service.ListTemplates(c.Request().Context(), uaID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, templates)
}

func (h *AttachmentTemplateHandler) SetActive(c echo.Context) error {
	uaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	tid, err := strconv.Atoi(c.Param("tid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid template id")
	}
	if err := h.service.SetActive(c.Request().Context(), uaID, tid); err != nil {
		return err
	}
	return RespondMessage(c, "Шаблон активирован")
}

func (h *AttachmentTemplateHandler) DeleteByID(c echo.Context) error {
	tid, err := strconv.Atoi(c.Param("tid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid template id")
	}
	if err := h.service.DeleteByID(c.Request().Context(), tid); err != nil {
		return err
	}
	return RespondMessage(c, "Шаблон удален")
}

func (h *AttachmentTemplateHandler) DownloadFileByID(c echo.Context) error {
	tid, err := strconv.Atoi(c.Param("tid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid template id")
	}
	t, err := h.service.GetByID(c.Request().Context(), tid)
	if err != nil {
		return err
	}
	if t.FilePath == "" {
		return echo.NewHTTPError(http.StatusNotFound, "Файл шаблона не загружен")
	}
	return c.File(t.FilePath)
}

// Upload godoc
// @Summary      Загрузить .xlsx шаблон бланка
// @Tags         attachment-templates
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID UniqueAttachment"
// @Param        file formData file true ".xlsx файл"
// @Param        list_start_row formData int true "Начало строк списка"
// @Param        list_end_row formData int true "Конец строк списка"
// @Param        max_list_rows formData int false "Макс. записей (авто = end-start+1)"
// @Success      201 {object} models.AttachmentTemplate
// @Router       /attachments/{id}/template [post]
func (h *AttachmentTemplateHandler) Upload(c echo.Context) error {
	uaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file required")
	}
	// Проверка расширения и MIME для .xlsx.
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".xlsx") {
		return echo.NewHTTPError(http.StatusBadRequest, "Файл должен быть .xlsx")
	}
	startRow, _ := strconv.Atoi(c.FormValue("list_start_row"))
	endRow, _ := strconv.Atoi(c.FormValue("list_end_row"))
	maxRows, _ := strconv.Atoi(c.FormValue("max_list_rows"))
	if startRow < 1 || endRow < startRow {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректный диапазон строк")
	}

	userID, _ := c.Get("user_id").(int)
	t, err := h.service.Upload(c.Request().Context(), uaID, file, models.CreateTemplateRequest{
		ListStartRow: startRow,
		ListEndRow:   endRow,
		MaxListRows:  maxRows,
	}, userID)
	if err != nil {
		return err
	}
	return RespondCreated(c, t)
}

// UpdateMappings godoc
// @Summary      Обновить маппинг ячеек на поля
// @Tags         attachment-templates
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID UniqueAttachment"
// @Param        request body models.UpdateMappingsRequest true "Список маппингов"
// @Success      200 {string} string "Маппинги обновлены"
// @Router       /attachments/{id}/template/mappings [put]
func (h *AttachmentTemplateHandler) UpdateMappings(c echo.Context) error {
	uaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.UpdateMappingsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateMappings(c.Request().Context(), uaID, req); err != nil {
		return err
	}
	return RespondMessage(c, "Маппинги обновлены")
}

// Delete godoc
// @Summary      Удалить шаблон бланка
// @Tags         attachment-templates
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID UniqueAttachment"
// @Success      200 {string} string "Шаблон удалён"
// @Router       /attachments/{id}/template [delete]
func (h *AttachmentTemplateHandler) Delete(c echo.Context) error {
	uaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.service.Delete(c.Request().Context(), uaID); err != nil {
		return err
	}
	return RespondMessage(c, "Шаблон удалён")
}

// DownloadFile godoc
// @Summary      Скачать файл шаблона для предпросмотра
// @Tags         attachment-templates
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Security     BearerAuth
// @Param        id path int true "ID UniqueAttachment"
// @Success      200 {file} binary
// @Router       /attachments/{id}/template/file [get]
func (h *AttachmentTemplateHandler) DownloadFile(c echo.Context) error {
	uaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	t, err := h.service.Get(c.Request().Context(), uaID)
	if err != nil {
		return err
	}
	if t.FilePath == "" {
		return echo.NewHTTPError(http.StatusNotFound, "Файл шаблона не загружен")
	}
	return c.File(t.FilePath)
}

// GetFields godoc
// @Summary      Справочник полей доступных для маппинга
// @Tags         attachment-templates
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID UniqueAttachment"
// @Success      200 {array} services.TemplateFieldGroup
// @Router       /attachments/{id}/template-fields [get]
func (h *AttachmentTemplateHandler) GetFields(c echo.Context) error {
	uaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	// Встроенные поля + динамические custom-поля этого UA.
	groups := services.BuiltinTemplateFields()
	customs, err := h.service.ListCustomFields(c.Request().Context(), uaID)
	if err == nil && len(customs) > 0 {
		customGroup := services.TemplateFieldGroup{
			Group: "custom",
			Label: "Дополнительные поля",
		}
		for _, cf := range customs {
			customGroup.Fields = append(customGroup.Fields, services.TemplateField{
				Path:  "custom." + strconv.Itoa(cf.ID),
				Label: cf.Label,
			})
		}
		groups = append(groups, customGroup)
	}
	return RespondSuccess(c, groups)
}

// ListCustomFields godoc
// @Summary      Список кастомных полей вложения
// @Tags         attachment-templates
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID UniqueAttachment"
// @Success      200 {array} models.AttachmentCustomField
// @Router       /attachments/{id}/custom-fields [get]
func (h *AttachmentTemplateHandler) ListCustomFields(c echo.Context) error {
	uaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	list, err := h.service.ListCustomFields(c.Request().Context(), uaID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, list)
}

// CreateCustomField godoc
// @Summary      Создать кастомное поле
// @Tags         attachment-templates
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID UniqueAttachment"
// @Param        request body models.CreateCustomFieldRequest true "Данные поля"
// @Success      201 {object} models.AttachmentCustomField
// @Router       /attachments/{id}/custom-fields [post]
func (h *AttachmentTemplateHandler) CreateCustomField(c echo.Context) error {
	uaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.CreateCustomFieldRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	cf, err := h.service.CreateCustomField(c.Request().Context(), uaID, req)
	if err != nil {
		return err
	}
	return RespondCreated(c, cf)
}

// UpdateCustomField godoc
// @Summary      Обновить кастомное поле
// @Tags         attachment-templates
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        fid path int true "ID поля"
// @Param        request body models.CreateCustomFieldRequest true "Данные"
// @Success      200 {string} string "Поле обновлено"
// @Router       /attachments/custom-fields/{fid} [put]
func (h *AttachmentTemplateHandler) UpdateCustomField(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("fid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.CreateCustomFieldRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateCustomField(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Поле обновлено")
}

// DeleteCustomField godoc
// @Summary      Удалить кастомное поле (soft delete)
// @Tags         attachment-templates
// @Produce      json
// @Security     BearerAuth
// @Param        fid path int true "ID поля"
// @Success      200 {string} string "Поле удалено"
// @Router       /attachments/custom-fields/{fid} [delete]
func (h *AttachmentTemplateHandler) DeleteCustomField(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("fid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.service.DeleteCustomField(c.Request().Context(), id); err != nil {
		return err
	}
	return RespondMessage(c, "Поле удалено")
}
