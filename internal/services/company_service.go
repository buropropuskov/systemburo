package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/normalize"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CompanyService определяет интерфейс бизнес-логики для работы с компаниями.
type CompanyService interface {
	// GetAll возвращает список всех компаний, отсортированных по имени.
	GetAll(ctx context.Context) ([]models.Company, error)

	// Suggest возвращает близкие к query проверенные компании (максимум пять) для
	// ручного ввода наименования в заявке.
	Suggest(ctx context.Context, query string) (DirectorySuggestAnswer, error)

	// ApproveModeration подтверждает компанию «на проверке», заведённую из заявки.
	ApproveModeration(ctx context.Context, callerUserID, id int) (DirectoryModerationResult, error)

	// RenameModeration исправляет наименование компании «на проверке» и разбирает её.
	RenameModeration(ctx context.Context, callerUserID, id int, name string) (DirectoryModerationResult, error)

	// MergeModeration переносит ссылки компании «на проверке» на существующую запись
	// справочника и удаляет черновик.
	MergeModeration(ctx context.Context, callerUserID, id, targetID int) (DirectoryMergeResult, error)

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

	// GetMembers возвращает пользователей, привязанных к компании через
	// users.company_id (участники), не ответственных из junction-таблицы.
	GetMembers(ctx context.Context, companyID int) ([]MemberResponse, error)

	// ReassignMembers переносит всех активных участников компании (те, что
	// блокируют её архивацию) в целевую компанию targetID, освобождая исходную.
	// Возвращает число перенесённых. callerUserID - актор для аудита смены company
	// у каждого пользователя (models.UserActionCompanyChanged).
	ReassignMembers(ctx context.Context, callerUserID, id, targetID int) (int, error)

	// UpdateUsers обновляет ответственных пользователей компании с поддержкой
	// обязательного согласования. callerUserID - актор для аудита «кто был -> кто стал».
	UpdateUsers(ctx context.Context, callerUserID, companyID int, req UpdateCompanyUsersRequest) error

	// GetUnloadPlaces возвращает активные места разгрузки компании.
	GetUnloadPlaces(ctx context.Context, companyID int) ([]CompanyUnloadPlaceResponse, error)

	// UpdateUnloadPlaces обновляет привязку мест разгрузки к компании. callerUserID - актор для аудита.
	UpdateUnloadPlaces(ctx context.Context, callerUserID, companyID int, req UpdateCompanyUnloadPlacesRequest) error

	// GetTables возвращает активные таблицы компании.
	GetTables(ctx context.Context, companyID int) ([]CompanyTableResponse, error)

	// UpdateTables обновляет привязку таблиц к компании. callerUserID - актор для аудита.
	UpdateTables(ctx context.Context, callerUserID, companyID int, req UpdateCompanyTablesRequest) error

	// --- Групповые операции (bulk). Переиспользуют одиночные методы в цикле,
	// частичный успех собирается в BulkOpResult. Валидация входа (тип, режим) -
	// один раз до цикла и возвращается ошибкой на весь запрос. Зеркало
	// организаций (см. OrganizationService). ---

	// BulkUpdateType меняет тип у набора компаний (nil снимает тип).
	BulkUpdateType(ctx context.Context, callerUserID int, ids []int, typ *string) (*BulkOpResult, error)

	// BulkAssignUnloadPlaces назначает места разгрузки набору компаний.
	// mode=replace затирает, mode=add объединяет с текущими. callerUserID - актор для аудита.
	BulkAssignUnloadPlaces(ctx context.Context, callerUserID int, ids, placeIDs []int, mode string) (*BulkOpResult, error)

	// BulkAssignTables назначает целевые таблицы набору компаний (replace|add). callerUserID - актор для аудита.
	BulkAssignTables(ctx context.Context, callerUserID int, ids, tableIDs []int, mode string) (*BulkOpResult, error)

	// BulkAssignUsers назначает ответственных набору компаний (replace|add).
	// primary в группе не назначается, сохраняется у существующих. callerUserID - актор для аудита.
	BulkAssignUsers(ctx context.Context, callerUserID int, ids []int, assignments []BulkUserAssignment, mode string) (*BulkOpResult, error)

	// BulkArchive архивирует набор компаний (частичный успех: активные с
	// пользователями попадают в Errors).
	BulkArchive(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error)

	// BulkRestore восстанавливает набор компаний из архива.
	BulkRestore(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error)

	// SetBlankExportEnqueuer подключает очередь файлового архива (#1615, B1) -
	// разбор справочника ставит на пересборку заявки, ссылающиеся на запись.
	SetBlankExportEnqueuer(e BlankExportEnqueuer)
}

// --- DTO: запросы ---

// CreateCompanyRequest тело запроса создания/обновления компании.
// Type валидируется в сервисе (условно): при создании обязателен и должен быть
// одним из models.OrgTypeValues; при обновлении опционален (nil снимает тип).
type CreateCompanyRequest struct {
	Name string  `json:"name" validate:"required,min=1,max=100"`
	Type *string `json:"type"`
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
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Type      *string `json:"type"`
	IsActive  bool    `json:"is_active"`
	UserCount int64   `json:"user_count"`
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
// ModerationStatus отдаётся, чтобы админский список отличал запись, заведённую подачей
// и ждущую разбора, от обычной (#1437) - по нему рисуется бейдж «на проверке».
type CompanyWithUsersExtendedResponse struct {
	ID               int                          `json:"id"`
	Name             string                       `json:"name"`
	Type             *string                      `json:"type"`
	IsActive         bool                         `json:"is_active"`
	UserCount        *int64                       `json:"user_count"`
	ModerationStatus string                       `json:"moderation_status"`
	UnloadPlaces     []CompanyUnloadPlaceResponse `json:"unload_places"`
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
	notifier NotificationService
	// blankExports - постановка затронутых заявок в очередь на выгрузку в файловый
	// архив (#1615, B1), зеркало organizationService.blankExports.
	blankExports BlankExportEnqueuer
}

// CompanyServiceOption конфигурирует companyService при создании.
type CompanyServiceOption func(*companyService)

// WithCompanyNotifications подключает уведомления инициатору о разборе заведённого им
// наименования (#1437), зеркало WithOrganizationNotifications.
func WithCompanyNotifications(n NotificationService) CompanyServiceOption {
	return func(s *companyService) { s.notifier = n }
}

// SetBlankExportEnqueuer подключает очередь файлового архива (#1615, B1).
func (s *companyService) SetBlankExportEnqueuer(e BlankExportEnqueuer) {
	s.blankExports = e
}

// NewCompanyService создаёт экземпляр сервиса компаний.
func NewCompanyService(db *gorm.DB, opts ...CompanyServiceOption) CompanyService {
	s := &companyService{db: db, recorder: NewAuditRecorder(db)}
	for _, opt := range opts {
		opt(s)
	}
	return s
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

// Suggest подбирает близкие компании по наименованию, см. suggestDirectory.
func (s *companyService) Suggest(ctx context.Context, query string) (DirectorySuggestAnswer, error) {
	return suggestDirectory(ctx, s.db, "companies", query)
}

// ApproveModeration - разбор компании «на проверке», см. approveDirectoryEntry.
func (s *companyService) ApproveModeration(ctx context.Context, callerUserID, id int) (DirectoryModerationResult, error) {
	return approveDirectoryEntry(ctx, s.db, s.recorder, s.blankExports, companyModeration, id, callerUserID)
}

// RenameModeration - исправление наименования при разборе, см. renameDirectoryEntry.
func (s *companyService) RenameModeration(ctx context.Context, callerUserID, id int, name string) (DirectoryModerationResult, error) {
	return renameDirectoryEntry(ctx, s.db, s.recorder, s.notifier, s.blankExports, companyModeration, id, name, callerUserID)
}

// MergeModeration - привязка черновика к существующей компании, см. mergeDirectoryEntry.
func (s *companyService) MergeModeration(ctx context.Context, callerUserID, id, targetID int) (DirectoryMergeResult, error) {
	return mergeDirectoryEntry(ctx, s.db, s.recorder, s.notifier, s.blankExports, companyModeration, id, targetID, callerUserID)
}

// GetWithUsers возвращает компании с количеством привязанных пользователей.
func (s *companyService) GetWithUsers(ctx context.Context, includeArchived bool) ([]CompanyWithUsersResponse, error) {
	result := make([]CompanyWithUsersResponse, 0)
	q := s.db.WithContext(ctx).
		Table("companies c").
		Select("c.id, c.name, c.type, c.is_active, COUNT(u.id) FILTER (WHERE u.is_active = true) as user_count").
		Joins("LEFT JOIN users u ON u.company_id = c.id").
		Group("c.id, c.name, c.type, c.is_active").
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
		ID               int
		Name             string
		Type             *string
		IsActive         bool
		UserCount        *int64
		ModerationStatus string
	}
	companies := make([]companyRow, 0)
	q := s.db.WithContext(ctx).
		Table("companies c").
		Select("c.id, c.name, c.type, c.is_active, c.moderation_status, COUNT(u.id) FILTER (WHERE u.is_active = true) as user_count").
		Joins("LEFT JOIN users u ON u.company_id = c.id").
		Group("c.id, c.name, c.type, c.is_active, c.moderation_status").
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
			ID:               c.ID,
			Name:             c.Name,
			Type:             c.Type,
			IsActive:         c.IsActive,
			UserCount:        c.UserCount,
			ModerationStatus: c.ModerationStatus,
			UnloadPlaces:     places,
		})
	}

	return result, nil
}

// Create создаёт новую компанию (admin-only). Тип обязателен и должен быть валидным.
func (s *companyService) Create(ctx context.Context, callerUserID int, req CreateCompanyRequest) (*models.Company, error) {
	if req.Type == nil || !models.IsValidOrgType(*req.Type) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный тип компании")
	}
	// Оформление наименования - к канону (#1437), см. organizationService.Create.
	req.Name = normalize.OrgNameDisplay(req.Name)
	// Наименование без букв и цифр («---», «"""») - мусор, с которым в справочнике
	// потом ничего не сделать: правило то же, что при подаче и при разборе (#1437).
	if normalize.OrgNameMeaningless(req.Name) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Укажите наименование компании")
	}

	// Сверяем по ключу дедупликации, а не по точному name (#1437), см. organizationService.Create.
	var active int64
	if err := applyNameDuplicateFilter(
		s.db.WithContext(ctx).Model(&models.Company{}).Where("is_active = ?", true),
		req.Name, normalize.OrgName(req.Name),
	).Count(&active).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking company")
	}
	if active > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Компания с таким названием уже существует")
	}

	// Запись из справочника сразу проверена, см. organizationService.Create (#1437).
	company := models.Company{Name: req.Name, Type: req.Type, IsActive: true, ModerationStatus: models.ModerationApproved}
	if err := s.db.WithContext(ctx).Create(&company).Error; err != nil {
		slog.Error("не удалось создать компанию", "error", err)
		return nil, directoryWriteError(err, "Компания с таким названием уже существует", "Error creating company")
	}
	slog.Info("компания создана", "id", company.ID, "name", company.Name)
	s.recorder.Log(ctx, nil, models.AuditEntityCompany, &company.ID, models.CompanyActionCreated, &callerUserID, map[string]any{"name": company.Name, "type": company.Type})
	return &company, nil
}

// Update обновляет название и тип компании (admin-only). Тип опционален: nil
// снимает его, непустое значение обязано быть валидным. Сохраняются вместе.
func (s *companyService) Update(ctx context.Context, callerUserID, companyID int, req CreateCompanyRequest) (*models.Company, error) {
	if req.Type != nil && !models.IsValidOrgType(*req.Type) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный тип компании")
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

	// Канонизируем оформление только при реальной смене наименования, см.
	// organizationService.Update (#1437).
	if req.Name != company.Name {
		req.Name = normalize.OrgNameDisplay(req.Name)
	}
	if normalize.OrgNameMeaningless(req.Name) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Укажите наименование компании")
	}

	normalized := normalize.OrgName(req.Name)
	var dup int64
	if err := applyNameDuplicateFilter(
		s.db.WithContext(ctx).Model(&models.Company{}).Where("is_active = ? AND id <> ?", true, companyID),
		req.Name, normalized,
	).Count(&dup).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking company")
	}
	if dup > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Компания с таким названием уже существует")
	}

	// map-обновление (а не Save структуры) - чтобы явно записать type=NULL при снятии типа.
	// name_normalized пишем явно: BeforeSave до map-обновления не достаёт.
	if err := s.db.WithContext(ctx).Model(&models.Company{}).
		Where("id = ?", companyID).Updates(map[string]any{"name": req.Name, "type": req.Type, "name_normalized": normalized}).Error; err != nil {
		slog.Error("не удалось обновить компанию", "id", companyID, "error", err)
		return nil, directoryWriteError(err, "Компания с таким названием уже существует", "Error updating company")
	}
	// Старые значения захватываем до перезаписи структуры - для определения,
	// что именно изменилось (имя/тип/оба).
	oldName := company.Name
	oldType := company.Type
	company.Name = req.Name
	company.Type = req.Type
	slog.Info("компания обновлена", "id", companyID, "name", company.Name)
	nameChanged := oldName != req.Name
	typeChanged := !strPtrEqual(oldType, req.Type)
	if !nameChanged && !typeChanged {
		// Ничего не поменялось (PUT с теми же значениями) - не пишем ложную
		// «переименована» в историю (как no-op в BulkUpdateType).
		return &company, nil
	}
	action := models.CompanyActionRenamed
	switch {
	case nameChanged && typeChanged:
		action = models.CompanyActionUpdated
	case typeChanged:
		action = models.CompanyActionRetyped
	}
	// name/type - новые значения (обратная совместимость рендера), from - старые.
	s.recorder.Log(ctx, nil, models.AuditEntityCompany, &companyID, action, &callerUserID, map[string]any{
		"name": company.Name, "type": company.Type,
		"from": map[string]any{"name": oldName, "type": oldType},
	})
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
	if err := applyNameDuplicateFilter(
		s.db.WithContext(ctx).Model(&models.Company{}).Where("is_active = ? AND id <> ?", true, companyID),
		company.Name, normalize.OrgName(company.Name),
	).Count(&active).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking company")
	}
	if active > 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Активная компания с таким названием уже существует - переименуйте перед восстановлением")
	}

	if err := s.db.WithContext(ctx).Model(&models.Company{}).
		Where("id = ?", companyID).Update("is_active", true).Error; err != nil {
		slog.Error("не удалось восстановить компанию", "id", companyID, "error", err)
		return directoryWriteError(err,
			"Активная компания с таким названием уже существует - переименуйте перед восстановлением",
			"Error restoring company")
	}
	slog.Info("компания восстановлена", "id", companyID)
	s.recorder.Log(ctx, nil, models.AuditEntityCompany, &companyID, models.CompanyActionRestored, &callerUserID, nil)
	return nil
}

// GetHistory возвращает историю изменений компании (admin-only, новые сверху).
// #870, финал F.2: запись и до-cutover строки живут в общем audit_log (старые
// перенесены backfill'ом BackfillAuditFromLegacy), поэтому чтение идёт только из
// audit_log. Замороженная company_histories дропнута в дроп-sweep (F.8).
// Форму ответа стережёт TestCompanies_History.
// Действия created/renamed хранят {name,type} (тип с #1046) - details как есть.
func (s *companyService) GetHistory(ctx context.Context, companyID int) ([]models.CompanyHistoryItem, error) {
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
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityCompany, companyID).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company history")
	}

	// Логин вместо ФИО у акторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]models.CompanyHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.CompanyHistoryItem{
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
	if masks := loadConsentMasks(ctx, s.db); len(masks) > 0 {
		for i := range users {
			maskUserParts(masks, users[i].ID, &users[i].LastName, &users[i].FirstName, &users[i].MiddleName)
		}
	}
	return users, nil
}

// GetMembers возвращает активных пользователей, привязанных к компании через
// users.company_id (участники). Это те же пользователи, что дают user_count.
func (s *companyService) GetMembers(ctx context.Context, companyID int) ([]MemberResponse, error) {
	members := make([]MemberResponse, 0)
	err := s.db.WithContext(ctx).
		Table("users u").
		Select("u.id, u.username, u.last_name, u.first_name, u.middle_name, u.position").
		Where("u.company_id = ? AND u.is_active = ?", companyID, true).
		Order("u.last_name, u.first_name, u.username").
		Scan(&members).Error
	if err != nil {
		slog.Error("не удалось получить участников компании", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company members")
	}
	if masks := loadConsentMasks(ctx, s.db); len(masks) > 0 {
		for i := range members {
			maskUserParts(masks, members[i].ID, &members[i].LastName, &members[i].FirstName, &members[i].MiddleName)
		}
	}
	return members, nil
}

// ReassignMembers переносит активных участников компании id (набор, блокирующий её
// архивацию: users.company_id=id AND is_active) в целевую компанию targetID.
// Освобождает исходную, чтобы её можно было архивировать. Целевая должна
// существовать, быть активной и отличаться от исходной. Идемпотентно: если
// блокеров нет, возвращает 0 без ошибки. Аудит смены company пишется на каждого
// пользователя (UserActionCompanyChanged) в той же транзакции - провал записи
// откатывает перенос. Зеркало organizationService.ReassignMembers.
func (s *companyService) ReassignMembers(ctx context.Context, callerUserID, id, targetID int) (int, error) {
	var src models.Company
	if err := s.db.WithContext(ctx).First(&src, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, echo.NewHTTPError(http.StatusNotFound, "Company not found")
		}
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company")
	}
	if targetID == id {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Нельзя перенести пользователей в ту же компанию")
	}

	var count int
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Целевую проверяем ВНУТРИ транзакции с блокировкой строки (FOR UPDATE):
		// закрывает окно TOCTOU, когда параллельная архивация target прошла бы между
		// проверкой активности и переносом - иначе пользователи ушли бы в архивную.
		var target models.Company
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&target, targetID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return echo.NewHTTPError(http.StatusBadRequest, "Целевая компания не найдена")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching target company")
		}
		if !target.IsActive {
			return echo.NewHTTPError(http.StatusBadRequest, "Нельзя перенести в архивную компанию")
		}
		var ids []int
		if err := tx.Model(&models.User{}).
			Where("company_id = ? AND is_active = ?", id, true).
			Pluck("id", &ids).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching users")
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Model(&models.User{}).
			Where("id IN ?", ids).
			Update("company_id", targetID).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error reassigning users")
		}
		for _, uid := range ids {
			u := uid
			if err := s.recorder.Record(ctx, tx, models.AuditEntityUser, &u, models.UserActionCompanyChanged, &callerUserID, map[string]any{"old": id, "new": targetID}); err != nil {
				return err
			}
		}
		count = len(ids)
		return nil
	}); err != nil {
		return 0, err
	}
	slog.Info("участники перенесены между компаниями", "from", id, "to", targetID, "count", count)
	return count, nil
}

// UpdateUsers заменяет ответственных пользователей компании.
// UpdateUsers заменяет ответственных компании и пишет в audit_log «кто был -> кто
// стал» (added/removed/approval_changed), если набор изменился. Логирование - в
// шаренном методе, поэтому bulk (реюз в цикле) и одиночная деталь-панель пишут
// историю консистентно и ровно один раз.
func (s *companyService) UpdateUsers(ctx context.Context, callerUserID, companyID int, req UpdateCompanyUsersRequest) error {
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

	oldUsers := s.attachedUserStates(ctx, companyID)

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

	var applied []auditUserState
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
		applied = append(applied, auditUserState{
			Username:         user.Username,
			Name:             fullName(user.LastName, user.FirstName, user.Username),
			RequiredApproval: requiredApproval,
			IsPrimary:        isPrimary,
		})
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("не удалось закоммитить транзакцию", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	if diff := diffUsers(oldUsers, applied); !diff.empty() {
		s.recorder.Log(ctx, nil, models.AuditEntityCompany, &companyID, models.CompanyActionResponsiblesChanged, &callerUserID, diff)
	}
	return nil
}

// attachedUserStates - снимок текущих ответственных компании (username, ФИО,
// флаг согласования) для diff аудита.
func (s *companyService) attachedUserStates(ctx context.Context, companyID int) []auditUserState {
	var rows []auditUserState
	if err := s.db.WithContext(ctx).
		Table("companies_users cu").
		Select("u.username AS username, "+auditUserNameSQL+" AS name, cu.required_approval AS required_approval, cu.is_primary AS is_primary").
		Joins("JOIN users u ON u.id = cu.user_id").
		Where("cu.company_id = ?", companyID).Scan(&rows).Error; err != nil {
		slog.Warn("audit: не удалось прочитать текущих ответственных компании", "company_id", companyID, "error", err)
	}
	return rows
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
// После применения пишет в audit_log added/removed по именам мест, если набор изменился.
func (s *companyService) UpdateUnloadPlaces(ctx context.Context, callerUserID, companyID int, req UpdateCompanyUnloadPlacesRequest) error {
	oldPlaces := s.attachedPlaceNames(ctx, companyID)

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

	if diff := diffIDNames(oldPlaces, req.UnloadPlaceIDs, s.placeNamesByIDs(ctx, req.UnloadPlaceIDs)); !diff.empty() {
		s.recorder.Log(ctx, nil, models.AuditEntityCompany, &companyID, models.CompanyActionUnloadPlacesChanged, &callerUserID, diff)
	}
	return nil
}

// attachedPlaceNames - id->name мест разгрузки, привязанных к компании (сырая junction).
func (s *companyService) attachedPlaceNames(ctx context.Context, companyID int) map[int]string {
	var rows []idName
	if err := s.db.WithContext(ctx).Table("companies_unload_places cup").
		Select("up.id AS id, up.name AS name").
		Joins("JOIN unload_places up ON up.id = cup.unload_place_id").
		Where("cup.company_id = ?", companyID).Scan(&rows).Error; err != nil {
		slog.Warn("audit: не удалось прочитать места разгрузки компании", "company_id", companyID, "error", err)
	}
	return idNameMap(rows)
}

func (s *companyService) placeNamesByIDs(ctx context.Context, ids []int) map[int]string {
	if len(ids) == 0 {
		return map[int]string{}
	}
	var rows []idName
	if err := s.db.WithContext(ctx).Table("unload_places").
		Select("id AS id, name AS name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		slog.Warn("audit: не удалось прочитать имена мест разгрузки", "error", err)
	}
	return idNameMap(rows)
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

// UpdateTables заменяет привязку таблиц к компании (admin-only). После применения
// пишет в audit_log added/removed по именам таблиц, если набор изменился.
func (s *companyService) UpdateTables(ctx context.Context, callerUserID, companyID int, req UpdateCompanyTablesRequest) error {
	oldTables := s.attachedTableNames(ctx, companyID)

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

	if diff := diffIDNames(oldTables, req.TableIDs, s.tableNamesByIDs(ctx, req.TableIDs)); !diff.empty() {
		s.recorder.Log(ctx, nil, models.AuditEntityCompany, &companyID, models.CompanyActionTablesChanged, &callerUserID, diff)
	}
	return nil
}

// attachedTableNames - id->display_name таблиц, привязанных к компании (сырая
// junction, без is_active). display_name nullable -> COALESCE к name.
func (s *companyService) attachedTableNames(ctx context.Context, companyID int) map[int]string {
	var rows []idName
	if err := s.db.WithContext(ctx).Table("companies_tables ct").
		Select("st.id AS id, COALESCE(NULLIF(st.display_name, ''), st.name) AS name").
		Joins("JOIN system_tables st ON st.id = ct.table_id").
		Where("ct.company_id = ?", companyID).Scan(&rows).Error; err != nil {
		slog.Warn("audit: не удалось прочитать таблицы компании", "company_id", companyID, "error", err)
	}
	return idNameMap(rows)
}

func (s *companyService) tableNamesByIDs(ctx context.Context, ids []int) map[int]string {
	if len(ids) == 0 {
		return map[int]string{}
	}
	var rows []idName
	if err := s.db.WithContext(ctx).Table("system_tables").
		Select("id AS id, COALESCE(NULLIF(display_name, ''), name) AS name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		slog.Warn("audit: не удалось прочитать имена таблиц", "error", err)
	}
	return idNameMap(rows)
}

// --- Групповые операции (bulk) ---

// loadCompany читает компанию по id (без ошибки-обёртки: bulk сам решает, что
// класть в BulkItemError при отсутствии).
func (s *companyService) loadCompany(ctx context.Context, id int) (models.Company, bool) {
	var company models.Company
	if err := s.db.WithContext(ctx).First(&company, id).Error; err != nil {
		return company, false
	}
	return company, true
}

// BulkUpdateType меняет тип у набора компаний через переиспользование Update
// (имя берётся из текущей записи, чтобы не переименовать). Тип валидируется один
// раз до цикла.
func (s *companyService) BulkUpdateType(ctx context.Context, callerUserID int, ids []int, typ *string) (*BulkOpResult, error) {
	if typ != nil && !models.IsValidOrgType(*typ) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный тип компании")
	}
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		company, ok := s.loadCompany(ctx, id)
		if !ok {
			res.addError(id, "", "Компания не найдена")
			continue
		}
		// Тип уже соответствует - no-op успех, без похода в Update: иначе оно
		// (имя тоже не меняется) залогировало бы ложную «переименована» в историю
		// (nameChanged=typeChanged=false -> дефолтный action Renamed). Для bulk
		// это частый кейс: в наборе часть компаний уже нужного типа.
		if strPtrEqual(company.Type, typ) {
			res.SuccessCount++
			continue
		}
		if _, err := s.Update(ctx, callerUserID, id, CreateCompanyRequest{Name: company.Name, Type: typ}); err != nil {
			res.addError(id, company.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkAssignUnloadPlaces назначает места разгрузки набору компаний. В режиме add
// текущие связи читаются из сырой junction (без фильтра is_active), чтобы
// объединение не отвязало неактивные-но-привязанные места. Чтение current идёт
// вне транзакции переиспользуемого UpdateUnloadPlaces - для admin-only
// последовательных операций окна гонки нет; конкурентный bulk по той же компании
// - осознанно не защищаем (принятый trade-off ради переиспользования).
func (s *companyService) BulkAssignUnloadPlaces(ctx context.Context, callerUserID int, ids, placeIDs []int, mode string) (*BulkOpResult, error) {
	if !isValidBulkMode(mode) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный режим (replace|add)")
	}
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		company, ok := s.loadCompany(ctx, id)
		if !ok {
			res.addError(id, "", "Компания не найдена")
			continue
		}
		target := placeIDs
		if mode == BulkModeAdd {
			var current []int
			if err := s.db.WithContext(ctx).Model(&models.CompaniesUnloadPlace{}).
				Where("company_id = ?", id).Pluck("unload_place_id", &current).Error; err != nil {
				res.addError(id, company.Name, "Ошибка чтения текущих мест разгрузки")
				continue
			}
			target = unionInts(current, placeIDs)
		}
		if err := s.UpdateUnloadPlaces(ctx, callerUserID, id, UpdateCompanyUnloadPlacesRequest{UnloadPlaceIDs: target}); err != nil {
			res.addError(id, company.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkAssignTables назначает целевые таблицы набору компаний (replace|add).
func (s *companyService) BulkAssignTables(ctx context.Context, callerUserID int, ids, tableIDs []int, mode string) (*BulkOpResult, error) {
	if !isValidBulkMode(mode) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный режим (replace|add)")
	}
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		company, ok := s.loadCompany(ctx, id)
		if !ok {
			res.addError(id, "", "Компания не найдена")
			continue
		}
		target := tableIDs
		if mode == BulkModeAdd {
			var current []int
			if err := s.db.WithContext(ctx).Model(&models.CompaniesTable{}).
				Where("company_id = ?", id).Pluck("table_id", &current).Error; err != nil {
				res.addError(id, company.Name, "Ошибка чтения текущих таблиц")
				continue
			}
			target = unionInts(current, tableIDs)
		}
		if err := s.UpdateTables(ctx, callerUserID, id, UpdateCompanyTablesRequest{TableIDs: target}); err != nil {
			res.addError(id, company.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkAssignUsers назначает ответственных набору компаний. primary в группе не
// назначается: за существующими сохраняется их is_primary/required_approval,
// новым выставляется is_primary=false и переданный required_approval. В режиме
// replace итоговый набор = выбранные (у оставшегося primary сохраняется), в add
// = текущие как есть + недостающие выбранные.
func (s *companyService) BulkAssignUsers(ctx context.Context, callerUserID int, ids []int, assignments []BulkUserAssignment, mode string) (*BulkOpResult, error) {
	if !isValidBulkMode(mode) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный режим (replace|add)")
	}
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		company, ok := s.loadCompany(ctx, id)
		if !ok {
			res.addError(id, "", "Компания не найдена")
			continue
		}
		users, err := s.buildBulkUsers(ctx, id, assignments, mode)
		if err != nil {
			res.addError(id, company.Name, "Ошибка чтения ответственных")
			continue
		}
		if err := s.UpdateUsers(ctx, callerUserID, id, UpdateCompanyUsersRequest{Users: users}); err != nil {
			res.addError(id, company.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// buildBulkUsers формирует итоговый список ответственных для одной компании,
// сохраняя primary существующих (см. BulkAssignUsers).
func (s *companyService) buildBulkUsers(ctx context.Context, companyID int, assignments []BulkUserAssignment, mode string) ([]CompanyUserRequest, error) {
	type curRow struct {
		Username         string
		IsPrimary        bool
		RequiredApproval bool
	}
	var rows []curRow
	if err := s.db.WithContext(ctx).
		Table("companies_users cu").
		Select("u.username, cu.is_primary, cu.required_approval").
		Joins("JOIN users u ON u.id = cu.user_id").
		Where("cu.company_id = ?", companyID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	existing := make(map[string]curRow, len(rows))
	for _, r := range rows {
		existing[r.Username] = r
	}

	users := make([]CompanyUserRequest, 0, len(rows)+len(assignments))
	if mode == BulkModeAdd {
		// Существующие сохраняем как есть (флаги, включая primary, не трогаем).
		for _, r := range rows {
			isP, ra := r.IsPrimary, r.RequiredApproval
			users = append(users, CompanyUserRequest{Username: r.Username, IsPrimary: &isP, RequiredApproval: &ra})
		}
	}
	for _, a := range assignments {
		un := a.Username
		if _, ok := existing[un]; ok && mode == BulkModeAdd {
			continue // add: уже в наборе - не дублируем и не трогаем флаги
		}
		isP := false
		if cur, ok := existing[un]; ok {
			isP = cur.IsPrimary // replace: primary оставшегося сохраняется
		}
		ra := a.RequiredApproval // индивидуальный флаг согласования
		users = append(users, CompanyUserRequest{Username: un, IsPrimary: &isP, RequiredApproval: &ra})
	}
	return users, nil
}

// BulkArchive архивирует набор компаний через Delete. Активные компании с
// пользователями честно попадают в Errors (частичный успех).
func (s *companyService) BulkArchive(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		company, ok := s.loadCompany(ctx, id)
		if !ok {
			res.addError(id, "", "Компания не найдена")
			continue
		}
		if err := s.Delete(ctx, callerUserID, id); err != nil {
			res.addError(id, company.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkRestore восстанавливает набор компаний через Restore.
func (s *companyService) BulkRestore(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		company, ok := s.loadCompany(ctx, id)
		if !ok {
			res.addError(id, "", "Компания не найдена")
			continue
		}
		if err := s.Restore(ctx, callerUserID, id); err != nil {
			res.addError(id, company.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}
