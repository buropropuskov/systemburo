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

// OrganizationService определяет интерфейс бизнес-логики организаций.
type OrganizationService interface {
	// GetAll возвращает список активных организаций (id, name) - для выпадающих списков.
	GetAll(ctx context.Context) ([]OrganizationInfoResponse, error)

	// Suggest возвращает близкие к query проверенные организации (максимум пять) для
	// ручного ввода наименования в заявке.
	Suggest(ctx context.Context, query string) (DirectorySuggestAnswer, error)

	// ApproveModeration подтверждает организацию «на проверке», заведённую из заявки.
	ApproveModeration(ctx context.Context, callerUserID, id int) (DirectoryModerationResult, error)

	// RenameModeration исправляет наименование организации «на проверке» и разбирает её.
	RenameModeration(ctx context.Context, callerUserID, id int, name string) (DirectoryModerationResult, error)

	// MergeModeration переносит ссылки организации «на проверке» на существующую запись
	// справочника и удаляет черновик.
	MergeModeration(ctx context.Context, callerUserID, id, targetID int) (DirectoryMergeResult, error)

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

	// GetMembers возвращает пользователей, привязанных к организации через
	// users.organization_id (участники), не ответственных из junction-таблицы.
	GetMembers(ctx context.Context, orgID int) ([]MemberResponse, error)

	// ReassignMembers переносит всех активных участников организации (те, что
	// блокируют её архивацию) в целевую организацию targetID, освобождая исходную.
	// Возвращает число перенесённых. callerUserID - актор для аудита смены org у
	// каждого пользователя (models.UserActionOrgChanged).
	ReassignMembers(ctx context.Context, callerUserID, id, targetID int) (int, error)

	// UpdateOrganizationUsers обновляет ответственных пользователей организации
	// (replace-стратегия). callerUserID - актор для аудита «кто был -> кто стал».
	UpdateOrganizationUsers(ctx context.Context, callerUserID, orgID int, req UpdateOrganizationUsersRequest) error

	// GetOrganizationTables возвращает таблицы, привязанные к организации.
	GetOrganizationTables(ctx context.Context, orgID int) ([]OrganizationTableResponse, error)

	// UpdateOrganizationTables заменяет привязку таблиц к организации. callerUserID - актор для аудита.
	UpdateOrganizationTables(ctx context.Context, callerUserID, orgID int, req UpdateOrganizationTablesRequest) error

	// GetOrganizationUnloadPlaces возвращает места разгрузки организации.
	GetOrganizationUnloadPlaces(ctx context.Context, orgID int) ([]OrganizationUnloadPlaceResponse, error)

	// UpdateOrganizationUnloadPlaces заменяет привязку мест разгрузки к организации. callerUserID - актор для аудита.
	UpdateOrganizationUnloadPlaces(ctx context.Context, callerUserID, orgID int, req UpdateOrganizationUnloadPlacesRequest) error

	// --- Групповые операции (bulk). Переиспользуют одиночные методы в цикле,
	// частичный успех собирается в BulkOpResult. Валидация входа (тип, режим) -
	// один раз до цикла и возвращается ошибкой на весь запрос. ---

	// BulkUpdateType меняет тип у набора организаций (nil снимает тип).
	BulkUpdateType(ctx context.Context, callerUserID int, ids []int, typ *string) (*BulkOpResult, error)

	// BulkAssignUnloadPlaces назначает места разгрузки набору организаций.
	// mode=replace затирает, mode=add объединяет с текущими. callerUserID - актор для аудита.
	BulkAssignUnloadPlaces(ctx context.Context, callerUserID int, ids, placeIDs []int, mode string) (*BulkOpResult, error)

	// BulkAssignTables назначает целевые таблицы набору организаций (replace|add). callerUserID - актор для аудита.
	BulkAssignTables(ctx context.Context, callerUserID int, ids, tableIDs []int, mode string) (*BulkOpResult, error)

	// BulkAssignUsers назначает ответственных набору организаций (replace|add).
	// primary в группе не назначается, сохраняется у существующих. callerUserID - актор для аудита.
	BulkAssignUsers(ctx context.Context, callerUserID int, ids []int, assignments []BulkUserAssignment, mode string) (*BulkOpResult, error)

	// BulkArchive архивирует набор организаций (частичный успех: активные с
	// пользователями попадают в Errors).
	BulkArchive(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error)

	// BulkRestore восстанавливает набор организаций из архива.
	BulkRestore(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error)

	// SetBlankExportEnqueuer подключает очередь файлового архива (#1615, B1) -
	// разбор справочника ставит на пересборку заявки, ссылающиеся на запись.
	SetBlankExportEnqueuer(e BlankExportEnqueuer)
}

// --- DTO: запросы ---

// CreateOrganizationRequest — тело запроса на создание/обновление организации.
// Type валидируется в сервисе (условно): при создании обязателен и должен быть
// одним из models.OrgTypeValues; при обновлении опционален (nil снимает тип).
type CreateOrganizationRequest struct {
	Name string  `json:"name" validate:"required,min=1,max=100"`
	Type *string `json:"type"`
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
	ID   int     `json:"id"`
	Name string  `json:"name"`
	Type *string `json:"type"`
	// ModerationStatus - approved у обычной записи, pending у заведённой подачей и
	// ещё не разобранной (#1437). Нужен разбору: привязывать черновик можно только
	// к проверенной записи, и список выбора обязан отсеять непроверенные.
	ModerationStatus string `json:"moderation_status"`
}

// OrganizationWithUsersResponse — организация с количеством пользователей.
// OrganizationWithUsersResponse - строка списка организаций для управления справочником.
// ModerationStatus отдаётся вместе с остальными полями, чтобы админский список отличал
// запись, заведённую подачей и ждущую разбора, от обычной (#1437): без него бейдж
// «на проверке» рисовать не по чему. Оба запроса (GetWithUsers и GetWithUsersExtended)
// обязаны селектить колонку - иначе поле молча приедет пустой строкой.
type OrganizationWithUsersResponse struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	Type             *string `json:"type"`
	IsActive         bool    `json:"is_active"`
	UserCount        int64   `json:"user_count"`
	ModerationStatus string  `json:"moderation_status"`
}

// MemberResponse — пользователь, привязанный к организации/компании через
// users.organization_id/company_id (участник, по нему же считается user_count).
// Не путать с ответственными (junction organization_users/companies_users) из
// GetOrganizationUsers/GetUsers - там связь многие-ко-многим, здесь прямое поле.
type MemberResponse struct {
	ID         int     `json:"id"`
	Username   string  `json:"username"`
	LastName   *string `json:"last_name"`
	FirstName  *string `json:"first_name"`
	MiddleName *string `json:"middle_name"`
	Position   *string `json:"position"`
}

// ReassignUsersRequest - тело запроса переноса всех блокирующих участников
// организации/компании в целевую сущность target_id. Общий DTO для org и company
// (одинаковая форма). Для типов пользователей (s3) - отдельный запрос с
// target_type_id, чтобы не путать семантику.
type ReassignUsersRequest struct {
	TargetID int `json:"target_id"`
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
	db       *gorm.DB
	recorder AuditRecorder
	notifier NotificationService
	// blankExports - постановка затронутых заявок в очередь на выгрузку в файловый
	// архив (#1615, B1): разбор справочника меняет наименование организации,
	// печатаемое в бланке/слепке, либо переезжает заявки на другую запись слиянием.
	blankExports BlankExportEnqueuer
}

// OrganizationServiceOption конфигурирует organizationService при создании.
type OrganizationServiceOption func(*organizationService)

// WithOrganizationNotifications подключает уведомления инициатору о разборе заведённого
// им наименования (#1437). Опционально: без них разбор работает молча.
func WithOrganizationNotifications(n NotificationService) OrganizationServiceOption {
	return func(s *organizationService) { s.notifier = n }
}

// SetBlankExportEnqueuer подключает очередь файлового архива (#1615, B1).
func (s *organizationService) SetBlankExportEnqueuer(e BlankExportEnqueuer) {
	s.blankExports = e
}

// NewOrganizationService создаёт новый экземпляр сервиса организаций.
func NewOrganizationService(db *gorm.DB, opts ...OrganizationServiceOption) OrganizationService {
	s := &organizationService{db: db, recorder: NewAuditRecorder(db)}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GetAll возвращает список всех организаций.
func (s *organizationService) GetAll(ctx context.Context) ([]OrganizationInfoResponse, error) {
	orgs := make([]OrganizationInfoResponse, 0)
	err := s.db.WithContext(ctx).
		Table("organizations").
		Select("id, name, moderation_status").
		Where("is_active = ?", true).
		Order("name").
		Scan(&orgs).Error
	if err != nil {
		slog.Error("Не удалось получить список организаций", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organizations")
	}
	return orgs, nil
}

// Suggest подбирает близкие организации по наименованию, см. suggestDirectory.
func (s *organizationService) Suggest(ctx context.Context, query string) (DirectorySuggestAnswer, error) {
	return suggestDirectory(ctx, s.db, "organizations", query)
}

// ApproveModeration - разбор организации «на проверке», см. approveDirectoryEntry.
func (s *organizationService) ApproveModeration(ctx context.Context, callerUserID, id int) (DirectoryModerationResult, error) {
	return approveDirectoryEntry(ctx, s.db, s.recorder, s.blankExports, organizationModeration, id, callerUserID)
}

// RenameModeration - исправление наименования при разборе, см. renameDirectoryEntry.
func (s *organizationService) RenameModeration(ctx context.Context, callerUserID, id int, name string) (DirectoryModerationResult, error) {
	return renameDirectoryEntry(ctx, s.db, s.recorder, s.notifier, s.blankExports, organizationModeration, id, name, callerUserID)
}

// MergeModeration - привязка черновика к существующей организации, см. mergeDirectoryEntry.
func (s *organizationService) MergeModeration(ctx context.Context, callerUserID, id, targetID int) (DirectoryMergeResult, error) {
	return mergeDirectoryEntry(ctx, s.db, s.recorder, s.notifier, s.blankExports, organizationModeration, id, targetID, callerUserID)
}

// Create создаёт новую организацию. Тип обязателен и должен быть валидным.
func (s *organizationService) Create(ctx context.Context, callerUserID int, req CreateOrganizationRequest) (*OrganizationInfoResponse, error) {
	if req.Type == nil || !models.IsValidOrgType(*req.Type) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный тип организации")
	}

	// Оформление наименования приводим к канону (#1437): в справочник не должны попадать
	// строчная ОПФ и незакрытая кавычка независимо от того, откуда пришла запись.
	req.Name = normalize.OrgNameDisplay(req.Name)
	// Наименование без букв и цифр («---», «"""») - мусор, с которым в справочнике
	// потом ничего не сделать: правило то же, что при подаче и при разборе (#1437).
	if normalize.OrgNameMeaningless(req.Name) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Укажите наименование организации")
	}

	// Сверяем по ключу дедупликации, а не по точному name: иначе рядом с
	// «ООО "Ромашка"» заводится «ооо ромашка» как отдельная организация (#1437).
	var active int64
	if err := applyNameDuplicateFilter(
		s.db.WithContext(ctx).Model(&models.Organization{}).Where("is_active = ?", true),
		req.Name, normalize.OrgName(req.Name),
	).Count(&active).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking organization")
	}
	if active > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Организация с таким названием уже существует")
	}

	// Запись из справочника проверена по определению: модерация (#1437) касается только
	// наименований, пришедших из формы подачи заявки.
	org := models.Organization{Name: req.Name, Type: req.Type, IsActive: true, ModerationStatus: models.ModerationApproved}
	if err := s.db.WithContext(ctx).Create(&org).Error; err != nil {
		slog.Error("Не удалось создать организацию", "error", err)
		return nil, directoryWriteError(err, "Организация с таким названием уже существует", "Error creating organization")
	}
	slog.Info("организация создана", "id", org.ID, "name", org.Name)
	s.recorder.Log(ctx, nil, models.AuditEntityOrganization, &org.ID, models.OrganizationActionCreated, &callerUserID, map[string]any{"name": org.Name, "type": org.Type})
	return &OrganizationInfoResponse{ID: org.ID, Name: org.Name, Type: org.Type, ModerationStatus: org.ModerationStatus}, nil
}

// Update обновляет название и тип организации по ID. Тип опционален: nil снимает
// его, непустое значение обязано быть валидным. Название и тип сохраняются вместе.
func (s *organizationService) Update(ctx context.Context, callerUserID, id int, req CreateOrganizationRequest) (*OrganizationInfoResponse, error) {
	if req.Type != nil && !models.IsValidOrgType(*req.Type) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный тип организации")
	}

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

	// Канонизируем оформление ТОЛЬКО когда наименование реально меняют (#1437): иначе
	// внутренние вызовы, передающие текущее имя ради смены одного лишь типа
	// (BulkUpdateType), тихо переписали бы легаси-запись и оставили в истории ложную
	// «переименована».
	if req.Name != org.Name {
		req.Name = normalize.OrgNameDisplay(req.Name)
	}
	if normalize.OrgNameMeaningless(req.Name) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Укажите наименование организации")
	}

	// Конфликт с другой активной организацией по ключу дедупликации (#1437):
	// переименование в другое написание существующего наименования - тот же дубль.
	normalized := normalize.OrgName(req.Name)
	var dup int64
	if err := applyNameDuplicateFilter(
		s.db.WithContext(ctx).Model(&models.Organization{}).Where("is_active = ? AND id <> ?", true, id),
		req.Name, normalized,
	).Count(&dup).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking organization")
	}
	if dup > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Организация с таким названием уже существует")
	}

	// map-обновление (а не struct) - чтобы явно записать type=NULL при снятии типа.
	// name_normalized пишем явно: BeforeSave до map-обновления не достаёт.
	if err := s.db.WithContext(ctx).Model(&models.Organization{}).
		Where("id = ?", id).Updates(map[string]any{"name": req.Name, "type": req.Type, "name_normalized": normalized}).Error; err != nil {
		slog.Error("Не удалось обновить организацию", "id", id, "error", err)
		return nil, directoryWriteError(err, "Организация с таким названием уже существует", "Error updating organization")
	}
	slog.Info("организация обновлена", "id", id, "name", req.Name)
	// org.* - старые значения (map-обновление структуру не трогает), req.* - новые.
	// Различаем, что изменилось, чтобы история не писала «переименована» при
	// смене одного лишь типа.
	nameChanged := org.Name != req.Name
	typeChanged := !strPtrEqual(org.Type, req.Type)
	if !nameChanged && !typeChanged {
		// Ничего не поменялось (PUT с теми же значениями) - не пишем ложную
		// «переименована» в историю (как no-op в BulkUpdateType).
		return &OrganizationInfoResponse{ID: id, Name: req.Name, Type: req.Type, ModerationStatus: org.ModerationStatus}, nil
	}
	action := models.OrganizationActionRenamed
	switch {
	case nameChanged && typeChanged:
		action = models.OrganizationActionUpdated
	case typeChanged:
		action = models.OrganizationActionRetyped
	}
	// name/type - новые значения (обратная совместимость рендера), from - старые
	// (FE рисует «было -> стало» когда from присутствует; у старых записей его нет).
	s.recorder.Log(ctx, nil, models.AuditEntityOrganization, &id, action, &callerUserID, map[string]any{
		"name": req.Name, "type": req.Type,
		"from": map[string]any{"name": org.Name, "type": org.Type},
	})
	return &OrganizationInfoResponse{ID: id, Name: req.Name, Type: req.Type, ModerationStatus: org.ModerationStatus}, nil
}

// applyNameDuplicateFilter добавляет к запросу условие поиска дубля наименования.
// Обычно это ключ дедупликации, но у вырожденных наименований (одни кавычки, точки
// или дефисы) ключ пуст, и по нему такие записи схлопнулись бы между собой. Для них
// сверяемся по точному имени - защита не должна стать слабее той, что была до ключа
// (#1437). По той же причине unique index на name_normalized ставится с условием
// name_normalized <> ”.
func applyNameDuplicateFilter(q *gorm.DB, name, normalized string) *gorm.DB {
	if normalized == "" {
		return q.Where("name = ?", name)
	}
	return q.Where("name_normalized = ?", normalized)
}

// strPtrEqual сравнивает два *string как значения (оба nil = равны). Используется
// org/company Update для определения, менялся ли тип (nil = «не указан»).
func strPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
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
	s.recorder.Log(ctx, nil, models.AuditEntityOrganization, &id, models.OrganizationActionArchived, &callerUserID, nil)
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
	if err := applyNameDuplicateFilter(
		s.db.WithContext(ctx).Model(&models.Organization{}).Where("is_active = ? AND id <> ?", true, id),
		org.Name, normalize.OrgName(org.Name),
	).Count(&active).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking organization")
	}
	if active > 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Активная организация с таким названием уже существует - переименуйте перед восстановлением")
	}

	if err := s.db.WithContext(ctx).Model(&models.Organization{}).
		Where("id = ?", id).Update("is_active", true).Error; err != nil {
		slog.Error("Не удалось восстановить организацию", "id", id, "error", err)
		return directoryWriteError(err,
			"Активная организация с таким названием уже существует - переименуйте перед восстановлением",
			"Error restoring organization")
	}
	slog.Info("организация восстановлена", "id", id)
	s.recorder.Log(ctx, nil, models.AuditEntityOrganization, &id, models.OrganizationActionRestored, &callerUserID, nil)
	return nil
}

// GetHistory возвращает историю изменений организации (admin-only, новые сверху).
// #870, финал F.2: запись и до-cutover строки живут в общем audit_log (старые
// перенесены backfill'ом BackfillAuditFromLegacy), поэтому чтение идёт только из
// audit_log. Замороженная organization_histories дропнута в дроп-sweep (F.8).
// Форму ответа стережёт TestOrganizations_History.
// Действия created/renamed хранят {name,type} (тип с #1046) - details как есть.
func (s *organizationService) GetHistory(ctx context.Context, id int) ([]models.OrganizationHistoryItem, error) {
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
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityOrganization, id).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organization history")
	}

	// Логин вместо ФИО у акторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]models.OrganizationHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.OrganizationHistoryItem{
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

// GetWithUsers возвращает организации с количеством привязанных пользователей.
func (s *organizationService) GetWithUsers(ctx context.Context, includeArchived bool) ([]OrganizationWithUsersResponse, error) {
	orgs := make([]OrganizationWithUsersResponse, 0)
	q := s.db.WithContext(ctx).
		Table("organizations o").
		Select("o.id, o.name, o.type, o.is_active, o.moderation_status, COUNT(u.id) FILTER (WHERE u.is_active = true) as user_count").
		Joins("LEFT JOIN users u ON u.organization_id = o.id").
		Group("o.id, o.name, o.type, o.is_active, o.moderation_status").
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
		Select("o.id, o.name, o.type, o.is_active, o.moderation_status, COUNT(u.id) FILTER (WHERE u.is_active = true) as user_count").
		Joins("LEFT JOIN users u ON u.organization_id = o.id").
		Group("o.id, o.name, o.type, o.is_active, o.moderation_status").
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
			"id":                org.ID,
			"name":              org.Name,
			"type":              org.Type,
			"is_active":         org.IsActive,
			"user_count":        org.UserCount,
			"moderation_status": org.ModerationStatus,
			"unload_places":     places,
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
		Where("ou.organization_id = ? AND u.is_active = ?", orgID, true).
		Order("ou.is_primary DESC, u.last_name, u.first_name").
		Scan(&users).Error
	if err != nil {
		slog.Error("Не удалось получить пользователей организации", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organization users")
	}
	if masks := loadConsentMasks(ctx, s.db); len(masks) > 0 {
		for i := range users {
			maskUserParts(masks, users[i].ID, &users[i].LastName, &users[i].FirstName, &users[i].MiddleName)
		}
	}
	return users, nil
}

// GetMembers возвращает активных пользователей, привязанных к организации через
// users.organization_id (участники). Это те же пользователи, что дают user_count.
func (s *organizationService) GetMembers(ctx context.Context, orgID int) ([]MemberResponse, error) {
	members := make([]MemberResponse, 0)
	err := s.db.WithContext(ctx).
		Table("users u").
		Select("u.id, u.username, u.last_name, u.first_name, u.middle_name, u.position").
		Where("u.organization_id = ? AND u.is_active = ?", orgID, true).
		Order("u.last_name, u.first_name, u.username").
		Scan(&members).Error
	if err != nil {
		slog.Error("Не удалось получить участников организации", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organization members")
	}
	if masks := loadConsentMasks(ctx, s.db); len(masks) > 0 {
		for i := range members {
			maskUserParts(masks, members[i].ID, &members[i].LastName, &members[i].FirstName, &members[i].MiddleName)
		}
	}
	return members, nil
}

// ReassignMembers переносит активных участников организации id (набор, блокирующий
// её архивацию: users.organization_id=id AND is_active) в целевую организацию
// targetID. Освобождает исходную, чтобы её можно было архивировать. Целевая должна
// существовать, быть активной и отличаться от исходной. Идемпотентно: если
// блокеров нет, возвращает 0 без ошибки. Аудит смены org пишется на каждого
// пользователя (UserActionOrgChanged) в той же транзакции - провал записи
// откатывает перенос.
func (s *organizationService) ReassignMembers(ctx context.Context, callerUserID, id, targetID int) (int, error) {
	var src models.Organization
	if err := s.db.WithContext(ctx).First(&src, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, echo.NewHTTPError(http.StatusNotFound, "Organization not found")
		}
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organization")
	}
	if targetID == id {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Нельзя перенести пользователей в ту же организацию")
	}

	var count int
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Целевую проверяем ВНУТРИ транзакции с блокировкой строки (FOR UPDATE):
		// закрывает окно TOCTOU, когда параллельная архивация target прошла бы между
		// проверкой активности и переносом - иначе пользователи ушли бы в архивную.
		var target models.Organization
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&target, targetID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return echo.NewHTTPError(http.StatusBadRequest, "Целевая организация не найдена")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching target organization")
		}
		if !target.IsActive {
			return echo.NewHTTPError(http.StatusBadRequest, "Нельзя перенести в архивную организацию")
		}
		var ids []int
		if err := tx.Model(&models.User{}).
			Where("organization_id = ? AND is_active = ?", id, true).
			Pluck("id", &ids).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching users")
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Model(&models.User{}).
			Where("id IN ?", ids).
			Update("organization_id", targetID).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error reassigning users")
		}
		for _, uid := range ids {
			u := uid
			if err := s.recorder.Record(ctx, tx, models.AuditEntityUser, &u, models.UserActionOrgChanged, &callerUserID, map[string]any{"old": id, "new": targetID}); err != nil {
				return err
			}
		}
		count = len(ids)
		return nil
	}); err != nil {
		return 0, err
	}
	slog.Info("участники перенесены между организациями", "from", id, "to", targetID, "count", count)
	return count, nil
}

// UpdateOrganizationUsers заменяет ответственных пользователей организации.
// После применения пишет в audit_log запись «кто был -> кто стал» (added/removed/
// approval_changed), если набор реально изменился. Логирование - в шаренном методе,
// поэтому bulk (реюз в цикле) и одиночная деталь-панель пишут историю консистентно.
func (s *organizationService) UpdateOrganizationUsers(ctx context.Context, callerUserID, orgID int, req UpdateOrganizationUsersRequest) error {
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

	oldUsers := s.attachedUserStates(ctx, orgID)
	var applied []auditUserState

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Удаляем старые связи
		if err := tx.Where("organization_id = ?", orgID).Delete(&models.OrganizationUser{}).Error; err != nil {
			slog.Error("Не удалось удалить старых пользователей организации", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating organization users")
		}

		applied = applied[:0]
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
			applied = append(applied, auditUserState{
				Username:         user.Username,
				Name:             fullName(user.LastName, user.FirstName, user.Username),
				RequiredApproval: requiredApproval,
				IsPrimary:        isPrimary,
			})
		}
		return nil
	}); err != nil {
		return err
	}

	if diff := diffUsers(oldUsers, applied); !diff.empty() {
		s.recorder.Log(ctx, nil, models.AuditEntityOrganization, &orgID, models.OrganizationActionResponsiblesChanged, &callerUserID, diff)
	}
	return nil
}

// attachedUserStates - снимок текущих ответственных организации (username, ФИО,
// флаг согласования) для diff аудита.
func (s *organizationService) attachedUserStates(ctx context.Context, orgID int) []auditUserState {
	var rows []auditUserState
	if err := s.db.WithContext(ctx).
		Table("organization_users ou").
		Select("u.username AS username, "+auditUserNameSQL+" AS name, ou.required_approval AS required_approval, ou.is_primary AS is_primary").
		Joins("JOIN users u ON u.id = ou.user_id").
		Where("ou.organization_id = ?", orgID).Scan(&rows).Error; err != nil {
		slog.Warn("audit: не удалось прочитать текущих ответственных организации", "org_id", orgID, "error", err)
	}
	return rows
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

// UpdateOrganizationTables заменяет привязку таблиц к организации. После применения
// пишет в audit_log added/removed по именам таблиц, если набор изменился.
func (s *organizationService) UpdateOrganizationTables(ctx context.Context, callerUserID, orgID int, req UpdateOrganizationTablesRequest) error {
	oldTables := s.attachedTableNames(ctx, orgID)

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	}); err != nil {
		return err
	}

	if diff := diffIDNames(oldTables, req.TableIDs, s.tableNamesByIDs(ctx, req.TableIDs)); !diff.empty() {
		s.recorder.Log(ctx, nil, models.AuditEntityOrganization, &orgID, models.OrganizationActionTablesChanged, &callerUserID, diff)
	}
	return nil
}

// attachedTableNames - id->display_name таблиц, привязанных к организации (сырая
// junction, без is_active: replace удаляет и неактивные привязки, diff их учитывает).
func (s *organizationService) attachedTableNames(ctx context.Context, orgID int) map[int]string {
	var rows []idName
	if err := s.db.WithContext(ctx).Table("organization_tables ot").
		Select("st.id AS id, COALESCE(NULLIF(st.display_name, ''), st.name) AS name").
		Joins("JOIN system_tables st ON st.id = ot.table_id").
		Where("ot.organization_id = ?", orgID).Scan(&rows).Error; err != nil {
		slog.Warn("audit: не удалось прочитать таблицы организации", "org_id", orgID, "error", err)
	}
	return idNameMap(rows)
}

func (s *organizationService) tableNamesByIDs(ctx context.Context, ids []int) map[int]string {
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
// После применения пишет в audit_log added/removed по именам мест, если набор изменился.
func (s *organizationService) UpdateOrganizationUnloadPlaces(ctx context.Context, callerUserID, orgID int, req UpdateOrganizationUnloadPlacesRequest) error {
	oldPlaces := s.attachedPlaceNames(ctx, orgID)

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	}); err != nil {
		return err
	}

	if diff := diffIDNames(oldPlaces, req.UnloadPlaceIDs, s.placeNamesByIDs(ctx, req.UnloadPlaceIDs)); !diff.empty() {
		s.recorder.Log(ctx, nil, models.AuditEntityOrganization, &orgID, models.OrganizationActionUnloadPlacesChanged, &callerUserID, diff)
	}
	return nil
}

// attachedPlaceNames - id->name мест разгрузки, привязанных к организации (сырая junction).
func (s *organizationService) attachedPlaceNames(ctx context.Context, orgID int) map[int]string {
	var rows []idName
	if err := s.db.WithContext(ctx).Table("organization_unload_places oup").
		Select("up.id AS id, up.name AS name").
		Joins("JOIN unload_places up ON up.id = oup.unload_place_id").
		Where("oup.organization_id = ?", orgID).Scan(&rows).Error; err != nil {
		slog.Warn("audit: не удалось прочитать места разгрузки организации", "org_id", orgID, "error", err)
	}
	return idNameMap(rows)
}

func (s *organizationService) placeNamesByIDs(ctx context.Context, ids []int) map[int]string {
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

// --- Групповые операции ---

// loadOrg загружает организацию по id для bulk-цикла. Возвращает имя (для
// BulkItemError) и ok=false, если записи нет.
func (s *organizationService) loadOrg(ctx context.Context, id int) (models.Organization, bool) {
	var org models.Organization
	if err := s.db.WithContext(ctx).First(&org, id).Error; err != nil {
		return org, false
	}
	return org, true
}

// BulkUpdateType меняет тип у набора организаций через переиспользование Update
// (имя берётся из текущей записи, чтобы не переименовать). Тип валидируется один
// раз до цикла.
func (s *organizationService) BulkUpdateType(ctx context.Context, callerUserID int, ids []int, typ *string) (*BulkOpResult, error) {
	if typ != nil && !models.IsValidOrgType(*typ) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный тип организации")
	}
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		org, ok := s.loadOrg(ctx, id)
		if !ok {
			res.addError(id, "", "Организация не найдена")
			continue
		}
		// Тип уже соответствует - no-op успех, без похода в Update: иначе оно
		// (имя тоже не меняется) залогировало бы ложную «переименована» в историю
		// (nameChanged=typeChanged=false -> дефолтный action Renamed). Для bulk
		// это частый кейс: в наборе часть организаций уже нужного типа.
		if strPtrEqual(org.Type, typ) {
			res.SuccessCount++
			continue
		}
		if _, err := s.Update(ctx, callerUserID, id, CreateOrganizationRequest{Name: org.Name, Type: typ}); err != nil {
			res.addError(id, org.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkAssignUnloadPlaces назначает места разгрузки набору организаций. В режиме
// add текущие связи читаются из сырой junction (без фильтра is_active), чтобы
// объединение не отвязало неактивные-но-привязанные места. Чтение current идёт
// вне транзакции переиспользуемого UpdateOrganizationUnloadPlaces - для
// admin-only последовательных операций окна гонки нет; конкурентный bulk по той
// же организации - осознанно не защищаем (принятый trade-off ради переиспользования).
func (s *organizationService) BulkAssignUnloadPlaces(ctx context.Context, callerUserID int, ids, placeIDs []int, mode string) (*BulkOpResult, error) {
	if !isValidBulkMode(mode) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный режим (replace|add)")
	}
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		org, ok := s.loadOrg(ctx, id)
		if !ok {
			res.addError(id, "", "Организация не найдена")
			continue
		}
		target := placeIDs
		if mode == BulkModeAdd {
			var current []int
			if err := s.db.WithContext(ctx).Model(&models.OrganizationUnloadPlace{}).
				Where("organization_id = ?", id).Pluck("unload_place_id", &current).Error; err != nil {
				res.addError(id, org.Name, "Ошибка чтения текущих мест разгрузки")
				continue
			}
			target = unionInts(current, placeIDs)
		}
		if err := s.UpdateOrganizationUnloadPlaces(ctx, callerUserID, id, UpdateOrganizationUnloadPlacesRequest{UnloadPlaceIDs: target}); err != nil {
			res.addError(id, org.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkAssignTables назначает целевые таблицы набору организаций (replace|add).
func (s *organizationService) BulkAssignTables(ctx context.Context, callerUserID int, ids, tableIDs []int, mode string) (*BulkOpResult, error) {
	if !isValidBulkMode(mode) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный режим (replace|add)")
	}
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		org, ok := s.loadOrg(ctx, id)
		if !ok {
			res.addError(id, "", "Организация не найдена")
			continue
		}
		target := tableIDs
		if mode == BulkModeAdd {
			var current []int
			if err := s.db.WithContext(ctx).Model(&models.OrganizationTable{}).
				Where("organization_id = ?", id).Pluck("table_id", &current).Error; err != nil {
				res.addError(id, org.Name, "Ошибка чтения текущих таблиц")
				continue
			}
			target = unionInts(current, tableIDs)
		}
		if err := s.UpdateOrganizationTables(ctx, callerUserID, id, UpdateOrganizationTablesRequest{TableIDs: target}); err != nil {
			res.addError(id, org.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkAssignUsers назначает ответственных набору организаций. primary в группе
// не назначается: за существующими сохраняется их is_primary/required_approval,
// новым выставляется is_primary=false и переданный required_approval. В режиме
// replace итоговый набор = выбранные (у оставшегося primary сохраняется), в add
// = текущие как есть + недостающие выбранные.
func (s *organizationService) BulkAssignUsers(ctx context.Context, callerUserID int, ids []int, assignments []BulkUserAssignment, mode string) (*BulkOpResult, error) {
	if !isValidBulkMode(mode) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный режим (replace|add)")
	}
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		org, ok := s.loadOrg(ctx, id)
		if !ok {
			res.addError(id, "", "Организация не найдена")
			continue
		}
		users, err := s.buildBulkUsers(ctx, id, assignments, mode)
		if err != nil {
			res.addError(id, org.Name, "Ошибка чтения ответственных")
			continue
		}
		if err := s.UpdateOrganizationUsers(ctx, callerUserID, id, UpdateOrganizationUsersRequest{Users: users}); err != nil {
			res.addError(id, org.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// buildBulkUsers формирует итоговый список ответственных для одной организации,
// сохраняя primary существующих (см. BulkAssignUsers).
func (s *organizationService) buildBulkUsers(ctx context.Context, orgID int, assignments []BulkUserAssignment, mode string) ([]OrganizationUserRequest, error) {
	type curRow struct {
		Username         string
		IsPrimary        bool
		RequiredApproval bool
	}
	var rows []curRow
	if err := s.db.WithContext(ctx).
		Table("organization_users ou").
		Select("u.username, ou.is_primary, ou.required_approval").
		Joins("JOIN users u ON u.id = ou.user_id").
		Where("ou.organization_id = ?", orgID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	existing := make(map[string]curRow, len(rows))
	for _, r := range rows {
		existing[r.Username] = r
	}

	users := make([]OrganizationUserRequest, 0, len(rows)+len(assignments))
	if mode == BulkModeAdd {
		// Существующие сохраняем как есть (флаги, включая primary, не трогаем).
		for _, r := range rows {
			isP, ra := r.IsPrimary, r.RequiredApproval
			users = append(users, OrganizationUserRequest{Username: r.Username, IsPrimary: &isP, RequiredApproval: &ra})
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
		users = append(users, OrganizationUserRequest{Username: un, IsPrimary: &isP, RequiredApproval: &ra})
	}
	return users, nil
}

// BulkArchive архивирует набор организаций через Delete. Активные организации с
// пользователями честно попадают в Errors (частичный успех).
func (s *organizationService) BulkArchive(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		org, ok := s.loadOrg(ctx, id)
		if !ok {
			res.addError(id, "", "Организация не найдена")
			continue
		}
		if err := s.Delete(ctx, callerUserID, id); err != nil {
			res.addError(id, org.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkRestore восстанавливает набор организаций через Restore.
func (s *organizationService) BulkRestore(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		org, ok := s.loadOrg(ctx, id)
		if !ok {
			res.addError(id, "", "Организация не найдена")
			continue
		}
		if err := s.Restore(ctx, callerUserID, id); err != nil {
			res.addError(id, org.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}
