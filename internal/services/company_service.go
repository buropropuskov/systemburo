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

// CompanyService определяет интерфейс бизнес-логики для работы с компаниями.
type CompanyService interface {
	// GetAll возвращает список всех компаний, отсортированных по имени.
	GetAll(ctx context.Context) ([]models.Company, error)

	// GetWithUsers возвращает компании с количеством пользователей. includeArchived добавляет архивные.
	GetWithUsers(ctx context.Context, includeArchived bool) ([]CompanyWithUsersResponse, error)

	// GetWithUsersExtended возвращает компании с количеством пользователей и местами разгрузки.
	GetWithUsersExtended(ctx context.Context, includeArchived bool) ([]CompanyWithUsersExtendedResponse, error)

	// Create создаёт новую компанию. callerUserID - актор для аудита. Авторизация (page.admin) - на роут-middleware.
	Create(ctx context.Context, callerUserID int, req CreateCompanyRequest) (*models.Company, error)

	// Update обновляет название компании. callerUserID - актор для аудита.
	Update(ctx context.Context, callerUserID, companyID int, req CreateCompanyRequest) (*models.Company, error)

	// Delete архивирует компанию (soft-delete). callerUserID - актор. Нельзя при активных пользователях.
	Delete(ctx context.Context, callerUserID, companyID int) error

	// Restore восстанавливает компанию из архива. callerUserID - актор.
	Restore(ctx context.Context, callerUserID, companyID int) error

	// GetHistory возвращает историю изменений компании.
	GetHistory(ctx context.Context, companyID int) ([]models.CompanyHistoryItem, error)

	// GetUsers возвращает ответственных пользователей компании.
	GetUsers(ctx context.Context, companyID int) ([]CompanyUserResponse, error)

	// UpdateUsers обновляет ответственных пользователей компании с поддержкой обязательного согласования.
	UpdateUsers(ctx context.Context, companyID int, req UpdateCompanyUsersRequest) error

	// GetUnloadPlaces возвращает активные места разгрузки компании.
	GetUnloadPlaces(ctx context.Context, companyID int) ([]CompanyUnloadPlaceResponse, error)

	// UpdateUnloadPlaces обновляет привязку мест разгрузки к компании.
	UpdateUnloadPlaces(ctx context.Context, companyID int, req UpdateCompanyUnloadPlacesRequest) error

	// GetTables возвращает активные таблицы компании.
	GetTables(ctx context.Context, companyID int) ([]CompanyTableResponse, error)

	// UpdateTables обновляет привязку таблиц к компании.
	UpdateTables(ctx context.Context, companyID int, req UpdateCompanyTablesRequest) error
}

// --- DTO: запросы ---

// CreateCompanyRequest тело запроса создания/обновления компании.
type CreateCompanyRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

// UpdateCompanyUsersRequest тело запроса обновления ответственных пользователей.
type UpdateCompanyUsersRequest struct {
	Users []CompanyUserRequest `json:"users"`
}

// CompanyUserRequest один пользователь в запросе обновления ответственных.
type CompanyUserRequest struct {
	Username         string `json:"username"`
	IsPrimary        *bool  `json:"is_primary"`
	RequiredApproval *bool  `json:"required_approval"`
}

// UpdateCompanyUnloadPlacesRequest тело запроса обновления мест разгрузки.
type UpdateCompanyUnloadPlacesRequest struct {
	UnloadPlaceIDs []int `json:"unload_place_ids"`
}

// UpdateCompanyTablesRequest тело запроса обновления таблиц.
type UpdateCompanyTablesRequest struct {
	TableIDs []int `json:"table_ids"`
}

// --- DTO: ответы ---

// CompanyWithUsersResponse компания с количеством пользователей.
type CompanyWithUsersResponse struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	IsActive  bool   `json:"is_active"`
	UserCount int64  `json:"user_count"`
}

// CompanyUnloadPlaceResponse место разгрузки компании.
type CompanyUnloadPlaceResponse struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// CompanyTableResponse таблица, привязанная к компании.
type CompanyTableResponse struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	TableType   string `json:"table_type"`
}

// CompanyWithUsersExtendedResponse расширенная информация о компании.
type CompanyWithUsersExtendedResponse struct {
	ID           int                          `json:"id"`
	Name         string                       `json:"name"`
	IsActive     bool                         `json:"is_active"`
	UserCount    *int64                       `json:"user_count"`
	UnloadPlaces []CompanyUnloadPlaceResponse `json:"unload_places"`
}

// CompanyUserResponse ответственный пользователь компании.
type CompanyUserResponse struct {
	ID               int     `json:"id"`
	Username         string  `json:"username"`
	LastName         *string `json:"last_name"`
	FirstName        *string `json:"first_name"`
	MiddleName       *string `json:"middle_name"`
	Position         *string `json:"position"`
	IsPrimary        *bool   `json:"is_primary"`
	RequiredApproval *bool   `json:"required_approval"`
}

// --- Реализация ---

type companyService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewCompanyService создаёт экземпляр сервиса компаний.
func NewCompanyService(db *gorm.DB) CompanyService {
	return &companyService{db: db, recorder: NewAuditRecorder(db)}
}

// GetAll возвращает список всех компаний.
func (s *companyService) GetAll(ctx context.Context) ([]models.Company, error) {
	companies := make([]models.Company, 0)
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Order("name").Find(&companies).Error; err != nil {
		slog.Error("не удалось получить компании", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching companies")
	}
	return companies, nil
}

// GetWithUsers возвращает компании с количеством привязанных пользователей.
func (s *companyService) GetWithUsers(ctx context.Context, includeArchived bool) ([]CompanyWithUsersResponse, error) {
	result := make([]CompanyWithUsersResponse, 0)
	q := s.db.WithContext(ctx).
		Table("companies c").
		Select("c.id, c.name, c.is_active, COUNT(u.id) FILTER (WHERE u.is_active = true) as user_count").
		Joins("LEFT JOIN users u ON u.company_id = c.id").
		Group("c.id, c.name, c.is_active").
		Order("c.name")
	if !includeArchived {
		q = q.Where("c.is_active = ?", true)
	}
	if err := q.Scan(&result).Error; err != nil {
		slog.Error("не удалось получить компании с пользователями", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching companies")
	}
	return result, nil
}

// GetWithUsersExtended возвращает компании с пользователями и местами разгрузки.
func (s *companyService) GetWithUsersExtended(ctx context.Context, includeArchived bool) ([]CompanyWithUsersExtendedResponse, error) {
	// Получаем базовые данные компаний с количеством пользователей
	type companyRow struct {
		ID        int
		Name      string
		IsActive  bool
		UserCount *int64
	}
	companies := make([]companyRow, 0)
	q := s.db.WithContext(ctx).
		Table("companies c").
		Select("c.id, c.name, c.is_active, COUNT(u.id) FILTER (WHERE u.is_active = true) as user_count").
		Joins("LEFT JOIN users u ON u.company_id = c.id").
		Group("c.id, c.name, c.is_active").
		Order("c.name")
	if !includeArchived {
		q = q.Where("c.is_active = ?", true)
	}
	if err := q.Scan(&companies).Error; err != nil {
		slog.Error("не удалось получить расширенную информацию о компаниях", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching companies")
	}

	result := make([]CompanyWithUsersExtendedResponse, 0, len(companies))
	for _, c := range companies {
		// Для каждой компании получаем места разгрузки
		places := make([]CompanyUnloadPlaceResponse, 0)
		err := s.db.WithContext(ctx).
			Table("unload_places up").
			Select("up.id, up.name, up.description").
			Joins("JOIN companies_unload_places cup ON up.id = cup.unload_place_id").
			Where("cup.company_id = ?", c.ID).
			Order("up.name").
			Scan(&places).Error
		if err != nil {
			slog.Warn("не удалось получить места разгрузки компании", "company_id", c.ID, "error", err)
			places = []CompanyUnloadPlaceResponse{}
		}

		result = append(result, CompanyWithUsersExtendedResponse{
			ID:           c.ID,
			Name:         c.Name,
			IsActive:     c.IsActive,
			UserCount:    c.UserCount,
			UnloadPlaces: places,
		})
	}

	return result, nil
}

// Create создаёт новую компанию (admin-only).
func (s *companyService) Create(ctx context.Context, callerUserID int, req CreateCompanyRequest) (*models.Company, error) {
	var active int64
	if err := s.db.WithContext(ctx).Model(&models.Company{}).
		Where("name = ? AND is_active = ?", req.Name, true).Count(&active).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking company")
	}
	if active > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Компания с таким названием уже существует")
	}

	company := models.Company{Name: req.Name, IsActive: true}
	if err := s.db.WithContext(ctx).Create(&company).Error; err != nil {
		slog.Error("не удалось создать компанию", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating company")
	}
	slog.Info("компания создана", "id", company.ID, "name", company.Name)
	s.recorder.Log(ctx, nil, models.AuditEntityCompany, &company.ID, models.CompanyActionCreated, &callerUserID, map[string]any{"name": company.Name})
	return &company, nil
}

// Update обновляет название компании (admin-only).
func (s *companyService) Update(ctx context.Context, callerUserID, companyID int, req CreateCompanyRequest) (*models.Company, error) {
	var company models.Company
	if err := s.db.WithContext(ctx).First(&company, companyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Company not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company")
	}
	if !company.IsActive {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Нельзя переименовать архивную компанию")
	}

	var dup int64
	if err := s.db.WithContext(ctx).Model(&models.Company{}).
		Where("name = ? AND is_active = ? AND id <> ?", req.Name, true, companyID).Count(&dup).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking company")
	}
	if dup > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Компания с таким названием уже существует")
	}

	company.Name = req.Name
	if err := s.db.WithContext(ctx).Save(&company).Error; err != nil {
		slog.Error("не удалось обновить компанию", "id", companyID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating company")
	}
	slog.Info("компания обновлена", "id", companyID, "name", company.Name)
	s.recorder.Log(ctx, nil, models.AuditEntityCompany, &companyID, models.CompanyActionRenamed, &callerUserID, map[string]any{"name": company.Name})
	return &company, nil
}

// Delete архивирует компанию (soft-delete: is_active=false). Строка остаётся,
// FK заявок/сотрудников/машин не осиротевают. Блокируется при активных
// пользователях компании (как у организаций, #412).
func (s *companyService) Delete(ctx context.Context, callerUserID, companyID int) error {
	var company models.Company
	if err := s.db.WithContext(ctx).First(&company, companyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Company not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company")
	}
	if !company.IsActive {
		return nil // уже в архиве
	}

	var activeUsers int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).
		Where("company_id = ? AND is_active = ?", companyID, true).Count(&activeUsers).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking users")
	}
	if activeUsers > 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Нельзя архивировать компанию с активными пользователями")
	}

	if err := s.db.WithContext(ctx).Model(&models.Company{}).
		Where("id = ?", companyID).Update("is_active", false).Error; err != nil {
		slog.Error("не удалось архивировать компанию", "id", companyID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error archiving company")
	}
	slog.Info("компания архивирована", "id", companyID)
	s.recorder.Log(ctx, nil, models.AuditEntityCompany, &companyID, models.CompanyActionArchived, &callerUserID, nil)
	return nil
}

// Restore восстанавливает компанию из архива (is_active=true). Конфликт активного
// имени -> 400.
func (s *companyService) Restore(ctx context.Context, callerUserID, companyID int) error {
	var company models.Company
	if err := s.db.WithContext(ctx).First(&company, companyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Company not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company")
	}
	if company.IsActive {
		return nil // уже активна
	}

	var active int64
	if err := s.db.WithContext(ctx).Model(&models.Company{}).
		Where("name = ? AND is_active = ? AND id <> ?", company.Name, true, companyID).Count(&active).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking company")
	}
	if active > 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Активная компания с таким названием уже существует - переименуйте перед восстановлением")
	}

	if err := s.db.WithContext(ctx).Model(&models.Company{}).
		Where("id = ?", companyID).Update("is_active", true).Error; err != nil {
		slog.Error("не удалось восстановить компанию", "id", companyID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error restoring company")
	}
	slog.Info("компания восстановлена", "id", companyID)
	s.recorder.Log(ctx, nil, models.AuditEntityCompany, &companyID, models.CompanyActionRestored, &callerUserID, nil)
	return nil
}

// GetHistory возвращает историю изменений компании (admin-only, новые сверху).
// Переходный период #870: запись уже идёт в audit_log, но старые строки лежат в
// замороженной company_histories до финального backfill. Чтение объединяет обе
// таблицы в одинаковую форму ответа (форму стережёт TestCompanies_History).
// Действие renamed хранит только {name:new} (без old) - details передаётся как есть.
func (s *companyService) GetHistory(ctx context.Context, companyID int) ([]models.CompanyHistoryItem, error) {
	const actorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`
	sql := `
		SELECT id, action_type, details, actor_user_id, actor_name, created_at FROM (
			SELECT h.id AS id, h.action_type AS action_type, h.details AS details,
				h.actor_user_id AS actor_user_id, ` + actorName + ` AS actor_name, h.created_at AS created_at
			FROM company_histories h LEFT JOIN users u ON u.id = h.actor_user_id
			WHERE h.company_id = ?
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
	if err := s.db.WithContext(ctx).Raw(sql, companyID, models.AuditEntityCompany, companyID).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company history")
	}

	items := make([]models.CompanyHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.CompanyHistoryItem{
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

// GetUsers возвращает ответственных пользователей компании.
func (s *companyService) GetUsers(ctx context.Context, companyID int) ([]CompanyUserResponse, error) {
	users := make([]CompanyUserResponse, 0)
	err := s.db.WithContext(ctx).
		Table("users u").
		Select("u.id, u.username, u.last_name, u.first_name, u.middle_name, u.position, cu.is_primary, cu.required_approval").
		Joins("INNER JOIN companies_users cu ON u.id = cu.user_id").
		Where("cu.company_id = ? AND u.is_active = ?", companyID, true).
		Order("cu.is_primary DESC, u.last_name, u.first_name").
		Scan(&users).Error
	if err != nil {
		slog.Error("не удалось получить пользователей компании", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company users")
	}
	return users, nil
}

// UpdateUsers заменяет ответственных пользователей компании.
func (s *companyService) UpdateUsers(ctx context.Context, companyID int, req UpdateCompanyUsersRequest) error {
	// Проверяем что не более одного primary пользователя
	primaryCount := 0
	for _, u := range req.Users {
		if u.IsPrimary != nil && *u.IsPrimary {
			primaryCount++
		}
	}
	if primaryCount > 1 {
		return echo.NewHTTPError(http.StatusBadRequest, "Только один пользователь может быть главным ответственным")
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		slog.Error("не удалось начать транзакцию", "error", tx.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	// Удаляем старые связи
	if err := tx.Where("company_id = ?", companyID).Delete(&models.CompaniesUser{}).Error; err != nil {
		tx.Rollback()
		slog.Error("не удалось удалить старых пользователей компании", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating company users")
	}

	// Добавляем новых пользователей
	for _, userReq := range req.Users {
		var user models.User
		if err := tx.Where("username = ?", userReq.Username).First(&user).Error; err != nil {
			slog.Warn("пользователь не найден", "username", userReq.Username)
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

		cu := models.CompaniesUser{
			CompanyID:        companyID,
			UserID:           user.ID,
			IsPrimary:        isPrimary,
			RequiredApproval: requiredApproval,
		}
		if err := tx.Create(&cu).Error; err != nil {
			tx.Rollback()
			slog.Error("не удалось добавить пользователя компании", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating company users")
		}
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("не удалось закоммитить транзакцию", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return nil
}

// GetUnloadPlaces возвращает активные места разгрузки компании.
func (s *companyService) GetUnloadPlaces(ctx context.Context, companyID int) ([]CompanyUnloadPlaceResponse, error) {
	places := make([]CompanyUnloadPlaceResponse, 0)
	err := s.db.WithContext(ctx).
		Table("unload_places up").
		Select("up.id, up.name, up.description").
		Joins("JOIN companies_unload_places cup ON up.id = cup.unload_place_id").
		Where("cup.company_id = ? AND up.is_active = true", companyID).
		Order("up.name").
		Scan(&places).Error
	if err != nil {
		slog.Error("не удалось получить места разгрузки компании", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company unload places")
	}
	return places, nil
}

// UpdateUnloadPlaces заменяет привязку мест разгрузки к компании (admin-only).
func (s *companyService) UpdateUnloadPlaces(ctx context.Context, companyID int, req UpdateCompanyUnloadPlacesRequest) error {
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		slog.Error("не удалось начать транзакцию", "error", tx.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	// Удаляем старые связи
	if err := tx.Where("company_id = ?", companyID).Delete(&models.CompaniesUnloadPlace{}).Error; err != nil {
		tx.Rollback()
		slog.Error("не удалось удалить старые места разгрузки", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating unload places")
	}

	// Добавляем новые связи
	for _, placeID := range req.UnloadPlaceIDs {
		cup := models.CompaniesUnloadPlace{
			CompanyID:     companyID,
			UnloadPlaceID: placeID,
		}
		if err := tx.Create(&cup).Error; err != nil {
			tx.Rollback()
			slog.Error("не удалось добавить место разгрузки", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating unload places")
		}
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("не удалось закоммитить транзакцию", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return nil
}

// GetTables возвращает активные таблицы компании.
func (s *companyService) GetTables(ctx context.Context, companyID int) ([]CompanyTableResponse, error) {
	tables := make([]CompanyTableResponse, 0)
	err := s.db.WithContext(ctx).
		Table("system_tables st").
		Select("st.id, st.name, st.display_name, st.table_type").
		Joins("JOIN companies_tables ct ON st.id = ct.table_id").
		Where("ct.company_id = ? AND st.is_active = true", companyID).
		Order("st.display_name").
		Scan(&tables).Error
	if err != nil {
		slog.Error("не удалось получить таблицы компании", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company tables")
	}
	return tables, nil
}

// UpdateTables заменяет привязку таблиц к компании (admin-only).
func (s *companyService) UpdateTables(ctx context.Context, companyID int, req UpdateCompanyTablesRequest) error {
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		slog.Error("не удалось начать транзакцию", "error", tx.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	// Удаляем старые связи
	if err := tx.Where("company_id = ?", companyID).Delete(&models.CompaniesTable{}).Error; err != nil {
		tx.Rollback()
		slog.Error("не удалось удалить старые таблицы компании", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating company tables")
	}

	// Добавляем новые связи
	for _, tableID := range req.TableIDs {
		ct := models.CompaniesTable{
			CompanyID: companyID,
			TableID:   tableID,
		}
		if err := tx.Create(&ct).Error; err != nil {
			tx.Rollback()
			slog.Error("не удалось добавить таблицу компании", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating company tables")
		}
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("не удалось закоммитить транзакцию", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return nil
}

