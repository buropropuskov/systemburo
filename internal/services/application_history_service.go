package services

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
)

// GetApplicationResponsibleUsers возвращает ответственных пользователей заявки.
func (s *applicationService) GetApplicationResponsibleUsers(ctx context.Context, applicationID int) ([]ResponsibleUserInfo, error) {
	return s.fetchResponsibleUsers(s.db.WithContext(ctx), applicationID)
}

// GetApplicationHistory возвращает историю изменений заявки.
func (s *applicationService) GetApplicationHistory(ctx context.Context, applicationID int) ([]ApplicationHistoryItem, error) {
	items := make([]ApplicationHistoryItem, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			h.id,
			h.application_id,
			h.user_id,
			CONCAT(COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
			) as user_name,
			u.last_name,
			u.first_name,
			u.middle_name,
			h.action_type,
			h.action_status,
			h.old_value,
			h.new_value,
			h.comment,
			h.created_at,
			h.metadata
		FROM application_history h
		JOIN users u ON h.user_id = u.id
		WHERE h.application_id = ?
		ORDER BY h.created_at DESC
	`, applicationID).Scan(&items).Error

	if err != nil {
		slog.Error("Ошибка получения истории заявки", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching application history")
	}

	return items, nil
}

// AddHistoryEntry добавляет запись в историю заявки.
func (s *applicationService) AddHistoryEntry(ctx context.Context, req AddHistoryEntryRequest) error {
	result := s.db.WithContext(ctx).Exec(`
		INSERT INTO application_history (application_id, user_id, action_type, action_status, old_value, new_value, comment, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, req.ApplicationID, req.UserID, req.ActionType, req.ActionStatus, req.OldValue, req.NewValue, req.Comment, req.Metadata)

	if result.Error != nil {
		slog.Error("Ошибка добавления записи истории", "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error adding history entry")
	}

	return nil
}
