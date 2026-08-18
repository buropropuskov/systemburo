package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// equalStrPtr возвращает true если оба указателя nil или ссылаются на равные строки.
func equalStrPtr(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// equalIntPtr возвращает true если оба указателя nil или ссылаются на равные int.
func equalIntPtr(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// copyStrPtr возвращает копию указателя на строку (не разделяет память).
func copyStrPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// timePtrToStrPtr форматирует *time.Time для записи в old_value/new_value аудита.
func timePtrToStrPtr(p *time.Time) *string {
	if p == nil {
		return nil
	}
	s := p.UTC().Format(time.RFC3339)
	return &s
}

// intPtrToStrPtr форматирует *int как *string для записи в old_value/new_value.
func intPtrToStrPtr(p *int) *string {
	if p == nil {
		return nil
	}
	s := strconv.Itoa(*p)
	return &s
}

// EmployeeOwnerInfo -- информация о владельце для фильтрации сотрудников.
type EmployeeOwnerInfo struct {
	HasOrganization  bool    `json:"has_organization"`
	HasCompany       bool    `json:"has_company"`
	OrganizationID   *int    `json:"organization_id"`
	CompanyID        *int    `json:"company_id"`
	UserID           int     `json:"user_id"`
	OrganizationName *string `json:"organization_name"`
	CompanyName      *string `json:"company_name"`
	// CanManageAll -- администратор системы: правит и удаляет любую запись реестра
	// независимо от привязки. Тот же признак фронт берёт для показа кнопок, см.
	// CarOwnerInfo.CanManageAll.
	CanManageAll bool `json:"can_manage_all"`
}

// UniqueEmployeeWithRelations -- сотрудник с данными связанных сущностей.
type UniqueEmployeeWithRelations struct {
	ID                   int        `json:"id"`
	LastName             *string    `json:"last_name"`
	FirstName            *string    `json:"first_name"`
	MiddleName           *string    `json:"middle_name"`
	OrganizationID       *int       `json:"organization_id"`
	CompanyID            *int       `json:"company_id"`
	CitizenshipID        *int       `json:"citizenship_id"`
	UserID               *int       `json:"user_id"`
	Position             *string    `json:"position"`
	PassportSeriesNumber *string    `json:"passport_series_number"`
	PatentNumber         *string    `json:"patent_number"`
	OtherPermission      *string    `json:"other_permission"`
	Status               bool       `json:"status"`
	CreatedAt            *time.Time `json:"created_at"`
	OrganizationName     *string    `json:"organization_name"`
	CompanyName          *string    `json:"company_name"`
	CitizenshipName      *string    `json:"citizenship_name"`
	// UserName -- за кем закреплена запись: ФИО владельца, а у не давшего согласия на
	// обработку своих данных - логин с собачкой (общая маска loadNameMasks). Логин сам
	// по себе человеку ничего не говорит, поэтому показываем имя. Отдаётся только
	// администратору (см. maskEmployeeOwners): для остальных привязка к чужой учётной
	// записи - лишние сведения о людях другой организации.
	UserName *string `json:"user_name"`
	// Поля владельца из выборки, из них собирается UserName; в ответ не уходят.
	OwnerUsername   *string `json:"-" gorm:"column:owner_username"`
	OwnerLastName   *string `json:"-" gorm:"column:owner_last_name"`
	OwnerFirstName  *string `json:"-" gorm:"column:owner_first_name"`
	OwnerMiddleName *string `json:"-" gorm:"column:owner_middle_name"`
	// PDConsentAt -- когда подтверждено согласие субъекта на обработку его данных.
	// NULL у записей, заведённых до введения поля: карточка так и пишет, что отметки нет.
	PDConsentAt          *time.Time `json:"pd_consent_at"`
	ActiveEntryDateTo    *string    `json:"active_entry_date_to"`
	ActivePassTime       *string    `json:"active_pass_time"`
	ActiveAppOrgName     *string    `json:"active_app_org_name"`
	ActiveAppCompanyName *string    `json:"active_app_company_name"`
	// ActiveEmployeeID -- id строки в employees активной заявки (заявочная таблица,
	// не реестр). Нужен фронту, чтобы подтянуть территориальный статус сотрудника
	// (current-status ключуется по employees.id, а не по unique_employees.id).
	ActiveEmployeeID *int `json:"active_employee_id"`
	// ActiveApplicationID -- id заявки (applications.id) той же активной заявки, что и
	// прочие active_*-поля. Нужен фронту для кнопки "Открыть заявку" на вкладке Сотрудники.
	ActiveApplicationID *int `json:"active_application_id"`
	// IsBlacklisted -- сотрудник в активном чёрном списке людей (совпадение по ФИО).
	// Считается на сервере, чтобы фронт не выгружал весь список ПД ради подсветки.
	IsBlacklisted bool `json:"is_blacklisted"`
}

// NewUniqueEmployeeRequest -- тело запроса на создание/обновление сотрудника.
type NewUniqueEmployeeRequest struct {
	LastName             *string `json:"last_name"`
	FirstName            *string `json:"first_name"`
	MiddleName           *string `json:"middle_name"`
	CitizenshipID        *int    `json:"citizenship_id"`
	Position             *string `json:"position"`
	PassportSeriesNumber *string `json:"passport_series_number"`
	PatentNumber         *string `json:"patent_number"`
	OtherPermission      *string `json:"other_permission"`
	OrganizationID       *int    `json:"organization_id"`
	CompanyID            *int    `json:"company_id"`
	UserID               *int    `json:"user_id"`
	// PDConsent - заявитель подтвердил, что субъект дал согласие на обработку своих
	// персональных данных (152-ФЗ). Для новой записи обязателен: в карточке вводят
	// паспорт и патент, то есть данные третьего лица. При правке существующей записи
	// флаг только ДОБАВЛЯЕТ отметку, если её не было, - снять согласие правкой нельзя.
	PDConsent bool `json:"pd_consent"`
}

// UniqueEmployeeResponse -- ответ при создании/обновлении сотрудника.
type UniqueEmployeeResponse struct {
	ID                   int        `json:"id"`
	LastName             *string    `json:"last_name"`
	FirstName            *string    `json:"first_name"`
	MiddleName           *string    `json:"middle_name"`
	CitizenshipID        *int       `json:"citizenship_id"`
	Position             *string    `json:"position"`
	PassportSeriesNumber *string    `json:"passport_series_number"`
	PatentNumber         *string    `json:"patent_number"`
	OtherPermission      *string    `json:"other_permission"`
	OrganizationID       *int       `json:"organization_id"`
	CompanyID            *int       `json:"company_id"`
	UserID               *int       `json:"user_id"`
	Status               bool       `json:"status"`
	CreatedAt            *time.Time `json:"created_at"`
	PDConsentAt          *time.Time `json:"pd_consent_at"`
}

// UniqueEmployeeHistoryItem -- запись истории мастер-сотрудника с username вызывающего.
type UniqueEmployeeHistoryItem struct {
	ID int `json:"id"`
	// Subject -- к кому относится событие (ФИО на момент действия). Пусто у событий,
	// записанных до введения снимка, если сама запись уже удалена.
	Subject          *string   `json:"subject"`
	UniqueEmployeeID int       `json:"unique_employee_id"`
	UserID           *int      `json:"user_id"`
	Username         *string   `json:"username"`
	UserLastName     *string   `json:"user_last_name"`
	UserFirstName    *string   `json:"user_first_name"`
	ActionType       string    `json:"action_type"`
	FieldName        *string   `json:"field_name"`
	OldValue         *string   `json:"old_value"`
	NewValue         *string   `json:"new_value"`
	Comment          *string   `json:"comment"`
	CreatedAt        time.Time `json:"created_at"`
}

// UniqueEmployeeService -- интерфейс бизнес-логики уникальных сотрудников.
type UniqueEmployeeService interface {
	GetOwnerInfo(ctx context.Context, username string) (*EmployeeOwnerInfo, error)
	GetAll(ctx context.Context, username string, filterType string) ([]UniqueEmployeeWithRelations, error)
	// GetAllPaginated возвращает страницу реестра с серверным поиском (#1158, срез 3):
	// используется EmployeeView вместо GetAll, как только запрос несёт per_page.
	GetAllPaginated(ctx context.Context, username, filterType, searchQuery string, page, perPage int) ([]UniqueEmployeeWithRelations, int64, error)
	// LookupByFIO ищет сотрудника по ФИО (LOWER(TRIM), как ЧС) для открытия карточки со
	// страницы чёрного списка. Возвращает nil, nil если совпадения нет.
	LookupByFIO(ctx context.Context, lastName, firstName, middleName string) (*UniqueEmployeeWithRelations, error)
	Create(ctx context.Context, username string, req NewUniqueEmployeeRequest) (*UniqueEmployeeResponse, error)
	Update(ctx context.Context, username string, id int, req NewUniqueEmployeeRequest) (*UniqueEmployeeResponse, error)
	Delete(ctx context.Context, username string, id int) error
	GetHistory(ctx context.Context, username string, id int) ([]UniqueEmployeeHistoryItem, error)
	// GetRegistryLog возвращает журнал по всему реестру, включая удалённые записи:
	// у исчезнувшей строки истории по id не открыть, а вопрос «кем и когда удалена»
	// задают именно про неё. Доступен администратору.
	GetRegistryLog(ctx context.Context, username string, limit int) ([]UniqueEmployeeHistoryItem, error)
}

type uniqueEmployeeService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewUniqueEmployeeService создаёт реализацию UniqueEmployeeService.
func NewUniqueEmployeeService(db *gorm.DB) UniqueEmployeeService {
	return &uniqueEmployeeService{db: db, recorder: NewAuditRecorder(db)}
}

// getEmployeeOwnerInfo получает информацию о владельце по username.
func (s *uniqueEmployeeService) getEmployeeOwnerInfo(ctx context.Context, username string) (*EmployeeOwnerInfo, error) {
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

	return &EmployeeOwnerInfo{
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

// GetOwnerInfo возвращает информацию о владельце для фильтрации сотрудников.
func (s *uniqueEmployeeService) GetOwnerInfo(ctx context.Context, username string) (*EmployeeOwnerInfo, error) {
	return s.getEmployeeOwnerInfo(ctx, username)
}

// LookupByFIO ищет сотрудника по ФИО без скоупинга по владельцу (вызывается из админ-
// страницы ЧС). Самый свежий при нескольких совпадениях. nil,nil если нет.
func (s *uniqueEmployeeService) LookupByFIO(ctx context.Context, lastName, firstName, middleName string) (*UniqueEmployeeWithRelations, error) {
	rows := make([]UniqueEmployeeWithRelations, 0, 1)
	err := s.db.WithContext(ctx).
		Table("unique_employees ue").
		Select(`ue.id, ue.last_name, ue.first_name, ue.middle_name,
			ue.organization_id, ue.company_id, ue.citizenship_id, ue.user_id,
			ue."position", ue.passport_series_number, ue.patent_number, ue.other_permission, ue.created_at,
			o.name as organization_name, c.name as company_name, cit.name as citizenship_name`).
		Joins("LEFT JOIN organizations o ON ue.organization_id = o.id").
		Joins("LEFT JOIN companies c ON ue.company_id = c.id").
		Joins("LEFT JOIN citizenships cit ON ue.citizenship_id = cit.id").
		Joins("LEFT JOIN users usr ON ue.user_id = usr.id").
		Where("LOWER(TRIM(ue.last_name)) = LOWER(TRIM(?))", lastName).
		Where("LOWER(TRIM(ue.first_name)) = LOWER(TRIM(?))", firstName).
		Where("LOWER(TRIM(COALESCE(ue.middle_name, ''))) = LOWER(TRIM(?))", middleName).
		Order("ue.id DESC").
		Limit(1).
		Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка поиска сотрудника")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	// Паспорт/патент хранятся зашифрованными - расшифровываем, как в GetAll.
	rows[0].PassportSeriesNumber = crypto.DecryptOptional(rows[0].PassportSeriesNumber)
	rows[0].PatentNumber = crypto.DecryptOptional(rows[0].PatentNumber)
	return &rows[0], nil
}

// employeesListSelect -- список колонок для реестра сотрудников (GetAll/GetAllPaginated).
// Вынесен в константу, т.к. переиспользуется обоими методами (#1158, срез 3) - раньше
// был только внутри GetAll. Активная заявка ищется по passport_series_number_hmac (тот
// же ключ, что связывает реестр с заявочными employees), как исторически было.
const employeesListSelect = `ue.id, ue.last_name, ue.first_name, ue.middle_name,
	ue.organization_id, ue.company_id, ue.citizenship_id, ue.user_id,
	ue."position", ue.passport_series_number, ue.patent_number,
	ue.other_permission, ue.created_at,
	o.name as organization_name, c.name as company_name,
	cit.name as citizenship_name, ue.pd_consent_at,
	usr.username as owner_username, usr.last_name as owner_last_name,
	usr.first_name as owner_first_name, usr.middle_name as owner_middle_name,
	COALESCE((
		SELECT true FROM employees e
		JOIN attachments a ON e.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		WHERE e.passport_series_number_hmac = ue.passport_series_number_hmac
		AND e.status = 1
		AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		LIMIT 1
	), false) as status,
	(SELECT a.entry_date_to FROM employees e
		JOIN attachments a ON e.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		WHERE e.passport_series_number_hmac = ue.passport_series_number_hmac
		AND e.status = 1 AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		ORDER BY a.entry_date_to DESC LIMIT 1
	) as active_entry_date_to,
	(SELECT CONCAT(a.entry_time_from, ' - ', a.entry_time_to) FROM employees e
		JOIN attachments a ON e.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		WHERE e.passport_series_number_hmac = ue.passport_series_number_hmac
		AND e.status = 1 AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		ORDER BY a.entry_date_to DESC LIMIT 1
	) as active_pass_time,
	(SELECT ao.name FROM employees e
		JOIN attachments a ON e.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		LEFT JOIN organizations ao ON app.organization_id = ao.id
		WHERE e.passport_series_number_hmac = ue.passport_series_number_hmac
		AND e.status = 1 AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		ORDER BY a.entry_date_to DESC LIMIT 1
	) as active_app_org_name,
	(SELECT ac.name FROM employees e
		JOIN attachments a ON e.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		LEFT JOIN companies ac ON app.company_id = ac.id
		WHERE e.passport_series_number_hmac = ue.passport_series_number_hmac
		AND e.status = 1 AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		ORDER BY a.entry_date_to DESC LIMIT 1
	) as active_app_company_name,
	(SELECT e.id FROM employees e
		JOIN attachments a ON e.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		WHERE e.passport_series_number_hmac = ue.passport_series_number_hmac
		AND e.status = 1 AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		ORDER BY a.entry_date_to DESC LIMIT 1
	) as active_employee_id,
	(SELECT app.id FROM employees e
		JOIN attachments a ON e.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		WHERE e.passport_series_number_hmac = ue.passport_series_number_hmac
		AND e.status = 1 AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE <= a.entry_date_to::date
		ORDER BY a.entry_date_to DESC LIMIT 1
	) as active_application_id,
	-- Флаг ЧС считает сервер (нормализация 1:1 с personBlacklistService.Check), чтобы
	-- клиенту не отдавать весь список ПД ради подсветки. ФИО в обеих таблицах - открытый
	-- текст (шифруются только паспорт/патент), поэтому сравнение идёт прямо в SQL.
	EXISTS(
		SELECT 1 FROM person_blacklists pbl
		WHERE pbl.is_active
		AND LOWER(TRIM(pbl.last_name)) = LOWER(TRIM(ue.last_name))
		AND LOWER(TRIM(pbl.first_name)) = LOWER(TRIM(ue.first_name))
		AND LOWER(TRIM(COALESCE(pbl.middle_name, ''))) = LOWER(TRIM(COALESCE(ue.middle_name, '')))
	) as is_blacklisted`

// buildEmployeesQuery строит базовый запрос реестра (джойны + фильтр владельца + поиск)
// БЕЗ Select/Order - переиспользуется отдельно для Count и для выборки данных (тот же
// паттерн, что buildCarsQuery в unique_car_service.go), чтобы Count считал по
// фильтрованному набору, не гоняя тяжёлые коррелированные подзапросы employeesListSelect
// дважды.
func (s *uniqueEmployeeService) buildEmployeesQuery(ctx context.Context, ownerInfo *EmployeeOwnerInfo, filterType, searchQuery string) *gorm.DB {
	query := s.db.WithContext(ctx).
		Table("unique_employees ue").
		Joins("LEFT JOIN organizations o ON ue.organization_id = o.id").
		Joins("LEFT JOIN companies c ON ue.company_id = c.id").
		Joins("LEFT JOIN citizenships cit ON ue.citizenship_id = cit.id").
		Joins("LEFT JOIN users usr ON ue.user_id = usr.id")

	switch filterType {
	case "organization":
		if ownerInfo.HasOrganization {
			orgID := 0
			if ownerInfo.OrganizationID != nil {
				orgID = *ownerInfo.OrganizationID
			}
			query = query.Where("ue.organization_id = ?", orgID)
		} else {
			query = query.Where("ue.user_id = ?", ownerInfo.UserID)
		}
	case "company":
		if ownerInfo.HasCompany {
			compID := 0
			if ownerInfo.CompanyID != nil {
				compID = *ownerInfo.CompanyID
			}
			query = query.Where("ue.company_id = ?", compID)
		} else {
			query = query.Where("ue.user_id = ?", ownerInfo.UserID)
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
		query = query.Where("ue.user_id = ? OR ue.organization_id = ? OR ue.company_id = ?",
			ownerInfo.UserID, orgID, compID)
	case "all_system":
		// Без фильтрации — все сотрудники системы.
	default:
		query = query.Where("ue.user_id = ?", ownerInfo.UserID)
	}

	if raw := strings.TrimSpace(searchQuery); raw != "" {
		// Паспорт/патент (passport_series_number/patent_number) зашифрованы (#1049 HMAC-
		// уроки) - ILIKE по ним не сработает (в БД шифротекст), поэтому ищем ТОЛЬКО по
		// незашифрованным полям: ФИО/должность + имена связанных организации/компании/
		// гражданства (через LEFT JOIN o/c/cit выше). ФИО дополнительно ищем через
		// strict_word_similarity - тот же приём, что Центр заявок использует для поиска
		// сотрудников в application_helpers.go (strict_, не word_, иначе порог 0.3 ловит
		// общие триграммы и даёт ложные совпадения).
		variants := buildSearchVariants(raw)
		cols := []string{"ue.last_name", "ue.first_name", "ue.middle_name", "ue.\"position\"", "o.name", "c.name", "cit.name"}
		cond, args := ilikePatternsArgs(cols, variants)
		// Форма с функцией по concat_ws индексом не покрывается (выражение не IMMUTABLE)
		// и даёт полный просмотр таблицы. Индексируемый вариант -- оператор по отдельным
		// колонкам: ue.last_name %>> ? и т.д. с порогом через SET LOCAL, так сделан
		// сквозной поиск (search_scope.go). Здесь оставлено как есть: замена меняет
		// разбор многословных запросов, а листинг открывают по кнопке, не на каждый
		// введённый символ.
		cond += " OR strict_word_similarity(?, concat_ws(' ', ue.last_name, ue.first_name, ue.middle_name)) > 0.3"
		args = append(args, raw)
		query = query.Where(cond, args...)
	}

	return query
}

// maskEmployeeOwners заполняет «за кем закреплена запись» для администратора и убирает
// это поле у остальных: привязка записи к учётной записи - служебные данные бюро, соседям
// по организации она ни к чему. Поле только читается (в запросах на запись его нет),
// поэтому скрытие не может обернуться стиранием при обратной отправке формы.
//
// Показываем ФИО владельца, а не логин: логин человеку ничего не говорит. У работника,
// не давшего согласия на обработку своих данных, ФИО скрыто по общему правилу (#1567) -
// тогда в строке стоит его логин с собачкой, тот же, что интерфейс показывает всюду.
func maskEmployeeOwners(ctx context.Context, db *gorm.DB, rows []UniqueEmployeeWithRelations, canManageAll bool) {
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

// GetAll возвращает список уникальных сотрудников с фильтрацией по типу владельца.
func (s *uniqueEmployeeService) GetAll(ctx context.Context, username string, filterType string) ([]UniqueEmployeeWithRelations, error) {
	ownerInfo, err := s.getEmployeeOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	if filterType == "all_system" && !ownerInfo.CanManageAll {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав для просмотра всех записей системы")
	}

	query := s.buildEmployeesQuery(ctx, ownerInfo, filterType, "").
		Select(employeesListSelect).
		// ue.id третий/четвёртый ключ - ФИО не уникально (нет unique-индекса), без
		// tie-breaker две равные строки могут переупорядочиться между offset-страницами
		// -> пропуск/дубль при бесшовной подгрузке (#1158).
		Order("ue.last_name, ue.first_name, ue.middle_name, ue.id")

	employees := make([]UniqueEmployeeWithRelations, 0)
	if err := query.Scan(&employees).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employees")
	}
	decryptEmployees(employees)
	maskEmployeeOwners(ctx, s.db, employees, ownerInfo.CanManageAll)

	return employees, nil
}

// GetAllPaginated возвращает страницу реестра с серверным поиском (#1158, срез 3).
func (s *uniqueEmployeeService) GetAllPaginated(ctx context.Context, username, filterType, searchQuery string, page, perPage int) ([]UniqueEmployeeWithRelations, int64, error) {
	ownerInfo, err := s.getEmployeeOwnerInfo(ctx, username)
	if err != nil {
		return nil, 0, err
	}

	if filterType == "all_system" && !ownerInfo.CanManageAll {
		return nil, 0, echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав для просмотра всех записей системы")
	}

	var total int64
	countQuery := s.buildEmployeesQuery(ctx, ownerInfo, filterType, searchQuery)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error counting employees")
	}

	offset := (page - 1) * perPage
	dataQuery := s.buildEmployeesQuery(ctx, ownerInfo, filterType, searchQuery).
		Select(employeesListSelect).
		Order("ue.last_name, ue.first_name, ue.middle_name, ue.id").
		Offset(offset).
		Limit(perPage)

	employees := make([]UniqueEmployeeWithRelations, 0)
	if err := dataQuery.Scan(&employees).Error; err != nil {
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employees")
	}
	decryptEmployees(employees)
	maskEmployeeOwners(ctx, s.db, employees, ownerInfo.CanManageAll)

	return employees, total, nil
}

// decryptEmployees расшифровывает паспорт/патент строк реестра на месте (общий шаг
// GetAll/GetAllPaginated).
func decryptEmployees(employees []UniqueEmployeeWithRelations) {
	for i := range employees {
		employees[i].PassportSeriesNumber = crypto.DecryptOptional(employees[i].PassportSeriesNumber)
		employees[i].PatentNumber = crypto.DecryptOptional(employees[i].PatentNumber)
	}
}

// employeeToResponse конвертирует модель UniqueEmployee в UniqueEmployeeResponse.
func employeeToResponse(emp *models.UniqueEmployee) *UniqueEmployeeResponse {
	status := false
	if emp.Status != nil && *emp.Status {
		status = true
	}
	return &UniqueEmployeeResponse{
		ID:                   emp.ID,
		LastName:             emp.LastName,
		FirstName:            emp.FirstName,
		MiddleName:           emp.MiddleName,
		CitizenshipID:        emp.CitizenshipID,
		Position:             emp.Position,
		PassportSeriesNumber: emp.PassportSeriesNumber,
		PatentNumber:         emp.PatentNumber,
		OtherPermission:      emp.OtherPermission,
		OrganizationID:       emp.OrganizationID,
		CompanyID:            emp.CompanyID,
		UserID:               emp.UserID,
		Status:               status,
		CreatedAt:            &emp.CreatedAt,
		PDConsentAt:          emp.PDConsentAt,
	}
}

// Create создаёт уникального сотрудника с проверкой уникальности паспортных данных.
func (s *uniqueEmployeeService) Create(ctx context.Context, username string, req NewUniqueEmployeeRequest) (*UniqueEmployeeResponse, error) {
	ownerInfo, err := s.getEmployeeOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	// Согласие субъекта - условие приёма его данных, а не настройка. Гейт стоит до
	// проверок уникальности: смысла искать дубликат паспорта, который нам нельзя
	// обрабатывать, нет.
	if !req.PDConsent {
		return nil, echo.NewHTTPError(http.StatusBadRequest,
			"Без согласия субъекта на обработку персональных данных запись создать нельзя")
	}

	// Проверка уникальности паспортных данных для пользователя
	if req.PassportSeriesNumber != nil {
		var count int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).
			Where("user_id = ? AND passport_series_number_hmac = ?", ownerInfo.UserID, crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey())).
			Count(&count).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking employee uniqueness")
		}
		if count > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Сотрудник с такими паспортными данными уже привязан к вашему аккаунту")
		}
	}

	// Проверка уникальности для организации
	if req.OrganizationID != nil && req.PassportSeriesNumber != nil {
		var orgCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).
			Where("organization_id = ? AND passport_series_number_hmac = ?", *req.OrganizationID, crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey())).
			Count(&orgCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking employee uniqueness")
		}
		if orgCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Сотрудник с такими паспортными данными уже существует в этой организации")
		}
	}

	// Проверка уникальности для компании
	if req.CompanyID != nil && req.PassportSeriesNumber != nil {
		var compCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).
			Where("company_id = ? AND passport_series_number_hmac = ?", *req.CompanyID, crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey())).
			Count(&compCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking employee uniqueness")
		}
		if compCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Сотрудник с такими паспортными данными уже существует в этой компании")
		}
	}

	userID := ownerInfo.UserID
	if req.UserID != nil {
		userID = *req.UserID
	}

	statusFalse := false
	consentGrantedAt := time.Now().UTC()
	employee := models.UniqueEmployee{
		PDConsentAt:          &consentGrantedAt,
		PDConsentByUserID:    &ownerInfo.UserID,
		LastName:             req.LastName,
		FirstName:            req.FirstName,
		MiddleName:           req.MiddleName,
		CitizenshipID:        req.CitizenshipID,
		Position:             req.Position,
		PassportSeriesNumber: req.PassportSeriesNumber,
		PatentNumber:         req.PatentNumber,
		OtherPermission:      req.OtherPermission,
		OrganizationID:       req.OrganizationID,
		CompanyID:            req.CompanyID,
		UserID:               &userID,
		Status:               &statusFalse,
	}

	if err := s.db.WithContext(ctx).Create(&employee).Error; err != nil {
		slog.Error("не удалось создать уникального сотрудника", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка при создании сотрудника")
	}

	// Запись о заведении - для журнала реестра: правки и удаления в нём есть, и без
	// создания история записи начиналась бы с середины. Ошибка аудита саму запись не
	// откатывает, она уже создана - логируем и идём дальше.
	created := strings.TrimSpace(strings.Join(nonEmptyStrings(employee.LastName, employee.FirstName, employee.MiddleName), " "))
	if created == "" {
		created = fmt.Sprintf("без имени (номер записи %d)", employee.ID)
	}
	createComment := fmt.Sprintf("Сотрудник %s заведён в реестр", created)
	if err := s.recorder.Record(ctx, nil, models.AuditEntityUniqueEmployee, &employee.ID, "create",
		&ownerInfo.UserID, carAuditDetails{Comment: &createComment, Subject: &created}); err != nil {
		slog.Error("не удалось записать создание сотрудника в журнал", "id", employee.ID, "error", err)
	}

	slog.Info("уникальный сотрудник создан", "id", employee.ID)
	return employeeToResponse(&employee), nil
}

// Update обновляет уникального сотрудника по ID с проверкой прав и уникальности.
func (s *uniqueEmployeeService) Update(ctx context.Context, username string, id int, req NewUniqueEmployeeRequest) (*UniqueEmployeeResponse, error) {
	ownerInfo, err := s.getEmployeeOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	// Полная запись «до апдейта» нужна для аудита изменений (data_changed).
	// Проверка прав делается по тем же полям, поэтому отдельный SELECT не нужен.
	var existing models.UniqueEmployee
	if err := s.db.WithContext(ctx).First(&existing, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Employee not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employee")
	}

	if !s.canEditEmployee(&existing, ownerInfo) {
		return nil, echo.NewHTTPError(http.StatusForbidden, "You don't have permission to edit this employee")
	}

	// Уникальность считается по владельцу ЗАПИСИ, а не по тому, кто правит:
	// администратор правит чужих сотрудников, и его собственный список тут ни при чём.
	ownerUserID := ownerInfo.UserID
	if existing.UserID != nil {
		ownerUserID = *existing.UserID
	}

	// Проверка уникальности паспортных данных у владельца записи (исключая текущего)
	if req.PassportSeriesNumber != nil {
		var count int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).
			Where("user_id = ? AND passport_series_number_hmac = ? AND id != ?", ownerUserID, crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey()), id).
			Count(&count).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking employee uniqueness")
		}
		if count > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Сотрудник с такими паспортными данными уже есть у владельца записи")
		}
	}

	// Проверка уникальности для организации (исключая текущего)
	if req.OrganizationID != nil && req.PassportSeriesNumber != nil {
		var orgCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).
			Where("organization_id = ? AND passport_series_number_hmac = ? AND id != ?", *req.OrganizationID, crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey()), id).
			Count(&orgCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking employee uniqueness")
		}
		if orgCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Сотрудник с такими паспортными данными уже существует в этой организации")
		}
	}

	// Проверка уникальности для компании (исключая текущего)
	if req.CompanyID != nil && req.PassportSeriesNumber != nil {
		var compCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).
			Where("company_id = ? AND passport_series_number_hmac = ? AND id != ?", *req.CompanyID, crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey()), id).
			Count(&compCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking employee uniqueness")
		}
		if compCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Сотрудник с такими паспортными данными уже существует в этой компании")
		}
	}

	updates := map[string]interface{}{
		"last_name":        req.LastName,
		"first_name":       req.FirstName,
		"middle_name":      req.MiddleName,
		"citizenship_id":   req.CitizenshipID,
		"position":         req.Position,
		"other_permission": req.OtherPermission,
		"organization_id":  req.OrganizationID,
		"company_id":       req.CompanyID,
	}
	// Владельца меняем только по явному указанию в запросе. Прежний код подставлял
	// сюда правящего пользователя, и правка чужой записи переводила её на себя;
	// у администратора, который правит сотрудников всей системы, это переписало бы
	// привязки реестра.
	if req.UserID != nil {
		updates["user_id"] = *req.UserID
	}
	// Согласие правкой только ДОБАВЛЯЕТСЯ: у записи, заведённой до введения поля,
	// отметку можно поставить, а снять её снятой галочкой нельзя - иначе полученное
	// согласие исчезало бы из базы вместе с исправлением опечатки в фамилии.
	if req.PDConsent && existing.PDConsentAt == nil {
		updates["pd_consent_at"] = time.Now().UTC()
		updates["pd_consent_by_user_id"] = ownerInfo.UserID
	}
	if req.PassportSeriesNumber != nil {
		enc, err := crypto.Encrypt(*req.PassportSeriesNumber, crypto.GetGlobalKey())
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "encryption error")
		}
		updates["passport_series_number"] = enc
		updates["passport_series_number_hmac"] = crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey())
	}
	if req.PatentNumber != nil {
		enc, err := crypto.Encrypt(*req.PatentNumber, crypto.GetGlobalKey())
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "encryption error")
		}
		updates["patent_number"] = enc
		updates["patent_number_hmac"] = crypto.ComputeHMAC(*req.PatentNumber, crypto.GetGlobalKey())
	}
	result := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		slog.Error("не удалось обновить уникального сотрудника", "id", id, "error", result.Error)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating employee")
	}
	slog.Info("уникальный сотрудник обновлён", "id", id)

	var updated models.UniqueEmployee
	if err := s.db.WithContext(ctx).First(&updated, id).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching updated employee")
	}

	// Аудит изменений: пишем по одной записи в audit_log[unique_employee]
	// на каждое изменённое поле. Ошибка аудита не должна откатывать апдейт,
	// поэтому только логируем — апдейт уже зафиксирован.
	if err := s.recordEmployeeChanges(ctx, &existing, &updated, ownerInfo.UserID); err != nil {
		slog.Error("не удалось записать аудит изменений сотрудника", "id", id, "error", err)
	}

	return employeeToResponse(&updated), nil
}

// recordEmployeeChanges сравнивает старое и новое состояние UniqueEmployee
// и пишет в audit_log по одной записи data_changed на каждое изменённое поле
// (#870, срез 1.13c). Поля HMAC и служебные поля игнорируются — они вычисляются
// автоматически. Плоские field_name/old/new кладутся в details: переиспользуем
// carAuditDetails (та же плоская схема, что у car/unique_car), но контракт здесь -
// именно JSON-ключи field_name/old_value/new_value; их извлекает union-SQL в
// GetHistory. Вызывается вне транзакции (caller логирует ошибку, апдейт уже
// завершён) - exec=nil.
func (s *uniqueEmployeeService) recordEmployeeChanges(ctx context.Context, before, after *models.UniqueEmployee, userID int) error {
	changes := diffUniqueEmployee(before, after)
	if len(changes) == 0 {
		return nil
	}

	uid := userID
	// Снимок ФИО в каждой записи: журнал реестра показывает, К КОМУ относится событие, а
	// после удаления записи по её номеру этого уже не узнать.
	subject := strings.TrimSpace(strings.Join(nonEmptyStrings(after.LastName, after.FirstName, after.MiddleName), " "))
	for _, c := range changes {
		field := c.Field
		details := carAuditDetails{FieldName: &field, OldValue: c.Old, NewValue: c.New}
		if subject != "" {
			details.Subject = &subject
		}
		if err := s.recorder.Record(ctx, nil, models.AuditEntityUniqueEmployee, &after.ID, "data_changed", &uid, details); err != nil {
			return fmt.Errorf("record unique_employee change: %w", err)
		}
	}
	return nil
}

// fieldChange описывает одно изменение поля для аудита.
type fieldChange struct {
	Field string
	Old   *string
	New   *string
}

// diffUniqueEmployee сравнивает значимые поля UniqueEmployee.
// Возвращает только реально изменившиеся поля.
func diffUniqueEmployee(before, after *models.UniqueEmployee) []fieldChange {
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
	addStr("last_name", before.LastName, after.LastName)
	addStr("first_name", before.FirstName, after.FirstName)
	addStr("middle_name", before.MiddleName, after.MiddleName)
	addInt("citizenship_id", before.CitizenshipID, after.CitizenshipID)
	addStr("position", before.Position, after.Position)
	addStr("passport_series_number", before.PassportSeriesNumber, after.PassportSeriesNumber)
	addStr("patent_number", before.PatentNumber, after.PatentNumber)
	addStr("other_permission", before.OtherPermission, after.OtherPermission)
	addInt("organization_id", before.OrganizationID, after.OrganizationID)
	addInt("company_id", before.CompanyID, after.CompanyID)
	addInt("user_id", before.UserID, after.UserID)
	// Появление отметки о согласии - событие для истории: по ней потом отвечают, на
	// каком основании обрабатывались данные человека.
	addStr("pd_consent_at", timePtrToStrPtr(before.PDConsentAt), timePtrToStrPtr(after.PDConsentAt))
	return changes
}

// Delete удаляет уникального сотрудника с проверкой прав.
func (s *uniqueEmployeeService) Delete(ctx context.Context, username string, id int) error {
	ownerInfo, err := s.getEmployeeOwnerInfo(ctx, username)
	if err != nil {
		return err
	}

	// ФИО читаем ДО удаления: после него по id уже ничего не прочитать, а вопрос «кем и
	// когда запись удалена» задают именно про исчезнувшую.
	var existing models.UniqueEmployee
	if err := s.db.WithContext(ctx).
		Select("user_id, organization_id, company_id, last_name, first_name, middle_name").
		First(&existing, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Employee not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employee")
	}

	if !s.canEditEmployee(&existing, ownerInfo) {
		return echo.NewHTTPError(http.StatusForbidden, "You don't have permission to delete this employee")
	}

	// Удаление и запись в журнал - одной транзакцией: запись, исчезнувшая без следа,
	// оставляет вопрос «кто её убрал» без ответа навсегда, а запись в журнале о живой
	// строке - врёт. Либо оба факта, либо ни один.
	fio := strings.TrimSpace(strings.Join(nonEmptyStrings(existing.LastName, existing.FirstName, existing.MiddleName), " "))
	if fio == "" {
		fio = fmt.Sprintf("без имени (номер записи %d)", id)
	}
	comment := fmt.Sprintf("Сотрудник %s удалён из реестра", fio)
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Delete(&models.UniqueEmployee{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return s.recorder.Record(ctx, tx, models.AuditEntityUniqueEmployee, &id, "delete",
			&ownerInfo.UserID, carAuditDetails{Comment: &comment, Subject: &fio})
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Employee not found")
		}
		slog.Error("не удалось удалить уникального сотрудника", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting employee")
	}

	slog.Info("уникальный сотрудник удалён", "id", id, "actor_user_id", ownerInfo.UserID)
	return nil
}

// nonEmptyStrings отбирает непустые значения указателей - для сборки ФИО из частей.
func nonEmptyStrings(parts ...*string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != nil && strings.TrimSpace(*p) != "" {
			out = append(out, strings.TrimSpace(*p))
		}
	}
	return out
}

// GetHistory возвращает историю изменений мастер-записи сотрудника.
// Доступ: у пользователя должны быть права редактирования (canEditEmployee) -
// иначе он не имеет права видеть аудит.
//
// Read-switch #870 (F.4): до-cutover строки unique_employees_history подняты в
// audit_log разовым backfill'ом (плоские field_name/old/new/comment свёрнуты в
// details jsonb в форме carAuditDetails), читаем только audit_log в прежнюю форму
// UniqueEmployeeHistoryItem. Форму стережёт TestUniqueEmployeeService_GetHistory_ReturnsRecords.
func (s *uniqueEmployeeService) GetHistory(ctx context.Context, username string, id int) ([]UniqueEmployeeHistoryItem, error) {
	ownerInfo, err := s.getEmployeeOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	var existing models.UniqueEmployee
	if err := s.db.WithContext(ctx).Select("user_id, organization_id, company_id").
		First(&existing, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Employee not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employee")
	}
	if !s.canEditEmployee(&existing, ownerInfo) {
		return nil, echo.NewHTTPError(http.StatusForbidden, "You don't have permission to view this employee history")
	}

	const sql = `
		SELECT m.id, m.unique_employee_id, m.user_id, u.username,
			u.last_name AS user_last_name, u.first_name AS user_first_name,
			m.action_type, m.field_name, m.old_value, m.new_value, m.comment, m.created_at
		FROM (
			SELECT a.id, a.entity_id AS unique_employee_id, a.actor_user_id AS user_id,
				a.action AS action_type, a.details->>'field_name' AS field_name,
				a.details->>'old_value' AS old_value, a.details->>'new_value' AS new_value,
				a.details->>'comment' AS comment, a.created_at
			FROM audit_log a
			WHERE a.entity_type = ? AND a.entity_id = ?
		) m
		LEFT JOIN users u ON u.id = m.user_id
		ORDER BY m.created_at DESC, m.id DESC`

	items := make([]UniqueEmployeeHistoryItem, 0)
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityUniqueEmployee, id).Scan(&items).Error; err != nil {
		slog.Error("failed to load unique_employee history", "id", id, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching history")
	}
	return items, nil
}

// GetRegistryLog отдаёт журнал реестра сотрудников целиком: создания, правки полей и
// удаления, с автором и временем. Единственный способ узнать, кем и когда убрана
// запись, - у удалённой строки карточки и истории по id больше нет.
//
// Доступ - администратору (тот же признак, что открывает правку чужих записей): журнал
// говорит, кто из работников что делал, и это служебные сведения бюро.
func (s *uniqueEmployeeService) GetRegistryLog(ctx context.Context, username string, limit int) ([]UniqueEmployeeHistoryItem, error) {
	ownerInfo, err := s.getEmployeeOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}
	if !ownerInfo.CanManageAll {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Журнал реестра доступен администратору")
	}
	if limit <= 0 || limit > registryLogMaxRows {
		limit = registryLogMaxRows
	}

	// subject: снимок из записи журнала, а при его отсутствии (события до введения
	// снимка) - ФИО живой записи реестра. У удалённой записи остаётся только снимок.
	const sql = `
		SELECT m.id, m.unique_employee_id, m.user_id, u.username,
			u.last_name AS user_last_name, u.first_name AS user_first_name,
			m.action_type, m.field_name, m.old_value, m.new_value, m.comment, m.created_at,
			COALESCE(
				NULLIF(TRIM(m.subject), ''),
				NULLIF(TRIM(CONCAT_WS(' ', ue.last_name, ue.first_name, ue.middle_name)), '')
			) AS subject
		FROM (
			SELECT a.id, a.entity_id AS unique_employee_id, a.actor_user_id AS user_id,
				a.action AS action_type, a.details->>'field_name' AS field_name,
				a.details->>'old_value' AS old_value, a.details->>'new_value' AS new_value,
				a.details->>'comment' AS comment, a.details->>'subject' AS subject, a.created_at
			FROM audit_log a
			WHERE a.entity_type = ?
		) m
		LEFT JOIN users u ON u.id = m.user_id
		LEFT JOIN unique_employees ue ON ue.id = m.unique_employee_id
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT ?`

	items := make([]UniqueEmployeeHistoryItem, 0)
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityUniqueEmployee, limit).Scan(&items).Error; err != nil {
		slog.Error("не удалось загрузить журнал реестра сотрудников", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching registry log")
	}
	return items, nil
}

// registryLogMaxRows - потолок выдачи журнала реестра. Журнал открывают, чтобы найти
// недавнее событие, а не выгрузить всю историю установки; без потолка один запрос на
// большой базе тянул бы сотни тысяч строк.
const registryLogMaxRows = 500

// canEditEmployee проверяет права пользователя на редактирование сотрудника.
func (s *uniqueEmployeeService) canEditEmployee(emp *models.UniqueEmployee, ownerInfo *EmployeeOwnerInfo) bool {
	// Администратор системы правит и удаляет любого сотрудника реестра: бюро ведёт
	// данные за контрагентов, чьей организации у него нет.
	if ownerInfo.CanManageAll {
		return true
	}
	if emp.UserID != nil && *emp.UserID == ownerInfo.UserID {
		return true
	}
	if emp.OrganizationID != nil && ownerInfo.OrganizationID != nil && *emp.OrganizationID == *ownerInfo.OrganizationID {
		return true
	}
	if emp.CompanyID != nil && ownerInfo.CompanyID != nil && *emp.CompanyID == *ownerInfo.CompanyID {
		return true
	}
	return false
}
