package services

import (
	"context"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// ApproverService определяет интерфейс управления утверждающими заявок.
// Авторизация изменяющих/листинговых операций (page.admin) - на роут-middleware
// RequirePermissionV2; история (GetHistory) доступна всем авторизованным.
type ApproverService interface {
	GetAll(ctx context.Context) ([]models.ApplicationApproverWithUser, error)
	GetAvailableUsers(ctx context.Context) ([]models.AvailableApproverUser, error)
	Create(ctx context.Context, userID int, createdByUsername string) error
	Delete(ctx context.Context, id int, actorUsername string) error
	GetHistory(ctx context.Context) ([]models.ApplicationApproverHistoryItem, error)
}

type approverService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewApproverService создаёт сервис для управления утверждающими заявок.
func NewApproverService(db *gorm.DB) ApproverService {
	return &approverService{db: db, recorder: NewAuditRecorder(db)}
}

// GetAll возвращает список всех утверждающих с информацией о пользователях.
func (s *approverService) GetAll(ctx context.Context) ([]models.ApplicationApproverWithUser, error) {
	result := make([]models.ApplicationApproverWithUser, 0)
	err := s.db.WithContext(ctx).
		Table("application_approvers aa").
		Select(`aa.id, aa.user_id, u.username, u.last_name, u.first_name, u.middle_name,
			u."position", o.name as organization, c.name as company, aa.created_at`).
		Joins("JOIN users u ON u.id = aa.user_id").
		Joins("LEFT JOIN organizations o ON o.id = u.organization_id").
		Joins("LEFT JOIN companies c ON c.id = u.company_id").
		Order("u.last_name, u.first_name").
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching approvers")
	}
	return result, nil
}

// GetAvailableUsers возвращает пользователей, которые ещё не назначены утверждающими.
func (s *approverService) GetAvailableUsers(ctx context.Context) ([]models.AvailableApproverUser, error) {
	result := make([]models.AvailableApproverUser, 0)
	err := s.db.WithContext(ctx).
		Table("users u").
		Select(`u.id, u.username, u.last_name, u.first_name, u.middle_name,
			u."position", o.name as organization, c.name as company`).
		Joins("LEFT JOIN organizations o ON o.id = u.organization_id").
		Joins("LEFT JOIN companies c ON c.id = u.company_id").
		Where("NOT EXISTS (SELECT 1 FROM application_approvers aa WHERE aa.user_id = u.id)").
		Order("u.last_name, u.first_name").
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching available users")
	}
	return result, nil
}

// Create назначает пользователя утверждающим заявок.
func (s *approverService) Create(ctx context.Context, userID int, createdByUsername string) error {
	var userExists int64
	s.db.WithContext(ctx).Table("users").Where("id = ?", userID).Count(&userExists)
	if userExists == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "User not found")
	}

	var alreadyApprover int64
	s.db.WithContext(ctx).Table("application_approvers").Where("user_id = ?", userID).Count(&alreadyApprover)
	if alreadyApprover > 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "User is already an approver")
	}

	var createdByID int
	if err := s.db.WithContext(ctx).Table("users").Select("id").Where("username = ?", createdByUsername).Row().Scan(&createdByID); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Current user not found")
	}

	approver := models.ApplicationApprover{
		UserID:    userID,
		CreatedBy: &createdByID,
	}
	if err := s.db.WithContext(ctx).Create(&approver).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error creating approver")
	}

	// Снимок имени: approver_name в details (поле было плоской колонкой в старой таблице).
	approverName := s.resolveUserName(ctx, userID)
	s.recorder.Log(ctx, nil, models.AuditEntityApprover, &userID, models.ApproverActionCreated, &createdByID,
		map[string]any{"approver_name": approverName})
	return nil
}

// Delete удаляет утверждающего по ID.
func (s *approverService) Delete(ctx context.Context, id int, actorUsername string) error {
	// Снимок до удаления: берём user_id и имя принимающего.
	type approverRow struct {
		UserID int
		Name   string
	}
	var snap approverRow
	snapErr := s.db.WithContext(ctx).
		Table("application_approvers aa").
		Select(`aa.user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name, u.middle_name)), ''), u.username, '') AS name`).
		Joins("JOIN users u ON u.id = aa.user_id").
		Where("aa.id = ?", id).
		Row().
		Scan(&snap.UserID, &snap.Name)
	// snapErr не роняет удаление — аудит best-effort.

	result := s.db.WithContext(ctx).Delete(&models.ApplicationApprover{}, id)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting approver")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Approver not found")
	}

	if snapErr == nil && snap.UserID > 0 {
		var actorID *int
		var aid int
		if err := s.db.WithContext(ctx).Table("users").Select("id").Where("username = ?", actorUsername).Row().Scan(&aid); err == nil {
			actorID = &aid
		}
		s.recorder.Log(ctx, nil, models.AuditEntityApprover, &snap.UserID, models.ApproverActionDeleted, actorID,
			map[string]any{"approver_name": snap.Name})
	}
	return nil
}

// GetHistory возвращает глобальный журнал принимающих (новые сверху).
// Переходный период #870: запись уже идёт в audit_log; старые строки лежат в
// application_approver_histories до финального backfill. Чтение объединяет обе
// таблицы в одинаковую форму ответа (форму стережёт TestApprovers_History_*).
func (s *approverService) GetHistory(ctx context.Context) ([]models.ApplicationApproverHistoryItem, error) {
	const actorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`
	sql := `
		SELECT id, approver_user_id, approver_name, action_type, actor_user_id, actor_name, created_at FROM (
			SELECT h.id AS id, h.approver_user_id AS approver_user_id, h.approver_name AS approver_name,
				h.action_type AS action_type, h.actor_user_id AS actor_user_id,
				` + actorName + ` AS actor_name, h.created_at AS created_at
			FROM application_approver_histories h LEFT JOIN users u ON u.id = h.actor_user_id
			UNION ALL
			SELECT a.id AS id, a.entity_id AS approver_user_id,
				COALESCE(a.details->>'approver_name', '') AS approver_name,
				a.action AS action_type, a.actor_user_id AS actor_user_id,
				` + actorName + ` AS actor_name, a.created_at AS created_at
			FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
			WHERE a.entity_type = ?
		) merged
		ORDER BY created_at DESC, id DESC`

	type row struct {
		ID             int       `gorm:"column:id"`
		ApproverUserID int       `gorm:"column:approver_user_id"`
		ApproverName   string    `gorm:"column:approver_name"`
		ActionType     string    `gorm:"column:action_type"`
		ActorUserID    *int      `gorm:"column:actor_user_id"`
		ActorName      string    `gorm:"column:actor_name"`
		CreatedAt      time.Time `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityApprover).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching approver history")
	}

	items := make([]models.ApplicationApproverHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.ApplicationApproverHistoryItem{
			ID:             r.ID,
			ApproverUserID: r.ApproverUserID,
			ApproverName:   r.ApproverName,
			ActionType:     r.ActionType,
			ActorUserID:    r.ActorUserID,
			ActorName:      r.ActorName,
			CreatedAt:      r.CreatedAt,
		})
	}
	return items, nil
}

// resolveUserName возвращает форматированное имя пользователя по ID
// (фамилия имя отчество или username как фолбэк).
func (s *approverService) resolveUserName(ctx context.Context, userID int) string {
	var name string
	_ = s.db.WithContext(ctx).
		Table("users").
		Select(`COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', last_name, first_name, middle_name)), ''), username, '')`).
		Where("id = ?", userID).
		Row().
		Scan(&name)
	return name
}
