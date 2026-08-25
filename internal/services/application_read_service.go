package services

import (
	"context"
	"log/slog"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// isArchived проверяет, является ли заявка архивной (определение - application_archive.go).
func (s *applicationService) isArchived(ctx context.Context, applicationID int) (bool, error) {
	var count int64
	cond, args := archivedApplicationCond("app")
	err := s.db.WithContext(ctx).
		Table("applications app").
		Where("app.id = ?", applicationID).
		Where(cond, args...).
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

// checkNotWithdrawn возвращает ошибку 409, если заявка отозвана отправителем (#951).
// Отзыв необратим штатными средствами: над "Отозвана" нельзя совершать рабочие и
// согласовательные действия (принять в работу, согласовать, отказать и обратные им).
func (s *applicationService) checkNotWithdrawn(ctx context.Context, applicationID int) error {
	var app struct{ Status *string }
	if err := s.db.WithContext(ctx).Raw("SELECT status FROM applications WHERE id = ?", applicationID).Scan(&app).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to check application status")
	}
	if app.Status != nil && *app.Status == models.StatusWithdrawn {
		return echo.NewHTTPError(http.StatusConflict, "Заявка отозвана отправителем - действия недоступны")
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

// MarkStatusSeen помечает текущий статус заявки просмотренным пользователем (#1349): гасит
// его флаг "статус обновился". Идемпотентно, зовётся при каждом открытии детали заявки
// (GET /:id/details); при смене статуса флаг загорится снова - status_updated_at станет
// позже seen_at.
func (s *applicationService) MarkStatusSeen(ctx context.Context, username string, applicationID int) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Exec(statusViewUpsert, applicationID, user.ID).Error; err != nil {
		slog.Error("Ошибка отметки просмотра статуса", "application_id", applicationID, "user_id", user.ID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to mark status seen")
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
	if masks := loadConsentMasks(ctx, s.db); len(masks) > 0 {
		for i := range reads {
			var middle *string
			maskUserParts(masks, reads[i].UserID, &reads[i].LastName, &reads[i].FirstName, &middle)
		}
	}
	return reads, nil
}

// GetUnreadCount возвращает количество непрочитанных активных заявок для пользователя.
// Учитывается доступ пользователя: если юзер не approver, считаются только заявки,
// где он responsible_user или viewer (тот же фильтр, что в листинге).
func (s *applicationService) GetUnreadCount(ctx context.Context, username string) (*models.UnreadCountResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	var count int64
	// Непрочитанные = нет записи в application_reads для пользователя + не архивные
	// (архив считаем тем же предикатом, что и листинг - application_archive.go).
	activeCond, activeArgs := activeApplicationCond("a")
	query := s.db.WithContext(ctx).
		Table("applications a").
		Where("NOT EXISTS (SELECT 1 FROM application_reads ar WHERE ar.application_id = a.id AND ar.user_id = ?)", user.ID).
		Where(activeCond, activeArgs...)

	// Permission filter: совпадает с GetApplications (см. application_service.go).
	// Если юзер не approver, видит только заявки, где он responsible или viewer.
	query = applyApplicationAccessFilter(query, user.ID, isApprover)

	if err := query.Count(&count).Error; err != nil {
		slog.Error("Ошибка подсчёта непрочитанных", "error", err, "user_id", user.ID)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	// Обновления статуса (#1349): ПРОЧИТАННЫЕ заявки того же скоупа (EXISTS ar), статус/
	// подтверждение которых менялись после последнего просмотра пользователем. Отличие от
	// unread-запроса: EXISTS(ar) вместо NOT EXISTS + предикат hasStatusUpdatePredicate.
	var statusUpdates int64
	suQuery := s.db.WithContext(ctx).
		Table("applications a").
		Where("EXISTS (SELECT 1 FROM application_reads ar WHERE ar.application_id = a.id AND ar.user_id = ?)", user.ID).
		Where(activeCond, activeArgs...).
		Where(hasStatusUpdatePredicate, user.ID, user.ID)
	suQuery = applyApplicationAccessFilter(suQuery, user.ID, isApprover)
	if err := suQuery.Count(&statusUpdates).Error; err != nil {
		slog.Error("Ошибка подсчёта обновлений статуса", "error", err, "user_id", user.ID)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	return &models.UnreadCountResponse{Count: int(count), StatusUpdates: int(statusUpdates)}, nil
}

// GetUserStatusUpdatesCount возвращает число заявок ЛК с обновлённым статусом (#1349): scope
// ЛК (applyUserApplicationsAccessFilter - sender или заявки его организации) + активные +
// предикат обновления. БЕЗ гейта прочтения (у отправителя нет строк application_reads).
// Отдельный от Центра эндпоинт: у ЛК другая матрица доступа, чем у approver/viewer.
func (s *applicationService) GetUserStatusUpdatesCount(ctx context.Context, username string) (*models.StatusUpdatesCountResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	activeCond, activeArgs := activeApplicationCond("a")
	query := s.db.WithContext(ctx).
		Table("applications a").
		Where(activeCond, activeArgs...).
		Where(hasStatusUpdatePredicate, user.ID, user.ID)
	query = applyUserApplicationsAccessFilter(query, user.ID, user.OrganizationID)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		slog.Error("Ошибка подсчёта обновлений статуса ЛК", "error", err, "user_id", user.ID)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	return &models.StatusUpdatesCountResponse{StatusUpdates: int(count)}, nil
}
