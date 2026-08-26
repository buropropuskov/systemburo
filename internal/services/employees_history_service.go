package services

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// EmployeesHistoryService -- интерфейс бизнес-логики истории сотрудников.
type EmployeesHistoryService interface {
	// GetByEmployee возвращает историю конкретного сотрудника.
	GetByEmployee(ctx context.Context, employeeID int) ([]EmployeeHistoryItem, error)
	// GetUnified возвращает объединённую историю по ФИО (все сотрудники с таким именем).
	GetUnified(ctx context.Context, lastName, firstName, middleName string) ([]EmployeeHistoryItem, error)
	// GetAll возвращает историю въездов/выходов всех сотрудников.
	GetAll(ctx context.Context) ([]EmployeeHistoryItem, error)
	// GetCurrentStatus возвращает текущий территориальный статус всех сотрудников.
	GetCurrentStatus(ctx context.Context) ([]EmployeeCurrentStatus, error)
	// GetByTable возвращает историю сотрудников для конкретной таблицы (места).
	GetByTable(ctx context.Context, tableID int) ([]EmployeeHistoryItem, error)
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
}

// EmployeeCurrentStatus -- текущий территориальный статус сотрудника.
type EmployeeCurrentStatus struct {
	EmployeeID      int     `json:"employee_id"`
	TerritoryStatus int     `json:"territory_status"`
	EntryTime       *string `json:"entry_time"`
	LastExitTime    *string `json:"last_exit_time"`
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
		COALESCE(CONCAT(u.last_name, ' ', u.first_name), 'Система') AS user_name,
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
		COALESCE(comp.name, '') AS company
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

func (s *employeesHistoryService) GetAll(ctx context.Context) ([]EmployeeHistoryItem, error) {
	rows := make([]employeeHistoryRow, 0)
	err := s.db.WithContext(ctx).Raw(baseSelectSQL + `
		WHERE eh.action_type IN ('entry', 'exit')
		ORDER BY eh.created_at DESC
	`).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching all employees history")
	}
	return mapEmployeeHistoryRows(rows), nil
}

func (s *employeesHistoryService) GetCurrentStatus(ctx context.Context) ([]EmployeeCurrentStatus, error) {
	type statusRow struct {
		ID                 int
		TerritoryStatus    *int
		TerritoryEntryTime *time.Time
		LastExitTime       *time.Time
	}

	rows := make([]statusRow, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			e.id,
			e.territory_status,
			e.territory_entry_time,
			(
				SELECT created_at
				FROM ` + employeesHistoryUnion + ` eh
				WHERE eh.employee_id = e.id AND eh.action_type = 'exit'
				ORDER BY eh.created_at DESC
				LIMIT 1
			) AS last_exit_time
		FROM employees e
		WHERE e.status = 1
	`).Scan(&rows).Error
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
		})
	}
	return items, nil
}

// GetByTable возвращает историю сотрудников, относящихся к конкретной таблице.
// Включает не только entry/exit с прямым eh.table_id, но и все события (create,
// update, delete, data_changed) сотрудников, привязанных к этой таблице через
// employee_target_tables - чтобы общая история таблицы показывала полный контекст,
// а не только проходы.
func (s *employeesHistoryService) GetByTable(ctx context.Context, tableID int) ([]EmployeeHistoryItem, error) {
	rows := make([]employeeHistoryRow, 0)
	err := s.db.WithContext(ctx).Raw(baseSelectSQL+`
		WHERE eh.table_id = ?
		   OR (
		     eh.table_id IS NULL
		     AND eh.employee_id IN (
		       SELECT ett.employee_id FROM employee_target_tables ett WHERE ett.table_id = ?
		     )
		   )
		ORDER BY eh.created_at DESC
	`, tableID, tableID).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employee history by table")
	}
	return mapEmployeeHistoryRows(rows), nil
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
		})
	}
	return items
}
