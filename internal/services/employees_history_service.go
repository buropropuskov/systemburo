package services

import (
	"context"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// EmployeesHistoryService -- интерфейс бизнес-логики истории сотрудников.
type EmployeesHistoryService interface {
	// GetByEmployee возвращает историю конкретного сотрудника.
	GetByEmployee(ctx context.Context, employeeID int) ([]EmployeeHistoryItem, error)
	// GetUnified возвращает объединённую историю по ФИО (все сотрудники с таким именем).
	GetUnified(ctx context.Context, lastName, firstName, middleName string) ([]EmployeeHistoryItem, error)
	// GetAll возвращает страницу истории входов/выходов всех сотрудников по фильтру и
	// общее число подходящих строк.
	GetAll(ctx context.Context, q models.PassageHistoryQuery) ([]EmployeeHistoryItem, int64, error)
	// GetCurrentStatus возвращает текущий территориальный статус всех сотрудников.
	// viewerID - кто спрашивает: от него зависит признак «отметку можно отменить».
	GetCurrentStatus(ctx context.Context, viewerID int) ([]EmployeeCurrentStatus, error)
	// GetByTable возвращает страницу истории сотрудников таблицы (места) по фильтру и
	// общее число подходящих строк.
	GetByTable(ctx context.Context, tableID int, q models.PassageHistoryQuery) ([]EmployeeHistoryItem, int64, error)
	// GetFilterOptions отдаёт значения выпадающих списков журнала людей; tableID
	// сужает до одной таблицы проходной, nil берёт весь журнал.
	GetFilterOptions(ctx context.Context, tableID *int) (EmployeesHistoryFilterOptions, error)
}

// EmployeeHistoryItem -- DTO элемента истории сотрудника.
type EmployeeHistoryItem struct {
	ID                 int     `json:"id"`
	EmployeeID         int     `json:"employee_id"`
	ApplicationID      *int    `json:"application_id"`
	UserID             *int    `json:"user_id"`
	TableID            *int    `json:"table_id"`
	TableName          *string `json:"table_name"`
	UserName           string  `json:"user_name"`
	ActionType         string  `json:"action_type"`
	FieldName          *string `json:"field_name"`
	OldValue           *string `json:"old_value"`
	NewValue           *string `json:"new_value"`
	Comment            *string `json:"comment"`
	Metadata           *string `json:"metadata"`
	CreatedAt          string  `json:"created_at"`
	EmployeeLastName   *string `json:"employee_last_name"`
	EmployeeFirstName  *string `json:"employee_first_name"`
	EmployeeMiddleName *string `json:"employee_middle_name"`
	Organization       *string `json:"organization"`
	Company            *string `json:"company"`
	// Reverted - отметка прохода отменена как ошибочная (#2437): в журнале видна с
	// пометкой, в цифрах не участвует.
	Reverted bool `json:"reverted"`
}

// EmployeeCurrentStatus -- текущий территориальный статус сотрудника.
type EmployeeCurrentStatus struct {
	EmployeeID      int     `json:"employee_id"`
	TerritoryStatus int     `json:"territory_status"`
	EntryTime       *string `json:"entry_time"`
	LastExitTime    *string `json:"last_exit_time"`
	// CanRevert - спрашивающий может отменить последнюю отметку прямо сейчас (#2437):
	// либо она его и свежая, либо он администратор. Считает бэк, чтобы правило и его
	// окно жили в одном месте, а не повторялись в трёх таблицах на фронте.
	CanRevert bool `json:"can_revert"`
	// LastMarkTableID - пост, на котором поставлена последняя отметка. Таблица прячет
	// кнопку отмены у чужих постов: отменять отметку можно только там, где её поставили.
	LastMarkTableID *int `json:"last_mark_table_id"`
}

// --- Реализация ---

type employeesHistoryService struct {
	db *gorm.DB
}

// NewEmployeesHistoryService создаёт новый экземпляр EmployeesHistoryService.
func NewEmployeesHistoryService(db *gorm.DB) EmployeesHistoryService {
	return &employeesHistoryService{db: db}
}

// employeeHistoryRow -- промежуточная структура для маппинга raw SQL.
type employeeHistoryRow struct {
	ID                 int
	EmployeeID         int
	ApplicationID      *int
	UserID             *int
	TableID            *int
	TableName          *string
	UserName           string
	ActionType         string
	FieldName          *string
	OldValue           *string
	NewValue           *string
	Comment            *string
	Metadata           *string
	CreatedAt          time.Time
	EmployeeLastName   *string
	EmployeeFirstName  *string
	EmployeeMiddleName *string
	Organization       *string
	Company            *string
	Reverted           bool
}

// baseSelectSQL -- общая часть SELECT для всех запросов истории сотрудников.
const baseSelectSQL = `
	SELECT
		eh.id,
		eh.employee_id,
		app.id AS application_id,
		eh.user_id,
		eh.table_id,
		st.display_name AS table_name,
		` + employeesHistoryUserNameSQL + ` AS user_name,
		eh.action_type,
		eh.field_name,
		eh.old_value,
		eh.new_value,
		eh.comment,
		eh.metadata::text AS metadata,
		eh.created_at,
		e.last_name AS employee_last_name,
		e.first_name AS employee_first_name,
		e.middle_name AS employee_middle_name,
		COALESCE(org.name, '') AS organization,
		COALESCE(comp.name, '') AS company,
		eh.reverted
	` + employeesHistoryFromSQL

// employeesHistoryUserNameSQL - ФИО отметившего. Выражение нужно и выборке, и поиску.
const employeesHistoryUserNameSQL = `COALESCE(CONCAT(u.last_name, ' ', u.first_name), 'Система')`

// employeesHistoryFromSQL - источник истории сотрудников со всеми соединениями. Один и
// тот же FROM читают страница, счётчик для meta.total и списки фильтра, иначе «показано
// 50 из 431» разошлось бы с самой выборкой.
//
// Своего WHERE здесь нет намеренно: журнал проходов берёт только entry/exit, а история
// таблицы - все события сотрудников этой таблицы (урок #1085), и базовое условие у них
// разное.
const employeesHistoryFromSQL = `
	FROM ` + employeesHistoryUnion + ` eh
	LEFT JOIN users u ON eh.user_id = u.id
	LEFT JOIN system_tables st ON eh.table_id = st.id
	JOIN employees e ON eh.employee_id = e.id
	LEFT JOIN attachments a ON e.attachment_id = a.id
	LEFT JOIN applications app ON a.application_id = app.id
	-- Ручные сотрудники (#1049) висят на вложении-сироте без заявки (app.* NULL, метка
	-- application_id пустой), поэтому org/company берём через COALESCE с самого вложения.
	LEFT JOIN organizations org ON org.id = COALESCE(app.organization_id, a.organization_id)
	LEFT JOIN companies comp ON comp.id = COALESCE(app.company_id, a.company_id)`

// employeesHistoryCountSQL - число строк журнала по тем же условиям, что и страница.
const employeesHistoryCountSQL = `SELECT COUNT(*)` + employeesHistoryFromSQL

// employeesHistoryPassageWhereSQL - базовое условие журнала проходов людей.
const employeesHistoryPassageWhereSQL = ` WHERE eh.action_type IN ('entry', 'exit')`

// employeesHistoryTableWhereSQL - скоуп таблицы (места). Кроме entry/exit с прямым
// table_id сюда входят все события сотрудников, привязанных к таблице через
// employee_target_tables: общая история места показывает контекст, а не только проходы.
const employeesHistoryTableWhereSQL = `
	WHERE eh.table_id = ?
	   OR (
	     eh.table_id IS NULL
	     AND eh.employee_id IN (
	       SELECT ett.employee_id FROM employee_target_tables ett WHERE ett.table_id = ?
	     )
	   )`

// employeesHistorySearchExprs - по чему ищет строка поиска в журнале людей.
var employeesHistorySearchExprs = []string{
	"e.last_name",
	"e.first_name",
	"e.middle_name",
	"org.name",
	"comp.name",
	employeesHistoryUserNameSQL,
}

func (s *employeesHistoryService) GetByEmployee(ctx context.Context, employeeID int) ([]EmployeeHistoryItem, error) {
	rows := make([]employeeHistoryRow, 0)
	err := s.db.WithContext(ctx).Raw(baseSelectSQL+`
		WHERE eh.employee_id = ?
		ORDER BY eh.created_at DESC
	`, employeeID).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employee history")
	}
	return mapEmployeeHistoryRows(rows), nil
}

func (s *employeesHistoryService) GetUnified(ctx context.Context, lastName, firstName, middleName string) ([]EmployeeHistoryItem, error) {
	rows := make([]employeeHistoryRow, 0)
	var err error

	if strings.TrimSpace(middleName) != "" {
		err = s.db.WithContext(ctx).Raw(baseSelectSQL+`
			WHERE LOWER(TRIM(e.last_name)) = LOWER(TRIM(?))
			  AND LOWER(TRIM(e.first_name)) = LOWER(TRIM(?))
			  AND LOWER(TRIM(e.middle_name)) = LOWER(TRIM(?))
			ORDER BY eh.created_at DESC
		`, lastName, firstName, middleName).Scan(&rows).Error
	} else {
		err = s.db.WithContext(ctx).Raw(baseSelectSQL+`
			WHERE LOWER(TRIM(e.last_name)) = LOWER(TRIM(?))
			  AND LOWER(TRIM(e.first_name)) = LOWER(TRIM(?))
			ORDER BY eh.created_at DESC
		`, lastName, firstName).Scan(&rows).Error
	}

	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching unified employee history")
	}
	return mapEmployeeHistoryRows(rows), nil
}

// GetAll возвращает страницу журнала входов и выходов и общее число строк по фильтру.
//
// До #2469 метод отдавал всю историю проходов сразу. Фронт им не пользуется - журнал
// людей открывается от таблицы проходной, - но границы нужны и здесь: метод открыт в
// API, и «отдай всё» остаётся «отдай всё» независимо от того, кто спрашивает.
func (s *employeesHistoryService) GetAll(ctx context.Context, q models.PassageHistoryQuery) ([]EmployeeHistoryItem, int64, error) {
	return s.queryHistoryPage(ctx, q, employeesHistoryPassageWhereSQL, nil)
}

func (s *employeesHistoryService) GetCurrentStatus(ctx context.Context, viewerID int) ([]EmployeeCurrentStatus, error) {
	type statusRow struct {
		ID                 int
		TerritoryStatus    *int
		TerritoryEntryTime *time.Time
		LastExitTime       *time.Time
		CanRevert          bool
		LastMarkTableID    *int
	}

	admin, err := isPassageRevertAdmin(ctx, s.db, viewerID)
	if err != nil {
		return nil, err
	}

	rows := make([]statusRow, 0)
	err = s.db.WithContext(ctx).Raw(`
		SELECT
			e.id,
			e.territory_status,
			e.territory_entry_time,
			(
				SELECT created_at
				FROM `+employeesHistoryUnion+` eh
				WHERE eh.employee_id = e.id AND eh.action_type = 'exit' AND NOT eh.reverted
				ORDER BY eh.created_at DESC
				LIMIT 1
			) AS last_exit_time,
			lm.table_id AS last_mark_table_id,
			lm.created_at IS NOT NULL
				AND (? OR (lm.user_id = ? AND lm.created_at > NOW() - ?::interval)) AS can_revert
		FROM employees e
		-- Последняя действительная отметка целиком (кто, когда, где), см. машины.
		LEFT JOIN LATERAL (
			SELECT eh.user_id, eh.created_at, eh.table_id
			FROM `+employeesHistoryUnion+` eh
			WHERE eh.employee_id = e.id AND eh.action_type IN ('entry', 'exit') AND NOT eh.reverted
			ORDER BY eh.created_at DESC, eh.id DESC
			LIMIT 1
		) lm ON TRUE
		WHERE e.status = 1
	`, admin, viewerID, passageRevertWindowSQL()).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employees current status")
	}

	items := make([]EmployeeCurrentStatus, 0, len(rows))
	for _, r := range rows {
		ts := 0
		if r.TerritoryStatus != nil {
			ts = *r.TerritoryStatus
		}
		items = append(items, EmployeeCurrentStatus{
			EmployeeID:      r.ID,
			TerritoryStatus: ts,
			EntryTime:       FormatUTCPtr(r.TerritoryEntryTime),
			LastExitTime:    FormatUTCPtr(r.LastExitTime),
			CanRevert:       r.CanRevert,
			LastMarkTableID: r.LastMarkTableID,
		})
	}
	return items, nil
}

// GetByTable возвращает историю сотрудников, относящихся к конкретной таблице.
// Включает не только entry/exit с прямым eh.table_id, но и все события (create,
// update, delete, data_changed) сотрудников, привязанных к этой таблице через
// employee_target_tables - чтобы общая история таблицы показывала полный контекст,
// а не только проходы.
func (s *employeesHistoryService) GetByTable(ctx context.Context, tableID int, q models.PassageHistoryQuery) ([]EmployeeHistoryItem, int64, error) {
	return s.queryHistoryPage(ctx, q, employeesHistoryTableWhereSQL, []any{tableID, tableID})
}

// queryHistoryPage - общая механика журнала людей: базовое условие (проходы или скоуп
// таблицы), фильтры, счётчик и страница.
//
// Порядок аргументов важен: базовое условие, затем фильтры, затем страница - в этой же
// последовательности `?` встречаются в собранном запросе.
func (s *employeesHistoryService) queryHistoryPage(ctx context.Context, q models.PassageHistoryQuery, baseWhere string, baseArgs []any) ([]EmployeeHistoryItem, int64, error) {
	filterSQL, filterArgs, err := passageHistoryConditions(q, passageFilterSpec{
		alias:        "eh",
		entityColumn: "eh.employee_id",
		entityID:     q.EmployeeID,
		searchExprs:  employeesHistorySearchExprs,
	})
	if err != nil {
		return nil, 0, err
	}

	// Базовое условие таблицы - это OR-ветка, поэтому фильтр приклеивается к ней
	// скобками: без них «И фамилия» относилось бы только к последней ветке OR и
	// история чужих сотрудников просочилась бы в отфильтрованную выдачу.
	where := "(" + strings.TrimPrefix(strings.TrimSpace(baseWhere), "WHERE") + ")"
	where = " WHERE " + where + filterSQL
	whereArgs := append(append([]any{}, baseArgs...), filterArgs...)

	var total int64
	if err := s.db.WithContext(ctx).Raw(employeesHistoryCountSQL+where, whereArgs...).Scan(&total).Error; err != nil {
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error counting employees history")
	}

	limitSQL, limitArgs := passageHistoryLimitSQL(q)
	rows := make([]employeeHistoryRow, 0, q.PerPage)
	err = s.db.WithContext(ctx).Raw(
		baseSelectSQL+where+passageHistoryOrderSQL(q, "eh")+limitSQL,
		append(append([]any{}, whereArgs...), limitArgs...)...,
	).Scan(&rows).Error
	if err != nil {
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employees history")
	}

	return mapEmployeeHistoryRows(rows), total, nil
}

// GetFilterOptions отдаёт значения выпадающих списков журнала людей: кто отмечал
// проходы и кого в журнале отмечали. tableID сужает до таблицы проходной.
//
// Списки собираются отдельным методом, а не из первой страницы: собранные из 50
// загруженных строк, они предлагали бы выбрать не то, что есть в журнале.
func (s *employeesHistoryService) GetFilterOptions(ctx context.Context, tableID *int) (EmployeesHistoryFilterOptions, error) {
	options := EmployeesHistoryFilterOptions{
		Users:     make([]PassageFilterUser, 0),
		Employees: make([]PassageFilterEmployee, 0),
	}

	where := employeesHistoryPassageWhereSQL
	var args []any
	if tableID != nil {
		where = " WHERE (" + strings.TrimPrefix(strings.TrimSpace(employeesHistoryTableWhereSQL), "WHERE") + ")"
		args = []any{*tableID, *tableID}
	}

	err := s.db.WithContext(ctx).Raw(
		`SELECT DISTINCT eh.user_id AS id, `+employeesHistoryUserNameSQL+` AS name`+
			employeesHistoryFromSQL+where+` AND eh.user_id IS NOT NULL ORDER BY name`,
		args...,
	).Scan(&options.Users).Error
	if err != nil {
		return options, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employees history users")
	}

	err = s.db.WithContext(ctx).Raw(
		`SELECT DISTINCT e.id, e.last_name, e.first_name, e.middle_name`+
			employeesHistoryFromSQL+where+` ORDER BY e.last_name, e.first_name`,
		args...,
	).Scan(&options.Employees).Error
	if err != nil {
		return options, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employees history employees")
	}

	return options, nil
}

// mapEmployeeHistoryRows преобразует сырые строки истории в DTO.
func mapEmployeeHistoryRows(rows []employeeHistoryRow) []EmployeeHistoryItem {
	items := make([]EmployeeHistoryItem, 0, len(rows))
	for _, r := range rows {
		userName := r.UserName
		if strings.TrimSpace(userName) == "" {
			userName = "Система"
		}
		items = append(items, EmployeeHistoryItem{
			ID:                 r.ID,
			EmployeeID:         r.EmployeeID,
			ApplicationID:      r.ApplicationID,
			UserID:             r.UserID,
			TableID:            r.TableID,
			TableName:          r.TableName,
			UserName:           userName,
			ActionType:         r.ActionType,
			FieldName:          r.FieldName,
			OldValue:           r.OldValue,
			NewValue:           r.NewValue,
			Comment:            r.Comment,
			Metadata:           r.Metadata,
			CreatedAt:          FormatUTC(r.CreatedAt),
			EmployeeLastName:   r.EmployeeLastName,
			EmployeeFirstName:  r.EmployeeFirstName,
			EmployeeMiddleName: r.EmployeeMiddleName,
			Organization:       r.Organization,
			Company:            r.Company,
			Reverted:           r.Reverted,
		})
	}
	return items
}
