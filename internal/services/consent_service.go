package services

import (
	"context"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type ConsentService interface {
	Grant(ctx context.Context, userID int, req models.GrantConsentRequest, ip, ua string) (*models.PDConsent, error)
	Revoke(ctx context.Context, userID int, consentType string) error
	List(ctx context.Context, userID int) ([]models.PDConsent, error)
	HasActive(ctx context.Context, userID int, consentType string) (bool, error)
}

type consentService struct {
	db *gorm.DB
}

// NewConsentService создаёт сервис для управления согласиями на обработку ПД.
func NewConsentService(db *gorm.DB) ConsentService {
	return &consentService{db: db}
}

// Grant выдаёт согласие на обработку персональных данных указанного типа.
func (s *consentService) Grant(ctx context.Context, userID int, req models.GrantConsentRequest, ip, ua string) (*models.PDConsent, error) {
	consent := models.PDConsent{
		UserID:      userID,
		ConsentType: req.ConsentType,
		Granted:     true,
		IPAddress:   ip,
		UserAgent:   ua,
		GrantedAt:   time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&consent).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error granting consent")
	}
	return &consent, nil
}

// Revoke отзывает активное согласие на обработку персональных данных.
func (s *consentService) Revoke(ctx context.Context, userID int, consentType string) error {
	now := time.Now()
	result := s.db.WithContext(ctx).
		Model(&models.PDConsent{}).
		Where("user_id = ? AND consent_type = ? AND revoked_at IS NULL", userID, consentType).
		Updates(map[string]interface{}{"granted": false, "revoked_at": now})
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error revoking consent")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Active consent not found")
	}
	return nil
}

// List возвращает список всех согласий пользователя.
func (s *consentService) List(ctx context.Context, userID int) ([]models.PDConsent, error) {
	var consents []models.PDConsent
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&consents).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error listing consents")
	}
	return consents, nil
}

// HasActive проверяет наличие активного согласия указанного типа у пользователя.
func (s *consentService) HasActive(ctx context.Context, userID int, consentType string) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.PDConsent{}).
		Where("user_id = ? AND consent_type = ? AND granted = true AND revoked_at IS NULL", userID, consentType).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
