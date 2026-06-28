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
	Create(ctx context.Context, callerUserID int, req models.RegisterRequest) error
	// GetAll возвращает список пользователей с организацией, компанией и типом.
	// includeArchived=false отдаёт только активных (is_active=true).
	GetAll(ctx context.Context, includeArchived bool) ([]models.UserInfoResponse, error)
	// UpdateType обновляет тип пользователя.
	UpdateType(ctx context.Context, callerUserID int, username string, req models.UpdateUserTypeRequest) error
	// UpdatePassword обновляет пароль пользователя.
	UpdatePassword(ctx context.Context, callerUserID int, username string, req models.UpdatePasswordRequest) error
	// UpdateInfo обновляет ФИО, должность, email и телефон пользователя.
	UpdateInfo(ctx context.Context, callerUserID int, username string, req models.UpdateUserInfoRequest) error
	// UpdateOrganization обновляет организацию пользователя.
	UpdateOrganization(ctx context.Context, callerUserID int, username string, req models.UpdateUserOrganizationRequest) error
	// UpdateCompany обновляет компанию пользователя.
	UpdateCompany(ctx context.Context, callerUserID int, username string, req models.UpdateUserCompanyRequest) error
	// Delete архивирует пользователя по username (soft-delete: is_active=false).
	Delete(ctx context.Context, callerUserID int, username string) error
	// Restore восстанавливает архивного пользователя (is_active=true).
	Restore(ctx context.Context, callerUserID int, username string) error
	// GetHistory возвращает историю действий над пользователем (по username).
	GetHistory(ctx context.Context, username string) ([]models.UserHistoryItem, error)
	// SetBanCache подключает кэш блокировок, чтобы архив/восстановление мгновенно
	// сбрасывали его (офбординг без ожидания TTL). Опционально (может не вызываться).
	SetBanCache(banCache *BanCheckService)
	// SetPasswordPolicyProvider подключает источник политики паролей.
	SetPasswordPolicyProvider(p PasswordPolicyProvider)

	// GetUserUnloadPlaces возвращает активные места разгрузки, привязанные к охраннику.
	GetUserUnloadPlaces(ctx context.Context, username string) ([]models.UnloadPlace, error)
	// SetUserUnloadPlaces заменяет привязку мест разгрузки для охранника (delete-all-then-recreate).
	SetUserUnloadPlaces(ctx context.Context, username string, req models.SetUserUnloadPlacesRequest) error
	// GetUserTables возвращает активные места прохода, привязанные к охраннику.
	GetUserTables(ctx context.Context, username string) ([]models.SystemTable, error)
	// SetUserTables заменяет привязку мест прохода для охранника (delete-all-then-recreate).
	SetUserTables(ctx context.Context, username string, req models.SetUserTablesRequest) error
}

// PasswordPolicyProvider отдаёт текущую политику паролей (реализуется SettingsService).
type PasswordPolicyProvider interface {
	GetPasswordPolicy() models.PasswordPolicy
}

type userService struct {
	db                  *gorm.DB
	notificationService NotificationService
	recorder            AuditRecorder
	banCache            *BanCheckService
	policy              PasswordPolicyProvider
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
		recorder:            NewAuditRecorder(db),
	}
}

// SetBanCache подключает кэш блокировок (опционально, после конструирования -
// в main.go banCheckService создаётся позже userService).
func (s *userService) SetBanCache(banCache *BanCheckService) {
	s.banCache = banCache
}

// SetPasswordPolicyProvider подключает источник политики паролей (опционально,
// после конструирования - settingsService в main.go создаётся позже userService).
func (s *userService) SetPasswordPolicyProvider(p PasswordPolicyProvider) {
	s.policy = p
}

// passwordPolicy возвращает активную политику, либо безопасный дефолт, если
// провайдер не подключён (валидация НЕ отключается - это критичная проверка).
func (s *userService) passwordPolicy() models.PasswordPolicy {
	if s.policy != nil {
		return s.policy.GetPasswordPolicy()
	}
	return models.DefaultPasswordPolicy()
}

// targetUserID резолвит id пользователя по username для записи в историю.
// Возвращает 0, если не найден (тогда лог пропускается).
func (s *userService) targetUserID(ctx context.Context, username string) int {
	var id int
	s.db.WithContext(ctx).Table("users").Select("id").Where("username = ?", username).Scan(&id)
	return id
}

// Create создаёт нового пользователя. Доступ - route-middleware page.admin.users.
//
// Пользователь может быть привязан к организации, компании или к обеим сразу.
// Хотя бы одно из двух поле должно быть > 0. Значение 0 означает "не привязан".
func (s *userService) Create(ctx context.Context, callerUserID int, req models.RegisterRequest) error {
	if req.OrganizationID <= 0 && req.CompanyID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Необходимо указать организацию или компанию (хотя бы одно)")
	}

	if err := ValidatePassword(s.passwordPolicy(), req.Password); err != nil {
		return err
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
	s.recorder.Log(ctx, nil, models.AuditEntityUser, &user.ID, models.UserActionCreated, &callerUserID, map[string]any{
		"username": user.Username,
		"type_id":  user.TypeID,
	})

	// Новый пользователь получает базовую роль "Пользователь" по умолчанию -- так роль
	// выдаёт стартовый набор прав (ТЗ). Best-effort: отсутствие базовой роли не валит создание.
	if user.RoleID == nil {
		var baseRole models.Role
		if err := s.db.WithContext(ctx).Where("code = ? AND is_system = ?", "user", true).First(&baseRole).Error; err == nil {
			if err := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", user.ID).
				Update("role_id", baseRole.ID).Error; err != nil {
				slog.Error("не удалось назначить базовую роль", "user_id", user.ID, "error", err)
			}
		}
	}
	return nil
}

// GetAll возвращает пользователей с JOIN на организацию, компанию и тип.
// По умолчанию только активные; includeArchived=true добавляет архивных.
func (s *userService) GetAll(ctx context.Context, includeArchived bool) ([]models.UserInfoResponse, error) {
	result := make([]models.UserInfoResponse, 0)
	q := s.db.WithContext(ctx).
		Table("users u").
		Select(`u.id, u.username, u.is_active, u.is_banned, u.is_super_admin, u.is_important,
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
func (s *userService) UpdateType(ctx context.Context, callerUserID int, username string, req models.UpdateUserTypeRequest) error {
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

	var oldType int
	s.db.WithContext(ctx).Table("users").Where("username = ?", username).Select("type_id").Row().Scan(&oldType)

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("type_id", req.TypeID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating user type")
	}

	if req.TypeID != oldType {
		if id := s.targetUserID(ctx, username); id != 0 {
			s.recorder.Log(ctx, nil, models.AuditEntityUser, &id, models.UserActionTypeChanged, &callerUserID, map[string]any{"old": oldType, "new": req.TypeID})
		}
	}
	return nil
}

// UpdatePassword хеширует и обновляет пароль пользователя.
// После смены пароля все refresh_tokens юзера отзываются - иначе старые
// сессии (возможно скомпрометированные) продолжили бы жить до истечения TTL.
func (s *userService) UpdatePassword(ctx context.Context, callerUserID int, username string, req models.UpdatePasswordRequest) error {
	if err := ValidatePassword(s.passwordPolicy(), req.Password); err != nil {
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
		s.recorder.Log(ctx, nil, models.AuditEntityUser, &user.ID, models.UserActionPasswordReset, &callerUserID, nil)

		// Уведомление о смене пароля. selfChange определяется по совпадению id
		// вызывающего с целевым (гейт page.admin.users допускает и смену
		// админом собственного пароля).
		s.notifyPasswordChanged(ctx, &user, callerUserID)
	}

	return nil
}

// notifyPasswordChanged создаёт уведомление о смене пароля. Вызывается
// после успешного апдейта; ошибки только логируются (уведомления не
// должны блокировать основной flow).
func (s *userService) notifyPasswordChanged(ctx context.Context, target *models.User, callerUserID int) {
	if s.notificationService == nil {
		return
	}

	// selfChange = пароль сменил сам владелец учётки (id вызывающего совпадает
	// с целевым). Иначе это сделал админ через раздел управления пользователями.
	selfChange := callerUserID != 0 && callerUserID == target.ID

	message := "Администратор изменил ваш пароль."
	if selfChange {
		message = "Ваш пароль был успешно изменён."
	}

	dataPayload := map[string]any{
		"changed_at":         time.Now().UTC().Format(time.RFC3339),
		"changed_by_user_id": callerUserID,
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
func (s *userService) UpdateInfo(ctx context.Context, callerUserID int, username string, req models.UpdateUserInfoRequest) error {
	// Снимок старых значений до апдейта - чтобы в историю писать дифф "старое -> новое"
	// и только по реально изменившимся полям (фронт шлёт все поля каждый раз).
	var prev struct {
		LastName    string
		FirstName   string
		MiddleName  string
		Position    string
		Email       string
		Phone       string
		IsImportant bool
	}
	s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Select("last_name", "first_name", "middle_name", "position", "email", "phone", "is_important").
		Scan(&prev)

	updates := map[string]interface{}{
		"last_name":   req.LastName,
		"first_name":  req.FirstName,
		"middle_name": req.MiddleName,
		"position":    req.Position,
		"email":       req.Email,
		"phone":       req.Phone,
	}
	if req.IsImportant != nil {
		updates["is_important"] = *req.IsImportant
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Updates(updates).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating user info")
	}

	if id := s.targetUserID(ctx, username); id != 0 {
		// Только изменившиеся поля, как {old, new}. Если ничего не поменялось - не логируем.
		details := map[string]any{}
		diff := func(key string, np *string, old string) {
			if np != nil && *np != old {
				details[key] = map[string]any{"old": old, "new": *np}
			}
		}
		diff("last_name", req.LastName, prev.LastName)
		diff("first_name", req.FirstName, prev.FirstName)
		diff("middle_name", req.MiddleName, prev.MiddleName)
		diff("position", req.Position, prev.Position)
		diff("email", req.Email, prev.Email)
		diff("phone", req.Phone, prev.Phone)
		if req.IsImportant != nil && *req.IsImportant != prev.IsImportant {
			details["is_important"] = map[string]any{"old": prev.IsImportant, "new": *req.IsImportant}
		}
		if len(details) > 0 {
			s.recorder.Log(ctx, nil, models.AuditEntityUser, &id, models.UserActionUpdated, &callerUserID, details)
		}
	}
	return nil
}

// UpdateOrganization обновляет organization_id пользователя.
func (s *userService) UpdateOrganization(ctx context.Context, callerUserID int, username string, req models.UpdateUserOrganizationRequest) error {
	var prev struct{ OrganizationID *int }
	s.db.WithContext(ctx).Table("users").Where("username = ?", username).Select("organization_id").Scan(&prev)
	oldVal := 0
	if prev.OrganizationID != nil {
		oldVal = *prev.OrganizationID
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("organization_id", req.OrganizationID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating organization")
	}

	if req.OrganizationID != oldVal {
		if id := s.targetUserID(ctx, username); id != 0 {
			s.recorder.Log(ctx, nil, models.AuditEntityUser, &id, models.UserActionOrgChanged, &callerUserID, map[string]any{"old": prev.OrganizationID, "new": req.OrganizationID})
		}
	}
	return nil
}

// UpdateCompany обновляет company_id пользователя.
func (s *userService) UpdateCompany(ctx context.Context, callerUserID int, username string, req models.UpdateUserCompanyRequest) error {
	var prev struct{ CompanyID *int }
	s.db.WithContext(ctx).Table("users").Where("username = ?", username).Select("company_id").Scan(&prev)
	oldVal := 0
	if prev.CompanyID != nil {
		oldVal = *prev.CompanyID
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("company_id", req.CompanyID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating company")
	}

	if req.CompanyID != oldVal {
		if id := s.targetUserID(ctx, username); id != 0 {
			s.recorder.Log(ctx, nil, models.AuditEntityUser, &id, models.UserActionCompanyChanged, &callerUserID, map[string]any{"old": prev.CompanyID, "new": req.CompanyID})
		}
	}
	return nil
}

// Delete архивирует пользователя (soft-delete: is_active=false). Строка остаётся,
// поэтому ссылки заявок (sender_user_id и др.) не осиротевают; login/refresh
// блокируются по is_active, активные refresh-токены отзываются.
func (s *userService) Delete(ctx context.Context, callerUserID int, username string) error {
	return s.setActive(ctx, username, false, callerUserID)
}

// Restore восстанавливает архивного пользователя (is_active=true).
func (s *userService) Restore(ctx context.Context, callerUserID int, username string) error {
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
	// Архив = мгновенный офбординг (BanCheck даёт 403), поэтому супер-админа,
	// как и при бане, архивировать нельзя - иначе админ может вырубить владельца.
	if !active && user.IsSuperAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "Нельзя архивировать супер-администратора")
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
	s.recorder.Log(ctx, nil, models.AuditEntityUser, &user.ID, action, &callerUserID, nil)

	// Сбрасываем кэш блокировок, чтобы архив/восстановление подействовали мгновенно
	// (BanCheck на следующем запросе перечитает is_active, не дожидаясь TTL).
	if s.banCache != nil {
		s.banCache.Invalidate(user.ID)
	}
	return nil
}

// GetHistory возвращает историю действий над пользователем по username (admin-only).
// Переходный период #870: новые записи идут в audit_log, старые лежат в user_histories.
// UNION объединяет обе таблицы в одинаковую форму ответа.
func (s *userService) GetHistory(ctx context.Context, username string) ([]models.UserHistoryItem, error) {
	id := s.targetUserID(ctx, username)
	if id == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "User not found")
	}
	const actorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`
	sql := `
		SELECT id, action_type, details, actor_user_id, actor_name, created_at FROM (
			SELECT h.id AS id, h.action_type AS action_type, h.details AS details,
				h.actor_user_id AS actor_user_id, ` + actorName + ` AS actor_name, h.created_at AS created_at
			FROM user_histories h LEFT JOIN users u ON u.id = h.actor_user_id
			WHERE h.target_user_id = ?
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
	if err := s.db.WithContext(ctx).Raw(sql, id, models.AuditEntityUser, id).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user history")
	}

	items := make([]models.UserHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.UserHistoryItem{
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

// resolveUserID резолвит id по username, возвращает 404 если не найден.
func (s *userService) resolveUserID(ctx context.Context, username string) (int, error) {
	id := s.targetUserID(ctx, username)
	if id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "User not found")
	}
	return id, nil
}

// GetUserUnloadPlaces возвращает активные места разгрузки, привязанные к пользователю.
// GET-пикер намеренно фильтрует is_active=true, чтобы архивные места не попадали в список назначения.
func (s *userService) GetUserUnloadPlaces(ctx context.Context, username string) ([]models.UnloadPlace, error) {
	userID, err := s.resolveUserID(ctx, username)
	if err != nil {
		return nil, err
	}
	var places []models.UnloadPlace
	if err := s.db.WithContext(ctx).
		Table("unload_places up").
		Select("up.*").
		Joins("JOIN security_user_unload_places sup ON sup.unload_place_id = up.id").
		Where("sup.user_id = ? AND up.is_active = ?", userID, true).
		Order("up.name").
		Scan(&places).Error; err != nil {
		slog.Error("не удалось получить места разгрузки пользователя", "user_id", userID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user unload places")
	}
	return places, nil
}

// SetUserUnloadPlaces заменяет привязку мест разгрузки (delete-all-then-recreate в транзакции).
func (s *userService) SetUserUnloadPlaces(ctx context.Context, username string, req models.SetUserUnloadPlacesRequest) error {
	userID, err := s.resolveUserID(ctx, username)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.SecurityUserUnloadPlace{}).Error; err != nil {
			slog.Error("не удалось удалить старые места разгрузки пользователя", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating unload places")
		}
		for _, placeID := range req.UnloadPlaceIDs {
			row := models.SecurityUserUnloadPlace{UserID: userID, UnloadPlaceID: placeID}
			if err := tx.Create(&row).Error; err != nil {
				slog.Error("не удалось добавить место разгрузки пользователю", "place_id", placeID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating unload places")
			}
		}
		return nil
	})
}

// GetUserTables возвращает активные места прохода, привязанные к пользователю.
func (s *userService) GetUserTables(ctx context.Context, username string) ([]models.SystemTable, error) {
	userID, err := s.resolveUserID(ctx, username)
	if err != nil {
		return nil, err
	}
	var tables []models.SystemTable
	if err := s.db.WithContext(ctx).
		Table("system_tables st").
		Select("st.*").
		Joins("JOIN security_user_tables sut ON sut.table_id = st.id").
		Where("sut.user_id = ? AND st.is_active = ?", userID, true).
		Order("st.name").
		Scan(&tables).Error; err != nil {
		slog.Error("не удалось получить места прохода пользователя", "user_id", userID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user tables")
	}
	return tables, nil
}

// SetUserTables заменяет привязку мест прохода (delete-all-then-recreate в транзакции).
func (s *userService) SetUserTables(ctx context.Context, username string, req models.SetUserTablesRequest) error {
	userID, err := s.resolveUserID(ctx, username)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.SecurityUserTable{}).Error; err != nil {
			slog.Error("не удалось удалить старые места прохода пользователя", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating tables")
		}
		for _, tableID := range req.TableIDs {
			row := models.SecurityUserTable{UserID: userID, TableID: tableID}
			if err := tx.Create(&row).Error; err != nil {
				slog.Error("не удалось добавить место прохода пользователю", "table_id", tableID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating tables")
			}
		}
		return nil
	})
}
