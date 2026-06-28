package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// UserTypeWithCount — тип пользователя с количеством связанных пользователей.
type UserTypeWithCount struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	IsSystem   bool   `json:"is_system"`
	UsersCount int64  `json:"users_count"`
}

// CreateUserTypeRequest — запрос на создание нового типа пользователя.
type CreateUserTypeRequest struct {
	Name string `json:"name" validate:"required,min=1,max=50"`
	Code string `json:"code" validate:"required,min=1,max=20"`
}

// UpdateUserTypeRequest — запрос на обновление типа пользователя.
type UpdateUserTypeRequest struct {
	Name string `json:"name" validate:"required,min=1,max=50"`
	Code string `json:"code" validate:"required,min=1,max=20"`
}

// UserTypeService — интерфейс бизнес-логики управления типами пользователей.
// Авторизация (page.admin) выполняется роут-middleware RequirePermissionV2.
type UserTypeService interface {
	// GetAllWithCount возвращает все типы пользователей с количеством пользователей каждого типа.
	GetAllWithCount(ctx context.Context) ([]UserTypeWithCount, error)
	// Create создаёт новый тип пользователя и возвращает его ID. callerUserID - актор для аудита.
	Create(ctx context.Context, callerUserID int, req CreateUserTypeRequest) (int, error)
	// Update обновляет имя типа пользователя по ID. callerUserID - актор для аудита.
	Update(ctx context.Context, callerUserID, id int, req UpdateUserTypeRequest) error
	// Delete удаляет тип пользователя по ID, если с ним не связаны пользователи. callerUserID - актор для аудита.
	Delete(ctx context.Context, callerUserID, id int) error
	// GetHistory возвращает историю изменений типа пользователя.
	GetHistory(ctx context.Context, id int) ([]models.UserTypeHistoryItem, error)
}

type userTypeService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewUserTypeService создаёт новый экземпляр сервиса управления типами пользователей.
func NewUserTypeService(db *gorm.DB) UserTypeService {
	return &userTypeService{db: db, recorder: NewAuditRecorder(db)}
}

// typeFlags возвращает признак системного типа и факт его существования.
func (s *userTypeService) typeFlags(ctx context.Context, id int) (isSystem bool, found bool, err error) {
	var row struct {
		IsSystem bool
	}
	err = s.db.WithContext(ctx).
		Table("user_types").
		Select("is_system").
		Where("id = ?", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, false, nil
	}
	if err != nil {
		return false, false, echo.NewHTTPError(http.StatusInternalServerError, "Error checking user type")
	}
	return row.IsSystem, true, nil
}

// GetAllWithCount возвращает все типы пользователей с количеством связанных пользователей.
func (s *userTypeService) GetAllWithCount(ctx context.Context) ([]UserTypeWithCount, error) {
	result := make([]UserTypeWithCount, 0)
	err := s.db.WithContext(ctx).
		Table("user_types ut").
		Select("ut.id, ut.name, ut.code, ut.is_system, COUNT(u.username) as users_count").
		Joins("LEFT JOIN users u ON ut.id = u.type_id").
		Group("ut.id, ut.name, ut.code, ut.is_system").
		Order("ut.name").
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user types")
	}

	return result, nil
}

// Create создаёт новый тип пользователя. Возвращает ошибку, если тип с таким кодом уже существует.
func (s *userTypeService) Create(ctx context.Context, callerUserID int, req CreateUserTypeRequest) (int, error) {
	// Проверяем уникальность кода
	var count int64
	if err := s.db.WithContext(ctx).Table("user_types").Where("code = ?", req.Code).Count(&count).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error checking user type existence")
	}
	if count > 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "User type with this code already exists")
	}

	// Вставляем новый тип и получаем ID
	var id int
	err := s.db.WithContext(ctx).
		Table("user_types").
		Raw("INSERT INTO user_types (name, code) VALUES (?, ?) RETURNING id", req.Name, req.Code).
		Row().
		Scan(&id)
	if err != nil {
		slog.Error("не удалось создать тип пользователя", "error", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error creating user type")
	}

	slog.Info("тип пользователя создан", "id", id)
	s.recorder.Log(ctx, nil, models.AuditEntityUserType, &id, models.UserTypeActionCreated, &callerUserID, map[string]any{
		"name": req.Name,
		"code": req.Code,
	})
	return id, nil
}

// Update обновляет имя типа пользователя. Возвращает ошибку, если тип не найден.
func (s *userTypeService) Update(ctx context.Context, callerUserID, id int, req UpdateUserTypeRequest) error {
	// Проверяем существование типа и его системность одним запросом
	isSystem, found, err := s.typeFlags(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return echo.NewHTTPError(http.StatusNotFound, "User type not found")
	}
	if isSystem {
		return echo.NewHTTPError(http.StatusBadRequest, "Системный тип пользователя нельзя переименовать")
	}

	if err := s.db.WithContext(ctx).Table("user_types").Where("id = ?", id).Update("name", req.Name).Error; err != nil {
		slog.Error("не удалось обновить тип пользователя", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating user type")
	}

	slog.Info("тип пользователя обновлён", "id", id)
	s.recorder.Log(ctx, nil, models.AuditEntityUserType, &id, models.UserTypeActionRenamed, &callerUserID, map[string]any{"name": req.Name})
	return nil
}

// Delete удаляет тип пользователя. Возвращает ошибку, если с типом связаны пользователи.
func (s *userTypeService) Delete(ctx context.Context, callerUserID, id int) error {
	// Снимок name/code до удаления - для деталей аудита (после удаления строки нет).
	var snapshot struct {
		Name     string
		Code     string
		IsSystem bool
	}
	err := s.db.WithContext(ctx).
		Table("user_types").
		Select("name, code, is_system").
		Where("id = ?", id).
		Take(&snapshot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "User type not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking user type")
	}

	// Системные типы (их code используется в авторизации) удалять запрещено.
	if snapshot.IsSystem {
		return echo.NewHTTPError(http.StatusBadRequest, "Системный тип пользователя нельзя удалить")
	}

	// Проверяем, есть ли пользователи с этим типом
	var usersCount int64
	if err := s.db.WithContext(ctx).Table("users").Where("type_id = ?", id).Count(&usersCount).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking users count")
	}
	if usersCount > 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Cannot delete user type that has associated users")
	}

	if err := s.db.WithContext(ctx).Table("user_types").Where("id = ?", id).Delete(nil).Error; err != nil {
		slog.Error("не удалось удалить тип пользователя", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting user type")
	}

	slog.Info("тип пользователя удалён", "id", id)
	s.recorder.Log(ctx, nil, models.AuditEntityUserType, &id, models.UserTypeActionDeleted, &callerUserID, map[string]any{
		"name": snapshot.Name,
		"code": snapshot.Code,
	})
	return nil
}

// GetHistory возвращает историю изменений типа пользователя (admin-only).
// Переходный период #870: запись уже идёт в audit_log, но старые строки лежат в
// замороженной user_type_histories до финального backfill. Чтение объединяет обе таблицы.
func (s *userTypeService) GetHistory(ctx context.Context, id int) ([]models.UserTypeHistoryItem, error) {
	const actorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`
	sql := `
		SELECT id, action_type, details, actor_user_id, actor_name, created_at FROM (
			SELECT h.id AS id, h.action_type AS action_type, h.details AS details,
				h.actor_user_id AS actor_user_id, ` + actorName + ` AS actor_name, h.created_at AS created_at
			FROM user_type_histories h LEFT JOIN users u ON u.id = h.actor_user_id
			WHERE h.user_type_id = ?
			UNION ALL
			SELECT a.id AS id, a.action AS action_type, a.details AS details,
				a.actor_user_id AS actor_user_id, ` + actorName + ` AS actor_name, a.created_at AS created_at
			FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
			WHERE a.entity_type = ? AND a.entity_id = ?
		) merged
		ORDER BY created_at DESC, id DESC`

	type row struct {
		ID          int             `gorm:"column:id"`
		ActionType  string          `gorm:"column:action_type"`
		Details     json.RawMessage `gorm:"column:details"`
		ActorUserID *int            `gorm:"column:actor_user_id"`
		ActorName   string          `gorm:"column:actor_name"`
		CreatedAt   time.Time       `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(sql, id, models.AuditEntityUserType, id).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user type history")
	}

	items := make([]models.UserTypeHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.UserTypeHistoryItem{
			ID:          r.ID,
			ActionType:  r.ActionType,
			Details:     r.Details,
			ActorUserID: r.ActorUserID,
			ActorName:   r.ActorName,
			CreatedAt:   r.CreatedAt,
		})
	}
	return items, nil
}
