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

// CompanyService определяет интерфейс бизнес-логики для работы с компаниями.
type CompanyService interface {
	// GetAll возвращает список всех компаний, отсортированных по имени.
	GetAll(ctx context.Context) ([]models.Company, error)

	// GetWithUsers возвращает компании с количеством пользователей. includeArchived добавляет архивные.
	GetWithUsers(ctx context.Context, includeArchived bool) ([]CompanyWithUsersResponse, error)

	// GetWithUsersExtended возвращает компании с количеством пользователей и местами разгрузки.
	GetWithUsersExtended(ctx context.Context, includeArchived bool) ([]CompanyWithUsersExtendedResponse, error)

	// Create создаёт новую компанию. callerUserID - актор для аудита. Требуются права buropropuskov.
	Create(ctx context.Context, username string, callerUserID int, req CreateCompanyRequest) (*models.Company, error)

	// Update обновляет название компании. callerUserID - актор для аудита. Требуются права buropropuskov.
	Update(ctx context.Context, username string, callerUserID, companyID int, req CreateCompanyRequest) (*models.Company, error)

	// Delete архивирует компанию (soft-delete). callerUserID - актор. Нельзя при активных пользователях. Требуются права buropropuskov.
	Delete(ctx context.Context, username string, callerUserID, companyID int) error

	// Restore восстанавливает компанию из архива. callerUserID - актор. Требуются права buropropuskov.
	Restore(ctx context.Context, username string, callerUserID, companyID int) error

	// GetHistory возвращает историю изменений компании. Требуются права buropropuskov.
	GetHistory(ctx context.Context, username string, companyID int) ([]models.CompanyHistoryItem, error)

	// GetUsers возвращает ответственных пользователей компании.
	GetUsers(ctx context.Context, companyID int) ([]CompanyUserResponse, error)

	// UpdateUsers обновляет ответственных пользователей компании с поддержкой обязательного согласования.
	UpdateUsers(ctx context.Context, companyID int, req UpdateCompanyUsersRequest) error

	// GetUnloadPlaces возвращает активные места разгрузки компании.
	GetUnloadPlaces(ctx context.Context, companyID int) ([]CompanyUnloadPlaceResponse, error)

	// UpdateUnloadPlaces обновляет привязку мест разгрузки к компании. Требуются права buropropuskov.
	UpdateUnloadPlaces(ctx context.Context, username string, companyID int, req UpdateCompanyUnloadPlacesRequest) error

	// GetTables возвращает активные таблицы компании.
	GetTables(ctx context.Context, companyID int) ([]CompanyTableResponse, error)

	// UpdateTables обновляет привязку таблиц к компании. Требуются права buropropuskov.
	UpdateTables(ctx context.Context, username string, companyID int, req UpdateCompanyTablesRequest) error
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
	db      *gorm.DB
	history CompanyHistoryService
}

// NewCompanyService создаёт экземпляр сервиса компаний.
func NewCompanyService(db *gorm.DB) CompanyService {
	return &companyService{db: db, history: NewCompanyHistoryService(db)}
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
func (s *companyService) Create(ctx context.Context, username string, callerUserID int, req CreateCompanyRequest) (*models.Company, error) {
	if err := s.checkAdmin(ctx, username); err != nil {
		return nil, err
	}

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
	s.history.Log(ctx, company.ID, &callerUserID, models.CompanyActionCreated, map[string]any{"name": company.Name})
	return &company, nil
}

// Update обновляет название компании (admin-only).
func (s *companyService) Update(ctx context.Context, username string, callerUserID, companyID int, req CreateCompanyRequest) (*models.Company, error) {
	if err := s.checkAdmin(ctx, username); err != nil {
		return nil, err
	}

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
	s.history.Log(ctx, companyID, &callerUserID, models.CompanyActionRenamed, map[string]any{"name": company.Name})
	return &company, nil
}

// Delete архивирует компанию (soft-delete: is_active=false). Строка остаётся,
// FK заявок/сотрудников/машин не осиротевают. Блокируется при активных
// пользователях компании (как у организаций, #412).
func (s *companyService) Delete(ctx context.Context, username string, callerUserID, companyID int) error {
	if err := s.checkAdmin(ctx, username); err != nil {
		return err
	}

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
	s.history.Log(ctx, companyID, &callerUserID, models.CompanyActionArchived, nil)
	return nil
}

// Restore восстанавливает компанию из архива (is_active=true). Конфликт активного
// имени -> 400.
func (s *companyService) Restore(ctx context.Context, username string, callerUserID, companyID int) error {
	if err := s.checkAdmin(ctx, username); err != nil {
		return err
	}

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
	s.history.Log(ctx, companyID, &callerUserID, models.CompanyActionRestored, nil)
	return nil
}

// GetHistory возвращает историю изменений компании (admin-only).
func (s *companyService) GetHistory(ctx context.Context, username string, companyID int) ([]models.CompanyHistoryItem, error) {
	if err := s.checkAdmin(ctx, username); err != nil {
		return nil, err
	}
	return s.history.GetHistory(ctx, companyID)
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
func (s *companyService) UpdateUnloadPlaces(ctx context.Context, username string, companyID int, req UpdateCompanyUnloadPlacesRequest) error {
	if err := s.checkAdmin(ctx, username); err != nil {
		return err
	}

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
func (s *companyService) UpdateTables(ctx context.Context, username string, companyID int, req UpdateCompanyTablesRequest) error {
	if err := s.checkAdmin(ctx, username); err != nil {
		return err
	}

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

// checkAdmin проверяет что пользователь имеет тип buropropuskov.
func (s *companyService) checkAdmin(ctx context.Context, username string) error {
	var result struct {
		UserType string
	}
	err := s.db.WithContext(ctx).
		Table("users u").
		Select("ut.code as user_type").
		Joins("JOIN user_types ut ON u.type_id = ut.id").
		Where("u.username = ?", username).
		Scan(&result).Error
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}
	if result.UserType != "buropropuskov" {
		return echo.NewHTTPError(http.StatusForbidden, "Insufficient permissions")
	}
	return nil
}
