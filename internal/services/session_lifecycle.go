package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func sessionUnavailable(cause error) error {
	return apperr.New(http.StatusServiceUnavailable, "Не удалось изменить сессию. Повторите попытку позже.", cause)
}

// Callers lock users (in ascending ID order for several users) before tokens.
func lockSessionUser(ctx context.Context, tx *gorm.DB, userID int) (*models.User, error) {
	var user models.User
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.Unauthorized("Пользователь не найден")
		}
		return nil, sessionUnavailable(err)
	}
	return &user, nil
}

// Caller holds the user lock. Empty exceptFamilyID means revoke all sessions.
func revokeUserSessions(ctx context.Context, tx *gorm.DB, userID int, exceptFamilyID string, now time.Time) (int64, error) {
	q := tx.WithContext(ctx).Model(&models.RefreshToken{}).Where("user_id = ? AND is_revoked = false", userID)
	if exceptFamilyID != "" {
		q = q.Where("(family_id IS NULL OR family_id <> ?)", exceptFamilyID)
	}
	r := q.Updates(map[string]any{"is_revoked": true, "revoked_at": now})
	if r.Error != nil {
		return 0, sessionUnavailable(r.Error)
	}
	return r.RowsAffected, nil
}

// The hash identifies an issued token; a revoked, expired or legacy token cannot
// preserve a family during password changes. Caller holds the owner's user lock.
func sessionFamilyFromRefresh(ctx context.Context, tx *gorm.DB, userID int, rawCookie string, now time.Time) (string, error) {
	if rawCookie == "" {
		return "", nil
	}
	var token models.RefreshToken
	err := tx.WithContext(ctx).Where("user_id = ? AND token_hash = ? AND is_revoked = false AND expires_at > ?", userID, hashRefreshToken(rawCookie), now).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", sessionUnavailable(err)
	}
	return token.FamilyID, nil
}

// Read fresh settings under the user lock, without taking settings row locks.
// Maintenance Enable uses the same user lock before revocation and settings.
func checkSessionMaintenance(ctx context.Context, tx *gorm.DB, user *models.User, now time.Time) error {
	if user.IsSuperAdmin {
		return nil
	}
	var settings []models.SystemSetting
	if err := tx.WithContext(ctx).Where("key IN ?", []string{keyMaintenanceEnabled, keyMaintenancePlannedEnd}).Find(&settings).Error; err != nil {
		return sessionUnavailable(err)
	}
	enabled, end := false, ""
	for _, setting := range settings {
		switch setting.Key {
		case keyMaintenanceEnabled:
			enabled = setting.Value == "true"
		case keyMaintenancePlannedEnd:
			end = setting.Value
		}
	}
	if enabled && !windowExpired(end, now) {
		return apperr.New(http.StatusServiceUnavailable, "Сервис временно недоступен: технические работы")
	}
	return nil
}

func sessionTransactionError(err error) error {
	var domain *apperr.Error
	if errors.As(err, &domain) {
		return err
	}
	return sessionUnavailable(err)
}
