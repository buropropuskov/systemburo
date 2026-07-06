package services

import (
	"context"
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/realtime"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// NotificationService -- интерфейс бизнес-логики уведомлений.
type NotificationService interface {
	GetByUserID(ctx context.Context, userID int) ([]models.Notification, error)
	MarkRead(ctx context.Context, userID int, id int, req models.MarkNotificationReadRequest) (*models.Notification, error)
	Delete(ctx context.Context, userID int, id int) error
	DeleteAll(ctx context.Context, userID int) error
	Create(ctx context.Context, req models.CreateNotificationRequest) (*models.Notification, error)
	CreateForUser(ctx context.Context, userID int, notifType, title, message string, data *string) error
}

type notificationService struct {
	db                *gorm.DB
	realtimePublisher realtime.Publisher
}

// NotificationServiceOption конфигурирует notificationService при создании.
type NotificationServiceOption func(*notificationService)

// WithNotificationRealtimePublisher включает публикацию real-time сигнала
// "новое уведомление" (#840) адресно юзеру, чтобы фронт мгновенно перезапросил
// колокольчик вместо ожидания 30с-поллинга. Опционально: без неё сигналы не
// шлются (тесты, offline).
func WithNotificationRealtimePublisher(p realtime.Publisher) NotificationServiceOption {
	return func(s *notificationService) { s.realtimePublisher = p }
}

// NewNotificationService создаёт реализацию NotificationService.
func NewNotificationService(db *gorm.DB, opts ...NotificationServiceOption) NotificationService {
	s := &notificationService{db: db}
	for _, opt := range opts {
		opt(s)
	}
	return s
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

// Create создаёт уведомление (admin endpoint + внутренние триггеры).
func (s *notificationService) Create(ctx context.Context, req models.CreateNotificationRequest) (*models.Notification, error) {
	if req.UserID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "user_id is required")
	}
	if req.Title == nil || *req.Title == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "title is required")
	}
	n := models.Notification{
		UserID:  req.UserID,
		Type:    req.Type,
		Title:   req.Title,
		Message: req.Message,
		Data:    req.Data,
	}
	if err := s.db.WithContext(ctx).Create(&n).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating notification")
	}
	return &n, nil
}

// CreateForUser -- helper для триггеров из других сервисов.
// Ошибки логируются но не прерывают основной flow (уведомления не должны блокировать бизнес-операции).
func (s *notificationService) CreateForUser(ctx context.Context, userID int, notifType, title, message string, data *string) error {
	if userID <= 0 || title == "" {
		return nil
	}
	t := notifType
	ti := title
	m := message
	n := models.Notification{
		UserID:  userID,
		Type:    &t,
		Title:   &ti,
		Message: &m,
		Data:    data,
	}
	if err := s.db.WithContext(ctx).Create(&n).Error; err != nil {
		return err
	}
	if s.realtimePublisher != nil {
		s.realtimePublisher.Publish(userID, realtime.Event{Type: "notification.new", Scope: "notifications"})
	}
	return nil
}
