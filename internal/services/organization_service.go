package services

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// OrganizationService определяет интерфейс бизнес-логики организаций.
type OrganizationService interface {
	// GetAll возвращает список активных организаций (id, name) - для выпадающих списков.
	GetAll(ctx context.Context) ([]OrganizationInfoResponse, error)

	// Create создаёт новую организацию. callerUserID - актор для аудита.
	Create(ctx context.Context, callerUserID int, req CreateOrganizationRequest) (*OrganizationInfoResponse, error)

	// Update обновляет название организации по ID. callerUserID - актор для аудита.
	Update(ctx context.Context, callerUserID, id int, req CreateOrganizationRequest) (*OrganizationInfoResponse, error)

	// Delete архивирует организацию (soft-delete). Нельзя при активных пользователях.
	Delete(ctx context.Context, callerUserID, id int) error

	// Restore восстанавливает организацию из архива.
	Restore(ctx context.Context, callerUserID, id int) error

	// GetHistory возвращает историю изменений организации.
	GetHistory(ctx context.Context, id int) ([]models.OrganizationHistoryItem, error)

	// GetWithUsers возвращает организации с количеством пользователей. includeArchived добавляет архивные.
	GetWithUsers(ctx context.Context, includeArchived bool) ([]OrganizationWithUsersResponse, error)

	// GetWithUsersExtended возвращает организации с количеством пользователей и местами разгрузки.
	GetWithUsersExtended(ctx context.Context, includeArchived bool) ([]map[string]any, error)

	// GetMyOrganization возвращает организацию текущего пользователя по username.
	GetMyOrganization(ctx context.Context, username string) (*MyOrganizationResponse, error)

	// GetOrganizationUsers возвращает ответственных пользователей организации.
	GetOrganizationUsers(ctx context.Context, orgID int) ([]OrganizationUserResponse, error)

	// UpdateOrganizationUsers обновляет ответственных пользователей организации (replace-стратегия).
	UpdateOrganizationUsers(ctx context.Context, orgID int, req UpdateOrganizationUsersRequest) error

	// GetOrganizationTables возвращает таблицы, привязанные к организации.
	GetOrganizationTables(ctx context.Context, orgID int) ([]OrganizationTableResponse, error)

	// UpdateOrganizationTables заменяет привязку таблиц к организации.
	UpdateOrganizationTables(ctx context.Context, orgID int, req UpdateOrganizationTablesRequest) error

	// GetOrganizationUnloadPlaces возвращает места разгрузки организации.
	GetOrganizationUnloadPlaces(ctx context.Context, orgID int) ([]OrganizationUnloadPlaceResponse, error)

	// UpdateOrganizationUnloadPlaces заменяет привязку мест разгрузки к организации.
	UpdateOrganizationUnloadPlaces(ctx context.Context, orgID int, req UpdateOrganizationUnloadPlacesRequest) error
}

// --- DTO: запросы ---

// CreateOrganizationRequest — тело запроса на создание/обновление организации.
type CreateOrganizationRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

// UpdateOrganizationUsersRequest — тело запроса на обновление ответственных.
type UpdateOrganizationUsersRequest struct {
	Users []OrganizationUserRequest `json:"users"`
}

// OrganizationUserRequest — один пользователь в запросе обновления ответственных.
type OrganizationUserRequest struct {
	Username         string `json:"username"`
	IsPrimary        *bool  `json:"is_primary"`
	RequiredApproval *bool  `json:"required_approval"`
}

// UpdateOrganizationTablesRequest — тело запроса на обновление таблиц организации.
type UpdateOrganizationTablesRequest struct {
	TableIDs []int `json:"table_ids"`
}

// UpdateOrganizationUnloadPlacesRequest — тело запроса на обновление мест разгрузки.
type UpdateOrganizationUnloadPlacesRequest struct {
	UnloadPlaceIDs []int `json:"unload_place_ids"`
}

// --- DTO: ответы ---

// OrganizationInfoResponse — краткая информация об организации.
type OrganizationInfoResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// OrganizationWithUsersResponse — организация с количеством пользователей.
type OrganizationWithUsersResponse struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	IsActive  bool   `json:"is_active"`
	UserCount int64  `json:"user_count"`
}

// OrganizationUserResponse — ответственный пользователь организации.
type OrganizationUserResponse struct {
	ID               int     `json:"id"`
	Username         string  `json:"username"`
	LastName         *string `json:"last_name"`
	FirstName        *string `json:"first_name"`
	MiddleName       *string `json:"middle_name"`
	Position         *string `json:"position"`
	IsPrimary        *bool   `json:"is_primary"`
	RequiredApproval *bool   `json:"required_approval"`
}

// OrganizationTableResponse — таблица, привязанная к организации.
type OrganizationTableResponse struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	DisplayName *string `json:"display_name"`
	TableType   string  `json:"table_type"`
}

// OrganizationUnloadPlaceResponse — место разгрузки организации.
type OrganizationUnloadPlaceResponse struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// MyOrganizationResponse — организация текущего пользователя.
type MyOrganizationResponse struct {
	Organization   string `json:"organization"`
	OrganizationID int    `json:"organization_id"`
}

// --- Реализация ---

type organizationService struct {
	db      *gorm.DB
	history OrganizationHistoryService
}

// NewOrganizationService создаёт новый экземпляр сервиса организаций.
func NewOrganizationService(db *gorm.DB) OrganizationService {
	return &organizationService{db: db, history: NewOrganizationHistoryService(db)}
}

// CheckAdminPermissions проверяет, что пользователь имеет тип buropropuskov.
// Вызывается из хендлера перед admin-операциями.
func CheckAdminPermissions(db *gorm.DB, ctx context.Context, username string) error {
	var result struct {
		Code string
	}
	err := db.WithContext(ctx).
		Table("users").
		Select("user_types.code").
		Joins("JOIN user_types ON users.type_id = user_types.id").
		Where("users.username = ?", username).
		Scan(&result).Error
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}
	if result.Code != "buropropuskov" {
		return echo.NewHTTPError(http.StatusForbidden, "Insufficient permissions")
	}
	return nil
}

// GetAll возвращает список всех организаций.
func (s *organizationService) GetAll(ctx context.Context) ([]OrganizationInfoResponse, error) {
	orgs := make([]OrganizationInfoResponse, 0)
	err := s.db.WithContext(ctx).
		Table("organizations").
		Select("id, name").
		Where("is_active = ?", true).
		Order("name").
		Scan(&orgs).Error
	if err != nil {
		slog.Error("Не удалось получить список организаций", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organizations")
	}
	return orgs, nil
}

// Create создаёт новую организацию.
func (s *organizationService) Create(ctx context.Context, callerUserID int, req CreateOrganizationRequest) (*OrganizationInfoResponse, error) {
	var active int64
	if err := s.db.WithContext(ctx).Model(&models.Organization{}).
		Where("name = ? AND is_active = ?", req.Name, true).Count(&active).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking organization")
	}
	if active > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Организация с таким названием уже существует")
	}

	org := models.Organization{Name: req.Name, IsActive: true}
	if err := s.db.WithContext(ctx).Create(&org).Error; err != nil {
		slog.Error("Не удалось создать организацию", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating organization")
	}
	slog.Info("организация создана", "id", org.ID, "name", org.Name)
	s.history.Log(ctx, org.ID, &callerUserID, models.OrganizationActionCreated, map[string]any{"name": org.Name})
	return &OrganizationInfoResponse{ID: org.ID, Name: org.Name}, nil
}

// Update обновляет название организации по ID.
func (s *organizationService) Update(ctx context.Context, callerUserID, id int, req CreateOrganizationRequest) (*OrganizationInfoResponse, error) {
	var org models.Organization
	if err := s.db.WithContext(ctx).First(&org, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Organization not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organization")
	}
	if !org.IsActive {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Нельзя переименовать архивную организацию")
	}

	// Конфликт с другой активной организацией по имени (partial unique).
	var dup int64
	if err := s.db.WithContext(ctx).Model(&models.Organization{}).
		Where("name = ? AND is_active = ? AND id <> ?", req.Name, true, id).Count(&dup).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking organization")
	}
	if dup > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Организация с таким названием уже существует")
	}

	if err := s.db.WithContext(ctx).Model(&models.Organization{}).
		Where("id = ?", id).Update("name", req.Name).Error; err != nil {
		slog.Error("Не удалось обновить организацию", "id", id, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating organization")
	}
	slog.Info("организация обновлена", "id", id, "name", req.Name)
	s.history.Log(ctx, id, &callerUserID, models.OrganizationActionRenamed, map[string]any{"name": req.Name})
	return &OrganizationInfoResponse{ID: id, Name: req.Name}, nil
}

// Delete архивирует организацию (soft-delete: is_active=false). Строка остаётся,
// поэтому FK заявок/сотрудников/машин не осиротевают. Блокируется при наличии
// АКТИВНЫХ пользователей в этой организации (их некуда переназначить из активного
// списка). Исторические заявки/сотрудники/машины архив не блокируют.
func (s *organizationService) Delete(ctx context.Context, callerUserID, id int) error {
	var org models.Organization
	if err := s.db.WithContext(ctx).First(&org, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Organization not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organization")
	}
	if !org.IsActive {
		return nil // уже в архиве
	}

	var activeUsers int64
	if err := s.db.WithContext(ctx).
		Model(&models.User{}).
		Where("organization_id = ? AND is_active = ?", id, true).
		Count(&activeUsers).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking users")
	}
	if activeUsers > 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Нельзя архивировать организацию с активными пользователями")
	}

	if err := s.db.WithContext(ctx).Model(&models.Organization{}).
		Where("id = ?", id).Update("is_active", false).Error; err != nil {
		slog.Error("Не удалось архивировать организацию", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error archiving organization")
	}
	slog.Info("организация архивирована", "id", id)
	s.history.Log(ctx, id, &callerUserID, models.OrganizationActionArchived, nil)
	return nil
}

// Restore восстанавливает организацию из архива (is_active=true). Если активная
// организация с таким именем уже есть - конфликт (partial unique), вернём 400.
func (s *organizationService) Restore(ctx context.Context, callerUserID, id int) error {
	var org models.Organization
	if err := s.db.WithContext(ctx).First(&org, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Organization not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organization")
	}
	if org.IsActive {
		return nil // уже активна
	}

	var active int64
	if err := s.db.WithContext(ctx).Model(&models.Organization{}).
		Where("name = ? AND is_active = ? AND id <> ?", org.Name, true, id).Count(&active).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking organization")
	}
	if active > 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Активная организация с таким названием уже существует - переименуйте перед восстановлением")
	}

	if err := s.db.WithContext(ctx).Model(&models.Organization{}).
		Where("id = ?", id).Update("is_active", true).Error; err != nil {
		slog.Error("Не удалось восстановить организацию", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error restoring organization")
	}
	slog.Info("организация восстановлена", "id", id)
	s.history.Log(ctx, id, &callerUserID, models.OrganizationActionRestored, nil)
	return nil
}

// GetHistory возвращает историю изменений организации.
func (s *organizationService) GetHistory(ctx context.Context, id int) ([]models.OrganizationHistoryItem, error) {
	return s.history.GetHistory(ctx, id)
}

// GetWithUsers возвращает организации с количеством привязанных пользователей.
func (s *organizationService) GetWithUsers(ctx context.Context, includeArchived bool) ([]OrganizationWithUsersResponse, error) {
	orgs := make([]OrganizationWithUsersResponse, 0)
	q := s.db.WithContext(ctx).
		Table("organizations o").
		Select("o.id, o.name, o.is_active, COUNT(u.id) FILTER (WHERE u.is_active = true) as user_count").
		Joins("LEFT JOIN users u ON u.organization_id = o.id").
		Group("o.id, o.name, o.is_active").
		Order("o.name")
	if !includeArchived {
		q = q.Where("o.is_active = ?", true)
	}
	if err := q.Scan(&orgs).Error; err != nil {
		slog.Error("Не удалось получить организации с пользователями", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organizations")
	}
	return orgs, nil
}

// GetWithUsersExtended возвращает организации с пользователями и местами разгрузки.
func (s *organizationService) GetWithUsersExtended(ctx context.Context, includeArchived bool) ([]map[string]any, error) {
	// Получаем базовые данные организаций
	orgs := make([]OrganizationWithUsersResponse, 0)
	q := s.db.WithContext(ctx).
		Table("organizations o").
		Select("o.id, o.name, o.is_active, COUNT(u.id) FILTER (WHERE u.is_active = true) as user_count").
		Joins("LEFT JOIN users u ON u.organization_id = o.id").
		Group("o.id, o.name, o.is_active").
		Order("o.name")
	if !includeArchived {
		q = q.Where("o.is_active = ?", true)
	}
	if err := q.Scan(&orgs).Error; err != nil {
		slog.Error("Не удалось получить расширенную информацию об организациях", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organizations")
	}

	result := make([]map[string]any, 0, len(orgs))
	for _, org := range orgs {
		// Для каждой организации получаем места разгрузки
		places := make([]OrganizationUnloadPlaceResponse, 0)
		s.db.WithContext(ctx).
			Table("unload_places up").
			Select("up.id, up.name, up.description").
			Joins("JOIN organization_unload_places oup ON up.id = oup.unload_place_id").
			Where("oup.organization_id = ?", org.ID).
			Order("up.name").
			Scan(&places)

		result = append(result, map[string]any{
			"id":            org.ID,
			"name":          org.Name,
			"is_active":     org.IsActive,
			"user_count":    org.UserCount,
			"unload_places": places,
		})
	}
	return result, nil
}

// GetMyOrganization возвращает организацию текущего пользователя.
func (s *organizationService) GetMyOrganization(ctx context.Context, username string) (*MyOrganizationResponse, error) {
	var resp MyOrganizationResponse
	err := s.db.WithContext(ctx).
		Table("users u").
		Select("o.name as organization, u.organization_id").
		Joins("JOIN organizations o ON u.organization_id = o.id").
		Where("u.username = ?", username).
		Scan(&resp).Error
	if err != nil {
		slog.Error("Не удалось получить организацию пользователя", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось получить организацию")
	}
	return &resp, nil
}

// GetOrganizationUsers возвращает ответственных пользователей организации.
func (s *organizationService) GetOrganizationUsers(ctx context.Context, orgID int) ([]OrganizationUserResponse, error) {
	users := make([]OrganizationUserResponse, 0)
	err := s.db.WithContext(ctx).
		Table("users u").
		Select("u.id, u.username, u.last_name, u.first_name, u.middle_name, u.position, ou.is_primary, ou.required_approval").
		Joins("INNER JOIN organization_users ou ON u.id = ou.user_id").
		Where("ou.organization_id = ?", orgID).
		Order("ou.is_primary DESC, u.last_name, u.first_name").
		Scan(&users).Error
	if err != nil {
		slog.Error("Не удалось получить пользователей организации", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organization users")
	}
	return users, nil
}

// UpdateOrganizationUsers заменяет ответственных пользователей организации.
func (s *organizationService) UpdateOrganizationUsers(ctx context.Context, orgID int, req UpdateOrganizationUsersRequest) error {
	// Проверяем, что только один пользователь назначен главным
	primaryCount := 0
	for _, u := range req.Users {
		if u.IsPrimary != nil && *u.IsPrimary {
			primaryCount++
		}
	}
	if primaryCount > 1 {
		return echo.NewHTTPError(http.StatusBadRequest, "Только один пользователь может быть главным ответственным")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Удаляем старые связи
		if err := tx.Where("organization_id = ?", orgID).Delete(&models.OrganizationUser{}).Error; err != nil {
			slog.Error("Не удалось удалить старых пользователей организации", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating organization users")
		}

		// Добавляем новых пользователей
		for _, userReq := range req.Users {
			var user models.User
			if err := tx.Where("username = ?", userReq.Username).First(&user).Error; err != nil {
				slog.Warn("Пользователь не найден", "username", userReq.Username)
				continue
			}

			isPrimary := false
			if userReq.IsPrimary != nil {
				isPrimary = *userReq.IsPrimary
			}
			requiredApproval := false
			if userReq.RequiredApproval != nil {
				requiredApproval = *userReq.RequiredApproval
			}

			ou := models.OrganizationUser{
				OrganizationID:   orgID,
				UserID:           user.ID,
				IsPrimary:        isPrimary,
				RequiredApproval: requiredApproval,
			}
			if err := tx.Create(&ou).Error; err != nil {
				slog.Error("Не удалось добавить пользователя в организацию", "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating organization users")
			}
		}
		return nil
	})
}

// GetOrganizationTables возвращает таблицы, привязанные к организации.
func (s *organizationService) GetOrganizationTables(ctx context.Context, orgID int) ([]OrganizationTableResponse, error) {
	tables := make([]OrganizationTableResponse, 0)
	err := s.db.WithContext(ctx).
		Table("system_tables st").
		Select("st.id, st.name, st.display_name, st.table_type").
		Joins("JOIN organization_tables ot ON st.id = ot.table_id").
		Where("ot.organization_id = ? AND st.is_active = true", orgID).
		Order("st.display_name").
		Scan(&tables).Error
	if err != nil {
		slog.Error("Не удалось получить таблицы организации", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organization tables")
	}
	return tables, nil
}

// UpdateOrganizationTables заменяет привязку таблиц к организации.
func (s *organizationService) UpdateOrganizationTables(ctx context.Context, orgID int, req UpdateOrganizationTablesRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Удаляем старые связи
		if err := tx.Where("organization_id = ?", orgID).Delete(&models.OrganizationTable{}).Error; err != nil {
			slog.Error("Не удалось удалить старые таблицы организации", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating organization tables")
		}

		// Добавляем новые связи
		for _, tableID := range req.TableIDs {
			ot := models.OrganizationTable{
				OrganizationID: orgID,
				TableID:        tableID,
			}
			if err := tx.Create(&ot).Error; err != nil {
				slog.Error("Не удалось добавить таблицу в организацию", "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating organization tables")
			}
		}
		return nil
	})
}

// GetOrganizationUnloadPlaces возвращает места разгрузки организации.
func (s *organizationService) GetOrganizationUnloadPlaces(ctx context.Context, orgID int) ([]OrganizationUnloadPlaceResponse, error) {
	places := make([]OrganizationUnloadPlaceResponse, 0)
	err := s.db.WithContext(ctx).
		Table("unload_places up").
		Select("up.id, up.name, up.description").
		Joins("JOIN organization_unload_places oup ON up.id = oup.unload_place_id").
		Where("oup.organization_id = ? AND up.is_active = true", orgID).
		Order("up.name").
		Scan(&places).Error
	if err != nil {
		slog.Error("Не удалось получить места разгрузки организации", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organization unload places")
	}
	return places, nil
}

// UpdateOrganizationUnloadPlaces заменяет привязку мест разгрузки к организации.
func (s *organizationService) UpdateOrganizationUnloadPlaces(ctx context.Context, orgID int, req UpdateOrganizationUnloadPlacesRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Удаляем старые связи
		if err := tx.Where("organization_id = ?", orgID).Delete(&models.OrganizationUnloadPlace{}).Error; err != nil {
			slog.Error("Не удалось удалить старые места разгрузки организации", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating unload places")
		}

		// Добавляем новые связи
		for _, placeID := range req.UnloadPlaceIDs {
			oup := models.OrganizationUnloadPlace{
				OrganizationID: orgID,
				UnloadPlaceID:  placeID,
			}
			if err := tx.Create(&oup).Error; err != nil {
				slog.Error("Не удалось добавить место разгрузки в организацию", "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating unload places")
			}
		}
		return nil
	})
}
