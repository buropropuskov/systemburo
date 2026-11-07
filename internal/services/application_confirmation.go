package services

import (
	"context"
	"errors"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ApplicationConfirmationInput is shared by submission and supplement requests.
type ApplicationConfirmationInput struct {
	DocumentRevision models.PDConsentRevision `json:"document_revision,omitzero"`
}

// CurrentApplicationConfirmationRevision reads the revision for synthetic submissions.
func CurrentApplicationConfirmationRevision(ctx context.Context, db *gorm.DB) (models.PDConsentRevision, error) {
	settings, err := readConsentSettings(db.WithContext(ctx))
	if err != nil {
		return models.PDConsentRevision{}, err
	}
	current := consentRequirement(settings)
	if !hasVisibleText(current.Text) {
		return models.PDConsentRevision{}, errors.New("документ обработки данных не настроен")
	}
	return models.PDConsentRevision{Version: current.Version, Hash: current.Hash}, nil
}

func loadApplicationConfirmationDocument(tx *gorm.DB, approved bool, shown models.PDConsentRevision) (models.ApplicationConfirmationDocument, error) {
	if !approved {
		return models.ApplicationConfirmationDocument{}, apperr.Validation("Подтвердите правомерность передачи данных")
	}
	if err := tx.Exec("SELECT pg_advisory_xact_lock_shared(?)", pdConsentRevisionLock).Error; err != nil {
		return models.ApplicationConfirmationDocument{}, err
	}
	settings, err := readConsentSettings(tx)
	if err != nil {
		return models.ApplicationConfirmationDocument{}, err
	}
	current := consentRequirement(settings)
	if !hasVisibleText(current.Text) || shown.Version != current.Version || shown.Hash != current.Hash {
		return models.ApplicationConfirmationDocument{}, apperr.Conflict("Подтвердите актуальную редакцию документа").WithHeader("X-Document-Revision-Changed", "true")
	}
	return models.ApplicationConfirmationDocument{Version: current.Version, Hash: current.Hash, Text: current.Text}, nil
}

func saveApplicationConfirmation(tx *gorm.DB, document models.ApplicationConfirmationDocument, applicationID int, supplementID *int, actorID int, at time.Time, rows []models.ApplicationConfirmedRow) error {
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&document).Error; err != nil {
		return err
	}
	return tx.Create(&models.ApplicationConfirmation{
		ApplicationID:   applicationID,
		SupplementID:    supplementID,
		ActorUserID:     &actorID,
		ConfirmedAt:     at,
		DocumentVersion: document.Version,
		DocumentHash:    document.Hash,
		Rows:            rows,
	}).Error
}

func applicationConfirmedRows(tx *gorm.DB, applicationID int, supplementID *int) ([]models.ApplicationConfirmedRow, error) {
	rows := make([]models.ApplicationConfirmedRow, 0)
	err := tx.Raw(`
		WITH application_attachments AS (
			SELECT id FROM attachments WHERE application_id = ?
		), operation_rows AS (
			SELECT 'car' AS kind, id, supplement_id FROM cars WHERE attachment_id IN (SELECT id FROM application_attachments)
			UNION ALL
			SELECT 'employee', id, supplement_id FROM employees WHERE attachment_id IN (SELECT id FROM application_attachments)
			UNION ALL
			SELECT 'item', id, supplement_id FROM items WHERE attachment_id IN (SELECT id FROM application_attachments)
		)
		SELECT kind, id FROM operation_rows
		WHERE supplement_id IS NOT DISTINCT FROM CAST(? AS integer)
		ORDER BY kind, id
	`, applicationID, supplementID).Scan(&rows).Error
	return rows, err
}
