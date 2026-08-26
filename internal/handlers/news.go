package handlers

import (
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// NewsHandler -- HTTP-обработчики новостей и объявлений.
type NewsHandler struct {
	service services.NewsService
}

// NewNewsHandler создаёт новый экземпляр NewsHandler.
func NewNewsHandler(service services.NewsService) *NewsHandler {
	return &NewsHandler{service: service}
}

// --- News ---

// GetActiveNews godoc
// @Summary      Получение активных новостей
// @Tags         news
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.NewsWithUser
// @Failure      401 {object} models.HTTPError
// @Router       /news [get]
func (h *NewsHandler) GetActiveNews(c echo.Context) error {
	news, err := h.service.GetActiveNews(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, news)
}

// GetAllNews godoc
// @Summary      Получение всех новостей (для управления)
// @Tags         news
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.NewsWithUser
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /news/all [get]
func (h *NewsHandler) GetAllNews(c echo.Context) error {
	news, err := h.service.GetAllNews(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, news)
}

// CreateNews godoc
// @Summary      Создание новости
// @Tags         news
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateNewsRequest true "Данные новости"
// @Success      201 {object} models.NewsWithUser
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /news [post]
func (h *NewsHandler) CreateNews(c echo.Context) error {
	userID := GetUserID(c)
	var req models.CreateNewsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	news, err := h.service.CreateNews(c.Request().Context(), userID, req)
	if err != nil {
		return err
	}
	return RespondCreated(c, news)
}

// UpdateNews godoc
// @Summary      Обновление новости
// @Tags         news
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID новости"
// @Param        request body models.UpdateNewsRequest true "Данные для обновления"
// @Success      200 {object} models.NewsWithUser
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /news/{id} [put]
func (h *NewsHandler) UpdateNews(c echo.Context) error {
	userID := GetUserID(c)
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	var req models.UpdateNewsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	news, err := h.service.UpdateNews(c.Request().Context(), userID, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, news)
}

// DeleteNews godoc
// @Summary      Удаление новости
// @Tags         news
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID новости"
// @Success      200 {string} string "Новость удалена"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /news/{id} [delete]
func (h *NewsHandler) DeleteNews(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	if err := h.service.DeleteNews(c.Request().Context(), id); err != nil {
		return err
	}
	return RespondMessage(c, "Новость удалена")
}

// --- Announcements ---

// GetActiveAnnouncement godoc
// @Summary      Получение активного объявления
// @Tags         announcements
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.AnnouncementWithUser
// @Failure      401 {object} models.HTTPError
// @Router       /announcements/active [get]
func (h *NewsHandler) GetActiveAnnouncement(c echo.Context) error {
	announcement, err := h.service.GetActiveAnnouncement(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, announcement)
}

// GetAllAnnouncements godoc
// @Summary      Получение всех объявлений
// @Tags         announcements
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.AnnouncementWithUser
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /announcements/all [get]
func (h *NewsHandler) GetAllAnnouncements(c echo.Context) error {
	announcements, err := h.service.GetAllAnnouncements(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, announcements)
}

// CreateAnnouncement godoc
// @Summary      Создание объявления
// @Tags         announcements
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateAnnouncementRequest true "Данные объявления"
// @Success      201 {object} models.AnnouncementWithUser
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /announcements [post]
func (h *NewsHandler) CreateAnnouncement(c echo.Context) error {
	userID := GetUserID(c)
	var req models.CreateAnnouncementRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	announcement, err := h.service.CreateAnnouncement(c.Request().Context(), userID, req)
	if err != nil {
		return err
	}
	return RespondCreated(c, announcement)
}

// SetActiveAnnouncement godoc
// @Summary      Установка активного объявления
// @Description  Деактивирует все объявления и активирует выбранное
// @Tags         announcements
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.SetActiveAnnouncementRequest true "ID объявления"
// @Success      200 {string} string "Активное объявление обновлено"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /announcements/set-active [post]
func (h *NewsHandler) SetActiveAnnouncement(c echo.Context) error {
	userID := GetUserID(c)
	var req models.SetActiveAnnouncementRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.SetActiveAnnouncement(c.Request().Context(), userID, req); err != nil {
		return err
	}
	return RespondMessage(c, "Активное объявление обновлено")
}

// HideAnnouncement godoc
// @Summary      Скрытие объявления
// @Description  Снимает is_active с конкретного объявления; не трогает остальные.
// @Tags         announcements
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID объявления"
// @Success      200 {string} string "Объявление скрыто"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /announcements/{id}/hide [post]
func (h *NewsHandler) HideAnnouncement(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	if err := h.service.HideAnnouncement(c.Request().Context(), id); err != nil {
		return err
	}
	return RespondMessage(c, "Объявление скрыто")
}

// UpdateAnnouncement godoc
// @Summary      Обновление объявления
// @Tags         announcements
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID объявления"
// @Param        request body models.UpdateAnnouncementRequest true "Данные для обновления"
// @Success      200 {object} models.AnnouncementWithUser
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /announcements/{id} [put]
func (h *NewsHandler) UpdateAnnouncement(c echo.Context) error {
	userID := GetUserID(c)
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	var req models.UpdateAnnouncementRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	announcement, err := h.service.UpdateAnnouncement(c.Request().Context(), userID, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, announcement)
}

// DeleteAnnouncement godoc
// @Summary      Удаление объявления
// @Tags         announcements
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID объявления"
// @Success      200 {string} string "Объявление удалено"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /announcements/{id} [delete]
func (h *NewsHandler) DeleteAnnouncement(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	if err := h.service.DeleteAnnouncement(c.Request().Context(), id); err != nil {
		return err
	}
	return RespondMessage(c, "Объявление удалено")
}
