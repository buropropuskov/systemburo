package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/normalize"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// diffUniqueCar сравнивает значимые поля UniqueCar до и после апдейта.
// Возвращает только реально изменившиеся поля.
func diffUniqueCar(before, after *models.UniqueCar) []fieldChange {
	changes := make([]fieldChange, 0)
	addStr := func(field string, oldP, newP *string) {
		if !equalStrPtr(oldP, newP) {
			changes = append(changes, fieldChange{Field: field, Old: copyStrPtr(oldP), New: copyStrPtr(newP)})
		}
	}
	addInt := func(field string, oldP, newP *int) {
		if !equalIntPtr(oldP, newP) {
			changes = append(changes, fieldChange{Field: field, Old: intPtrToStrPtr(oldP), New: intPtrToStrPtr(newP)})
		}
	}
	addStr("number", before.Number, after.Number)
	addStr("mark", before.Mark, after.Mark)
	addInt("organization_id", before.OrganizationID, after.OrganizationID)
	addInt("company_id", before.CompanyID, after.CompanyID)
	addInt("format_id", before.FormatID, after.FormatID)
	addInt("user_id", before.UserID, after.UserID)
	return changes
}

// CarOwnerInfo -- информация о владельце для фильтрации машин.
type CarOwnerInfo struct {
	HasOrganization  bool    `json:"has_organization"`
	HasCompany       bool    `json:"has_company"`
	OrganizationID   *int    `json:"organization_id"`
	CompanyID        *int    `json:"company_id"`
	UserID           int     `json:"user_id"`
	OrganizationName *string `json:"organization_name"`
	CompanyName      *string `json:"company_name"`
	// CanManageAll -- администратор системы: правит и удаляет любую запись реестра,
	// кому бы она ни принадлежала. Уходит на фронт тем же ответом ownership-info,
	// который вью и так запрашивает: показ кнопок правки и серверный гейт обязаны
	// стоять на одном признаке, иначе получается "кнопка есть, а в ответ 403".
	CanManageAll bool `json:"can_manage_all"`
}

// UniqueCarWithRelations -- машина с данными связанных сущностей.
type UniqueCarWithRelations struct {
	ID               int        `json:"id"`
	Number           *string    `json:"number"`
	Mark             *string    `json:"mark"`
	OrganizationID   *int       `json:"organization_id"`
	CompanyID        *int       `json:"company_id"`
	FormatID         *int       `json:"format_id"`
	UserID           *int       `json:"user_id"`
	Status           bool       `json:"status"`
	CreatedAt        *time.Time `json:"created_at"`
	OrganizationName *string    `json:"organization_name"`
	CompanyName      *string    `json:"company_name"`
	FormatName       *string    `json:"format_name"`
	// UserName -- за кем закреплена запись: ФИО владельца, у не давшего согласия -
	// логин с собачкой. Отдаётся только администратору, см. maskCarOwners.
	UserName             *string `json:"user_name"`
	OwnerUsername        *string `json:"-" gorm:"column:owner_username"`
	OwnerLastName        *string `json:"-" gorm:"column:owner_last_name"`
	OwnerFirstName       *string `json:"-" gorm:"column:owner_first_name"`
	OwnerMiddleName      *string `json:"-" gorm:"column:owner_middle_name"`
	ActiveEntryDateTo    *string `json:"active_entry_date_to"`
	ActiveEntryTimeFrom  *string `json:"active_entry_time_from"`
	ActiveEntryTimeTo    *string `json:"active_entry_time_to"`
	ActiveAppOrgName     *string `json:"active_app_org_name"`
	ActiveAppCompanyName *string `json:"active_app_company_name"`
	// ActiveCarID -- id строки в cars активной заявки (заявочная таблица, не реестр).
	// Нужен фронту, чтобы подтянуть статус территории и места разгрузки активной машины
	// (current-status и cars/unload-places ключуются по cars.id, а не по unique_cars.id).
	ActiveCarID *int `json:"active_car_id"`
	// ActiveApplicationID -- id заявки (applications.id) той же активной заявки, что и
	// прочие active_*-поля. Нужен фронту для кнопки "Открыть заявку" на вкладке Автомобили.
	ActiveApplicationID *int `json:"active_application_id"`
	// IsBlacklisted -- машина в активном чёрном списке (совпадение по номеру и марке).
	// Считается на сервере, чтобы фронт не выгружал весь список ЧС ради подсветки.
	IsBlacklisted bool `json:"is_blacklisted"`
}

// NewUniqueCarRequest -- тело запроса на создание/обновление машины.
type NewUniqueCarRequest struct {
	Number         string `json:"number" validate:"required,min=1,max=50"`
	Mark           string `json:"mark" validate:"omitempty,max=100"`
	OrganizationID *int   `json:"organization_id"`
	CompanyID      *int   `json:"company_id"`
	FormatID       *int   `json:"format_id"`
	UserID         *int   `json:"user_id"`
}

// UniqueCarResponse -- ответ при создании/обновлении машины.
type UniqueCarResponse struct {
	ID             int        `json:"id"`
	Number         *string    `json:"number"`
	Mark           *string    `json:"mark"`
	OrganizationID *int       `json:"organization_id"`
	CompanyID      *int       `json:"company_id"`
	FormatID       *int       `json:"format_id"`
	UserID         *int       `json:"user_id"`
	Status         bool       `json:"status"`
	CreatedAt      *time.Time `json:"created_at"`
}

// UpdateCarByNumberRequest -- запрос на обновление машины по номеру и марке.
type UpdateCarByNumberRequest struct {
	Number     string              `json:"number" validate:"required"`
	Mark       string              `json:"mark"`
	UpdateData NewUniqueCarRequest `json:"update_data"`
}

// BatchCreateCarsResponse -- результат пакетного создания машин.
type BatchCreateCarsResponse struct {
	CreatedCars  []UniqueCarResponse `json:"created_cars"`
	Errors       []string            `json:"errors"`
	SuccessCount int                 `json:"success_count"`
	ErrorCount   int                 `json:"error_count"`
}

// UniqueCarHistoryItem -- запись истории мастер-машины с username вызывающего.
type UniqueCarHistoryItem struct {
	ID int `json:"id"`
	// Subject -- к какой машине относится событие (номер с маркой на момент действия).
	Subject       *string   `json:"subject"`
	UniqueCarID   int       `json:"unique_car_id"`
	UserID        *int      `json:"user_id"`
	Username      *string   `json:"username"`
	UserLastName  *string   `json:"user_last_name"`
	UserFirstName *string   `json:"user_first_name"`
	ActionType    string    `json:"action_type"`
	FieldName     *string   `json:"field_name"`
	OldValue      *string   `json:"old_value"`
	NewValue      *string   `json:"new_value"`
	Comment       *string   `json:"comment"`
	CreatedAt     time.Time `json:"created_at"`
}

// UniqueCarService -- интерфейс бизнес-логики уникальных машин.
type UniqueCarService interface {
	GetOwnerInfo(ctx context.Context, username string) (*CarOwnerInfo, error)
	GetAll(ctx context.Context, username string, filterType string) ([]UniqueCarWithRelations, error)
	// GetAllPaginated возвращает страницу реестра с серверным поиском (#1158, срез 2):
	// используется CarsView вместо GetAll, как только запрос несёт per_page.
	GetAllPaginated(ctx context.Context, username, filterType, searchQuery string, page, perPage int) ([]UniqueCarWithRelations, int64, error)
	// LookupByNumberMark ищет машину по номеру и марке (LOWER(TRIM), как ЧС) для открытия
	// карточки со страницы чёрного списка. Возвращает nil, nil если совпадения нет.
	LookupByNumberMark(ctx context.Context, number, mark string) (*UniqueCarWithRelations, error)
	Create(ctx context.Context, username string, req NewUniqueCarRequest) (*UniqueCarResponse, error)
	CreateBatch(ctx context.Context, username string, reqs []NewUniqueCarRequest) (*BatchCreateCarsResponse, int, error)
	Update(ctx context.Context, username string, id int, req NewUniqueCarRequest) (*UniqueCarResponse, error)
	UpdateByNumber(ctx context.Context, username string, req UpdateCarByNumberRequest) (*UniqueCarResponse, error)
	Delete(ctx context.Context, username string, id int) error
	GetHistory(ctx context.Context, username string, id int) ([]UniqueCarHistoryItem, error)
	// GetRegistryLog - журнал по всему реестру машин, включая удалённые записи.
	// Зеркало UniqueEmployeeService.GetRegistryLog, причина описана там.
	GetRegistryLog(ctx context.Context, username string, limit int) ([]UniqueCarHistoryItem, error)
}

type uniqueCarService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewUniqueCarService создаёт реализацию UniqueCarService.
func NewUniqueCarService(db *gorm.DB) UniqueCarService {
	return &uniqueCarService{db: db, recorder: NewAuditRecorder(db)}
}

// getCarOwnerInfo получает информацию о владельце по username.
func (s *uniqueCarService) getCarOwnerInfo(ctx context.Context, username string) (*CarOwnerInfo, error) {
	var result struct {
		UserID           int     `gorm:"column:user_id"`
		OrganizationID   *int    `gorm:"column:organization_id"`
		CompanyID        *int    `gorm:"column:company_id"`
		HasOrganization  bool    `gorm:"column:has_organization"`
		HasCompany       bool    `gorm:"column:has_company"`
		OrganizationName *string `gorm:"column:organization_name"`
		CompanyName      *string `gorm:"column:company_name"`
		CanManageAll     bool    `gorm:"column:can_manage_all"`
	}

	err := s.db.WithContext(ctx).
		Table("users u").
		Select(`u.id as user_id, u.organization_id, u.company_id,
			CASE WHEN o.id IS NOT NULL THEN true ELSE false END as has_organization,
			CASE WHEN c.id IS NOT NULL THEN true ELSE false END as has_company,
			o.name as organization_name, c.name as company_name,
			`+systemAdminExpr+` as can_manage_all`).
		Joins("LEFT JOIN organizations o ON u.organization_id = o.id").
		Joins("LEFT JOIN companies c ON u.company_id = c.id").
		Where("u.username = ?", username).
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user info")
	}

	return &CarOwnerInfo{
		HasOrganization:  result.HasOrganization,
		HasCompany:       result.HasCompany,
		OrganizationID:   result.OrganizationID,
		CompanyID:        result.CompanyID,
		UserID:           result.UserID,
		OrganizationName: result.OrganizationName,
		CompanyName:      result.CompanyName,
		CanManageAll:     result.CanManageAll,
	}, nil
}

// GetOwnerInfo возвращает информацию о владельце для фильтрации машин.
func (s *uniqueCarService) GetOwnerInfo(ctx context.Context, username string) (*CarOwnerInfo, error) {
	return s.getCarOwnerInfo(ctx, username)
}

// LookupByNumberMark ищет машину по номеру+марке без скоупинга по владельцу (вызывается
// из админ-страницы ЧС). Берёт самую свежую при нескольких совпадениях. nil,nil если нет.
func (s *uniqueCarService) LookupByNumberMark(ctx context.Context, number, mark string) (*UniqueCarWithRelations, error) {
	rows := make([]UniqueCarWithRelations, 0, 1)
	err := s.db.WithContext(ctx).
		Table("unique_cars uc").
		Select(`uc.id, uc.number, uc.mark, uc.organization_id, uc.company_id, uc.format_id, uc.user_id,
			uc.status, uc.created_at,
			o.name as organization_name, c.name as company_name, lpf.name as format_name`).
		Joins("LEFT JOIN organizations o ON uc.organization_id = o.id").
		Joins("LEFT JOIN companies c ON uc.company_id = c.id").
		Joins("LEFT JOIN license_plate_formats lpf ON uc.format_id = lpf.id").
		Where("LOWER(TRIM(uc.number)) = LOWER(TRIM(?))", number).
		Where("LOWER(TRIM(COALESCE(uc.mark, ''))) = LOWER(TRIM(?))", mark).
		Order("uc.id DESC").
		Limit(1).
		Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка поиска машины")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// systemAdminExpr -- SQL-признак администратора системы (супер-админ или админ) в
// терминах алиаса users u. Единственное место, где это правило записано: и точечная
// проверка userIsSystemAdmin, и выборка владельца (getCarOwnerInfo /
// getEmployeeOwnerInfo) берут признак отсюда. Две формулы рядом разъехались бы при
// первой же правке модели прав, а от этого признака зависит и видимость системного
// среза, и право править чужую запись.
const systemAdminExpr = "(u.is_super_admin OR u.is_admin)"

// maskCarOwners заполняет «за кем закреплена запись» администратору и убирает поле у
// остальных. Зеркало maskEmployeeOwners: причина, маска имени и оговорка про write-path
// описаны там.
func maskCarOwners(ctx context.Context, db *gorm.DB, rows []UniqueCarWithRelations, canManageAll bool) {
	if !canManageAll {
		for i := range rows {
			rows[i].UserName = nil
		}
		return
	}
	masks := loadNameMasks(ctx, db)
	for i := range rows {
		rows[i].UserName = ownerDisplayName(masks, rows[i].UserID,
			rows[i].OwnerUsername, rows[i].OwnerLastName, rows[i].OwnerFirstName, rows[i].OwnerMiddleName)
	}
}

// GetAll возвращает список уникальных автомобилей с фильтрацией по типу владельца.
// userIsSystemAdmin сообщает, администратор ли пользователь. От этого зависят две
// вещи: видимость системного среза (filter_type=all_system - все записи системы без
// фильтра; остальным запрещено, иначе любой авторизованный вытащил бы все машины и
// сотрудников системы, broken access control) и право править или удалять запись,
// закреплённую за чужой организацией.
func userIsSystemAdmin(ctx context.Context, db *gorm.DB, userID int) bool {
	var row struct {
		IsAdmin bool `gorm:"column:is_admin"`
	}
	if err := db.WithContext(ctx).Table("users u").
		Select(systemAdminExpr+" AS is_admin").
		Where("u.id = ?", userID).Scan(&row).Error; err != nil {
		return false
	}
	return row.IsAdmin
}

// carsListSelect -- список колонок для реестра машин (GetAll/GetAllPaginated).
// Вынесен в константу, т.к. переиспользуется обоими методами (#1158, срез 2) -
// раньше был только внутри GetAll. Активная заявка ищется по LOWER(TRIM(uc.number))
// без учёта марки (как исторически было) - при полном совпадении номера у разных
// машин с разными марками возможна редкая коллизия, вне объёма этого среза.
const carsListSelect = `uc.id, uc.number, uc.mark, uc.organization_id, uc.company_id,
	uc.format_id, uc.user_id, uc.created_at,
	o.name as organization_name, c.name as company_name,
	lpf.name as format_name,
	u.username as owner_username, u.last_name as owner_last_name,
	u.first_name as owner_first_name, u.middle_name as owner_middle_name,
	COALESCE((
		SELECT true FROM cars cr
		JOIN attachments a ON cr.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
		AND cr.status = 1
		AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		LIMIT 1
	), false) as status,
	(SELECT a.entry_date_to FROM cars cr
		JOIN attachments a ON cr.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
		AND cr.status = 1 AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		ORDER BY a.entry_date_to DESC LIMIT 1
	) as active_entry_date_to,
	(SELECT a.entry_time_from FROM cars cr
		JOIN attachments a ON cr.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
		AND cr.status = 1 AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		ORDER BY a.entry_date_to DESC LIMIT 1
	) as active_entry_time_from,
	(SELECT a.entry_time_to FROM cars cr
		JOIN attachments a ON cr.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
		AND cr.status = 1 AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		ORDER BY a.entry_date_to DESC LIMIT 1
	) as active_entry_time_to,
	(SELECT ao.name FROM cars cr
		JOIN attachments a ON cr.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		LEFT JOIN organizations ao ON app.organization_id = ao.id
		WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
		AND cr.status = 1 AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		ORDER BY a.entry_date_to DESC LIMIT 1
	) as active_app_org_name,
	(SELECT ac.name FROM cars cr
		JOIN attachments a ON cr.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		LEFT JOIN companies ac ON app.company_id = ac.id
		WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
		AND cr.status = 1 AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		ORDER BY a.entry_date_to DESC LIMIT 1
	) as active_app_company_name,
	(SELECT cr.id FROM cars cr
		JOIN attachments a ON cr.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
		AND cr.status = 1 AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		ORDER BY a.entry_date_to DESC LIMIT 1
	) as active_car_id,
	(SELECT app.id FROM cars cr
		JOIN attachments a ON cr.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
		AND cr.status = 1 AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		ORDER BY a.entry_date_to DESC LIMIT 1
	) as active_application_id,
	-- Флаг ЧС считает сервер (нормализация 1:1 с vehicleBlacklistService.CheckByName),
	-- чтобы фронт не выгружал весь список машин ЧС ради подсветки в реестре.
	EXISTS(
		SELECT 1 FROM vehicle_blacklists vbl
		WHERE vbl.is_active
		AND LOWER(TRIM(vbl.car_number)) = LOWER(TRIM(uc.number))
		AND LOWER(TRIM(COALESCE(vbl.mark_name, ''))) = LOWER(TRIM(COALESCE(uc.mark, '')))
	) as is_blacklisted`

// buildCarsQuery строит базовый запрос реестра (джойны + фильтр владельца + поиск)
// БЕЗ Select/Order - переиспользуется отдельно для Count и для выборки данных
// (тот же паттерн, что buildApplicationsBaseQuery в application_service.go), чтобы
// Count считал по фильтрованному набору, не гоняя тяжёлые коррелированные
// подзапросы carsListSelect дважды.
func (s *uniqueCarService) buildCarsQuery(ctx context.Context, ownerInfo *CarOwnerInfo, filterType, searchQuery string) *gorm.DB {
	query := s.db.WithContext(ctx).
		Table("unique_cars uc").
		Joins("LEFT JOIN organizations o ON uc.organization_id = o.id").
		Joins("LEFT JOIN companies c ON uc.company_id = c.id").
		Joins("LEFT JOIN license_plate_formats lpf ON uc.format_id = lpf.id").
		Joins("LEFT JOIN users u ON uc.user_id = u.id")

	switch filterType {
	case "organization":
		if ownerInfo.HasOrganization {
			orgID := 0
			if ownerInfo.OrganizationID != nil {
				orgID = *ownerInfo.OrganizationID
			}
			query = query.Where("uc.organization_id = ?", orgID)
		} else {
			query = query.Where("uc.user_id = ?", ownerInfo.UserID)
		}
	case "company":
		if ownerInfo.HasCompany {
			compID := 0
			if ownerInfo.CompanyID != nil {
				compID = *ownerInfo.CompanyID
			}
			query = query.Where("uc.company_id = ?", compID)
		} else {
			query = query.Where("uc.user_id = ?", ownerInfo.UserID)
		}
	case "all":
		orgID := 0
		if ownerInfo.OrganizationID != nil {
			orgID = *ownerInfo.OrganizationID
		}
		compID := 0
		if ownerInfo.CompanyID != nil {
			compID = *ownerInfo.CompanyID
		}
		query = query.Where("uc.user_id = ? OR uc.organization_id = ? OR uc.company_id = ?",
			ownerInfo.UserID, orgID, compID)
	case "all_system":
		// Без фильтрации
	default:
		query = query.Where("uc.user_id = ?", ownerInfo.UserID)
	}

	if raw := strings.TrimSpace(searchQuery); raw != "" {
		// Поиск по номеру/марке/формату/организации/компании (реальные колонки uc.number,
		// uc.mark, lpf.name, o.name, c.name). Переиспользуем buildSearchVariants/
		// ilikePatternsArgs из application_helpers.go (тот же пакет services) - раскладка
		// и омоглифы номера уже покрыты. "Статус" НЕ ищем: показанное поле status в
		// carsListSelect - это коррелированный подзапрос (активная заявка на текущий
		// момент), а не колонка uc.status (та почти всегда false и в списке не
		// показывается) - фильтровать по тексту "активна/неактивна" пришлось бы тем же
		// дорогим подзапросом в WHERE; решили не дублировать ради второстепенного поля.
		variants := buildSearchVariants(raw)
		cols := []string{"uc.number", "uc.mark", "lpf.name", "o.name", "c.name"}
		cond, args := ilikePatternsArgs(cols, variants)
		if strings.ContainsAny(raw, "0123456789") {
			cond += " OR REPLACE(uc.number, ' ', '') ILIKE ?"
			args = append(args, "%"+normalize.Plate(raw)+"%")
		}
		query = query.Where(cond, args...)
	}

	return query
}

func (s *uniqueCarService) GetAll(ctx context.Context, username string, filterType string) ([]UniqueCarWithRelations, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	if filterType == "all_system" && !ownerInfo.CanManageAll {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав для просмотра всех записей системы")
	}

	query := s.buildCarsQuery(ctx, ownerInfo, filterType, "").
		Select(carsListSelect).
		Order("uc.number, uc.mark, uc.id")

	cars := make([]UniqueCarWithRelations, 0)
	if err := query.Scan(&cars).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching cars")
	}
	maskCarOwners(ctx, s.db, cars, ownerInfo.CanManageAll)

	return cars, nil
}

// GetAllPaginated возвращает страницу реестра с серверным поиском (#1158, срез 2).
func (s *uniqueCarService) GetAllPaginated(ctx context.Context, username, filterType, searchQuery string, page, perPage int) ([]UniqueCarWithRelations, int64, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, 0, err
	}

	if filterType == "all_system" && !ownerInfo.CanManageAll {
		return nil, 0, echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав для просмотра всех записей системы")
	}

	var total int64
	countQuery := s.buildCarsQuery(ctx, ownerInfo, filterType, searchQuery)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error counting cars")
	}

	offset := (page - 1) * perPage
	dataQuery := s.buildCarsQuery(ctx, ownerInfo, filterType, searchQuery).
		Select(carsListSelect).
		// uc.id третий ключ - number/mark не уникальны (нет unique-индекса), без
		// tie-breaker две равные строки могут переупорядочиться между offset-страницами
		// -> пропуск/дубль при бесшовной подгрузке (dedup прячет дубль, не пропуск).
		Order("uc.number, uc.mark, uc.id").
		Offset(offset).
		Limit(perPage)

	cars := make([]UniqueCarWithRelations, 0)
	if err := dataQuery.Scan(&cars).Error; err != nil {
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching cars")
	}
	maskCarOwners(ctx, s.db, cars, ownerInfo.CanManageAll)

	return cars, total, nil
}

// carToResponse конвертирует модель UniqueCar в UniqueCarResponse.
func carToResponse(car *models.UniqueCar) *UniqueCarResponse {
	status := false
	if car.Status != nil && *car.Status {
		status = true
	}
	return &UniqueCarResponse{
		ID:             car.ID,
		Number:         car.Number,
		Mark:           car.Mark,
		OrganizationID: car.OrganizationID,
		CompanyID:      car.CompanyID,
		FormatID:       car.FormatID,
		UserID:         car.UserID,
		Status:         status,
		CreatedAt:      &car.CreatedAt,
	}
}

// Create создаёт уникальный автомобиль с проверкой уникальности.
func (s *uniqueCarService) Create(ctx context.Context, username string, req NewUniqueCarRequest) (*UniqueCarResponse, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	// Проверка уникальности для пользователя
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
		Where("user_id = ? AND number = ? AND mark = ?", ownerInfo.UserID, req.Number, req.Mark).
		Count(&count).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
	}
	if count > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Автомобиль уже привязан к вашему аккаунту")
	}

	// Проверка уникальности для организации
	if req.OrganizationID != nil {
		var orgCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
			Where("organization_id = ? AND number = ? AND mark = ?", *req.OrganizationID, req.Number, req.Mark).
			Count(&orgCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
		}
		if orgCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Автомобиль с этим номером и маркой уже существует в этой организации")
		}
	}

	// Проверка уникальности для компании
	if req.CompanyID != nil {
		var compCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
			Where("company_id = ? AND number = ? AND mark = ?", *req.CompanyID, req.Number, req.Mark).
			Count(&compCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
		}
		if compCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Автомобиль с этим номером и маркой уже существует в этой компании")
		}
	}

	userID := ownerInfo.UserID
	if req.UserID != nil {
		userID = *req.UserID
	}

	statusFalse := false
	car := models.UniqueCar{
		Number:         &req.Number,
		Mark:           &req.Mark,
		OrganizationID: req.OrganizationID,
		CompanyID:      req.CompanyID,
		FormatID:       req.FormatID,
		UserID:         &userID,
		Status:         &statusFalse,
	}

	if err := s.db.WithContext(ctx).Create(&car).Error; err != nil {
		slog.Error("не удалось создать уникальный автомобиль", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка при создании автомобиля")
	}

	// Запись о заведении - для журнала реестра, см. uniqueEmployeeService.Create.
	createComment := fmt.Sprintf("Автомобиль %s заведён в реестр",
		strings.TrimSpace(strings.Join(nonEmptyStrings(car.Number, car.Mark), " ")))
	createSubject := strings.TrimSpace(strings.Join(nonEmptyStrings(car.Number, car.Mark), " "))
	if err := s.recorder.Record(ctx, nil, models.AuditEntityUniqueCar, &car.ID, "create",
		&ownerInfo.UserID, carAuditDetails{Comment: &createComment, Subject: &createSubject}); err != nil {
		slog.Error("не удалось записать создание автомобиля в журнал", "id", car.ID, "error", err)
	}

	slog.Info("уникальный автомобиль создан", "id", car.ID)
	return carToResponse(&car), nil
}

// CreateBatch создаёт несколько уникальных автомобилей пакетно.
func (s *uniqueCarService) CreateBatch(ctx context.Context, username string, reqs []NewUniqueCarRequest) (*BatchCreateCarsResponse, int, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, 0, err
	}

	createdCars := make([]UniqueCarResponse, 0)
	errors := make([]string, 0)

	for _, req := range reqs {
		// Проверка уникальности для пользователя
		var count int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
			Where("user_id = ? AND number = ? AND mark = ?", ownerInfo.UserID, req.Number, req.Mark).
			Count(&count).Error; err != nil {
			return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
		}
		if count > 0 {
			errors = append(errors, "Автомобиль "+req.Number+" "+req.Mark+" уже привязан к вашему аккаунту")
			continue
		}

		// Проверка уникальности для организации
		if req.OrganizationID != nil {
			var orgCount int64
			if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
				Where("organization_id = ? AND number = ? AND mark = ?", *req.OrganizationID, req.Number, req.Mark).
				Count(&orgCount).Error; err != nil {
				return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
			}
			if orgCount > 0 {
				errors = append(errors, "Автомобиль "+req.Number+" "+req.Mark+" уже существует в этой организации")
				continue
			}
		}

		// Проверка уникальности для компании
		if req.CompanyID != nil {
			var compCount int64
			if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
				Where("company_id = ? AND number = ? AND mark = ?", *req.CompanyID, req.Number, req.Mark).
				Count(&compCount).Error; err != nil {
				return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
			}
			if compCount > 0 {
				errors = append(errors, "Автомобиль "+req.Number+" "+req.Mark+" уже существует в этой компании")
				continue
			}
		}

		userID := ownerInfo.UserID
		if req.UserID != nil {
			userID = *req.UserID
		}

		statusFalse := false
		car := models.UniqueCar{
			Number:         &req.Number,
			Mark:           &req.Mark,
			OrganizationID: req.OrganizationID,
			CompanyID:      req.CompanyID,
			FormatID:       req.FormatID,
			UserID:         &userID,
			Status:         &statusFalse,
		}

		if err := s.db.WithContext(ctx).Create(&car).Error; err != nil {
			slog.Error("не удалось создать автомобиль в пакетной операции", "number", req.Number, "mark", req.Mark, "error", err)
			errors = append(errors, "Ошибка при создании автомобиля "+req.Number+" "+req.Mark)
			continue
		}

		createdCars = append(createdCars, *carToResponse(&car))
	}

	httpStatus := http.StatusOK
	if len(errors) > 0 {
		httpStatus = http.StatusMultiStatus
	}

	return &BatchCreateCarsResponse{
		CreatedCars:  createdCars,
		Errors:       errors,
		SuccessCount: len(createdCars),
		ErrorCount:   len(errors),
	}, httpStatus, nil
}

// Update обновляет уникальный автомобиль по ID с проверкой прав и уникальности.
func (s *uniqueCarService) Update(ctx context.Context, username string, id int, req NewUniqueCarRequest) (*UniqueCarResponse, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	// Полная запись «до апдейта» — нужна для проверки прав и аудита изменений.
	var existing models.UniqueCar
	if err := s.db.WithContext(ctx).First(&existing, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Car not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching car")
	}

	if !s.canEditCar(&existing, ownerInfo) {
		return nil, echo.NewHTTPError(http.StatusForbidden, "You don't have permission to edit this car")
	}

	// Проверка уникальности считается по владельцу ЗАПИСИ, а не по тому, кто правит:
	// администратор правит чужие машины, и его собственный список тут ни при чём.
	ownerUserID := ownerInfo.UserID
	if existing.UserID != nil {
		ownerUserID = *existing.UserID
	}

	// Проверка уникальности для владельца записи (исключая текущую)
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
		Where("user_id = ? AND number = ? AND mark = ? AND id != ?", ownerUserID, req.Number, req.Mark, id).
		Count(&count).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
	}
	if count > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Автомобиль с этим номером и маркой уже есть у владельца записи")
	}

	// Проверка уникальности для организации (исключая текущую)
	if req.OrganizationID != nil {
		var orgCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
			Where("organization_id = ? AND number = ? AND mark = ? AND id != ?", *req.OrganizationID, req.Number, req.Mark, id).
			Count(&orgCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
		}
		if orgCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Автомобиль с этим номером и маркой уже существует в этой организации")
		}
	}

	// Проверка уникальности для компании (исключая текущую)
	if req.CompanyID != nil {
		var compCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
			Where("company_id = ? AND number = ? AND mark = ? AND id != ?", *req.CompanyID, req.Number, req.Mark, id).
			Count(&compCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
		}
		if compCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Автомобиль с этим номером и маркой уже существует в этой компании")
		}
	}

	updates := map[string]interface{}{
		"number":          req.Number,
		"mark":            req.Mark,
		"organization_id": req.OrganizationID,
		"company_id":      req.CompanyID,
		"format_id":       req.FormatID,
	}
	// Владельца меняем только по явному указанию в запросе. Прежний код подставлял
	// сюда правящего пользователя, и любая правка чужой записи переводила её на себя;
	// у администратора, который правит машины всей системы, это переписало бы реестр.
	if req.UserID != nil {
		updates["user_id"] = *req.UserID
	}

	result := s.db.WithContext(ctx).Model(&models.UniqueCar{}).Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		slog.Error("не удалось обновить уникальный автомобиль", "id", id, "error", result.Error)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating car")
	}
	slog.Info("уникальный автомобиль обновлён", "id", id)

	var updated models.UniqueCar
	if err := s.db.WithContext(ctx).First(&updated, id).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching updated car")
	}

	if err := s.recordCarChanges(ctx, &existing, &updated, ownerInfo.UserID); err != nil {
		slog.Error("не удалось записать аудит изменений автомобиля", "id", id, "error", err)
	}

	return carToResponse(&updated), nil
}

// UpdateByNumber обновляет уникальный автомобиль по номеру и марке.
func (s *uniqueCarService) UpdateByNumber(ctx context.Context, username string, req UpdateCarByNumberRequest) (*UniqueCarResponse, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	var existing models.UniqueCar
	if err := s.db.WithContext(ctx).
		Where("number = ? AND mark = ?", req.Number, req.Mark).
		First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Car not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching car")
	}

	if !s.canEditCar(&existing, ownerInfo) {
		return nil, echo.NewHTTPError(http.StatusForbidden, "You don't have permission to edit this car")
	}

	updates := map[string]interface{}{
		"number":          req.UpdateData.Number,
		"mark":            req.UpdateData.Mark,
		"organization_id": req.UpdateData.OrganizationID,
		"company_id":      req.UpdateData.CompanyID,
		"format_id":       req.UpdateData.FormatID,
	}
	// Владелец сохраняется, если его не передали явно (см. Update).
	if req.UpdateData.UserID != nil {
		updates["user_id"] = *req.UpdateData.UserID
	}

	result := s.db.WithContext(ctx).Model(&models.UniqueCar{}).Where("id = ?", existing.ID).
		Updates(updates)
	if result.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating car")
	}

	var updated models.UniqueCar
	if err := s.db.WithContext(ctx).First(&updated, existing.ID).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching updated car")
	}

	if err := s.recordCarChanges(ctx, &existing, &updated, ownerInfo.UserID); err != nil {
		slog.Error("не удалось записать аудит изменений автомобиля", "id", existing.ID, "error", err)
	}

	return carToResponse(&updated), nil
}

// Delete удаляет уникальный автомобиль с проверкой прав.
func (s *uniqueCarService) Delete(ctx context.Context, username string, id int) error {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return err
	}

	// Номер и марку читаем ДО удаления: после него по id уже ничего не прочитать, а
	// вопрос «кем и когда запись удалена» задают именно про исчезнувшую. Зеркало
	// uniqueEmployeeService.Delete, там же объяснение общей транзакции.
	var existing models.UniqueCar
	if err := s.db.WithContext(ctx).Select("user_id, organization_id, company_id, number, mark").
		First(&existing, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Car not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching car")
	}

	if !s.canEditCar(&existing, ownerInfo) {
		return echo.NewHTTPError(http.StatusForbidden, "You don't have permission to delete this car")
	}

	plate := strings.TrimSpace(strings.Join(nonEmptyStrings(existing.Number, existing.Mark), " "))
	if plate == "" {
		plate = fmt.Sprintf("без номера (номер записи %d)", id)
	}
	comment := fmt.Sprintf("Автомобиль %s удалён из реестра", plate)
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Delete(&models.UniqueCar{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return s.recorder.Record(ctx, tx, models.AuditEntityUniqueCar, &id, "delete",
			&ownerInfo.UserID, carAuditDetails{Comment: &comment, Subject: &plate})
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Car not found")
		}
		slog.Error("не удалось удалить уникальный автомобиль", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting car")
	}

	slog.Info("уникальный автомобиль удалён", "id", id, "actor_user_id", ownerInfo.UserID)
	return nil
}

// recordCarChanges сравнивает старое и новое состояние UniqueCar и пишет в
// audit_log по одной записи data_changed на каждое изменённое поле (#870, срез
// 1.12d). Плоские field_name/old/new кладутся в details: намеренно переиспользуем
// carAuditDetails (та же плоская схема, что у car), но контракт здесь - именно
// JSON-ключи field_name/old_value/new_value; их извлекает union-SQL в GetHistory.
// Доп. поля carAuditDetails (table_id/metadata) для unique_car не пишутся и не
// читаются - при расширении типа реальный контракт остаётся в этих трёх ключах.
// Вызывается вне транзакции (caller логирует ошибку, апдейт уже завершён) - exec=nil.
func (s *uniqueCarService) recordCarChanges(ctx context.Context, before, after *models.UniqueCar, userID int) error {
	changes := diffUniqueCar(before, after)
	if len(changes) == 0 {
		return nil
	}

	uid := userID
	// Снимок номера с маркой в каждой записи, см. recordEmployeeChanges.
	subject := strings.TrimSpace(strings.Join(nonEmptyStrings(after.Number, after.Mark), " "))
	for _, c := range changes {
		field := c.Field
		details := carAuditDetails{FieldName: &field, OldValue: c.Old, NewValue: c.New}
		if subject != "" {
			details.Subject = &subject
		}
		if err := s.recorder.Record(ctx, nil, models.AuditEntityUniqueCar, &after.ID, "data_changed", &uid, details); err != nil {
			return fmt.Errorf("record unique_car change: %w", err)
		}
	}
	return nil
}

// GetHistory возвращает историю изменений мастер-записи машины.
// Доступ: у пользователя должны быть права редактирования (canEditCar) -
// иначе он не имеет права видеть аудит.
//
// Read-switch #870 (F.4): до-cutover строки unique_cars_history подняты в
// audit_log разовым backfill'ом (плоские field_name/old/new/comment свёрнуты в
// details jsonb в форме carAuditDetails), читаем только audit_log в прежнюю форму
// UniqueCarHistoryItem. Форму стережёт TestUniqueCarService_GetHistory_ReturnsRecords.
func (s *uniqueCarService) GetHistory(ctx context.Context, username string, id int) ([]UniqueCarHistoryItem, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	var existing models.UniqueCar
	if err := s.db.WithContext(ctx).Select("user_id, organization_id, company_id").
		First(&existing, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Car not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching car")
	}
	if !s.canEditCar(&existing, ownerInfo) {
		return nil, echo.NewHTTPError(http.StatusForbidden, "You don't have permission to view this car history")
	}

	const sql = `
		SELECT m.id, m.unique_car_id, m.user_id, u.username,
			u.last_name AS user_last_name, u.first_name AS user_first_name,
			m.action_type, m.field_name, m.old_value, m.new_value, m.comment, m.created_at
		FROM (
			SELECT a.id, a.entity_id AS unique_car_id, a.actor_user_id AS user_id,
				a.action AS action_type, a.details->>'field_name' AS field_name,
				a.details->>'old_value' AS old_value, a.details->>'new_value' AS new_value,
				a.details->>'comment' AS comment, a.created_at
			FROM audit_log a
			WHERE a.entity_type = ? AND a.entity_id = ?
		) m
		LEFT JOIN users u ON u.id = m.user_id
		ORDER BY m.created_at DESC, m.id DESC`

	items := make([]UniqueCarHistoryItem, 0)
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityUniqueCar, id).Scan(&items).Error; err != nil {
		slog.Error("failed to load unique_car history", "id", id, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching history")
	}
	return items, nil
}

// GetRegistryLog отдаёт журнал реестра машин целиком: создания, правки полей и удаления,
// с автором и временем. Зеркало uniqueEmployeeService.GetRegistryLog.
func (s *uniqueCarService) GetRegistryLog(ctx context.Context, username string, limit int) ([]UniqueCarHistoryItem, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}
	if !ownerInfo.CanManageAll {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Журнал реестра доступен администратору")
	}
	if limit <= 0 || limit > registryLogMaxRows {
		limit = registryLogMaxRows
	}

	const sql = `
		SELECT m.id, m.unique_car_id, m.user_id, u.username,
			u.last_name AS user_last_name, u.first_name AS user_first_name,
			m.action_type, m.field_name, m.old_value, m.new_value, m.comment, m.created_at,
			COALESCE(
				NULLIF(TRIM(m.subject), ''),
				NULLIF(TRIM(CONCAT_WS(' ', uc.number, uc.mark)), '')
			) AS subject
		FROM (
			SELECT a.id, a.entity_id AS unique_car_id, a.actor_user_id AS user_id,
				a.action AS action_type, a.details->>'field_name' AS field_name,
				a.details->>'old_value' AS old_value, a.details->>'new_value' AS new_value,
				a.details->>'comment' AS comment, a.details->>'subject' AS subject, a.created_at
			FROM audit_log a
			WHERE a.entity_type = ?
		) m
		LEFT JOIN users u ON u.id = m.user_id
		LEFT JOIN unique_cars uc ON uc.id = m.unique_car_id
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT ?`

	items := make([]UniqueCarHistoryItem, 0)
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityUniqueCar, limit).Scan(&items).Error; err != nil {
		slog.Error("не удалось загрузить журнал реестра машин", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching registry log")
	}
	return items, nil
}

// canEditCar проверяет права пользователя на редактирование машины.
func (s *uniqueCarService) canEditCar(car *models.UniqueCar, ownerInfo *CarOwnerInfo) bool {
	// Администратор системы правит и удаляет любую машину: бюро обязано чинить и
	// убирать записи контрагентов, к организации которых оно не относится.
	if ownerInfo.CanManageAll {
		return true
	}
	if car.UserID != nil && *car.UserID == ownerInfo.UserID {
		return true
	}
	if car.OrganizationID != nil && ownerInfo.OrganizationID != nil && *car.OrganizationID == *ownerInfo.OrganizationID {
		return true
	}
	if car.CompanyID != nil && ownerInfo.CompanyID != nil && *car.CompanyID == *ownerInfo.CompanyID {
		return true
	}
	return false
}
