package handlers

import (
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

var allowedConsentTypes = map[string]bool{
	"pd_processing": true,
	"pd_transfer":   true,
}

type ConsentHandler struct {
	service  services.ConsentService
	gate     *services.PDConsentGateService
	settings services.SettingsService
	db       *gorm.DB
}

// NewConsentHandler создаёт хендлер для управления согласиями на обработку ПД.
func NewConsentHandler(
	service services.ConsentService,
	gate *services.PDConsentGateService,
	settings services.SettingsService,
	db *gorm.DB,
) *ConsentHandler {
	return &ConsentHandler{service: service, gate: gate, settings: settings, db: db}
}

// Grant обрабатывает запрос на предоставление согласия на обработку ПД.
func (h *ConsentHandler) Grant(c echo.Context) error {
	var req models.GrantConsentRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	userID, err := h.resolveUserID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()

	// Редакцию и хэш ставим из настроек, а не из запроса. Для согласия на передачу
	// данных редакция текста обработки не имеет смысла - остаётся 1 без хэша.
	version, hash := 1, ""
	if req.ConsentType == services.ConsentTypePDProcessing {
		gateReq, gateErr := h.gate.Requirement(ctx)
		if gateErr != nil {
			return gateErr
		}
		version, hash = gateReq.Version, gateReq.Hash
	}

	consent, err := h.service.Grant(ctx, userID, req, c.RealIP(), c.Request().UserAgent(), version, hash)
	if err != nil {
		return err
	}
	h.gate.Invalidate(userID)
	return RespondSuccess(c, consent)
}

// Revoke обрабатывает запрос на отзыв согласия.
func (h *ConsentHandler) Revoke(c echo.Context) error {
	consentType := c.Param("type")
	if !allowedConsentTypes[consentType] {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid consent type")
	}
	userID, err := h.resolveUserID(c)
	if err != nil {
		return err
	}
	if err := h.service.Revoke(c.Request().Context(), userID, consentType); err != nil {
		return err
	}
	// Отзыв обязан закрыть доступ сразу, а не по истечении TTL кэша.
	h.gate.Invalidate(userID)
	return RespondMessage(c, "Consent revoked")
}

// List возвращает список согласий текущего пользователя.
func (h *ConsentHandler) List(c echo.Context) error {
	userID, err := h.resolveUserID(c)
	if err != nil {
		return err
	}
	consents, err := h.service.List(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, consents)
}

// Check проверяет наличие активного согласия указанного типа.
func (h *ConsentHandler) Check(c echo.Context) error {
	consentType := c.Param("type")
	if !allowedConsentTypes[consentType] {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid consent type")
	}
	userID, err := h.resolveUserID(c)
	if err != nil {
		return err
	}
	active, err := h.service.HasActive(c.Request().Context(), userID, consentType)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]bool{"active": active})
}

func (h *ConsentHandler) resolveUserID(c echo.Context) (int, error) {
	username := GetUsername(c)
	var user models.User
	if err := h.db.Where("username = ?", username).First(&user).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}
	return user.ID, nil
}
