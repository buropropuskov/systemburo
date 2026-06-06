package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// UserService — интерфейс бизнес-логики управления пользователями (admin-only).
type UserService interface {
	// Create создаёт нового пользователя (admin-only). callerUserID - id админа для аудита.
	Create(ctx context.Context, callerTypeID, callerUserID int, req models.RegisterRequest) error
	// GetAll возвращает список пользователей с организацией, компанией и типом.
	// includeArchived=false отдаёт только активных (is_active=true).
	GetAll(ctx context.Context, callerTypeID int, includeArchived bool) ([]models.UserInfoResponse, error)
	// UpdateType обновляет тип пользователя.
	UpdateType(ctx context.Context, callerTypeID, callerUserID int, username string, req models.UpdateUserTypeRequest) error
	// UpdatePassword обновляет пароль пользователя.
	UpdatePassword(ctx context.Context, callerTypeID, callerUserID int, username string, req models.UpdatePasswordRequest) error
	// UpdateInfo обновляет ФИО, должность, email и телефон пользователя.
	UpdateInfo(ctx context.Context, callerTypeID, callerUserID int, username string, req models.UpdateUserInfoRequest) error
	// UpdateOrganization обновляет организацию пользователя.
	UpdateOrganization(ctx context.Context, callerTypeID, callerUserID int, username string, req models.UpdateUserOrganizationRequest) error
	// UpdateCompany обновляет компанию пользователя.
	UpdateCompany(ctx context.Context, callerTypeID, callerUserID int, username string, req models.UpdateUserCompanyRequest) error
	// Delete архивирует пользователя по username (soft-delete: is_active=false).
	Delete(ctx context.Context, callerTypeID, callerUserID int, username string) error
	// Restore восстанавливает архивного пользователя (is_active=true).
	Restore(ctx context.Context, callerTypeID, callerUserID int, username string) error
	// GetHistory возвращает историю действий над пользователем (по username).
	GetHistory(ctx context.Context, callerTypeID int, username string) ([]models.UserHistoryItem, error)
}

type userService struct {
	db                  *gorm.DB
	notificationService NotificationService
	history             UserHistoryService
}

// NewUserService создаёт новый экземпляр сервиса управления пользователями.
// notificationService может быть nil — в этом случае уведомления просто
// не будут создаваться (legacy совместимость в местах, где notification
// не подключён). Триггерные методы проверяют nil перед использованием.
// История аудита создаётся внутри из db (сигнатура конструктора не меняется).
func NewUserService(db *gorm.DB, notificationService NotificationService) UserService {
	return &userService{
		db:                  db,
		notificationService: notificationService,
		history:             NewUserHistoryService(db),
	}
}

// targetUserID резолвит id пользователя по username для записи в историю.
// Возвращает 0, если не найден (тогда лог пропускается).
func (s *userService) targetUserID(ctx context.Context, username string) int {
	var id int
	s.db.WithContext(ctx).Table("users").Select("id").Where("username = ?", username).Scan(&id)
	return id
}

// checkAdmin проверяет, что вызывающий пользователь является администратором
// (код типа "manager" или "buropropuskov").
func (s *userService) checkAdmin(ctx context.Context, typeID int) error {
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

// Create создаёт нового пользователя. Только admin (manager/buropropuskov).
//
// Пользователь может быть привязан к организации, компании или к обеим сразу.
// Хотя бы одно из двух поле должно быть > 0. Значение 0 означает "не привязан".
func (s *userService) Create(ctx context.Context, callerTypeID, callerUserID int, req models.RegisterRequest) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}

	if req.OrganizationID <= 0 && req.CompanyID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Необходимо указать организацию или компанию (хотя бы одно)")
	}

	user := models.User{
		Username:       req.Username,
		Password:       hashPassword(req.Password),
		OrganizationID: intPtrOrNil(req.OrganizationID),
		CompanyID:      intPtrOrNil(req.CompanyID),
		TypeID:         req.TypeID,
		LastName:       req.LastName,
		FirstName:      req.FirstName,
		MiddleName:     req.MiddleName,
		Position:       req.Position,
		Email:          req.Email,
		Phone:          req.Phone,
	}
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return echo.NewHTTPError(http.StatusBadRequest, "Username already exists")
		}
		slog.Error("не удалось создать пользователя", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error creating user")
	}
	s.history.Log(ctx, user.ID, &callerUserID, models.UserActionCreated, map[string]any{
		"username": user.Username,
		"type_id":  user.TypeID,
	})
	return nil
}

// GetAll возвращает пользователей с JOIN на организацию, компанию и тип.
// По умолчанию только активные; includeArchived=true добавляет архивных.
func (s *userService) GetAll(ctx context.Context, callerTypeID int, includeArchived bool) ([]models.UserInfoResponse, error) {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return nil, err
	}

	result := make([]models.UserInfoResponse, 0)
	q := s.db.WithContext(ctx).
		Table("users u").
		Select(`u.id, u.username, u.is_active, u.is_banned, u.is_super_admin,
			o.name as organization, u.organization_id,
			c.name as company, u.company_id,
			u.type_id, ut.name as user_type, u.role_id,
			u.last_name, u.first_name, u.middle_name,
			u.position, u.email, u.phone`).
		Joins("LEFT JOIN organizations o ON u.organization_id = o.id").
		Joins("LEFT JOIN companies c ON u.company_id = c.id").
		Joins("LEFT JOIN user_types ut ON u.type_id = ut.id")
	if !includeArchived {
		q = q.Where("u.is_active = ?", true)
	}
	if err := q.Order("u.username").Scan(&result).Error; err != nil {
		slog.Error("не удалось получить список пользователей", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching users")
	}

	return result, nil
}

// UpdateType обновляет type_id пользователя с проверкой существования типа.
func (s *userService) UpdateType(ctx context.Context, callerTypeID, callerUserID int, username string, req models.UpdateUserTypeRequest) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}

	// Проверяем существование типа
	var exists bool
	if err := s.db.WithContext(ctx).
		Table("user_types").
		Select("COUNT(1) > 0").
		Where("id = ?", req.TypeID).
		Row().Scan(&exists); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking user type")
	}
	if !exists {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user type")
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("type_id", req.TypeID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating user type")
	}

	if id := s.targetUserID(ctx, username); id != 0 {
		s.history.Log(ctx, id, &callerUserID, models.UserActionTypeChanged, map[string]any{"type_id": req.TypeID})
	}
	return nil
}

// UpdatePassword хеширует и обновляет пароль пользователя.
// После смены пароля все refresh_tokens юзера отзываются - иначе старые
// сессии (возможно скомпрометированные) продолжили бы жить до истечения TTL.
func (s *userService) UpdatePassword(ctx context.Context, callerTypeID, callerUserID int, username string, req models.UpdatePasswordRequest) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}

	hashed := hashPassword(req.Password)

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("password", hashed).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating password")
	}

	// Revoke all active refresh tokens: чтобы существующие сессии с этой учёткой
	// (возможно скомпрометированные) не дожили до своего TTL. Юзеру придётся
	// перелогиниться на всех устройствах. Not-found = пользователь без активных
	// сессий - это ок.
	var user models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err == nil {
		s.db.WithContext(ctx).
			Model(&models.RefreshToken{}).
			Where("user_id = ? AND is_revoked = false", user.ID).
			Update("is_revoked", true)

		// Аудит: факт сброса пароля без значения.
		s.history.Log(ctx, user.ID, &callerUserID, models.UserActionPasswordReset, nil)

		// Уведомление о смене пароля. Сейчас UpdatePassword вызывается только
		// admin'ом (manager/buropropuskov) — даже если username совпадает с
		// callerTypeID, это всё равно admin-action. Считаем "сам себе сменил"
		// если в БД у юзера тот же type_id, что и у вызывающего, и юзер админ.
		s.notifyPasswordChanged(ctx, &user, callerTypeID)
	}

	return nil
}

// notifyPasswordChanged создаёт уведомление о смене пароля. Вызывается
// после успешного апдейта; ошибки только логируются (уведомления не
// должны блокировать основной flow).
func (s *userService) notifyPasswordChanged(ctx context.Context, target *models.User, callerTypeID int) {
	if s.notificationService == nil {
		return
	}

	// Определяем, сам ли пользователь сменил пароль или это сделал админ.
	// Под "сам собой" понимаем: callerTypeID == target.TypeID и сам юзер - админ.
	// Уточнить вызывающего через user_id мы не можем (в API передаётся
	// только callerTypeID), поэтому ограничиваемся типом.
	selfChange := false
	if callerTypeID == target.TypeID {
		var code string
		err := s.db.WithContext(ctx).
			Table("user_types").
			Select("code").
			Where("id = ?", target.TypeID).
			Row().
			Scan(&code)
		if err == nil && (code == "manager" || code == "buropropuskov") {
			selfChange = true
		}
	}

	message := "Администратор изменил ваш пароль."
	if selfChange {
		message = "Ваш пароль был успешно изменён."
	}

	dataPayload := map[string]any{
		"changed_at": time.Now().UTC().Format(time.RFC3339),
		// changed_by_user_id мы не знаем точно (передаётся только typeID),
		// поэтому пишем тип вызывающего — это всё, что у нас есть.
		"changed_by_type_id": callerTypeID,
	}
	dataBytes, err := json.Marshal(dataPayload)
	if err != nil {
		slog.Error("не удалось сериализовать payload уведомления", "error", err)
		return
	}
	dataStr := string(dataBytes)

	if err := s.notificationService.CreateForUser(
		ctx, target.ID,
		"password_changed",
		"Пароль изменён",
		message,
		&dataStr,
	); err != nil {
		slog.Error("не удалось создать уведомление о смене пароля", "user_id", target.ID, "error", err)
	}
}

// UpdateInfo обновляет персональные данные пользователя.
func (s *userService) UpdateInfo(ctx context.Context, callerTypeID, callerUserID int, username string, req models.UpdateUserInfoRequest) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Updates(map[string]interface{}{
			"last_name":   req.LastName,
			"first_name":  req.FirstName,
			"middle_name": req.MiddleName,
			"position":    req.Position,
			"email":       req.Email,
			"phone":       req.Phone,
		}).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating user info")
	}

	if id := s.targetUserID(ctx, username); id != 0 {
		// В детали пишем только переданные (non-nil) поля - иначе история покажет "поле: -".
		details := map[string]any{}
		if req.LastName != nil {
			details["last_name"] = *req.LastName
		}
		if req.FirstName != nil {
			details["first_name"] = *req.FirstName
		}
		if req.MiddleName != nil {
			details["middle_name"] = *req.MiddleName
		}
		if req.Position != nil {
			details["position"] = *req.Position
		}
		if req.Email != nil {
			details["email"] = *req.Email
		}
		if req.Phone != nil {
			details["phone"] = *req.Phone
		}
		s.history.Log(ctx, id, &callerUserID, models.UserActionUpdated, details)
	}
	return nil
}

// UpdateOrganization обновляет organization_id пользователя.
func (s *userService) UpdateOrganization(ctx context.Context, callerTypeID, callerUserID int, username string, req models.UpdateUserOrganizationRequest) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("organization_id", req.OrganizationID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating organization")
	}

	if id := s.targetUserID(ctx, username); id != 0 {
		s.history.Log(ctx, id, &callerUserID, models.UserActionOrgChanged, map[string]any{"organization_id": req.OrganizationID})
	}
	return nil
}

// UpdateCompany обновляет company_id пользователя.
func (s *userService) UpdateCompany(ctx context.Context, callerTypeID, callerUserID int, username string, req models.UpdateUserCompanyRequest) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("company_id", req.CompanyID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating company")
	}

	if id := s.targetUserID(ctx, username); id != 0 {
		s.history.Log(ctx, id, &callerUserID, models.UserActionCompanyChanged, map[string]any{"company_id": req.CompanyID})
	}
	return nil
}

// Delete архивирует пользователя (soft-delete: is_active=false). Строка остаётся,
// поэтому ссылки заявок (sender_user_id и др.) не осиротевают; login/refresh
// блокируются по is_active, активные refresh-токены отзываются.
func (s *userService) Delete(ctx context.Context, callerTypeID, callerUserID int, username string) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}
	return s.setActive(ctx, username, false, callerUserID)
}

// Restore восстанавливает архивного пользователя (is_active=true).
func (s *userService) Restore(ctx context.Context, callerTypeID, callerUserID int, username string) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}
	return s.setActive(ctx, username, true, callerUserID)
}

func (s *userService) setActive(ctx context.Context, username string, active bool, callerUserID int) error {
	var user models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "User not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user")
	}
	if user.IsActive == active {
		return nil // no-op
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).Where("id = ?", user.ID).Update("is_active", active).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating user")
		}
		if !active {
			// Отзываем активные refresh-токены: существующая сессия гаснет в пределах
			// TTL access-токена (login/refresh уже блокируются по is_active).
			if err := tx.Model(&models.RefreshToken{}).
				Where("user_id = ? AND is_revoked = ?", user.ID, false).
				Updates(map[string]any{"is_revoked": true, "revoked_at": time.Now().UTC()}).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error revoking tokens")
			}
		}
		return nil
	}); err != nil {
		return err
	}
	action := models.UserActionRestored
	if !active {
		action = models.UserActionArchived
	}
	s.history.Log(ctx, user.ID, &callerUserID, action, nil)
	return nil
}

// GetHistory возвращает историю действий над пользователем по username (admin-only).
func (s *userService) GetHistory(ctx context.Context, callerTypeID int, username string) ([]models.UserHistoryItem, error) {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return nil, err
	}
	id := s.targetUserID(ctx, username)
	if id == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "User not found")
	}
	return s.history.GetHistory(ctx, id)
}
