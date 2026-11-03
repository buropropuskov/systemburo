package services

import (
	"context"
	"net/http"

	"systemburo/internal/apperr"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// resolveForwardSelection runs under the application's existing row lock, before
// assignments. An empty result means all only for an unrestricted actor.
func resolveForwardSelection(ctx context.Context, tx *gorm.DB, applicationID, actorID int, requested []int) ([]int, error) {
	allowed, restricted, err := resolveForwardFilterWithDB(ctx, tx, applicationID, actorID)
	if err != nil {
		return nil, err
	}
	if len(requested) == 0 && !restricted {
		return nil, nil
	}
	invalid := func() error {
		// Do not distinguish a hidden attachment from an absent/foreign ID.
		return echo.NewHTTPError(http.StatusBadRequest, "Выбранные вложения недоступны. Обновите заявку и повторите выбор")
	}
	selected := make([]int, 0, len(requested))
	seen := make(map[int]bool, len(requested))
	for _, id := range requested {
		if id <= 0 {
			return nil, invalid()
		}
		if !seen[id] {
			seen[id] = true
			selected = append(selected, id)
		}
	}
	var actual []int
	query := tx.WithContext(ctx).Raw("SELECT id FROM attachments WHERE application_id = ? ORDER BY id", applicationID)
	if len(selected) > 0 {
		query = tx.WithContext(ctx).Raw("SELECT id FROM attachments WHERE application_id = ? AND id IN ? ORDER BY id", applicationID, selected)
	}
	if err := query.Scan(&actual).Error; err != nil {
		return nil, apperr.Internal("Не удалось загрузить вложения заявки", err)
	}
	if len(requested) == 0 {
		for _, id := range actual {
			if allowed[id] {
				selected = append(selected, id)
			}
		}
	} else {
		if len(actual) != len(selected) {
			return nil, invalid()
		}
		for _, id := range selected {
			if restricted && !allowed[id] {
				return nil, invalid()
			}
		}
	}
	if len(selected) == 0 {
		return nil, invalid()
	}
	return selected, nil
}

// Snapshot prior full access before any membership INSERT. A new recipient with
// no forwarding rows must not be mistaken for an existing unrestricted viewer.
func priorFullForwardRecipients(ctx context.Context, tx *gorm.DB, applicationID int, users []ForwardUser) (map[int]bool, error) {
	ids := make([]int, 0, len(users))
	seen := make(map[int]bool, len(users))
	for _, user := range users {
		if !seen[user.UserID] {
			seen[user.UserID] = true
			ids = append(ids, user.UserID)
		}
	}
	full := make(map[int]bool)
	if len(ids) == 0 {
		return full, nil
	}
	var existing []int
	if err := tx.WithContext(ctx).Raw(`
		SELECT u.id FROM users u WHERE u.id IN ?
		AND NOT EXISTS (SELECT 1 FROM forward_attachments fa WHERE fa.application_id = ? AND fa.recipient_user_id = u.id)
		AND (u.is_super_admin
			OR EXISTS (SELECT 1 FROM applications a WHERE a.id = ? AND a.sender_user_id = u.id)
			OR EXISTS (SELECT 1 FROM application_viewers av WHERE av.application_id = ? AND av.user_id = u.id)
			OR EXISTS (SELECT 1 FROM application_responsible_users ar WHERE ar.application_id = ? AND ar.user_id = u.id)
			OR (u.is_active AND NOT u.is_banned AND EXISTS (SELECT 1 FROM application_approvers ap WHERE ap.user_id = u.id)))
	`, ids, applicationID, applicationID, applicationID, applicationID).Scan(&existing).Error; err != nil {
		return nil, apperr.Internal("Не удалось проверить доступ получателей заявки", err)
	}
	for _, id := range existing {
		full[id] = true
	}
	return full, nil
}
