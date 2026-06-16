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
	// DeactivateEmployee деактивирует сотрудника (мягкое удаление) и пишет в историю.
	DeactivateEmployee(ctx context.Context, employeeID int, req DeactivateEmployeeRequest) error
	// ActivateEmployee вводит сотрудника в работу и пишет в историю.
	ActivateEmployee(ctx context.Context, employeeID int, req ActivateEmployeeRequest) error
	// RestoreEmployee восстанавливает удалённого сотрудника и пишет в историю.
	RestoreEmployee(ctx context.Context, employeeID int, req RestoreEmployeeRequest) error
}

// DeactivateEmployeeRequest -- тело запроса деактивации сотрудника.
type DeactivateEmployeeRequest struct {
	Status  int  `json:"status"`
	UserID  *int `json:"user_id"`
	TableID *int `json:"table_id"`
}

// ActivateEmployeeRequest -- тело запроса активации сотрудника.
type ActivateEmployeeRequest struct {
	UserID *int `json:"user_id"`
}

// RestoreEmployeeRequest -- тело запроса восстановления сотрудника.
type RestoreEmployeeRequest struct {
	UserID *int `json:"user_id"`
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
// CitizenshipName / Position / Company / PassPlaces добавлены для отображения
// соответствующих колонок в PeopleTable.vue (#116 пункт 10).
type TableEmployeeResponse struct {
	ID                int     `json:"id"`
	LastName          string  `json:"last_name"`
	FirstName         string  `json:"first_name"`
	MiddleName        *string `json:"middle_name"`
	Organization      *string `json:"organization"`
	Company           *string `json:"company"`
	CitizenshipName   *string `json:"citizenship_name"`
	Position          *string `json:"position"`
	PassPlaces        *string `json:"pass_places"`
	EntryDateTo       *string `json:"entry_date_to"`
	PassTime          *string `json:"pass_time"`
	Status            int     `json:"status"`
	ApplicationID     *int    `json:"application_id"`
	ApplicationNumber *string `json:"application_number"`
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
// Включает citizenship / position / company / pass_places (#116 пункт 10) чтобы
// PeopleTable.vue мог отрисовать соответствующие колонки.
func (s *employeeService) GetActiveEmployeesForTable(ctx context.Context, tableID int) ([]TableEmployeeResponse, error) {
	type employeeRow struct {
		ID                int
		LastName          string
		FirstName         string
		MiddleName        *string
		Organization      *string
		Company           *string
		CitizenshipName   *string
		Position          *string
		PassPlaces        *string
		EntryDateTo       *string
		PassTime          *string
		Status            *int
		ApplicationID     *int
		ApplicationNumber *string
	}

	rows := make([]employeeRow, 0)
	// Оконная функция считается после GROUP BY: для каждого непустого паспорта
	// оставляем строку с максимальным entry_date_to (rn=1). Строки с NULL-паспортом
	// ("По факту") не схлопываем - условие (hmac IS NULL OR rn = 1).
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			id,
			last_name,
			first_name,
			middle_name,
			organization,
			company,
			citizenship_name,
			position,
			pass_places,
			entry_date_to,
			pass_time,
			status,
			application_id,
			application_number
		FROM (
			SELECT
				e.id,
				e.last_name,
				e.first_name,
				e.middle_name,
				COALESCE(o.name, co.name) AS organization,
				COALESCE(co.name, '') AS company,
				c.name AS citizenship_name,
				e.position,
				(
					SELECT STRING_AGG(DISTINCT st.display_name, ', ' ORDER BY st.display_name)
					FROM employee_target_tables ett2
					JOIN system_tables st ON ett2.table_id = st.id
					WHERE ett2.employee_id = e.id
				) AS pass_places,
				a.entry_date_to,
				CONCAT(a.entry_time_from, ' - ', a.entry_time_to) AS pass_time,
				e.status,
				app.id AS application_id,
				app.application_number AS application_number,
				e.passport_series_number_hmac,
				ROW_NUMBER() OVER (
					PARTITION BY e.passport_series_number_hmac
					ORDER BY a.entry_date_to DESC NULLS LAST, e.id DESC
				) AS rn
			FROM employees e
			JOIN employee_target_tables ett ON e.id = ett.employee_id
			JOIN attachments a ON e.attachment_id = a.id
			JOIN applications app ON a.application_id = app.id
			LEFT JOIN organizations o ON app.organization_id = o.id
			LEFT JOIN companies co ON app.company_id = co.id
			LEFT JOIN citizenships c ON e.citizenship_id = c.id
			WHERE ett.table_id = ?
			AND e.status = 1
			AND app.confirmation = ?
			AND app.status IN (?, ?)
			AND CURRENT_DATE BETWEEN a.entry_date_from::date AND a.entry_date_to::date
			GROUP BY e.id, e.last_name, e.first_name, e.middle_name,
					 o.name, co.name, c.name, e.position,
					 a.entry_date_to, a.entry_time_from,
					 a.entry_time_to, e.status, app.id, app.application_number,
					 e.passport_series_number_hmac
		) sub
		WHERE sub.passport_series_number_hmac IS NULL OR sub.rn = 1
		ORDER BY last_name, first_name
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
			ID:                r.ID,
			LastName:          r.LastName,
			FirstName:         r.FirstName,
			MiddleName:        r.MiddleName,
			Organization:      r.Organization,
			Company:           r.Company,
			CitizenshipName:   r.CitizenshipName,
			Position:          r.Position,
			PassPlaces:        r.PassPlaces,
			EntryDateTo:       r.EntryDateTo,
			PassTime:          r.PassTime,
			Status:            status,
			ApplicationID:     r.ApplicationID,
			ApplicationNumber: r.ApplicationNumber,
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

// DeactivateEmployee деактивирует сотрудника и записывает удаление в историю.
// Аналог DeactivateCar: меняет status на req.Status, ставит date_deleted=now,
// пишет в employees_history запись с action_type=delete.
func (s *employeeService) DeactivateEmployee(ctx context.Context, employeeID int, req DeactivateEmployeeRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var employee models.Employee
		if err := tx.Select("id", "last_name", "first_name", "middle_name").
			First(&employee, employeeID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "Employee not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}

		now := time.Now().UTC()
		if err := tx.Model(&models.Employee{}).Where("id = ?", employeeID).Updates(map[string]interface{}{
			"status":       req.Status,
			"date_deleted": now,
			"updated_at":   now,
		}).Error; err != nil {
			slog.Error("не удалось деактивировать сотрудника", "employee_id", employeeID, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error deactivating employee")
		}

		fullName := formatFullName(employee.LastName, employee.FirstName, employee.MiddleName)
		comment := fmt.Sprintf("Сотрудник %s удалён пользователем", fullName)
		actionType := "delete"
		history := models.EmployeeHistory{
			EmployeeID: employeeID,
			UserID:     req.UserID,
			ActionType: actionType,
			Comment:    &comment,
			CreatedAt:  now,
			TableID:    req.TableID,
		}
		if err := tx.Create(&history).Error; err != nil {
			slog.Error("не удалось добавить запись в историю сотрудника", "employee_id", employeeID, "action_type", actionType, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding employee history entry")
		}
		slog.Info("сотрудник деактивирован", "employee_id", employeeID)
		return nil
	})
}

// ActivateEmployee вводит сотрудника в работу и записывает активацию в историю.
// Аналог ActivateCar: ставит status=1, очищает date_deleted, пишет history с action_type=activate.
func (s *employeeService) ActivateEmployee(ctx context.Context, employeeID int, req ActivateEmployeeRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var employee models.Employee
		if err := tx.Select("id", "last_name", "first_name", "middle_name").
			First(&employee, employeeID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "Employee not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}

		now := time.Now().UTC()
		if err := tx.Model(&models.Employee{}).Where("id = ?", employeeID).Updates(map[string]interface{}{
			"status":       1,
			"date_deleted": nil,
			"updated_at":   now,
		}).Error; err != nil {
			slog.Error("не удалось активировать сотрудника", "employee_id", employeeID, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error activating employee")
		}

		fullName := formatFullName(employee.LastName, employee.FirstName, employee.MiddleName)
		comment := fmt.Sprintf("Сотрудник %s введён в работу", fullName)
		actionType := "activate"
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
		slog.Info("сотрудник активирован", "employee_id", employeeID)
		return nil
	})
}

// RestoreEmployee восстанавливает удалённого сотрудника и записывает восстановление в историю.
// Аналог RestoreCar: ставит status=1, очищает date_deleted, пишет history с action_type=restore.
func (s *employeeService) RestoreEmployee(ctx context.Context, employeeID int, req RestoreEmployeeRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var employee models.Employee
		if err := tx.Select("id", "last_name", "first_name", "middle_name").
			First(&employee, employeeID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "Employee not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}

		now := time.Now().UTC()
		if err := tx.Model(&models.Employee{}).Where("id = ?", employeeID).Updates(map[string]interface{}{
			"status":       1,
			"date_deleted": nil,
			"updated_at":   now,
		}).Error; err != nil {
			slog.Error("не удалось восстановить сотрудника", "employee_id", employeeID, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error restoring employee")
		}

		fullName := formatFullName(employee.LastName, employee.FirstName, employee.MiddleName)
		comment := fmt.Sprintf("Сотрудник %s восстановлен", fullName)
		actionType := "restore"
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
		slog.Info("сотрудник восстановлен", "employee_id", employeeID)
		return nil
	})
}
