package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// EmployeeService -- интерфейс бизнес-логики сотрудников в заявках.
type EmployeeService interface {
	// CreateEmployee создаёт сотрудника и связи с целевыми таблицами (транзакция).
	CreateEmployee(ctx context.Context, req CreateEmployeeRequest) (*CreateEmployeeResponse, error)
	// GetActiveEmployeesForTable возвращает активных сотрудников для конкретной таблицы.
	GetActiveEmployeesForTable(ctx context.Context, tableID int) ([]TableEmployeeResponse, error)
	// UpdateEmployeeTerritoryStatus обновляет статус нахождения сотрудника на территории (въезд/выезд).
	// Аналогично UpdateCarTerritoryStatus: пишет в employees_history запись
	// с action_type=entry/exit, обновляет territory_status + territory_entry_time.
	UpdateEmployeeTerritoryStatus(ctx context.Context, employeeID int, req UpdateTerritoryStatusRequest) error
}

// --- DTO запросов ---

// CreateEmployeeRequest -- тело запроса на создание сотрудника.
type CreateEmployeeRequest struct {
	LastName             string  `json:"last_name" validate:"required,min=1"`
	FirstName            string  `json:"first_name" validate:"required,min=1"`
	MiddleName           *string `json:"middle_name"`
	CitizenshipID        int     `json:"citizenship_id" validate:"gte=1"`
	Position             string  `json:"position" validate:"required,min=1"`
	PassportSeriesNumber string  `json:"passport_series_number" validate:"required,min=1"`
	PatentNumber         *string `json:"patent_number"`
	OtherPermission      *string `json:"other_permission"`
	TargetTables         []int   `json:"target_tables"`
}

// CreateEmployeeResponse -- ответ после создания сотрудника.
type CreateEmployeeResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	EmployeeID int    `json:"employee_id"`
}

// --- DTO ответов ---

// TableEmployeeResponse -- сотрудник для отображения в таблице.
type TableEmployeeResponse struct {
	ID           int     `json:"id"`
	LastName     string  `json:"last_name"`
	FirstName    string  `json:"first_name"`
	MiddleName   *string `json:"middle_name"`
	Organization *string `json:"organization"`
	EntryDateTo  *string `json:"entry_date_to"`
	PassTime     *string `json:"pass_time"`
	Status       int     `json:"status"`
}

// --- Реализация ---

type employeeService struct {
	db *gorm.DB
}

// NewEmployeeService создаёт новый экземпляр EmployeeService.
func NewEmployeeService(db *gorm.DB) EmployeeService {
	return &employeeService{db: db}
}

// CreateEmployee создаёт сотрудника и связи с целевыми таблицами в транзакции.
func (s *employeeService) CreateEmployee(ctx context.Context, req CreateEmployeeRequest) (*CreateEmployeeResponse, error) {
	var employeeID int

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		statusZero := 0
		employee := models.Employee{
			LastName:             &req.LastName,
			FirstName:            &req.FirstName,
			MiddleName:           req.MiddleName,
			CitizenshipID:        &req.CitizenshipID,
			Position:             &req.Position,
			PassportSeriesNumber: &req.PassportSeriesNumber,
			PatentNumber:         req.PatentNumber,
			OtherPermission:      req.OtherPermission,
			Status:               &statusZero,
		}
		if err := tx.Create(&employee).Error; err != nil {
			slog.Error("не удалось создать сотрудника", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error creating employee")
		}
		employeeID = employee.ID

		for _, tableID := range req.TargetTables {
			orderIdx := 1
			ett := models.EmployeeTargetTable{
				EmployeeID: employeeID,
				TableID:    tableID,
				OrderIndex: &orderIdx,
			}
			if err := tx.Create(&ett).Error; err != nil {
				slog.Error("не удалось создать связь сотрудника с таблицей", "employee_id", employeeID, "table_id", tableID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error creating employee target table")
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	slog.Info("сотрудник создан", "employee_id", employeeID)
	return &CreateEmployeeResponse{
		Success:    true,
		Message:    "Employee created successfully",
		EmployeeID: employeeID,
	}, nil
}

// GetActiveEmployeesForTable возвращает активных сотрудников для указанной таблицы.
func (s *employeeService) GetActiveEmployeesForTable(ctx context.Context, tableID int) ([]TableEmployeeResponse, error) {
	type employeeRow struct {
		ID           int
		LastName     string
		FirstName    string
		MiddleName   *string
		Organization *string
		EntryDateTo  *string
		PassTime     *string
		Status       *int
	}

	rows := make([]employeeRow, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			e.id,
			e.last_name,
			e.first_name,
			e.middle_name,
			COALESCE(o.name, co.name) AS organization,
			a.entry_date_to,
			CONCAT(a.entry_time_from, ' - ', a.entry_time_to) AS pass_time,
			e.status
		FROM employees e
		JOIN employee_target_tables ett ON e.id = ett.employee_id
		JOIN attachments a ON e.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		LEFT JOIN organizations o ON app.organization_id = o.id
		LEFT JOIN companies co ON app.company_id = co.id
		WHERE ett.table_id = ?
		AND e.status = 1
		AND app.confirmation = ?
		AND app.status IN (?, ?)
		AND CURRENT_DATE BETWEEN a.entry_date_from::date AND a.entry_date_to::date
		GROUP BY e.id, e.last_name, e.first_name, e.middle_name,
				 o.name, co.name, a.entry_date_to, a.entry_time_from,
				 a.entry_time_to, e.status
		ORDER BY e.last_name, e.first_name
	`, tableID, models.ConfirmationApproved, models.StatusInWork, models.StatusCompleted).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching active employees")
	}

	employees := make([]TableEmployeeResponse, 0, len(rows))
	for _, r := range rows {
		status := 0
		if r.Status != nil {
			status = *r.Status
		}
		employees = append(employees, TableEmployeeResponse{
			ID:           r.ID,
			LastName:     r.LastName,
			FirstName:    r.FirstName,
			MiddleName:   r.MiddleName,
			Organization: r.Organization,
			EntryDateTo:  r.EntryDateTo,
			PassTime:     r.PassTime,
			Status:       status,
		})
	}
	return employees, nil
}

// UpdateEmployeeTerritoryStatus обновляет территориальный статус сотрудника
// (въезд=1 / выезд=2) и пишет в employees_history запись с action_type. Полный
// аналог UpdateCarTerritoryStatus из car_status_service.go.
func (s *employeeService) UpdateEmployeeTerritoryStatus(ctx context.Context, employeeID int, req UpdateTerritoryStatusRequest) error {
	now := time.Now().UTC()
	actionType := "unknown"
	if req.TerritoryStatus == 1 {
		actionType = "entry"
	} else if req.TerritoryStatus == 2 {
		actionType = "exit"
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var employee models.Employee
		if err := tx.Select("id", "last_name", "first_name", "middle_name", "territory_status").
			First(&employee, employeeID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "Employee not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}

		updates := map[string]interface{}{
			"territory_status": req.TerritoryStatus,
			"updated_at":       now,
		}
		if req.TerritoryStatus == 1 {
			updates["territory_entry_time"] = now
		}
		if err := tx.Model(&models.Employee{}).Where("id = ?", employeeID).Updates(updates).Error; err != nil {
			slog.Error("не удалось обновить территориальный статус сотрудника", "employee_id", employeeID, "status", req.TerritoryStatus, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating employee territory status")
		}

		fullName := ""
		if employee.LastName != nil {
			fullName += *employee.LastName
		}
		if employee.FirstName != nil {
			fullName += " " + *employee.FirstName
		}
		var comment string
		if req.TerritoryStatus == 1 {
			comment = fmt.Sprintf("Сотрудник %s прошёл на территорию", fullName)
		} else if req.TerritoryStatus == 2 {
			comment = fmt.Sprintf("Сотрудник %s вышел с территории", fullName)
		}

		history := models.EmployeeHistory{
			EmployeeID: employeeID,
			UserID:     req.UserID,
			ActionType: actionType,
			Comment:    &comment,
			CreatedAt:  now,
		}
		if err := tx.Create(&history).Error; err != nil {
			slog.Error("не удалось добавить запись в историю сотрудника", "employee_id", employeeID, "action_type", actionType, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding employee history entry")
		}
		slog.Info("территориальный статус сотрудника обновлён", "employee_id", employeeID, "action_type", actionType, "status", req.TerritoryStatus)
		return nil
	})
}
