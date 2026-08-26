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
	GetAll(ctx context.Context, username string) ([]models.FeedbackWithUser, error)
	GetStats(ctx context.Context, username string) (*models.FeedbackStats, error)
	GetMy(ctx context.Context, username string) ([]models.MyFeedback, error)
	UpdateStatus(ctx context.Context, actorUserID int, id int, req models.UpdateFeedbackStatusRequest) error
	MarkAsRead(ctx context.Context, id int, username string) error
	SetFlag(ctx context.Context, id int, flagged bool) error
}

type feedbackService struct {
	db                  *gorm.DB
	realtimePublisher   realtime.Publisher
	notificationService NotificationService
	permissionResolver  *PermissionResolver
}

// FeedbackServiceOption конфигурирует feedbackService при создании.
type FeedbackServiceOption func(*feedbackService)

// WithFeedbackRealtimePublisher включает real-time сигнал feedback.new при новом
// обращении (#840): бейдж обратной связи у супер-админов обновляется мгновенно,
// не дожидаясь 30с-опроса. Опционально.
func WithFeedbackRealtimePublisher(p realtime.Publisher) FeedbackServiceOption {
	return func(s *feedbackService) { s.realtimePublisher = p }
}

// WithFeedbackNotifications включает персональные уведомления feedback_created/
// feedback_answered (#1748). Опционально: без неё уведомления не шлются (тесты, offline).
func WithFeedbackNotifications(ns NotificationService) FeedbackServiceOption {
	return func(s *feedbackService) { s.notificationService = ns }
}

// WithFeedbackPermissionResolver подключает резолвер прав - им считается аудитория
// нового обращения (кто видит page.admin.feedback). Опционально: без него
// notifyFeedbackCreated просто не находит кому слать (пустая аудитория).
func WithFeedbackPermissionResolver(pr *PermissionResolver) FeedbackServiceOption {
	return func(s *feedbackService) { s.permissionResolver = pr }
}

// feedbackPreviewLimit - сколько рун сообщения обращения показывать в превью
// уведомления (само сообщение - до 1000 символов, тащить его целиком в колокольчик
// незачем).
const feedbackPreviewLimit = 160

// truncateForNotification обрезает текст по границе рун до limit символов,
// добавляя многоточие, если текст длиннее.
func truncateForNotification(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	r := []rune(trimmed)
	if len(r) <= limit {
		return trimmed
	}
	return string(r[:limit]) + "..."
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
// активным администраторам (super + admin), у которых бейдж виден по праву
// page.admin.feedback. Явные гранты права не-админам догоняет 30с-опрос.
// Best-effort, nil-safe.
func (s *feedbackService) notifyFeedbackChanged(ctx context.Context) {
	if s.realtimePublisher == nil {
		return
	}
	var ids []int
	if err := s.db.WithContext(ctx).
		Table("users").
		Where("is_active = ? AND (is_super_admin = ? OR is_admin = ?)", true, true, true).
		Pluck("id", &ids).Error; err != nil {
		slog.Warn("feedback.new: load feedback admins failed", "err", err)
		return
	}
	if len(ids) == 0 {
		return
	}
	s.realtimePublisher.PublishMany(ids, realtime.Event{Type: "feedback.new", Scope: "feedback"})
}

// feedbackAudience - кто разбирает обратную связь: активные пользователи с правом
// раздела «Обратная связь» (page.admin.feedback), кроме самого автора обращения.
// Право спрашиваем у резолвера - того же источника, что стоит за requireFeedbackAdmin
// в роутере и за гейтом пункта меню на фронте, по образцу directoryModerators
// (directory_pending_notify.go): «пришло уведомление, а действий нет» так не бывает.
func (s *feedbackService) feedbackAudience(ctx context.Context, authorID int) []int {
	if s.permissionResolver == nil {
		return nil
	}
	ids, err := activeUserIDs(ctx, s.db)
	if err != nil {
		slog.Warn("обратная связь: не удалось собрать аудиторию уведомления", "err", err)
		return nil
	}
	audience := make([]int, 0, len(ids))
	for _, uid := range ids {
		if uid == authorID {
			continue
		}
		set, err := s.permissionResolver.Resolve(ctx, uid)
		if err != nil {
			// best-effort: сбой резолва одного юзера сужает аудиторию, но не должен
			// отменять уведомление остальным.
			slog.Warn("обратная связь: резолв прав не удался", "user_id", uid, "error", err)
			continue
		}
		if set.Has(KeyPageAdminFeedback) {
			audience = append(audience, uid)
		}
	}
	return audience
}

// notifyFeedbackCreated шлёт NotificationTypeFeedbackCreated тем, кто разбирает
// обратную связь.
func (s *feedbackService) notifyFeedbackCreated(ctx context.Context, feedbackID, authorID int, message string) {
	if s.notificationService == nil {
		return
	}
	audience := s.feedbackAudience(ctx, authorID)
	if len(audience) == 0 {
		return
	}
	title := "Новое обращение обратной связи"
	body := truncateForNotification(message, feedbackPreviewLimit)
	for _, uid := range audience {
		if err := s.notificationService.CreateForUser(ctx, uid, NotificationTypeFeedbackCreated, title, body, nil); err != nil {
			slog.Warn("не удалось уведомить о новом обращении", "feedback_id", feedbackID, "user_id", uid, "error", err)
		}
	}
}

// notifyFeedbackAnswered сообщает автору обращения о реальном ответе - записанном
// непустом resolution_comment, а не о любой смене статуса (перевод в "Решено" без
// комментария или возврат в работу ответом не считаются). Если отвечает сам автор
// (админ разбирает собственное обращение) - не шлём, он и так знает.
func (s *feedbackService) notifyFeedbackAnswered(ctx context.Context, feedbackID, authorID, actorID int, comment string) {
	if s.notificationService == nil || authorID == actorID {
		return
	}
	title := "Ответ по обращению"
	body := truncateForNotification(comment, feedbackPreviewLimit)
	if err := s.notificationService.CreateForUser(ctx, authorID, NotificationTypeFeedbackAnswered, title, body, nil); err != nil {
		slog.Warn("не удалось уведомить автора обращения об ответе", "feedback_id", feedbackID, "user_id", authorID, "error", err)
	}
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
	s.notifyFeedbackCreated(ctx, feedback.ID, userID, trimmed)
	return feedback.ID, nil
}

// GetAll возвращает все обращения обратной связи с информацией о пользователях.
// is_read вычисляется персонально для запрашивающего администратора (feedback_reads).
func (s *feedbackService) GetAll(ctx context.Context, username string) ([]models.FeedbackWithUser, error) {
	userID, err := s.getUserIDByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	results := make([]models.FeedbackWithUser, 0)
	err = s.db.WithContext(ctx).
		Table("feedback f").
		Select(`f.id, f.user_id,
			CONCAT(u.last_name, ' ', u.first_name) AS user_name,
			f.message, f.status,
			EXISTS(SELECT 1 FROM feedback_reads fr WHERE fr.feedback_id = f.id AND fr.user_id = ?) AS is_read,
			f.flagged, f.created_at, f.updated_at,
			f.resolution_comment, f.resolved_at`, userID).
		Joins("JOIN users u ON f.user_id = u.id").
		Order("f.created_at DESC").
		Scan(&results).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching feedback")
	}

	// Логин вместо ФИО у авторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	for i := range results {
		results[i].UserName = maskName(masks, &results[i].UserID, results[i].UserName)
		if strings.TrimSpace(results[i].UserName) == "" {
			results[i].UserName = "Неизвестный пользователь"
		}
	}

	return results, nil
}

// GetStats возвращает статистику обращений обратной связи.
// unread считается персонально для администратора: обращения без записи о его
// прочтении в feedback_reads.
func (s *feedbackService) GetStats(ctx context.Context, username string) (*models.FeedbackStats, error) {
	userID, err := s.getUserIDByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	var stats models.FeedbackStats
	err = s.db.WithContext(ctx).
		Table("feedback f").
		Select(`COUNT(*) AS total,
			COUNT(CASE WHEN f.status = ? THEN 1 END) AS resolved,
			COUNT(CASE WHEN f.status = ? THEN 1 END) AS unresolved,
			COUNT(CASE WHEN NOT EXISTS(
				SELECT 1 FROM feedback_reads fr WHERE fr.feedback_id = f.id AND fr.user_id = ?
			) THEN 1 END) AS unread`,
			models.FeedbackResolved, models.FeedbackOpen, userID).
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

// UpdateStatus обновляет статус обращения обратной связи. actorUserID - кто вносит
// изменение (нужен, чтобы не уведомлять автора обращения о его же собственном ответе).
func (s *feedbackService) UpdateStatus(ctx context.Context, actorUserID int, id int, req models.UpdateFeedbackStatusRequest) error {
	// Заодно проверяем существование обращения и берём автора для уведомления об ответе.
	var authorID int
	if err := s.db.WithContext(ctx).Table("feedback").Select("user_id").Where("id = ?", id).Row().Scan(&authorID); err != nil {
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

	// Уведомляем только когда реально записан ответ (непустой resolution_comment) -
	// "Решено" без комментария или возврат в работу ответом не считаются.
	if comment, ok := updates["resolution_comment"].(string); ok && comment != "" {
		s.notifyFeedbackAnswered(ctx, id, authorID, actorUserID, comment)
	}

	return nil
}

// MarkAsRead фиксирует прочтение обращения администратором (персонально,
// идемпотентно). Вызывается автоматически при открытии обращения в админке.
func (s *feedbackService) MarkAsRead(ctx context.Context, id int, username string) error {
	userID, err := s.getUserIDByUsername(ctx, username)
	if err != nil {
		return err
	}

	var count int64
	if err := s.db.WithContext(ctx).Table("feedback").Where("id = ?", id).Count(&count).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking feedback existence")
	}
	if count == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Feedback not found")
	}

	// ON CONFLICT DO NOTHING - идемпотентная вставка (эталон application_reads).
	if err := s.db.WithContext(ctx).Exec(
		"INSERT INTO feedback_reads (feedback_id, user_id) VALUES (?, ?) ON CONFLICT (feedback_id, user_id) DO NOTHING",
		id, userID,
	).Error; err != nil {
		slog.Error("feedback read: insert failed", "err", err, "feedback_id", id, "user_id", userID)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error marking feedback as read")
	}

	return nil
}

// SetFlag устанавливает/снимает общий флажок "важное / взять в работу" на обращении.
func (s *feedbackService) SetFlag(ctx context.Context, id int, flagged bool) error {
	res := s.db.WithContext(ctx).
		Table("feedback").
		Where("id = ?", id).
		Update("flagged", flagged)
	if res.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating feedback flag")
	}
	if res.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Feedback not found")
	}

	return nil
}
