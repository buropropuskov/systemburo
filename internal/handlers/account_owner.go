package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"systemburo/internal/services"

	"github.com/labstack/echo/v5"
)

type AccountOwnerHandler struct {
	store *services.AccountOwnerStore
	apps  services.ApplicationService
}

func NewAccountOwnerHandler(store *services.AccountOwnerStore, apps services.ApplicationService) *AccountOwnerHandler {
	return &AccountOwnerHandler{store: store, apps: apps}
}

func (h *AccountOwnerHandler) access(c *echo.Context) (services.AccountOwnerAccess, error) {
	c.Response().Header().Set("Cache-Control", "no-store")
	if h == nil || h.store == nil {
		return services.AccountOwnerAccess{}, nil
	}
	access, err := h.store.Access(c.Request().Context(), GetUserID(c))
	if err != nil {
		return services.AccountOwnerAccess{}, echo.NewHTTPError(http.StatusServiceUnavailable, "Не удалось проверить полномочия")
	}
	return access, nil
}

func (h *AccountOwnerHandler) Access(c *echo.Context) error {
	access, err := h.access(c)
	if err != nil {
		return err
	}
	return RespondSuccess(c, access)
}

func (h *AccountOwnerHandler) Confirmation(c *echo.Context) error {
	access, err := h.access(c)
	if err != nil {
		return err
	}
	if !access.Confirm {
		return echo.NewHTTPError(http.StatusForbidden, "Подтверждение владельца недоступно")
	}
	userID, _ := strconv.Atoi(c.Param("id"))
	status, err := h.store.ConfirmationStatus(c.Request().Context(), userID)
	if err != nil {
		return accountOwnerError(err)
	}
	return RespondSuccess(c, status)
}

type accountOwnerConfirmationRequest struct {
	FullName         string    `json:"full_name"`
	SourceRef        string    `json:"source_ref"`
	EffectiveFrom    time.Time `json:"effective_from"`
	ExpectedRevision *int      `json:"expected_revision"`
	Confirmed        bool      `json:"confirmed"`
}

func (h *AccountOwnerHandler) Confirm(c *echo.Context) error {
	access, err := h.access(c)
	if err != nil {
		return err
	}
	if !access.Confirm {
		return echo.NewHTTPError(http.StatusForbidden, "Подтверждение владельца недоступно")
	}
	var req accountOwnerConfirmationRequest
	if c.Bind(&req) != nil || req.ExpectedRevision == nil || !req.Confirmed {
		return echo.NewHTTPError(http.StatusBadRequest, "Проверьте сведения и подтвердите выдачу")
	}
	userID, _ := strconv.Atoi(c.Param("id"))
	revision, err := h.store.Confirm(c.Request().Context(), userID, GetUserID(c), services.AccountOwnerConfirmationInput{
		Evidence:      services.AccountOwnerEvidence{FullName: req.FullName, SourceRef: req.SourceRef},
		EffectiveFrom: req.EffectiveFrom, ExpectedRevision: *req.ExpectedRevision,
	})
	if err != nil {
		return accountOwnerError(err)
	}
	effectiveFrom := req.EffectiveFrom.UTC()
	return RespondSuccess(c, services.AccountOwnerConfirmationStatus{Revision: revision, EffectiveFrom: &effectiveFrom})
}

func (h *AccountOwnerHandler) Verify(c *echo.Context) error {
	access, err := h.access(c)
	if err != nil {
		return err
	}
	if !access.Verify {
		return echo.NewHTTPError(http.StatusForbidden, "Проверка владельца недоступна")
	}
	applicationID, _ := strconv.Atoi(c.Param("id"))
	eventID, _ := strconv.Atoi(c.Param("event_id"))
	if applicationID <= 0 || eventID <= 0 {
		return accountOwnerError(services.ErrInvalidAccountOwnerConfirmation)
	}
	ctx := c.Request().Context()
	allowed, err := services.CheckApplicationAccess(ctx, h.apps, applicationID, GetUsername(c), IsSuperAdmin(c))
	if err != nil {
		return err
	}
	if !allowed {
		return echo.NewHTTPError(http.StatusForbidden, "Нет доступа к заявке")
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if c.Bind(&req) != nil {
		return accountOwnerError(services.ErrInvalidAccountOwnerConfirmation)
	}
	result, err := h.store.VerifyApproval(ctx, applicationID, eventID, GetUserID(c), req.Reason)
	if err != nil {
		return accountOwnerError(err)
	}
	return RespondSuccess(c, result)
}

func accountOwnerError(err error) error {
	switch {
	case services.IsAccountOwnerRecordNotFound(err):
		return echo.NewHTTPError(http.StatusNotFound, "Запись не найдена")
	case errors.Is(err, services.ErrInvalidAccountOwnerConfirmation):
		return echo.NewHTTPError(http.StatusBadRequest, "Проверьте сведения и основание")
	case errors.Is(err, services.ErrAccountOwnerConfirmationChanged):
		return echo.NewHTTPError(http.StatusConflict, "Подтверждение изменилось. Откройте форму заново")
	case errors.Is(err, services.ErrAccountOwnerConfirmationDenied):
		return echo.NewHTTPError(http.StatusForbidden, "Нельзя подтвердить владельца этой учётной записи")
	default:
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Не удалось завершить операцию")
	}
}
