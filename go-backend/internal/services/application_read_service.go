package services

import (
	"context"
	"log/slog"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// isArchived проверяет, является ли заявка архивной.
func (s *applicationService) isArchived(ctx context.Context, applicationID int) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Table("applications app").
		Joins("JOIN attachments a ON a.application_id = app.id").
		Where("app.id = ?", applicationID).
		Where("app.status IN ?", []string{models.StatusCompleted, models.StatusRejected}).
		Where("CAST(a.entry_date_to AS DATE) + INTERVAL '1 month' < NOW()").
		Count(&count).Error
	if err != nil {
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Failed to check archive status")
	}
	return count > 0, nil
}

// checkNotArchived возвращает ошибку 403, если заявка архивная.
func (s *applicationService) checkNotArchived(ctx context.Context, applicationID int) error {
	archived, err := s.isArchived(ctx, applicationID)
	if err != nil {
		return err
	}
	if archived {
		return echo.NewHTTPError(http.StatusForbidden, "Архивная заявка доступна только для чтения")
	}
	return nil
}

// MarkAsRead фиксирует прочтение заявки пользователем (идемпотентно).
func (s *applicationService) MarkAsRead(ctx context.Context, applicationID int, username string) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	// Проверяем существование заявки
	var appCount int64
	if err := s.db.WithContext(ctx).Model(&models.Application{}).Where("id = ?", applicationID).Count(&appCount).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if appCount == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}

	// ON CONFLICT DO NOTHING — идемпотентная вставка
	err = s.db.WithContext(ctx).Exec(
		"INSERT INTO application_reads (application_id, user_id) VALUES (?, ?) ON CONFLICT (application_id, user_id) DO NOTHING",
		applicationID, user.ID,
	).Error
	if err != nil {
		slog.Error("Ошибка записи прочтения", "error", err, "application_id", applicationID, "user_id", user.ID)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to mark as read")
	}

	return nil
}

// GetReads возвращает список пользователей, прочитавших заявку.
func (s *applicationService) GetReads(ctx context.Context, applicationID int) ([]models.ApplicationReadResponse, error) {
	var reads []models.ApplicationReadResponse
	err := s.db.WithContext(ctx).
		Table("application_reads ar").
		Select("ar.user_id, u.username, u.last_name, u.first_name, ar.read_at").
		Joins("JOIN users u ON ar.user_id = u.id").
		Where("ar.application_id = ?", applicationID).
		Order("ar.read_at").
		Find(&reads).Error
	if err != nil {
		slog.Error("Ошибка получения прочтений", "error", err, "application_id", applicationID)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return reads, nil
}

// GetUnreadCount возвращает количество непрочитанных активных заявок для пользователя.
func (s *applicationService) GetUnreadCount(ctx context.Context, username string) (*models.UnreadCountResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	var count int64
	// Непрочитанные = нет записи в application_reads для пользователя + не архивные
	archiveExclude := `
		NOT (
			a.status IN (?, ?) AND EXISTS(
				SELECT 1 FROM attachments att WHERE att.application_id = a.id
				AND att.entry_date_to IS NOT NULL
				AND CAST(att.entry_date_to AS DATE) + INTERVAL '1 month' < NOW()
			)
		)
	`
	err = s.db.WithContext(ctx).
		Table("applications a").
		Where("NOT EXISTS (SELECT 1 FROM application_reads ar WHERE ar.application_id = a.id AND ar.user_id = ?)", user.ID).
		Where(archiveExclude, models.StatusCompleted, models.StatusRejected).
		Count(&count).Error
	if err != nil {
		slog.Error("Ошибка подсчёта непрочитанных", "error", err, "user_id", user.ID)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	return &models.UnreadCountResponse{Count: int(count)}, nil
}
