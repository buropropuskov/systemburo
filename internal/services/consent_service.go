package services

import (
	"context"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// ConsentTypePDProcessing -- согласие на обработку персональных данных, которое
// запрашивается при первом входе (#1567).
const ConsentTypePDProcessing = "pd_processing"

type ConsentService interface {
	Grant(ctx context.Context, userID int, req models.GrantConsentRequest, ip, ua string, version int, hash string) (*models.PDConsent, error)
	Revoke(ctx context.Context, userID int, consentType string, actorID int) error
	List(ctx context.Context, userID int) ([]models.PDConsent, error)
	HasActive(ctx context.Context, userID int, consentType string) (bool, error)
	ActiveVersion(ctx context.Context, userID int, consentType string) (int, error)
}

type consentService struct {
	db *gorm.DB
	// recorder пишет выдачу и отзыв согласия в историю учётной записи. Через Log,
	// а не Record: провал записи в журнал не должен отменять само согласие -
	// иначе человек останется заперт в окне из-за сбоя аудита.
	recorder AuditRecorder
}

// NewConsentService создаёт сервис для управления согласиями на обработку ПД.
func NewConsentService(db *gorm.DB) ConsentService {
	return &consentService{db: db, recorder: NewAuditRecorder(db)}
}

// Grant выдаёт согласие на обработку персональных данных указанного типа. Редакцию
// и хэш текста передаёт вызывающий, взяв их из настроек: клиент на них не влияет.
func (s *consentService) Grant(ctx context.Context, userID int, req models.GrantConsentRequest, ip, ua string, version int, hash string) (*models.PDConsent, error) {
	if version < 1 {
		version = 1
	}
	consent := models.PDConsent{
		UserID:          userID,
		ConsentType:     req.ConsentType,
		Granted:         true,
		IPAddress:       ip,
		UserAgent:       ua,
		GrantedAt:       time.Now().UTC(),
		DocumentVersion: version,
		DocumentHash:    hash,
	}
	if err := s.db.WithContext(ctx).Create(&consent).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error granting consent")
	}
	// Актор - сам работник: согласие даёт он, а не администратор.
	s.recorder.Log(ctx, nil, models.AuditEntityUser, &userID, models.UserActionConsentGranted, &userID, map[string]any{
		"consent_type": req.ConsentType,
		"version":      consent.DocumentVersion,
	})
	return &consent, nil
}

// Revoke отзывает активное согласие на обработку персональных данных. actorID -
// кто отзывает: сам работник или администратор по его обращению; в историю
// учётной записи попадает именно он.
func (s *consentService) Revoke(ctx context.Context, userID int, consentType string, actorID int) error {
	now := time.Now().UTC()
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
	details := map[string]any{"consent_type": consentType}
	if actorID != userID {
		// Отзыв «за работника» - отдельный случай: в истории должно быть видно, что
		// это сделал администратор, а не сам человек передумал.
		details["by_admin"] = true
	}
	s.recorder.Log(ctx, nil, models.AuditEntityUser, &userID, models.UserActionConsentRevoked, &actorID, details)
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

// ActiveVersion возвращает максимальную редакцию среди действующих согласий
// пользователя указанного типа; 0 означает "действующего согласия нет". По ней гейт
// решает, устроит ли его прежнее согласие после подъёма редакции текста.
func (s *consentService) ActiveVersion(ctx context.Context, userID int, consentType string) (int, error) {
	var version *int
	err := s.db.WithContext(ctx).Model(&models.PDConsent{}).
		Where("user_id = ? AND consent_type = ? AND granted = true AND revoked_at IS NULL", userID, consentType).
		Select("MAX(document_version)").
		Scan(&version).Error
	if err != nil {
		return 0, err
	}
	if version == nil {
		return 0, nil
	}
	return *version, nil
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
