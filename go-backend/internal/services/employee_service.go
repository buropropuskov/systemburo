package services

import (
	"context"
	"net/http"

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
}

// --- DTO запросов ---

// CreateEmployeeRequest -- тело запроса на создание сотрудника.
type CreateEmployeeRequest struct {
	LastName             string  `json:"last_name"`
	FirstName            string  `json:"first_name"`
	MiddleName           *string `json:"middle_name"`
	CitizenshipID        int     `json:"citizenship_id"`
	Position             string  `json:"position"`
	PassportSeriesNumber string  `json:"passport_series_number"`
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
				return echo.NewHTTPError(http.StatusInternalServerError, "Error creating employee target table")
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &CreateEmployeeResponse{
		Success:    true,
		Message:    "Employee created successfully",
		EmployeeID: employeeID,
	}, nil
}

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
		AND app.confirmation = 'Согласовано'
		AND app.status IN ('В работе', 'Завершено')
		AND CURRENT_DATE BETWEEN a.entry_date_from::date AND a.entry_date_to::date
		GROUP BY e.id, e.last_name, e.first_name, e.middle_name,
				 o.name, co.name, a.entry_date_to, a.entry_time_from,
				 a.entry_time_to, e.status
		ORDER BY e.last_name, e.first_name
	`, tableID).Scan(&rows).Error
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
