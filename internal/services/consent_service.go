package services

import (
	"context"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ConsentTypePDProcessing -- согласие на обработку персональных данных, которое
// запрашивается при первом входе (#1567).
const ConsentTypePDProcessing = "pd_processing"

type ConsentService interface {
	AcceptProcessing(ctx context.Context, userID int, shown models.PDConsentRevision, ip, ua string, onlyIfRequired bool) (PDConsentRequirement, *models.PDConsent, error)
	Grant(ctx context.Context, userID int, req models.GrantConsentRequest, ip, ua string, version int, hash string) (*models.PDConsent, error)
	Revoke(ctx context.Context, userID int, consentType string, actorID int) error
	List(ctx context.Context, userID int) ([]models.PDConsent, error)
	HasActive(ctx context.Context, userID int, consentType string) (bool, error)
	ActiveVersion(ctx context.Context, userID int, consentType string) (int, error)
	IsRevoked(ctx context.Context, userID int) (bool, error)
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

// Grant выдаёт согласие на обработку персональных данных указанного типа. Учебный генератор передаёт серверный
// снимок напрямую; запросы пользователей на обработку идут через AcceptProcessing.
func (s *consentService) Grant(ctx context.Context, userID int, req models.GrantConsentRequest, ip, ua string, version int, hash string) (*models.PDConsent, error) {
	var consent *models.PDConsent
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockConsentSubject(tx, userID); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error granting consent").SetInternal(err)
		}
		var err error
		consent, err = insertConsent(tx, userID, req.ConsentType, ip, ua, version, hash)
		return err
	})
	if err == nil {
		s.logGrant(ctx, consent)
	}
	return consent, err
}

// AcceptProcessing сравнивает предъявленный текст и записывает согласие до того,
// как издатель сможет изменить документ, в том числе на другой реплике.
func (s *consentService) AcceptProcessing(ctx context.Context, userID int, shown models.PDConsentRevision, ip, ua string, onlyIfRequired bool) (PDConsentRequirement, *models.PDConsent, error) {
	var req PDConsentRequirement
	var consent *models.PDConsent
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock_shared(?)", pdConsentRevisionLock).Error; err != nil {
			return err
		}
		settings, err := readConsentSettings(tx)
		if err != nil {
			return err
		}
		req = consentRequirement(settings)
		if onlyIfRequired && !req.Enabled {
			return nil
		}
		if !hasVisibleText(req.Text) || shown.Version != req.Version || shown.Hash != req.Hash {
			return echo.NewHTTPError(http.StatusConflict, "Обновите страницу и прочитайте актуальную редакцию согласия.")
		}
		if err := lockConsentSubject(tx, userID); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error granting consent").SetInternal(err)
		}
		consent, err = insertConsent(tx, userID, ConsentTypePDProcessing, ip, ua, req.Version, req.Hash)
		return err
	})
	if err != nil {
		return PDConsentRequirement{}, nil, err
	}
	if consent != nil {
		s.logGrant(ctx, consent)
	}
	return req, consent, nil
}

// lockConsentSubject serializes consent writes with administrative updates, which
// already hold the same users row through lockAdminTarget. It reads only the ID.
// The document lock, when needed, is acquired first; publishers never lock users.
func lockConsentSubject(tx *gorm.DB, userID int) error {
	var subject struct{ ID int }
	return tx.Table("users").Select("id").Where("id = ?", userID).
		Clauses(clause.Locking{Strength: "UPDATE"}).Take(&subject).Error
}

func insertConsent(db *gorm.DB, userID int, consentType, ip, ua string, version int, hash string) (*models.PDConsent, error) {
	if version < 1 {
		version = 1
	}
	consent := models.PDConsent{
		UserID:          userID,
		ConsentType:     consentType,
		Granted:         true,
		IPAddress:       ip,
		UserAgent:       ua,
		GrantedAt:       time.Now().UTC(),
		DocumentVersion: version,
		DocumentHash:    hash,
	}
	if err := db.Create(&consent).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error granting consent")
	}
	return &consent, nil
}

func (s *consentService) logGrant(ctx context.Context, consent *models.PDConsent) {
	userID := consent.UserID
	// Актор - сам работник: согласие даёт он, а не администратор.
	s.recorder.Log(ctx, nil, models.AuditEntityUser, &userID, models.UserActionConsentGranted, &userID, map[string]any{
		"consent_type": consent.ConsentType,
		"version":      consent.DocumentVersion,
	})
}

// Revoke отзывает активное согласие на обработку персональных данных. actorID -
// кто отзывает: сам работник или администратор по его обращению; в историю
// учётной записи попадает именно он.
func (s *consentService) Revoke(ctx context.Context, userID int, consentType string, actorID int) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockConsentSubject(tx, userID); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error revoking consent").SetInternal(err)
		}
		result := tx.Model(&models.PDConsent{}).
			Where("user_id = ? AND consent_type = ? AND revoked_at IS NULL", userID, consentType).
			Updates(map[string]interface{}{"granted": false, "revoked_at": time.Now().UTC()})
		if result.Error != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error revoking consent").SetInternal(result.Error)
		}
		if result.RowsAffected == 0 {
			return echo.NewHTTPError(http.StatusNotFound, "Active consent not found")
		}
		return nil
	})
	if err != nil {
		return err
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

// IsRevoked сообщает, отозвал ли работник согласие на обработку своих данных.
//
// Отзыв и отсутствие согласия - разные состояния, и различать их обязательно.
// Новый работник согласия ещё не давал: его карточку заводят и правят обычным
// порядком. Отозвавший согласие - другое дело: обработка по прежним целям
// прекращена, и данные с этого момента только хранятся. По закону это называется
// блокированием, и правка данных в таком состоянии недопустима (#2361).
//
// Новое согласие снимает блокирование само: появляется действующая запись, и
// функция возвращает false, ничего дополнительно отменять не требуется.
func consentRevoked(ctx context.Context, db *gorm.DB, userID int) (bool, error) {
	var active int64
	if err := db.WithContext(ctx).Model(&models.PDConsent{}).
		Where("user_id = ? AND consent_type = ? AND granted = ? AND revoked_at IS NULL", userID, ConsentTypePDProcessing, true).
		Count(&active).Error; err != nil {
		return false, err
	}
	if active > 0 {
		return false, nil
	}

	var revoked int64
	if err := db.WithContext(ctx).Model(&models.PDConsent{}).
		Where("user_id = ? AND consent_type = ? AND revoked_at IS NOT NULL", userID, ConsentTypePDProcessing).
		Count(&revoked).Error; err != nil {
		return false, err
	}
	return revoked > 0, nil
}

// IsRevoked - тот же вопрос через сервис, для вызывающих за пределами пакета.
func (s *consentService) IsRevoked(ctx context.Context, userID int) (bool, error) {
	return consentRevoked(ctx, s.db, userID)
}
