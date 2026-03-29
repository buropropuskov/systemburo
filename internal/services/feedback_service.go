package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// FeedbackService -- интерфейс бизнес-логики обратной связи.
type FeedbackService interface {
	Create(ctx context.Context, username string, req models.CreateFeedbackRequest) (int, error)
	GetAll(ctx context.Context, typeID int) ([]models.FeedbackWithUser, error)
	GetStats(ctx context.Context, typeID int) (*models.FeedbackStats, error)
	GetMy(ctx context.Context, username string) ([]models.MyFeedback, error)
	UpdateStatus(ctx context.Context, typeID int, id int, req models.UpdateFeedbackStatusRequest) error
	MarkAsRead(ctx context.Context, typeID int, id int, req models.MarkAsReadRequest) error
}

type feedbackService struct {
	db *gorm.DB
}

// NewFeedbackService создаёт реализацию FeedbackService.
func NewFeedbackService(db *gorm.DB) FeedbackService {
	return &feedbackService{db: db}
}

// checkAdmin проверяет, что пользователь является администратором.
func (s *feedbackService) checkAdmin(ctx context.Context, typeID int) error {
	var code string
	err := s.db.WithContext(ctx).
		Table("user_types").
		Select("code").
		Where("id = ?", typeID).
		Row().
		Scan(&code)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}
	if code != "manager" && code != "buropropuskov" {
		return echo.NewHTTPError(http.StatusForbidden, "Insufficient permissions")
	}
	return nil
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
		Status:    "Нерешено",
		IsRead:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.db.WithContext(ctx).Create(&feedback).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error creating feedback")
	}

	return feedback.ID, nil
}

func (s *feedbackService) GetAll(ctx context.Context, typeID int) ([]models.FeedbackWithUser, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

	results := make([]models.FeedbackWithUser, 0)
	err := s.db.WithContext(ctx).
		Table("feedback f").
		Select(`f.id, f.user_id,
			CONCAT(u.last_name, ' ', u.first_name) AS user_name,
			f.message, f.status, f.is_read, f.created_at, f.updated_at`).
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

func (s *feedbackService) GetStats(ctx context.Context, typeID int) (*models.FeedbackStats, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

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

func (s *feedbackService) UpdateStatus(ctx context.Context, typeID int, id int, req models.UpdateFeedbackStatusRequest) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

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
	err := s.db.WithContext(ctx).
		Table("feedback").
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     req.Status,
			"updated_at": now,
		}).Error
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating feedback status")
	}

	return nil
}

func (s *feedbackService) MarkAsRead(ctx context.Context, typeID int, id int, req models.MarkAsReadRequest) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

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
