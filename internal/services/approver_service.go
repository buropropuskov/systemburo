package services

import (
	"context"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// ApproverService определяет интерфейс управления утверждающими заявок.
type ApproverService interface {
	GetAll(ctx context.Context, typeID int) ([]models.ApplicationApproverWithUser, error)
	GetAvailableUsers(ctx context.Context, typeID int) ([]models.AvailableApproverUser, error)
	Create(ctx context.Context, typeID int, userID int, createdByUsername string) error
	Delete(ctx context.Context, typeID int, id int, actorUsername string) error
	GetHistory(ctx context.Context) ([]models.ApplicationApproverHistoryItem, error)
}

type approverService struct {
	db      *gorm.DB
	history ApproverHistoryService
}

// NewApproverService создаёт сервис для управления утверждающими заявок.
func NewApproverService(db *gorm.DB) ApproverService {
	return &approverService{db: db, history: NewApproverHistoryService(db)}
}

func (s *approverService) checkAdmin(ctx context.Context, typeID int) error {
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

// GetAll возвращает список всех утверждающих с информацией о пользователях.
func (s *approverService) GetAll(ctx context.Context, typeID int) ([]models.ApplicationApproverWithUser, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

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
func (s *approverService) GetAvailableUsers(ctx context.Context, typeID int) ([]models.AvailableApproverUser, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

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
func (s *approverService) Create(ctx context.Context, typeID int, userID int, createdByUsername string) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

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

	// Снимок имени добавляемого принимающего для аудита.
	approverName := s.resolveUserName(ctx, userID)
	s.history.Log(ctx, userID, approverName, &createdByID, models.ApproverActionCreated)
	return nil
}

// Delete удаляет утверждающего по ID.
func (s *approverService) Delete(ctx context.Context, typeID int, id int, actorUsername string) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

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
		s.history.Log(ctx, snap.UserID, snap.Name, actorID, models.ApproverActionDeleted)
	}
	return nil
}

// GetHistory возвращает глобальный журнал принимающих (новые сверху).
func (s *approverService) GetHistory(ctx context.Context) ([]models.ApplicationApproverHistoryItem, error) {
	return s.history.GetAll(ctx)
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
