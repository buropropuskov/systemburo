package services

import (
	"context"
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

	// GetWithUsers возвращает компании с количеством привязанных пользователей.
	GetWithUsers(ctx context.Context) ([]CompanyWithUsersResponse, error)

	// GetWithUsersExtended возвращает компании с количеством пользователей и местами разгрузки.
	GetWithUsersExtended(ctx context.Context) ([]CompanyWithUsersExtendedResponse, error)

	// Create создаёт новую компанию. Требуются права buropropuskov.
	Create(ctx context.Context, username string, req CreateCompanyRequest) (*models.Company, error)

	// Update обновляет название компании. Требуются права buropropuskov.
	Update(ctx context.Context, username string, companyID int, req CreateCompanyRequest) (*models.Company, error)

	// Delete удаляет компанию. Нельзя удалить если есть пользователи. Требуются права buropropuskov.
	Delete(ctx context.Context, username string, companyID int) error

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
	Name string `json:"name"`
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
	db *gorm.DB
}

// NewCompanyService создаёт экземпляр сервиса компаний.
func NewCompanyService(db *gorm.DB) CompanyService {
	return &companyService{db: db}
}

func (s *companyService) GetAll(ctx context.Context) ([]models.Company, error) {
	var companies []models.Company
	if err := s.db.WithContext(ctx).Order("name").Find(&companies).Error; err != nil {
		slog.Error("не удалось получить компании", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching companies")
	}
	return companies, nil
}

func (s *companyService) GetWithUsers(ctx context.Context) ([]CompanyWithUsersResponse, error) {
	var result []CompanyWithUsersResponse
	err := s.db.WithContext(ctx).
		Table("companies c").
		Select("c.id, c.name, COUNT(u.id) as user_count").
		Joins("LEFT JOIN users u ON u.company_id = c.id").
		Group("c.id").
		Order("c.name").
		Scan(&result).Error
	if err != nil {
		slog.Error("не удалось получить компании с пользователями", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching companies")
	}
	return result, nil
}

func (s *companyService) GetWithUsersExtended(ctx context.Context) ([]CompanyWithUsersExtendedResponse, error) {
	// Получаем базовые данные компаний с количеством пользователей
	type companyRow struct {
		ID        int
		Name      string
		UserCount *int64
	}
	var companies []companyRow
	err := s.db.WithContext(ctx).
		Table("companies c").
		Select("c.id, c.name, COUNT(u.id) as user_count").
		Joins("LEFT JOIN users u ON u.company_id = c.id").
		Group("c.id, c.name").
		Order("c.name").
		Scan(&companies).Error
	if err != nil {
		slog.Error("не удалось получить расширенную информацию о компаниях", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching companies")
	}

	result := make([]CompanyWithUsersExtendedResponse, 0, len(companies))
	for _, c := range companies {
		// Для каждой компании получаем места разгрузки
		var places []CompanyUnloadPlaceResponse
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
		if places == nil {
			places = []CompanyUnloadPlaceResponse{}
		}

		result = append(result, CompanyWithUsersExtendedResponse{
			ID:           c.ID,
			Name:         c.Name,
			UserCount:    c.UserCount,
			UnloadPlaces: places,
		})
	}

	return result, nil
}

func (s *companyService) Create(ctx context.Context, username string, req CreateCompanyRequest) (*models.Company, error) {
	if err := s.checkAdmin(ctx, username); err != nil {
		return nil, err
	}

	company := models.Company{Name: req.Name}
	if err := s.db.WithContext(ctx).Create(&company).Error; err != nil {
		slog.Error("не удалось создать компанию", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating company")
	}
	return &company, nil
}

func (s *companyService) Update(ctx context.Context, username string, companyID int, req CreateCompanyRequest) (*models.Company, error) {
	if err := s.checkAdmin(ctx, username); err != nil {
		return nil, err
	}

	var company models.Company
	if err := s.db.WithContext(ctx).First(&company, companyID).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Company not found")
	}

	company.Name = req.Name
	if err := s.db.WithContext(ctx).Save(&company).Error; err != nil {
		slog.Error("не удалось обновить компанию", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating company")
	}
	return &company, nil
}

func (s *companyService) Delete(ctx context.Context, username string, companyID int) error {
	if err := s.checkAdmin(ctx, username); err != nil {
		return err
	}

	// Проверяем наличие пользователей у компании
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("company_id = ?", companyID).Count(&count).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking users")
	}
	if count > 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Cannot delete company with users")
	}

	if err := s.db.WithContext(ctx).Delete(&models.Company{}, companyID).Error; err != nil {
		slog.Error("не удалось удалить компанию", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting company")
	}
	return nil
}

func (s *companyService) GetUsers(ctx context.Context, companyID int) ([]CompanyUserResponse, error) {
	var users []CompanyUserResponse
	err := s.db.WithContext(ctx).
		Table("users u").
		Select("u.id, u.username, u.last_name, u.first_name, u.middle_name, u.position, cu.is_primary, cu.required_approval").
		Joins("INNER JOIN companies_users cu ON u.id = cu.user_id").
		Where("cu.company_id = ?", companyID).
		Order("cu.is_primary DESC, u.last_name, u.first_name").
		Scan(&users).Error
	if err != nil {
		slog.Error("не удалось получить пользователей компании", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company users")
	}
	return users, nil
}

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

func (s *companyService) GetUnloadPlaces(ctx context.Context, companyID int) ([]CompanyUnloadPlaceResponse, error) {
	var places []CompanyUnloadPlaceResponse
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

func (s *companyService) GetTables(ctx context.Context, companyID int) ([]CompanyTableResponse, error) {
	var tables []CompanyTableResponse
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
