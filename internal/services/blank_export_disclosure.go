package services

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
)

// CheckOriginalDisclosure protects legacy originals whose snapshot does not
// identify every account represented in the file. Storage is not disclosure.
// With consent enforcement active their consent cannot be reliably verified.
// An outstanding withdrawal also blocks disclosure after enforcement is turned
// off; changing a setting must not undo a recorded withdrawal.
func (s *ArchiveDownloadService) CheckOriginalDisclosure(ctx context.Context) error {
	restricted, err := s.originalDisclosureRestricted(ctx)
	if err != nil {
		return err
	}
	if restricted {
		return echo.NewHTTPError(http.StatusForbidden,
			"Невозможно проверить доступность персональных данных в историческом файле. Сформируйте актуальный бланк в заявке")
	}
	return nil
}

func (s *ArchiveDownloadService) originalDisclosureRestricted(ctx context.Context) (bool, error) {
	active, err := pdConsentMaskingActive(ctx, s.db)
	if err != nil {
		return false, err
	}
	if !active {
		var restricted bool
		err = s.db.WithContext(ctx).Raw(`SELECT EXISTS (
			SELECT 1 FROM pd_consents revoked
			WHERE revoked.consent_type = ? AND revoked.revoked_at IS NOT NULL
			AND NOT EXISTS (
				SELECT 1 FROM pd_consents current_consent
				WHERE current_consent.user_id = revoked.user_id
				AND current_consent.consent_type = ?
				AND current_consent.granted = true
				AND current_consent.revoked_at IS NULL
			))`, ConsentTypePDProcessing, ConsentTypePDProcessing).Scan(&restricted).Error
		if err != nil {
			return false, nameMaskUnavailable(err)
		}
		active = restricted
	}
	return active, nil
}
