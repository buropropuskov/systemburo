package handlers

import (
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// DocumentGroupHandler -- HTTP-обработчики групп документов.
type DocumentGroupHandler struct {
	service services.DocumentGroupService
}

// NewDocumentGroupHandler создаёт новый DocumentGroupHandler.
func NewDocumentGroupHandler(service services.DocumentGroupService) *DocumentGroupHandler {
	return &DocumentGroupHandler{service: service}
}

// List godoc
// @Summary      Список групп документов с количеством документов
// @Tags         document-groups
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.DocumentGroupWithCount
// @Failure      401 {object} models.HTTPError
// @Router       /document-groups [get]
func (h *DocumentGroupHandler) List(c echo.Context) error {
	groups, err := h.service.List(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, groups)
}

// Create godoc
// @Summary      Создание группы документов
// @Tags         document-groups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateDocumentGroupRequest true "Данные группы"
// @Success      201 {object} models.DocumentGroup
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Router       /document-groups [post]
func (h *DocumentGroupHandler) Create(c echo.Context) error {
	userID := GetUserID(c)
	var req models.CreateDocumentGroupRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	group, err := h.service.Create(c.Request().Context(), userID, req)
	if err != nil {
		return err
	}
	return RespondCreated(c, group)
}

// Update godoc
// @Summary      Переименование группы документов
// @Tags         document-groups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID группы"
// @Param        request body models.UpdateDocumentGroupRequest true "Новое название"
// @Success      200 {object} models.DocumentGroup
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Router       /document-groups/{id} [put]
func (h *DocumentGroupHandler) Update(c echo.Context) error {
	userID := GetUserID(c)
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	var req models.UpdateDocumentGroupRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	group, err := h.service.Update(c.Request().Context(), userID, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, group)
}

// Delete godoc
// @Summary      Удаление группы документов
// @Description  Документы группы переходят в «Прочее» (group_id = NULL)
// @Tags         document-groups
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID группы"
// @Success      200 {string} string "Группа удалена"
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /document-groups/{id} [delete]
func (h *DocumentGroupHandler) Delete(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	return RespondMessage(c, "Группа документов удалена")
}

// Reorder godoc
// @Summary      Изменение порядка групп документов
// @Tags         document-groups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.ReorderDocumentGroupsRequest true "Новый порядок ID групп"
// @Success      200 {string} string "Порядок обновлён"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /document-groups/reorder [put]
func (h *DocumentGroupHandler) Reorder(c echo.Context) error {
	var req models.ReorderDocumentGroupsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Reorder(c.Request().Context(), req); err != nil {
		return err
	}
	return RespondMessage(c, "Порядок групп обновлён")
}
