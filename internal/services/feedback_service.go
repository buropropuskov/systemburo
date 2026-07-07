package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/realtime"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// FeedbackService -- интерфейс бизнес-логики обратной связи.
// Админ-операции (page.admin.feedback) авторизуются роут-middleware RequirePermissionV2;
// Create и GetMy доступны любому авторизованному (своя обратная связь).
type FeedbackService interface {
	Create(ctx context.Context, username string, req models.CreateFeedbackRequest) (int, error)
	GetAll(ctx context.Context) ([]models.FeedbackWithUser, error)
	GetStats(ctx context.Context) (*models.FeedbackStats, error)
	GetMy(ctx context.Context, username string) ([]models.MyFeedback, error)
	UpdateStatus(ctx context.Context, id int, req models.UpdateFeedbackStatusRequest) error
	MarkAsRead(ctx context.Context, id int, req models.MarkAsReadRequest) error
}

type feedbackService struct {
	db                *gorm.DB
	realtimePublisher realtime.Publisher
}

// FeedbackServiceOption конфигурирует feedbackService при создании.
type FeedbackServiceOption func(*feedbackService)

// WithFeedbackRealtimePublisher включает real-time сигнал feedback.new при новом
// обращении (#840): бейдж обратной связи у супер-админов обновляется мгновенно,
// не дожидаясь 30с-опроса. Опционально.
func WithFeedbackRealtimePublisher(p realtime.Publisher) FeedbackServiceOption {
	return func(s *feedbackService) { s.realtimePublisher = p }
}

// NewFeedbackService создаёт реализацию FeedbackService.
func NewFeedbackService(db *gorm.DB, opts ...FeedbackServiceOption) FeedbackService {
	s := &feedbackService{db: db}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// notifyFeedbackChanged шлёт feedback.new аудитории бейджа обратной связи -
// активным супер-админам (FE гейтит бейдж по isSuperAdmin). Best-effort, nil-safe.
func (s *feedbackService) notifyFeedbackChanged(ctx context.Context) {
	if s.realtimePublisher == nil {
		return
	}
	var ids []int
	if err := s.db.WithContext(ctx).
		Table("users").
		Where("is_active = ? AND is_super_admin = ?", true, true).
		Pluck("id", &ids).Error; err != nil {
		slog.Warn("feedback.new: load super admins failed", "err", err)
		return
	}
	if len(ids) == 0 {
		return
	}
	s.realtimePublisher.PublishMany(ids, realtime.Event{Type: "feedback.new", Scope: "feedback"})
}

// getUserIDByUsername возвращает ID пользователя по username.
func (s *feedbackService) getUserIDByUsername(ctx context.Context, username string) (int, error) {
	var userID int
	err := s.db.WithContext(ctx).
		Table("users").
		Select("id").
		Where("username = ?", username).
		Row().
		Scan(&userID)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}
	return userID, nil
}

// Create создаёт новое обращение обратной связи.
func (s *feedbackService) Create(ctx context.Context, username string, req models.CreateFeedbackRequest) (int, error) {
	userID, err := s.getUserIDByUsername(ctx, username)
	if err != nil {
		return 0, err
	}

	trimmed := strings.TrimSpace(req.Message)

	if len(trimmed) < 10 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Message must be at least 10 characters")
	}
	if len(req.Message) > 1000 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Message cannot exceed 1000 characters")
	}

	now := time.Now().UTC()
	feedback := models.Feedback{
		UserID:    userID,
		Message:   trimmed,
		Status:    models.FeedbackOpen,
		IsRead:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.db.WithContext(ctx).Create(&feedback).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error creating feedback")
	}

	s.notifyFeedbackChanged(ctx)
	return feedback.ID, nil
}

// GetAll возвращает все обращения обратной связи с информацией о пользователях.
func (s *feedbackService) GetAll(ctx context.Context) ([]models.FeedbackWithUser, error) {
	results := make([]models.FeedbackWithUser, 0)
	err := s.db.WithContext(ctx).
		Table("feedback f").
		Select(`f.id, f.user_id,
			CONCAT(u.last_name, ' ', u.first_name) AS user_name,
			f.message, f.status, f.is_read, f.created_at, f.updated_at,
			f.resolution_comment, f.resolved_at`).
		Joins("JOIN users u ON f.user_id = u.id").
		Order("f.created_at DESC").
		Scan(&results).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching feedback")
	}

	// Замена пустых имён на значение по умолчанию (как в Rust)
	for i := range results {
		if strings.TrimSpace(results[i].UserName) == "" {
			results[i].UserName = "Неизвестный пользователь"
		}
	}

	return results, nil
}

// GetStats возвращает статистику обращений обратной связи.
func (s *feedbackService) GetStats(ctx context.Context) (*models.FeedbackStats, error) {
	var stats models.FeedbackStats
	err := s.db.WithContext(ctx).
		Table("feedback").
		Select(`COUNT(*) AS total,
			COUNT(CASE WHEN status = ? THEN 1 END) AS resolved,
			COUNT(CASE WHEN status = ? THEN 1 END) AS unresolved,
			COUNT(CASE WHEN is_read = false THEN 1 END) AS unread`,
			models.FeedbackResolved, models.FeedbackOpen).
		Row().
		Scan(&stats.Total, &stats.Resolved, &stats.Unresolved, &stats.Unread)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching feedback stats")
	}

	return &stats, nil
}

// GetMy возвращает обращения текущего пользователя.
func (s *feedbackService) GetMy(ctx context.Context, username string) ([]models.MyFeedback, error) {
	userID, err := s.getUserIDByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	results := make([]models.MyFeedback, 0)
	err = s.db.WithContext(ctx).
		Table("feedback").
		Select("id, user_id, message, status, is_read, created_at, updated_at").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Scan(&results).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user feedback")
	}

	return results, nil
}

// UpdateStatus обновляет статус обращения обратной связи.
func (s *feedbackService) UpdateStatus(ctx context.Context, id int, req models.UpdateFeedbackStatusRequest) error {
	// Проверяем существование обращения
	var count int64
	if err := s.db.WithContext(ctx).Table("feedback").Where("id = ?", id).Count(&count).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking feedback existence")
	}
	if count == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Feedback not found")
	}

	// Проверяем допустимость статуса
	if req.Status != models.FeedbackResolved && req.Status != models.FeedbackOpen {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid status. Must be '%s' or '%s'", models.FeedbackResolved, models.FeedbackOpen))
	}

	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":     req.Status,
		"updated_at": now,
	}
	if req.Status == models.FeedbackResolved {
		updates["resolved_at"] = now
		if req.Comment != nil {
			if trimmed := strings.TrimSpace(*req.Comment); trimmed != "" {
				updates["resolution_comment"] = trimmed
			}
		}
	} else {
		// Возврат в работу очищает ответ и дату решения.
		updates["resolved_at"] = nil
		updates["resolution_comment"] = nil
	}

	if err := s.db.WithContext(ctx).
		Table("feedback").
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating feedback status")
	}

	return nil
}

// MarkAsRead отмечает обращение обратной связи как прочитанное или непрочитанное.
func (s *feedbackService) MarkAsRead(ctx context.Context, id int, req models.MarkAsReadRequest) error {
	// Обновляем только is_read без updated_at (как в Rust)
	err := s.db.WithContext(ctx).
		Table("feedback").
		Where("id = ?", id).
		Update("is_read", req.IsRead).Error
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating feedback read status")
	}

	return nil
}
