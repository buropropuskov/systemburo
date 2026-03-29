package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

type ApproverHandler struct {
	service services.ApproverService
}

func NewApproverHandler(service services.ApproverService) *ApproverHandler {
	return &ApproverHandler{service: service}
}

// GetAll godoc
// @Summary      Список утверждающих заявок
// @Tags         application-approvers
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.ApplicationApproverWithUser
// @Failure      403 {object} models.HTTPError
// @Router       /application-approvers [get]
func (h *ApproverHandler) GetAll(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	result, err := h.service.GetAll(c.Request().Context(), typeID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// GetAvailableUsers godoc
// @Summary      Пользователи, доступные для назначения утверждающими
// @Tags         application-approvers
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.AvailableApproverUser
// @Failure      403 {object} models.HTTPError
// @Router       /application-approvers/available-users [get]
func (h *ApproverHandler) GetAvailableUsers(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	result, err := h.service.GetAvailableUsers(c.Request().Context(), typeID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// Create godoc
// @Summary      Добавить утверждающего
// @Tags         application-approvers
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body models.CreateApproverRequest true "User ID"
// @Success      201 {object} map[string]string
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /application-approvers [post]
func (h *ApproverHandler) Create(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	username := c.Get("username").(string)

	var req models.CreateApproverRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.service.Create(c.Request().Context(), typeID, req.UserID, username); err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, map[string]string{
		"message": "Approver added successfully",
	})
}

// Delete godoc
// @Summary      Удалить утверждающего
// @Tags         application-approvers
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Approver ID"
// @Success      200 {object} map[string]string
// @Failure      404 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /application-approvers/{id} [delete]
func (h *ApproverHandler) Delete(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.service.Delete(c.Request().Context(), typeID, id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{
		"message": "Approver deleted successfully",
	})
}
