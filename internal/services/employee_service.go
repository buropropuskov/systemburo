package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// EmployeeService -- интерфейс бизнес-логики сотрудников в заявках.
type EmployeeService interface {
	// CreateEmployee создаёт сотрудника и связи с целевыми таблицами (транзакция).
	CreateEmployee(ctx context.Context, req CreateEmployeeRequest, actorID int) (*CreateEmployeeResponse, error)
	// CreateManualEmployees добавляет сотрудников прямо в таблицу без заявки (#1049,
	// режим-1): создаёт вложение-сироту (application_id NULL, is_manual, org/company и
	// время действия на вложении), сотрудников со status=1 и привязку к целевым таблицам.
	CreateManualEmployees(ctx context.Context, req ManualEmployeeRequest, userID int) (*ManualEmployeeResponse, error)
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
	// BulkMoveTable переносит набор сотрудников из FromTableID в каждую из ToTableIDs
	// (#1194): снимает привязку к исходной таблице, добавляет к целевым (дедуп).
	BulkMoveTable(ctx context.Context, req EmployeeBulkMoveTableRequest, actorID int) (*BulkOpResult, error)
	// BulkAddTable добавляет набор сотрудников в дополнительные таблицы (#1194),
	// не трогая уже существующие привязки.
	BulkAddTable(ctx context.Context, req EmployeeBulkAddTableRequest, actorID int) (*BulkOpResult, error)
	// BulkUnbindTable снимает у набора сотрудников привязку к одной таблице (#1194).
	// Если это была последняя привязка - сотрудник деактивируется (как одиночный delete).
	BulkUnbindTable(ctx context.Context, req EmployeeBulkUnbindTableRequest, actorID int) (*BulkOpResult, error)

	// SetBlankExportEnqueuer подключает очередь файлового архива (#1615, B1).
	SetBlankExportEnqueuer(e BlankExportEnqueuer)
}

// EmployeeBulkMoveTableRequest -- тело POST /employees/bulk/move-table: снимает у
// набора сотрудников привязку к FromTableID и привязывает к каждой из ToTableIDs
// (дедуп с уже существующими связями, прочие таблицы прохода не трогаются).
type EmployeeBulkMoveTableRequest struct {
	IDs         []int `json:"ids"`
	FromTableID int   `json:"from_table_id"`
	ToTableIDs  []int `json:"to_table_ids"`
}

// EmployeeBulkAddTableRequest -- тело POST /employees/bulk/add-table: добавляет
// набору сотрудников привязку к TableIDs, не отвязывая существующие таблицы.
type EmployeeBulkAddTableRequest struct {
	IDs      []int `json:"ids"`
	TableIDs []int `json:"table_ids"`
}

// EmployeeBulkUnbindTableRequest -- тело POST /employees/bulk/unbind-table: снимает
// у набора сотрудников привязку к одной TableID. Если привязка была последней -
// сотрудник деактивируется (зеркало одиночного DeactivateEmployee, status=0).
type EmployeeBulkUnbindTableRequest struct {
	IDs     []int `json:"ids"`
	TableID int   `json:"table_id"`
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

// ManualEmployeeRequest -- тело запроса ручного добавления сотрудников в таблицу (#1049,
// режим-1 без заявки, зеркало ManualCarRequest). org/company и время действия живут на
// вложении-сироте (is_manual=true, application_id NULL). У сотрудника нет полей времени
// на сущности - «когда действует пропуск» берётся с вложения. TableID -- таблица, из
// шапки которой нажали «Добавить вручную»: сотрудник гарантированно попадёт в неё (плюс
// любые таблицы прохода, выбранные в форме, через Employees[].TargetTables).
type ManualEmployeeRequest struct {
	OrganizationID int              `json:"organization_id"`
	CompanyID      *int             `json:"company_id"`
	TableID        int              `json:"table_id"`
	EntryDateFrom  *string          `json:"entry_date_from"`
	EntryDateTo    *string          `json:"entry_date_to"`
	EntryTimeFrom  *string          `json:"entry_time_from"`
	EntryTimeTo    *string          `json:"entry_time_to"`
	Employees      []ManualEmployee `json:"employees"`
}

// ManualEmployee -- один сотрудник в запросе ручного добавления (зеркало полей EmployeeForm).
type ManualEmployee struct {
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

// ManualEmployeeResponse -- ответ после ручного добавления сотрудников.
type ManualEmployeeResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	AttachmentID int    `json:"attachment_id"`
	EmployeeIDs  []int  `json:"employee_ids"`
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
	// TerritoryStatus - территориальный статус сотрудника (0 - не отмечен, 1 - вошёл,
	// 2 - вышел) из той же колонки employees.territory_status, что читает
	// /employees/history/current-status. Отдаём вместе со строкой, чтобы счётчик
	// «Людей зашло» и кнопки входа/выхода были верны сразу, а не проваливались в
	// «никто не отмечен» до ответа второго запроса.
	TerritoryStatus *int `json:"territory_status"`
	// TargetTablesCount - число таблиц проходной, к которым привязан сотрудник
	// (employee_target_tables). Используется FE (#1194) для решения, показывать ли
	// per-row выбор «из этой/из всех» при отвязке (>1) или сразу деактивировать (=1).
	TargetTablesCount int `json:"target_tables_count"`
	// TargetTables - список привязок сотрудника к таблицам проходной с источником
	// (#1227): application/manual. Карточка сотрудника из контекста проходной раньше
	// видела только count - теперь получает и сам список для секции «Проезд».
	TargetTables []EmployeePassageTableRef `json:"target_tables"`
}

// EmployeeEmployeePassageTableRef -- ссылка на таблицу проходной с источником привязки
// (#1227): application - привязана при подаче заявки, manual - добавлена вручную/
// переносом. Локальный тип (не расширяет общий TableInfoRef из application_service.go,
// у которого нет понятия source). Зеркало у машин - CarEmployeePassageTableRef (car_service.go).
type EmployeePassageTableRef struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
}

// --- Реализация ---

type employeeService struct {
	db             *gorm.DB
	recorder       AuditRecorder
	tablesProducer *TablesRefreshPublisher
	// blankExports - постановка заявки в очередь на выгрузку в файловый архив
	// (#1615, B1): bulk-перенос сотрудника между таблицами меняет то, что хранит
	// слепок заявки (заявка.json).
	blankExports BlankExportEnqueuer
	// notificationService - уведомление инициатора о первом проходе по заявке
	// (#1748, S4). Опционально: без неё UpdateEmployeeTerritoryStatus просто не шлёт.
	notificationService NotificationService
}

// EmployeeServiceOption конфигурирует employeeService при создании.
type EmployeeServiceOption func(*employeeService)

// WithEmployeeTablesProducer включает публикацию tables.refresh при въезде/выезде
// сотрудника (#840 V2.3): обновляем его целевые таблицы проходной live.
func WithEmployeeTablesProducer(p *TablesRefreshPublisher) EmployeeServiceOption {
	return func(s *employeeService) { s.tablesProducer = p }
}

// WithEmployeeNotifications включает уведомление инициатора заявки о первом
// проходе по ней (#1748, S4) при входе сотрудника.
func WithEmployeeNotifications(n NotificationService) EmployeeServiceOption {
	return func(s *employeeService) { s.notificationService = n }
}

// SetBlankExportEnqueuer подключает очередь файлового архива (#1615, B1).
func (s *employeeService) SetBlankExportEnqueuer(e BlankExportEnqueuer) {
	s.blankExports = e
}

// NewEmployeeService создаёт новый экземпляр EmployeeService.
func NewEmployeeService(db *gorm.DB, recorder AuditRecorder, opts ...EmployeeServiceOption) EmployeeService {
	s := &employeeService{db: db, recorder: recorder}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CreateEmployee создаёт сотрудника и связи с целевыми таблицами в транзакции.
func (s *employeeService) CreateEmployee(ctx context.Context, req CreateEmployeeRequest, actorID int) (*CreateEmployeeResponse, error) {
	var employeeID int

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		statusZero := 0
		employee := models.Employee{
			LastName:             &req.LastName,
			FirstName:            &req.FirstName,
			MiddleName:           req.MiddleName,
			CitizenshipID:        &req.CitizenshipID,
			Position:             &req.Position,
			PassportSeriesNumber: nilIfBlank(req.PassportSeriesNumber),
			PatentNumber:         nilIfBlankPtr(req.PatentNumber),
			OtherPermission:      req.OtherPermission,
			Status:               &statusZero,
		}
		if err := tx.Create(&employee).Error; err != nil {
			slog.Error("не удалось создать сотрудника", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error creating employee")
		}
		employeeID = employee.ID

		empComment := "Сотрудник создан"
		s.recorder.Log(ctx, tx, models.AuditEntityEmployee, &employeeID, "create", &actorID, carAuditDetails{Comment: &empComment})

		for _, tableID := range req.TargetTables {
			orderIdx := 1
			ett := models.EmployeeTargetTable{
				EmployeeID: employeeID,
				TableID:    tableID,
				OrderIndex: &orderIdx,
				Source:     "manual",
			}
			if err := tx.Create(&ett).Error; err != nil {
				slog.Error("не удалось создать связь сотрудника с таблицей", "employee_id", employeeID, "table_id", tableID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error creating employee target table")
			}
			// Историю попадания в таблицу пишем при активации (status->1), а не здесь -
			// сотрудник создаётся неактивным (status=0) и в таблице проходной не виден (#1085).
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

// CreateManualEmployees добавляет сотрудников в таблицу без заявки (#1049, режим-1).
// Создаёт вложение-сироту (application_id NULL, is_manual, org/company и время действия
// на вложении), затем самих сотрудников со status=1 (одобрения нет - сразу активны),
// привязку к целевым таблицам и записи аудита. Всё одной транзакцией: частичного
// добавления быть не должно.
func (s *employeeService) CreateManualEmployees(ctx context.Context, req ManualEmployeeRequest, userID int) (*ManualEmployeeResponse, error) {
	if req.OrganizationID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не указана организация")
	}
	if req.TableID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не указана таблица")
	}
	if len(req.Employees) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не указаны сотрудники")
	}
	for _, emp := range req.Employees {
		if strings.TrimSpace(emp.LastName) == "" || strings.TrimSpace(emp.FirstName) == "" {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "У сотрудника не указаны фамилия или имя")
		}
	}

	var attID int
	employeeIDs := make([]int, 0, len(req.Employees))

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		statusOne := 1
		att := models.Attachment{
			ApplicationID:   nil,
			AttachmentType:  "people",
			EntryDateFrom:   req.EntryDateFrom,
			EntryDateTo:     req.EntryDateTo,
			EntryTimeFrom:   req.EntryTimeFrom,
			EntryTimeTo:     req.EntryTimeTo,
			OrganizationID:  &req.OrganizationID,
			CompanyID:       req.CompanyID,
			IsManual:        true,
			CreatedByUserID: &userID,
			Status:          &statusOne,
		}
		if err := tx.Create(&att).Error; err != nil {
			slog.Error("не удалось создать ручное вложение сотрудников", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error creating manual attachment")
		}
		attID = att.ID

		for _, emp := range req.Employees {
			empStatus := statusOne
			var citizenshipID *int
			if emp.CitizenshipID > 0 {
				citizenshipID = &emp.CitizenshipID
			}
			lastName, firstName, position := emp.LastName, emp.FirstName, emp.Position
			employee := models.Employee{
				AttachmentID:         &attID,
				LastName:             &lastName,
				FirstName:            &firstName,
				MiddleName:           emp.MiddleName,
				CitizenshipID:        citizenshipID,
				Position:             &position,
				PassportSeriesNumber: nilIfBlank(emp.PassportSeriesNumber),
				PatentNumber:         nilIfBlankPtr(emp.PatentNumber),
				OtherPermission:      emp.OtherPermission,
				Status:               &empStatus,
			}
			if err := tx.Create(&employee).Error; err != nil {
				slog.Error("не удалось создать ручного сотрудника", "last_name", emp.LastName, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error creating manual employee")
			}
			employeeIDs = append(employeeIDs, employee.ID)

			// Целевые таблицы: таблица со страницы (req.TableID, гарантирует показ там,
			// откуда добавили) объединяется с выбранными в форме таблицами прохода.
			targetTables := map[int]struct{}{req.TableID: {}}
			for _, tableID := range emp.TargetTables {
				if tableID > 0 {
					targetTables[tableID] = struct{}{}
				}
			}
			for tableID := range targetTables {
				orderIdx := 1
				ett := models.EmployeeTargetTable{EmployeeID: employee.ID, TableID: tableID, OrderIndex: &orderIdx, Source: "manual"}
				if err := tx.Create(&ett).Error; err != nil {
					slog.Error("не удалось привязать сотрудника к таблице", "employee_id", employee.ID, "table_id", tableID, "error", err)
					return echo.NewHTTPError(http.StatusInternalServerError, "Error linking employee to table")
				}
				// История «добавлен в таблицу проходной» (#1085), в той же tx (как соседний create-Record).
				if err := recordAddedToTable(ctx, s.recorder, tx, models.AuditEntityEmployee, employee.ID, tableID, &userID); err != nil {
					slog.Error("не удалось записать историю попадания сотрудника в таблицу", "employee_id", employee.ID, "table_id", tableID, "error", err)
					return echo.NewHTTPError(http.StatusInternalServerError, "Error adding employee table history entry")
				}
			}

			comment := fmt.Sprintf("Сотрудник %s %s добавлен вручную", emp.LastName, emp.FirstName)
			if err := s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &employee.ID, "create", &userID, carAuditDetails{Comment: &comment}); err != nil {
				slog.Error("не удалось записать историю ручного сотрудника", "employee_id", employee.ID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error adding employee history entry")
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Ручные сотрудники появились в целевых таблицах live - обновляем их аудиторию
	// (#1049, по employee_target_tables, т.к. заявки нет). Best-effort, вне транзакции.
	s.tablesProducer.NotifyEmployeesChangedBatch(ctx, employeeIDs)

	slog.Info("ручные сотрудники добавлены", "attachment_id", attID, "count", len(employeeIDs), "user_id", userID)
	return &ManualEmployeeResponse{
		Success:      true,
		Message:      "Employees added successfully",
		AttachmentID: attID,
		EmployeeIDs:  employeeIDs,
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
		TerritoryStatus   *int
		TargetTablesCount int
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
			application_number,
			territory_status,
			target_tables_count
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
				e.territory_status,
				(
					SELECT COUNT(*) FROM employee_target_tables ett3
					WHERE ett3.employee_id = e.id
				) AS target_tables_count,
				e.passport_series_number_hmac,
				ROW_NUMBER() OVER (
					PARTITION BY e.passport_series_number_hmac
					ORDER BY a.entry_date_to DESC NULLS LAST, e.id DESC
				) AS rn
			FROM employees e
			JOIN employee_target_tables ett ON e.id = ett.employee_id
			JOIN attachments a ON e.attachment_id = a.id
			-- LEFT JOIN: ручные сотрудники (#1049) висят на вложении-сироте без заявки
			-- (a.application_id IS NULL, a.is_manual). org/company тогда берутся с самого
			-- вложения (COALESCE), а app.* остаются NULL - это и есть метка «добавлено вручную».
			LEFT JOIN applications app ON a.application_id = app.id
			LEFT JOIN organizations o ON o.id = COALESCE(app.organization_id, a.organization_id)
			LEFT JOIN companies co ON co.id = COALESCE(app.company_id, a.company_id)
			LEFT JOIN citizenships c ON e.citizenship_id = c.id
			WHERE ett.table_id = ?
			AND e.status = 1
			-- Заявочные сотрудники видны только по согласованной активной заявке в окне
			-- действия пропуска; ручные минуют оба требования - заявки у них нет вовсе,
			-- гейт видимости берёт на себя принадлежность целевой таблице (employee_target_tables)
			-- + security-видимость (S6). Показываются, пока активны (e.status = 1), как ручные машины.
			AND (a.is_manual OR (app.confirmation = ? AND app.status IN (?, ?)))
			AND (a.is_manual OR CURRENT_DATE BETWEEN a.entry_date_from::date AND a.entry_date_to::date)
			GROUP BY e.id, e.last_name, e.first_name, e.middle_name,
					 o.name, co.name, c.name, e.position,
					 a.entry_date_to, a.entry_time_from,
					 a.entry_time_to, e.status, app.id, app.application_number,
					 e.territory_status, e.passport_series_number_hmac
		) sub
		WHERE sub.passport_series_number_hmac IS NULL OR sub.rn = 1
		ORDER BY last_name, first_name
	`, tableID, models.ConfirmationApproved, models.StatusInWork, models.StatusCompleted).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching active employees")
	}

	employeeIDs := make([]int, 0, len(rows))
	for _, r := range rows {
		employeeIDs = append(employeeIDs, r.ID)
	}
	targetTablesMap, err := s.loadEmployeeTargetTables(ctx, employeeIDs)
	if err != nil {
		return nil, err
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
			TerritoryStatus:   r.TerritoryStatus,
			TargetTablesCount: r.TargetTablesCount,
			TargetTables:      targetTablesMap[r.ID],
		})
	}
	return employees, nil
}

// loadEmployeeTargetTables резолвит для набора сотрудников их привязки к таблицам
// проходной вместе с источником (#1227), одним батч-запросом (не N+1 на сотрудника).
func (s *employeeService) loadEmployeeTargetTables(ctx context.Context, employeeIDs []int) (map[int][]EmployeePassageTableRef, error) {
	result := make(map[int][]EmployeePassageTableRef, len(employeeIDs))
	if len(employeeIDs) == 0 {
		return result, nil
	}

	type targetTableRow struct {
		EmployeeID int
		ID         int
		Name       string
		Source     string
	}
	var rows []targetTableRow
	if err := s.db.WithContext(ctx).
		Table("employee_target_tables ett").
		Select("ett.employee_id AS employee_id, st.id AS id, COALESCE(NULLIF(st.display_name, ''), st.name) AS name, ett.source AS source").
		Joins("JOIN system_tables st ON st.id = ett.table_id").
		Where("ett.employee_id IN ?", employeeIDs).
		Order("ett.employee_id, ett.order_index, ett.table_id").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employee target tables")
	}

	for _, r := range rows {
		result[r.EmployeeID] = append(result[r.EmployeeID], EmployeePassageTableRef{ID: r.ID, Name: r.Name, Source: r.Source})
	}
	return result, nil
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

	var employee models.Employee
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("id", "last_name", "first_name", "middle_name", "territory_status", "attachment_id").
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

		if err := s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &employeeID, actionType, req.UserID, carAuditDetails{Comment: &comment, TableID: req.TableID}); err != nil {
			slog.Error("не удалось добавить запись в историю сотрудника", "employee_id", employeeID, "action_type", actionType, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding employee history entry")
		}
		slog.Info("территориальный статус сотрудника обновлён", "employee_id", employeeID, "action_type", actionType, "status", req.TerritoryStatus)
		return nil
	})
	if err != nil {
		return err
	}

	// Въезд/выезд изменил строку сотрудника - обновляем его целевые таблицы live
	// (#840 V2.3).
	s.tablesProducer.NotifyEmployeeChanged(ctx, employeeID)

	return nil
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
		if err := s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &employeeID, actionType, req.UserID, carAuditDetails{Comment: &comment, TableID: req.TableID}); err != nil {
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
		if err := s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &employeeID, actionType, req.UserID, carAuditDetails{Comment: &comment}); err != nil {
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
		if err := s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &employeeID, actionType, req.UserID, carAuditDetails{Comment: &comment}); err != nil {
			slog.Error("не удалось добавить запись в историю сотрудника", "employee_id", employeeID, "action_type", actionType, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding employee history entry")
		}
		slog.Info("сотрудник восстановлен", "employee_id", employeeID)
		return nil
	})
}

// --- Групповые операции над привязкой к таблицам проходной (#1194) ---

// loadEmployeeBasic загружает сотрудника по id для bulk-цикла (ФИО - для BulkItemError
// и текста истории). Возвращает ok=false, если записи нет.
func (s *employeeService) loadEmployeeBasic(ctx context.Context, id int) (models.Employee, bool) {
	var employee models.Employee
	if err := s.db.WithContext(ctx).Select("id", "last_name", "first_name", "middle_name", "status").
		First(&employee, id).Error; err != nil {
		return employee, false
	}
	return employee, true
}

// validatePeopleTables проверяет, что каждая из tableIDs существует и относится к
// таблицам людей (table_type=people) - иначе групповая операция сотрудников молча
// привяжет их к cars-таблице (тип-матч, зеркало car-стороны #1194).
func (s *employeeService) validatePeopleTables(ctx context.Context, tableIDs []int) error {
	unique := uniqueInts(tableIDs)
	if len(unique) == 0 {
		return nil
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.SystemTable{}).
		Where("id IN ? AND table_type = ?", unique, models.TableTypePeople).
		Count(&count).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки таблиц")
	}
	if int(count) != len(unique) {
		return echo.NewHTTPError(http.StatusBadRequest, "Таблица не найдена или не относится к таблицам людей")
	}
	return nil
}

// loadTableNames резолвит отображаемые имена таблиц для текста истории (комментарии
// переноса/отвязки).
func (s *employeeService) loadTableNames(ctx context.Context, tableIDs []int) (map[int]string, error) {
	type tableRow struct {
		ID   int
		Name string
	}
	var rows []tableRow
	if err := s.db.WithContext(ctx).Model(&models.SystemTable{}).
		Select("id, COALESCE(display_name, name) AS name").
		Where("id IN ?", uniqueInts(tableIDs)).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения таблиц")
	}
	out := make(map[int]string, len(rows))
	for _, r := range rows {
		out[r.ID] = r.Name
	}
	return out, nil
}

// bindEmployeeToTableIfMissing привязывает сотрудника к таблице, если связи ещё нет
// (дедуп). recordAdd управляет записью истории «добавлен в таблицу»: при добавлении
// (BulkAddTable) она нужна, при переносе (BulkMoveTable) - нет, там пишется одна
// сводная запись moved_between_tables (зеркало car-стороны, чтобы перенос не порождал
// лишних added_to_table на каждую целевую таблицу).
func (s *employeeService) bindEmployeeToTableIfMissing(ctx context.Context, tx *gorm.DB, employeeID, tableID int, actorID int, recordAdd bool) error {
	var exists int64
	if err := tx.Model(&models.EmployeeTargetTable{}).
		Where("employee_id = ? AND table_id = ?", employeeID, tableID).Count(&exists).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки привязки")
	}
	if exists > 0 {
		return nil
	}
	orderIdx := 1
	ett := models.EmployeeTargetTable{EmployeeID: employeeID, TableID: tableID, OrderIndex: &orderIdx, Source: "manual"}
	if err := tx.Create(&ett).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка привязки к таблице")
	}
	if recordAdd {
		if err := recordAddedToTable(ctx, s.recorder, tx, models.AuditEntityEmployee, employeeID, tableID, &actorID); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка записи истории")
		}
	}
	return nil
}

// BulkMoveTable переносит набор сотрудников из FromTableID в каждую из ToTableIDs
// (#1194): в транзакции на сотрудника снимает привязку к исходной таблице и
// добавляет (дедуп) к целевым, затем пишет одну запись moved_between_tables.
// Частичный успех: провал одного сотрудника не откатывает остальных.
func (s *employeeService) BulkMoveTable(ctx context.Context, req EmployeeBulkMoveTableRequest, actorID int) (*BulkOpResult, error) {
	if req.FromTableID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не указана исходная таблица")
	}
	toIDs := uniqueInts(req.ToTableIDs)
	if len(toIDs) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не выбраны целевые таблицы")
	}
	if err := s.validatePeopleTables(ctx, append([]int{req.FromTableID}, toIDs...)); err != nil {
		return nil, err
	}
	names, err := s.loadTableNames(ctx, append([]int{req.FromTableID}, toIDs...))
	if err != nil {
		return nil, err
	}
	toNames := make([]string, 0, len(toIDs))
	for _, tableID := range toIDs {
		toNames = append(toNames, names[tableID])
	}
	fromTableID := req.FromTableID

	res := newBulkResult()
	for _, id := range uniqueInts(req.IDs) {
		employee, ok := s.loadEmployeeBasic(ctx, id)
		if !ok {
			res.addError(id, "", "Сотрудник не найден")
			continue
		}
		fullName := formatFullName(employee.LastName, employee.FirstName, employee.MiddleName)
		if employee.Status == nil || *employee.Status != 1 {
			res.addError(id, fullName, "Сотрудник не активен")
			continue
		}
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			del := tx.Where("employee_id = ? AND table_id = ?", id, fromTableID).Delete(&models.EmployeeTargetTable{})
			if del.Error != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка отвязки от исходной таблицы")
			}
			if del.RowsAffected == 0 {
				return echo.NewHTTPError(http.StatusBadRequest, "Сотрудник не привязан к исходной таблице")
			}
			for _, tableID := range toIDs {
				if err := s.bindEmployeeToTableIfMissing(ctx, tx, id, tableID, actorID, false); err != nil {
					return err
				}
			}
			comment := fmt.Sprintf("Сотрудник %s перенесён из таблицы «%s» в «%s»", fullName, names[fromTableID], strings.Join(toNames, ", "))
			if err := s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &id, models.AuditActionMovedBetweenTables, &actorID, carAuditDetails{Comment: &comment, TableID: &fromTableID}); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка записи истории переноса")
			}
			return nil
		})
		if err != nil {
			res.addError(id, fullName, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	// Явные id (не пост-состояние employee_target_tables, #1194 S6): после переноса
	// исходная таблица уже не содержит сотрудника, NotifyEmployeesChangedBatch её
	// аудиторию не увидела бы (см. TablesRefreshPublisher.NotifyTables) - её зрителям
	// нужен сигнал, чтобы строка live исчезла. Зеркало carService.BulkMoveTable.
	s.tablesProducer.NotifyTables(ctx, append([]int{fromTableID}, toIDs...))
	s.enqueueArchiveExportForEmployees(ctx, req.IDs)
	return res.finalize(), nil
}

// BulkAddTable добавляет набору сотрудников привязку к TableIDs (#1194), не трогая
// уже существующие связи. Дедуп: повторная привязка - no-op успех.
func (s *employeeService) BulkAddTable(ctx context.Context, req EmployeeBulkAddTableRequest, actorID int) (*BulkOpResult, error) {
	tableIDs := uniqueInts(req.TableIDs)
	if len(tableIDs) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не выбраны таблицы")
	}
	if err := s.validatePeopleTables(ctx, tableIDs); err != nil {
		return nil, err
	}

	res := newBulkResult()
	changedIDs := make([]int, 0, len(req.IDs))
	for _, id := range uniqueInts(req.IDs) {
		employee, ok := s.loadEmployeeBasic(ctx, id)
		if !ok {
			res.addError(id, "", "Сотрудник не найден")
			continue
		}
		fullName := formatFullName(employee.LastName, employee.FirstName, employee.MiddleName)
		if employee.Status == nil || *employee.Status != 1 {
			res.addError(id, fullName, "Сотрудник не активен")
			continue
		}
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, tableID := range tableIDs {
				if err := s.bindEmployeeToTableIfMissing(ctx, tx, id, tableID, actorID, true); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			res.addError(id, fullName, bulkErrMsg(err))
			continue
		}
		changedIDs = append(changedIDs, id)
		res.SuccessCount++
	}
	s.tablesProducer.NotifyEmployeesChangedBatch(ctx, changedIDs)
	s.enqueueArchiveExportForEmployees(ctx, req.IDs)
	return res.finalize(), nil
}

// BulkUnbindTable снимает у набора сотрудников привязку к одной TableID (#1194).
// Если это была последняя привязка сотрудника - он деактивируется (status=0, зеркало
// одиночного commitDelete из PeopleTable.vue), с отдельной записью "delete" в истории.
func (s *employeeService) BulkUnbindTable(ctx context.Context, req EmployeeBulkUnbindTableRequest, actorID int) (*BulkOpResult, error) {
	if req.TableID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не указана таблица")
	}
	if err := s.validatePeopleTables(ctx, []int{req.TableID}); err != nil {
		return nil, err
	}
	names, err := s.loadTableNames(ctx, []int{req.TableID})
	if err != nil {
		return nil, err
	}
	tableName := names[req.TableID]
	tableID := req.TableID

	res := newBulkResult()
	for _, id := range uniqueInts(req.IDs) {
		employee, ok := s.loadEmployeeBasic(ctx, id)
		if !ok {
			res.addError(id, "", "Сотрудник не найден")
			continue
		}
		fullName := formatFullName(employee.LastName, employee.FirstName, employee.MiddleName)
		if employee.Status == nil || *employee.Status != 1 {
			res.addError(id, fullName, "Сотрудник не активен")
			continue
		}
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			del := tx.Where("employee_id = ? AND table_id = ?", id, tableID).Delete(&models.EmployeeTargetTable{})
			if del.Error != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка отвязки от таблицы")
			}
			if del.RowsAffected == 0 {
				return echo.NewHTTPError(http.StatusBadRequest, "Сотрудник не привязан к этой таблице")
			}

			comment := fmt.Sprintf("Сотрудник %s отвязан от таблицы «%s»", fullName, tableName)
			if err := s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &id, models.AuditActionUnboundFromTable, &actorID, carAuditDetails{Comment: &comment, TableID: &tableID}); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка записи истории")
			}

			var remaining int64
			if err := tx.Model(&models.EmployeeTargetTable{}).Where("employee_id = ?", id).Count(&remaining).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки оставшихся таблиц")
			}
			if remaining == 0 {
				now := time.Now().UTC()
				if err := tx.Model(&models.Employee{}).Where("id = ?", id).Updates(map[string]interface{}{
					"status":       0,
					"date_deleted": now,
					"updated_at":   now,
				}).Error; err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка деактивации сотрудника")
				}
				deactComment := fmt.Sprintf("Сотрудник %s удалён (снята последняя привязка к таблице)", fullName)
				if err := s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &id, "delete", &actorID, carAuditDetails{Comment: &deactComment, TableID: &tableID}); err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка записи истории деактивации")
				}
			}
			return nil
		})
		if err != nil {
			res.addError(id, fullName, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	// Явный id (не пост-состояние employee_target_tables, #1194 S6): после снятия
	// привязки таблица уже не содержит сотрудника, NotifyEmployeesChangedBatch её
	// аудиторию не увидела бы (см. TablesRefreshPublisher.NotifyTables) - её зрителям
	// нужен сигнал, чтобы строка live исчезла. Зеркало carService.BulkUnbindTable.
	s.tablesProducer.NotifyTables(ctx, []int{tableID})
	s.enqueueArchiveExportForEmployees(ctx, req.IDs)
	return res.finalize(), nil
}

// enqueueArchiveExportForEmployees резолвит сотрудников в заявки через их
// вложение и ставит заявки в очередь на пересборку файлового архива (#1615, B1):
// слепок заявки хранит посты каждого сотрудника, а bulk-операции меняют их в
// обход application_assignment_service, у которого свой enqueue после commit.
func (s *employeeService) enqueueArchiveExportForEmployees(ctx context.Context, employeeIDs []int) {
	if s.blankExports == nil {
		return
	}
	unique := uniqueInts(employeeIDs)
	if len(unique) == 0 {
		return
	}
	var appIDs []int
	err := s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT a.application_id
		FROM employees e
		JOIN attachments a ON a.id = e.attachment_id
		WHERE e.id IN ? AND a.application_id IS NOT NULL
	`, unique).Scan(&appIDs).Error
	if err != nil {
		slog.Warn("не удалось резолвить заявки для пересборки архива после bulk-операции с сотрудниками", "error", err)
		return
	}
	s.blankExports.EnqueueApplications(appIDs, BlankExportReasonUpdate)
}
