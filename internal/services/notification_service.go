package services

import (
	"context"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// NotificationService -- интерфейс бизнес-логики уведомлений.
type NotificationService interface {
	GetByUserID(ctx context.Context, userID int) ([]models.Notification, error)
	MarkRead(ctx context.Context, userID int, id int, req models.MarkNotificationReadRequest) (*models.Notification, error)
	Delete(ctx context.Context, userID int, id int) error
	DeleteAll(ctx context.Context, userID int) error
}

type notificationService struct {
	db *gorm.DB
}

// NewNotificationService создаёт реализацию NotificationService.
func NewNotificationService(db *gorm.DB) NotificationService {
	return &notificationService{db: db}
}

func (s *notificationService) GetByUserID(ctx context.Context, userID int) ([]models.Notification, error) {
	notifications := make([]models.Notification, 0)
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&notifications).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching notifications")
	}
	return notifications, nil
}

func (s *notificationService) findOwned(ctx context.Context, userID int, id int) (*models.Notification, error) {
	var n models.Notification
	err := s.db.WithContext(ctx).First(&n, id).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Notification not found")
	}
	if n.UserID != userID {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}
	return &n, nil
}

func (s *notificationService) MarkRead(ctx context.Context, userID int, id int, req models.MarkNotificationReadRequest) (*models.Notification, error) {
	n, err := s.findOwned(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).Model(n).Update("is_read", req.IsRead).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating notification")
	}

	n.IsRead = req.IsRead
	return n, nil
}

func (s *notificationService) Delete(ctx context.Context, userID int, id int) error {
	_, err := s.findOwned(ctx, userID, id)
	if err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Delete(&models.Notification{}, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting notification")
	}
	return nil
}

func (s *notificationService) DeleteAll(ctx context.Context, userID int) error {
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.Notification{}).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting notifications")
	}
	return nil
}
