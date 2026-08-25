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
	"gorm.io/gorm/clause"
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

// ReassignUserTypeRequest — тело запроса переноса всех пользователей типа в
// целевой тип target_type_id. Отдельный DTO от org/company ReassignUsersRequest
// (target_id): семантика типа иная и общий пакет services не терпит коллизий
// символов между срезами (урок #1227).
type ReassignUserTypeRequest struct {
	TargetTypeID int `json:"target_type_id"`
}

// UserTypeMemberResponse — пользователь, привязанный к типу через users.type_id.
// В отличие от MemberResponse (org/company, active-only) несёт is_active: набор
// блокирующих удаление типа включает АРХИВНЫХ (Delete считает все type_id, не
// только активных), поэтому фронт различает активных и архивных бейджем.
type UserTypeMemberResponse struct {
	ID         int     `json:"id"`
	Username   string  `json:"username"`
	LastName   *string `json:"last_name"`
	FirstName  *string `json:"first_name"`
	MiddleName *string `json:"middle_name"`
	Position   *string `json:"position"`
	IsActive   bool    `json:"is_active"`
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
	// GetTypeUsers возвращает ВСЕХ пользователей типа (включая архивных) - набор,
	// блокирующий удаление типа (Delete считает все type_id, не только активных).
	GetTypeUsers(ctx context.Context, typeID int) ([]UserTypeMemberResponse, error)
	// ReassignTypeUsers переносит всех пользователей типа id в целевой тип targetTypeID,
	// освобождая исходный для удаления. callerUserID - актор для аудита.
	ReassignTypeUsers(ctx context.Context, callerUserID, id, targetTypeID int) (int, error)
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
// #870, финал F.2: запись и до-cutover строки живут в общем audit_log (старые
// перенесены backfill'ом BackfillAuditFromLegacy), поэтому чтение идёт только из
// audit_log. Замороженная user_type_histories дропнута в дроп-sweep (F.8).
// Форму ответа стережёт TestUserTypes_History.
func (s *userTypeService) GetHistory(ctx context.Context, id int) ([]models.UserTypeHistoryItem, error) {
	const actorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`
	sql := `
		SELECT a.id AS id, a.action AS action_type, a.details AS details,
			a.actor_user_id AS actor_user_id, ` + actorName + ` AS actor_name, a.created_at AS created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.entity_type = ? AND a.entity_id = ?
		ORDER BY a.created_at DESC, a.id DESC`

	type row struct {
		ID          int             `gorm:"column:id"`
		ActionType  string          `gorm:"column:action_type"`
		Details     json.RawMessage `gorm:"column:details"`
		ActorUserID *int            `gorm:"column:actor_user_id"`
		ActorName   string          `gorm:"column:actor_name"`
		CreatedAt   time.Time       `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityUserType, id).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user type history")
	}

	// Логин вместо ФИО у акторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]models.UserTypeHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.UserTypeHistoryItem{
			ID:          r.ID,
			ActionType:  r.ActionType,
			Details:     r.Details,
			ActorUserID: r.ActorUserID,
			ActorName:   maskName(masks, r.ActorUserID, r.ActorName),
			CreatedAt:   r.CreatedAt,
		})
	}
	return items, nil
}

// GetTypeUsers возвращает всех пользователей типа typeID (включая архивных) -
// набор, который блокирует удаление типа: Delete считает users.type_id независимо
// от is_active. Активные идут первыми, флаг is_active позволяет фронту пометить
// архивных бейджем. Для несуществующего типа - пустой список (идемпотентно).
func (s *userTypeService) GetTypeUsers(ctx context.Context, typeID int) ([]UserTypeMemberResponse, error) {
	members := make([]UserTypeMemberResponse, 0)
	err := s.db.WithContext(ctx).
		Table("users u").
		Select("u.id, u.username, u.last_name, u.first_name, u.middle_name, u.position, u.is_active").
		Where("u.type_id = ?", typeID).
		Order("u.is_active DESC, u.last_name, u.first_name, u.username").
		Scan(&members).Error
	if err != nil {
		slog.Error("Не удалось получить пользователей типа", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user type members")
	}
	if masks := loadConsentMasks(ctx, s.db); len(masks) > 0 {
		for i := range members {
			maskUserParts(masks, members[i].ID, &members[i].LastName, &members[i].FirstName, &members[i].MiddleName)
		}
	}
	return members, nil
}

// ReassignTypeUsers переносит ВСЕХ пользователей типа id (набор, блокирующий его
// удаление: users.type_id=id, любой is_active) в целевой тип targetTypeID, освобождая
// исходный. Системный тип нельзя освободить как источник (его и удалять нельзя).
// Целевой тип может быть системным (в дефолтный тип переносить допустимо), должен
// существовать и отличаться от исходного. Идемпотентно: без пользователей - 0 без
// ошибки. Аудит смены типа (UserActionTypeChanged) пишется на каждого в той же
// транзакции - провал записи откатывает перенос.
func (s *userTypeService) ReassignTypeUsers(ctx context.Context, callerUserID, id, targetTypeID int) (int, error) {
	srcSystem, found, err := s.typeFlags(ctx, id)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, echo.NewHTTPError(http.StatusNotFound, "User type not found")
	}
	if srcSystem {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Системный тип пользователя нельзя освободить")
	}
	if targetTypeID == id {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Нельзя перенести пользователей в тот же тип")
	}

	var count int
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Целевой тип проверяем ВНУТРИ транзакции с блокировкой строки (FOR UPDATE):
		// закрывает окно TOCTOU, когда параллельное удаление target прошло бы между
		// проверкой существования и переносом - иначе пользователи ушли бы в удалённый тип.
		var target models.UserType
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&target, targetTypeID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return echo.NewHTTPError(http.StatusBadRequest, "Целевой тип пользователя не найден")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching target user type")
		}
		// Атомарный UPDATE ... RETURNING id: перенос и выборка затронутых - один
		// стейтмент, поэтому конкурентная смена type_id тем же пользователем не
		// разъедется с набором для аудита, а count отражает РЕАЛЬНО применённые строки.
		var ids []int
		if err := tx.Raw("UPDATE users SET type_id = ? WHERE type_id = ? RETURNING id", targetTypeID, id).
			Scan(&ids).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error reassigning users")
		}
		if len(ids) == 0 {
			return nil
		}
		for _, uid := range ids {
			u := uid
			if err := s.recorder.Record(ctx, tx, models.AuditEntityUser, &u, models.UserActionTypeChanged, &callerUserID, map[string]any{"old": id, "new": targetTypeID}); err != nil {
				return err
			}
		}
		count = len(ids)
		return nil
	}); err != nil {
		return 0, err
	}
	slog.Info("пользователи перенесены между типами", "from", id, "to", targetTypeID, "count", count)
	return count, nil
}
