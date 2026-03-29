package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)


// ApplicationService определяет интерфейс бизнес-логики для работы с заявками.
type ApplicationService interface {
	// GetApplications возвращает список заявок для Центра заявок с фильтрацией.
	GetApplications(ctx context.Context, username string, filter ApplicationFilter) ([]ApplicationWithDetails, error)

	// GetUserApplications возвращает заявки текущего пользователя с фильтрацией.
	GetUserApplications(ctx context.Context, username string, filter ApplicationFilter) ([]ApplicationWithDetails, error)

	// GetApplicationByID возвращает заявку по ID с обновлением статуса при первом прочтении.
	GetApplicationByID(ctx context.Context, username string, applicationID int) (map[string]interface{}, error)

	// GetApplicationDetails возвращает расширенную информацию о заявке.
	GetApplicationDetails(ctx context.Context, applicationID int) (map[string]interface{}, error)

	// CreateApplication создаёт новую заявку.
	CreateApplication(ctx context.Context, username string, req ApplicationCreateRequest) (*ApplicationCreateResponse, error)

	// SubmitCompleteApplication создаёт полную заявку с вложениями.
	SubmitCompleteApplication(ctx context.Context, username string, req CompleteApplicationRequest) (*CompleteApplicationResponse, error)

	// UpdateApplication обновляет данные заявки.
	UpdateApplication(ctx context.Context, username string, applicationID int, req ApplicationUpdateRequest) (*ApplicationUpdateResponse, error)

	// ForwardApplication пересылает заявку ответственным/просматривающим.
	ForwardApplication(ctx context.Context, username string, applicationID int, req ForwardApplicationRequest) error

	// ApproveApplicationByUser согласование/отказ заявки пользователем.
	ApproveApplicationByUser(ctx context.Context, username string, applicationID int, req UserApprovalRequest) error

	// CheckApprovalStatus проверяет текущий статус согласования заявки.
	CheckApprovalStatus(ctx context.Context, applicationID int) (*ApprovalStatusResponse, error)

	// TakeApplicationToWork принятие заявки в работу или отказ.
	TakeApplicationToWork(ctx context.Context, username string, applicationID int, req TakeToWorkRequest) error

	// RevokeApplicationFromWork отзыв заявки из работы.
	RevokeApplicationFromWork(ctx context.Context, username string, applicationID int, req RevokeFromWorkRequest) error

	// RestoreApplicationToWork возврат заявки в обработку.
	RestoreApplicationToWork(ctx context.Context, username string, applicationID int, req RevokeFromWorkRequest) error

	// GetApplicationResponsibleUsers возвращает ответственных пользователей заявки.
	GetApplicationResponsibleUsers(ctx context.Context, applicationID int) ([]ResponsibleUserInfo, error)

	// GetApplicationHistory возвращает историю заявки.
	GetApplicationHistory(ctx context.Context, applicationID int) ([]ApplicationHistoryItem, error)

	// AddHistoryEntry добавляет запись в историю заявки.
	AddHistoryEntry(ctx context.Context, req AddHistoryEntryRequest) error

	// RevokeApproval отзывает ранее данное согласование.
	RevokeApproval(ctx context.Context, username string, applicationID int, req RevokeApprovalRequest) (*RevokeApprovalResponse, error)

	// GetApplicationViewers возвращает просматривающих заявки.
	GetApplicationViewers(ctx context.Context, applicationID int) ([]ViewerWithUser, error)

	// GetApplicationAttachments возвращает вложения заявки.
	GetApplicationAttachments(ctx context.Context, applicationID int) ([]AttachmentInfo, error)

	// GetAttachmentCars возвращает автомобили вложения.
	GetAttachmentCars(ctx context.Context, attachmentID int) ([]CarWithPlaces, error)

	// GetAttachmentEmployees возвращает сотрудников вложения.
	GetAttachmentEmployees(ctx context.Context, attachmentID int) ([]EmployeeWithTables, error)

	// GetAttachmentItems возвращает ТМЦ вложения.
	GetAttachmentItems(ctx context.Context, attachmentID int) ([]ItemInfo, error)

	// UpdateApplicationItemsStatus обновляет статусы всех машин и сотрудников заявки.
	UpdateApplicationItemsStatus(ctx context.Context, applicationID int) error

	// CheckExpiredAttachments проверяет и деактивирует истекшие вложения.
	CheckExpiredAttachments(ctx context.Context) error

	// MarkAsRead фиксирует прочтение заявки пользователем.
	MarkAsRead(ctx context.Context, applicationID int, username string) error

	// GetReads возвращает список пользователей, прочитавших заявку.
	GetReads(ctx context.Context, applicationID int) ([]models.ApplicationReadResponse, error)

	// GetUnreadCount возвращает количество непрочитанных заявок для пользователя.
	GetUnreadCount(ctx context.Context, username string) (*models.UnreadCountResponse, error)
}

// --- DTO: запросы ---

// ApplicationFilter параметры фильтрации списка заявок.
type ApplicationFilter struct {
	SearchQuery    *string `query:"search_query"`
	OrganizationID *int   `query:"organization_id"`
	CompanyID      *int   `query:"company_id"`
	Confirmation   *string `query:"confirmation"`
	Status         *string `query:"status"`
	DateFrom       *string `query:"date_from"`
	DateTo         *string `query:"date_to"`
	Archive        *bool   `query:"archive"`
}

// ApplicationCreateRequest тело запроса на создание простой заявки.
type ApplicationCreateRequest struct {
	OrganizationID *int    `json:"organization_id"`
	CompanyID      *int    `json:"company_id"`
	Message        *string `json:"message"`
	DataApproval   bool    `json:"data_approval"`
}

// CompleteApplicationRequest тело запроса на создание полной заявки с вложениями.
type CompleteApplicationRequest struct {
	Message           *string              `json:"message"`
	Organization      string               `json:"organization"`
	Company           *string              `json:"company"`
	ResponsiblePerson string               `json:"responsible_person"`
	ContactPhone      string               `json:"contact_phone"`
	DataApproval      bool                 `json:"data_approval"`
	Attachments       []AttachmentData     `json:"attachments"`
	RequiredUsers     *[]RequiredUserInput `json:"required_users"`
}

// AttachmentData данные вложения при создании заявки.
type AttachmentData struct {
	AttachmentType        string                `json:"attachment_type"`
	AttachmentName        string                `json:"attachment_name"`
	AttachmentDisplayName string                `json:"attachment_display_name"`
	UniqueAttachmentID    int                   `json:"unique_attachment_id"`
	EntryDateFrom         *string               `json:"entry_date_from"`
	EntryDateTo           *string               `json:"entry_date_to"`
	EntryTimeFrom         *string               `json:"entry_time_from"`
	EntryTimeTo           *string               `json:"entry_time_to"`
	Data                  AttachmentContentData  `json:"data"`
}

// AttachmentContentData содержимое вложения: машины, сотрудники или ТМЦ.
type AttachmentContentData struct {
	Vehicles  *[]VehicleInput  `json:"vehicles"`
	Employees *[]EmployeeInput `json:"employees"`
	Items     *[]ItemInput     `json:"items"`
}

// VehicleInput данные автомобиля при создании.
type VehicleInput struct {
	CarNumber    string `json:"car_number"`
	CarBrand     string `json:"car_brand"`
	UnloadPlace  *string `json:"unload_place"`
	UnloadPlaces []int  `json:"unload_places"`
}

// EmployeeInput данные сотрудника при создании.
type EmployeeInput struct {
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

// ItemInput данные ТМЦ при создании.
type ItemInput struct {
	Name       string `json:"name"`
	Count      int    `json:"count"`
	OrderIndex int    `json:"order_index"`
}

// RequiredUserInput обязательный пользователь при создании заявки.
type RequiredUserInput struct {
	UserID           int  `json:"user_id"`
	RequiredApproval bool `json:"required_approval"`
}

// ApplicationUpdateRequest тело запроса на обновление заявки.
type ApplicationUpdateRequest struct {
	Confirmation       *string `json:"confirmation"`
	Status             *string `json:"status"`
	ResponsibleComment *string `json:"responsible_comment"`
}

// ForwardApplicationRequest тело запроса на пересылку заявки.
type ForwardApplicationRequest struct {
	Users []ForwardUser `json:"users"`
}

// ForwardUser пользователь для пересылки. required_approval и can_view не могут быть оба true.
type ForwardUser struct {
	UserID           int  `json:"user_id"`
	RequiredApproval bool `json:"required_approval"`
	CanView          bool `json:"can_view"`
}

// UserApprovalRequest тело запроса на согласование заявки.
type UserApprovalRequest struct {
	UserID  int     `json:"user_id"`
	Status  string  `json:"status"`
	Comment *string `json:"comment"`
}

// TakeToWorkRequest тело запроса на принятие заявки в работу.
type TakeToWorkRequest struct {
	UserID  int     `json:"user_id"`
	Action  string  `json:"action"`
	Comment *string `json:"comment"`
}

// RevokeFromWorkRequest тело запроса на отзыв заявки из работы.
type RevokeFromWorkRequest struct {
	UserID  int     `json:"user_id"`
	Comment *string `json:"comment"`
}

// AddHistoryEntryRequest тело запроса на добавление записи в историю.
type AddHistoryEntryRequest struct {
	ApplicationID int              `json:"application_id"`
	UserID        int              `json:"user_id"`
	ActionType    string           `json:"action_type"`
	ActionStatus  *string          `json:"action_status"`
	OldValue      *string          `json:"old_value"`
	NewValue      *string          `json:"new_value"`
	Comment       *string          `json:"comment"`
	Metadata      *json.RawMessage `json:"metadata" swaggertype:"object"`
}

// RevokeApprovalRequest тело запроса на отзыв согласования.
type RevokeApprovalRequest struct {
	Comment *string `json:"comment"`
}

// --- DTO: ответы ---

// ApplicationWithDetails заявка с развёрнутой информацией для списков.
type ApplicationWithDetails struct {
	ID                   int        `json:"id"`
	ApplicationNumber    string     `json:"application_number"`
	Confirmation         string     `json:"confirmation"`
	SendingDatetime      time.Time  `json:"sending_datetime"`
	ReadingDatetime      *time.Time `json:"reading_datetime"`
	ConfirmationDatetime *time.Time `json:"confirmation_datetime"`
	OrganizationID       int        `json:"organization_id"`
	OrganizationName     string     `json:"organization_name"`
	CompanyID            *int       `json:"company_id"`
	CompanyName          string     `json:"company_name"`
	SenderUserID         int        `json:"sender_user_id"`
	SenderFullName       *string    `json:"sender_full_name"`
	SenderName           string     `json:"sender_name"`
	Message              *string    `json:"message"`
	Status               string     `json:"status"`
	ResponsibleUserID    *int       `json:"responsible_user_id"`
	ResponsibleFullName  *string    `json:"responsible_full_name"`
	ResponsibleName      string     `json:"responsible_name"`
	ResponsibleComment   *string    `json:"responsible_comment"`
	DataApproval         bool       `json:"data_approval"`
}

// ApplicationCreateResponse ответ при создании заявки.
type ApplicationCreateResponse struct {
	Success           bool   `json:"success"`
	Message           string `json:"message"`
	ApplicationID     int    `json:"application_id"`
	ApplicationNumber string `json:"application_number"`
}

// CompleteApplicationResponse ответ при создании полной заявки.
type CompleteApplicationResponse struct {
	Success           bool   `json:"success"`
	Message           string `json:"message"`
	ApplicationID     int    `json:"application_id"`
	ApplicationNumber string `json:"application_number"`
}

// ApplicationUpdateResponse ответ при обновлении заявки.
type ApplicationUpdateResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	RowsAffected int64  `json:"rows_affected"`
}

// ApprovalStatusResponse ответ проверки статуса согласования.
type ApprovalStatusResponse struct {
	Confirmation *string `json:"confirmation"`
	Status       *string `json:"status"`
}

// ResponsibleUserInfo информация об ответственном пользователе с данными согласования.
type ResponsibleUserInfo struct {
	ID               int        `json:"id"`
	Username         string     `json:"username"`
	LastName         *string    `json:"last_name"`
	FirstName        *string    `json:"first_name"`
	MiddleName       *string    `json:"middle_name"`
	Position         *string    `json:"position"`
	IsPrimary        bool       `json:"is_primary"`
	RequiredApproval bool       `json:"required_approval"`
	ApprovalStatus   *string    `json:"approval_status"`
	ApprovalComment  *string    `json:"approval_comment"`
	ApprovalDatetime *time.Time `json:"approval_datetime"`
}

// ApplicationHistoryItem запись истории заявки.
type ApplicationHistoryItem struct {
	ID            int              `json:"id"`
	ApplicationID int              `json:"application_id"`
	UserID        int              `json:"user_id"`
	UserName      string           `json:"user_name"`
	LastName      *string          `json:"last_name"`
	FirstName     *string          `json:"first_name"`
	MiddleName    *string          `json:"middle_name"`
	ActionType    string           `json:"action_type"`
	ActionStatus  *string          `json:"action_status"`
	OldValue      *string          `json:"old_value"`
	NewValue      *string          `json:"new_value"`
	Comment       *string          `json:"comment"`
	CreatedAt     time.Time        `json:"created_at"`
	Metadata      *json.RawMessage `json:"metadata" swaggertype:"object"`
}

// RevokeApprovalResponse ответ при отзыве согласования.
type RevokeApprovalResponse struct {
	Success      bool    `json:"success"`
	Message      string  `json:"message"`
	Confirmation *string `json:"confirmation"`
	Status       *string `json:"status"`
}

// ViewerWithUser просматривающий заявки с информацией о пользователе.
type ViewerWithUser struct {
	ID         int        `json:"id"`
	UserID     int        `json:"user_id"`
	Username   string     `json:"username"`
	LastName   *string    `json:"last_name"`
	FirstName  *string    `json:"first_name"`
	MiddleName *string    `json:"middle_name"`
	Position   *string    `json:"position"`
	CreatedAt  *time.Time `json:"created_at"`
}

// AttachmentInfo информация о вложении заявки.
type AttachmentInfo struct {
	ID                           int        `json:"id"`
	AttachmentType               string     `json:"attachment_type"`
	AttachmentName               string     `json:"attachment_name"`
	AttachmentDisplayName        string     `json:"attachment_display_name"`
	EntryDateFrom                *string    `json:"entry_date_from"`
	EntryDateTo                  *string    `json:"entry_date_to"`
	EntryTimeFrom                *string    `json:"entry_time_from"`
	EntryTimeTo                  *string    `json:"entry_time_to"`
	CreatedAt                    *time.Time `json:"created_at"`
	UniqueAttachmentID           *int       `json:"unique_attachment_id"`
	UniqueAttachmentTitle        *string    `json:"unique_attachment_title"`
	UniqueAttachmentDisplayName  *string    `json:"unique_attachment_display_name"`
}

// CarWithPlaces автомобиль с привязанными местами разгрузки.
type CarWithPlaces struct {
	ID            int              `json:"id"`
	CarNumber     string           `json:"car_number"`
	CarBrand      string           `json:"car_brand"`
	UnloadPlace   *string          `json:"unload_place"`
	EntryDateFrom *string          `json:"entry_date_from"`
	EntryTimeFrom *string          `json:"entry_time_from"`
	EntryDateTo   *string          `json:"entry_date_to"`
	EntryTimeTo   *string          `json:"entry_time_to"`
	UnloadPlaces  []UnloadPlaceRef `json:"unload_places"`
}

// UnloadPlaceRef ссылка на место разгрузки.
type UnloadPlaceRef struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// EmployeeWithTables сотрудник с привязанными таблицами.
type EmployeeWithTables struct {
	ID                   int            `json:"id"`
	LastName             string         `json:"last_name"`
	FirstName            string         `json:"first_name"`
	MiddleName           *string        `json:"middle_name"`
	Position             *string        `json:"position"`
	CitizenshipID        *int           `json:"citizenship_id"`
	PassportSeriesNumber *string        `json:"passport_series_number"`
	PatentNumber         *string        `json:"patent_number"`
	OtherPermission      *string        `json:"other_permission"`
	TargetTables         []TableInfoRef `json:"target_tables"`
}

// TableInfoRef ссылка на системную таблицу.
type TableInfoRef struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// ItemInfo информация о ТМЦ.
type ItemInfo struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	Count       int        `json:"count"`
	DateCreated *time.Time `json:"date_created"`
}

// --- Реализация ---

type applicationService struct {
	db *gorm.DB
}

// NewApplicationService создаёт экземпляр сервиса заявок.
func NewApplicationService(db *gorm.DB) ApplicationService {
	return &applicationService{db: db}
}

// --- Основные методы ---

func (s *applicationService) GetApplications(ctx context.Context, username string, filter ApplicationFilter) ([]ApplicationWithDetails, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	query := s.db.WithContext(ctx).Table("applications a").
		Select(`
			a.*,
			COALESCE(o.name, c.name) as organization_name,
			c.name as company_name,
			CONCAT(COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
			) as sender_full_name,
			CONCAT(COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || LEFT(u.first_name, 1) || '.' ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || LEFT(u.middle_name, 1) || '.' ELSE '' END
			) as sender_name,
			CONCAT(COALESCE(ru.last_name, ''),
				CASE WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN ' ' || ru.first_name ELSE '' END,
				CASE WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN ' ' || ru.middle_name ELSE '' END
			) as responsible_full_name,
			CONCAT(COALESCE(ru.last_name, ''),
				CASE WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN ' ' || LEFT(ru.first_name, 1) || '.' ELSE '' END,
				CASE WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN ' ' || LEFT(ru.middle_name, 1) || '.' ELSE '' END
			) as responsible_name
		`).
		Joins("LEFT JOIN organizations o ON a.organization_id = o.id").
		Joins("LEFT JOIN companies c ON a.company_id = c.id").
		Joins("LEFT JOIN users u ON a.sender_user_id = u.id").
		Joins("LEFT JOIN users ru ON a.responsible_user_id = ru.id")

	if !isApprover {
		query = query.Where(`
			EXISTS(SELECT 1 FROM application_responsible_users aru WHERE aru.application_id = a.id AND aru.user_id = ?)
			OR EXISTS(SELECT 1 FROM application_viewers av WHERE av.application_id = a.id AND av.user_id = ?)
		`, user.ID, user.ID)
	}

	query = applyApplicationFilters(query, filter, false)
	query = query.Order("a.sending_datetime DESC")

	rows := make([]ApplicationWithDetails, 0)
	if err := query.Find(&rows).Error; err != nil {
		slog.Error("Ошибка получения заявок", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	return rows, nil
}

func (s *applicationService) GetUserApplications(ctx context.Context, username string, filter ApplicationFilter) ([]ApplicationWithDetails, error) {
	if _, err := s.getUserByUsername(ctx, username); err != nil {
		return nil, err
	}

	query := s.db.WithContext(ctx).Table("applications a").
		Select(`
			a.*,
			COALESCE(o.name, c.name) as organization_name,
			c.name as company_name,
			CONCAT(COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
			) as sender_full_name,
			CONCAT(COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || LEFT(u.first_name, 1) || '.' ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || LEFT(u.middle_name, 1) || '.' ELSE '' END
			) as sender_name,
			CONCAT(COALESCE(ru.last_name, ''),
				CASE WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN ' ' || ru.first_name ELSE '' END,
				CASE WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN ' ' || ru.middle_name ELSE '' END
			) as responsible_full_name,
			CONCAT(COALESCE(ru.last_name, ''),
				CASE WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN ' ' || LEFT(ru.first_name, 1) || '.' ELSE '' END,
				CASE WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN ' ' || LEFT(ru.middle_name, 1) || '.' ELSE '' END
			) as responsible_name
		`).
		Joins("LEFT JOIN organizations o ON a.organization_id = o.id").
		Joins("LEFT JOIN companies c ON a.company_id = c.id").
		Joins("LEFT JOIN users u ON a.sender_user_id = u.id").
		Joins("LEFT JOIN users ru ON a.responsible_user_id = ru.id")

	query = applyApplicationFilters(query, filter, true)
	query = query.Order("a.sending_datetime DESC")

	rows := make([]ApplicationWithDetails, 0)
	if err := query.Find(&rows).Error; err != nil {
		slog.Error("Ошибка получения пользовательских заявок", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	return rows, nil
}


func (s *applicationService) GetApplicationByID(ctx context.Context, username string, applicationID int) (map[string]interface{}, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Получаем заявку с JOINами
	var row struct {
		models.Application
		OrganizationName    *string `gorm:"column:organization_name"`
		CompanyName         *string `gorm:"column:company_name"`
		SenderFullName      *string `gorm:"column:sender_full_name"`
		SenderName          *string `gorm:"column:sender_name"`
		ResponsibleFullName *string `gorm:"column:responsible_full_name"`
		ResponsibleName     *string `gorm:"column:responsible_name"`
	}

	result := tx.Table("applications a").
		Select(`
			a.*,
			COALESCE(o.name, c.name) as organization_name,
			c.name as company_name,
			CONCAT(COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
			) as sender_full_name,
			CONCAT(COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || LEFT(u.first_name, 1) || '.' ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || LEFT(u.middle_name, 1) || '.' ELSE '' END
			) as sender_name,
			CONCAT(COALESCE(ru.last_name, ''),
				CASE WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN ' ' || ru.first_name ELSE '' END,
				CASE WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN ' ' || ru.middle_name ELSE '' END
			) as responsible_full_name,
			CONCAT(COALESCE(ru.last_name, ''),
				CASE WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN ' ' || LEFT(ru.first_name, 1) || '.' ELSE '' END,
				CASE WHEN ru.middle_name IS NOT NULL AND ru.middle_name != '' THEN ' ' || LEFT(ru.middle_name, 1) || '.' ELSE '' END
			) as responsible_name
		`).
		Joins("LEFT JOIN organizations o ON a.organization_id = o.id").
		Joins("LEFT JOIN companies c ON a.company_id = c.id").
		Joins("LEFT JOIN users u ON a.sender_user_id = u.id").
		Joins("LEFT JOIN users ru ON a.responsible_user_id = ru.id").
		Where("a.id = ?", applicationID).
		First(&row)

	if result.Error != nil {
		tx.Rollback()
		if result.Error == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Application not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	// Обновляем статус при первом прочтении не отправителем
	if row.Status != nil && *row.Status == "Непрочитано" && row.SenderUserID != user.ID {
		if err := tx.Exec("UPDATE applications SET status = 'В обработке', reading_datetime = NOW() WHERE id = ?", applicationID).Error; err != nil {
			tx.Rollback()
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating application status")
		}
		// Записываем прочтение в историю
		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, old_value, new_value, created_at)
			VALUES (?, ?, 'read', 'Непрочитано', 'В обработке', NOW())
		`, applicationID, user.ID)
	}

	// Получаем ответственных
	responsibles, err := s.fetchResponsibleUsers(tx, applicationID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	orgName := ""
	if row.OrganizationName != nil {
		orgName = *row.OrganizationName
	}
	companyName := ""
	if row.CompanyName != nil {
		companyName = *row.CompanyName
	}
	senderName := ""
	if row.SenderName != nil {
		senderName = *row.SenderName
	}
	responsibleName := ""
	if row.ResponsibleName != nil {
		responsibleName = *row.ResponsibleName
	}

	response := map[string]interface{}{
		"id":                    row.ID,
		"application_number":    row.ApplicationNumber,
		"confirmation":          row.Confirmation,
		"sending_datetime":      row.SendingDatetime,
		"reading_datetime":      row.ReadingDatetime,
		"confirmation_datetime": row.ConfirmationDatetime,
		"organization_id":       row.OrganizationID,
		"organization_name":     orgName,
		"company_id":            row.CompanyID,
		"company_name":          companyName,
		"sender_user_id":        row.SenderUserID,
		"sender_full_name":      row.SenderFullName,
		"sender_name":           senderName,
		"message":               row.Message,
		"status":                row.Status,
		"responsible_user_id":   row.ResponsibleUserID,
		"responsible_full_name": row.ResponsibleFullName,
		"responsible_name":      responsibleName,
		"responsible_comment":   row.ResponsibleComment,
		"data_approval":         row.DataApproval,
		"responsible_users":     responsibles,
	}

	return response, nil
}

func (s *applicationService) GetApplicationDetails(ctx context.Context, applicationID int) (map[string]interface{}, error) {
	var row struct {
		models.Application
		OrganizationName    *string `gorm:"column:organization_name"`
		CompanyName         *string `gorm:"column:company_name"`
		SenderFullName      *string `gorm:"column:sender_full_name"`
		SenderName          *string `gorm:"column:sender_name"`
		ResponsibleFullName *string `gorm:"column:responsible_full_name"`
		ResponsibleName     *string `gorm:"column:responsible_name"`
	}

	result := s.db.WithContext(ctx).Table("applications a").
		Select(`
			a.*,
			COALESCE(o.name, c.name) as organization_name,
			c.name as company_name,
			CONCAT(COALESCE(u.last_name, ''), ' ', COALESCE(u.first_name, ''), ' ', COALESCE(u.middle_name, '')) as sender_full_name,
			CONCAT(COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || LEFT(u.first_name, 1) || '.' ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || LEFT(u.middle_name, 1) || '.' ELSE '' END
			) as sender_name,
			CONCAT(COALESCE(ru.last_name, ''), ' ', COALESCE(ru.first_name, ''), ' ', COALESCE(ru.middle_name, '')) as responsible_full_name,
			CONCAT(COALESCE(ru.last_name, ''),
				CASE WHEN ru.first_name IS NOT NULL AND ru.first_name != '' THEN ' ' || LEFT(ru.first_name, 1) || '.' ELSE '' END,
				CASE WHEN ru.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || LEFT(ru.middle_name, 1) || '.' ELSE '' END
			) as responsible_name
		`).
		Joins("LEFT JOIN organizations o ON a.organization_id = o.id").
		Joins("LEFT JOIN companies c ON a.company_id = c.id").
		Joins("LEFT JOIN users u ON a.sender_user_id = u.id").
		Joins("LEFT JOIN users ru ON a.responsible_user_id = ru.id").
		Where("a.id = ?", applicationID).
		First(&row)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Application not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	responsibles, _ := s.fetchResponsibleUsers(s.db.WithContext(ctx), applicationID)

	orgName := ""
	if row.OrganizationName != nil {
		orgName = *row.OrganizationName
	}
	companyName := ""
	if row.CompanyName != nil {
		companyName = *row.CompanyName
	}
	senderName := ""
	if row.SenderName != nil {
		senderName = *row.SenderName
	}
	responsibleName := ""
	if row.ResponsibleName != nil {
		responsibleName = *row.ResponsibleName
	}

	response := map[string]interface{}{
		"id":                    row.ID,
		"application_number":    row.ApplicationNumber,
		"confirmation":          row.Confirmation,
		"sending_datetime":      row.SendingDatetime,
		"reading_datetime":      row.ReadingDatetime,
		"confirmation_datetime": row.ConfirmationDatetime,
		"organization_id":       row.OrganizationID,
		"organization_name":     orgName,
		"company_id":            row.CompanyID,
		"company_name":          companyName,
		"sender_user_id":        row.SenderUserID,
		"sender_full_name":      row.SenderFullName,
		"sender_name":           senderName,
		"message":               row.Message,
		"status":                row.Status,
		"responsible_user_id":   row.ResponsibleUserID,
		"responsible_full_name": row.ResponsibleFullName,
		"responsible_name":      responsibleName,
		"responsible_comment":   row.ResponsibleComment,
		"data_approval":         row.DataApproval,
		"responsible_users":     responsibles,
	}

	return response, nil
}


func (s *applicationService) CreateApplication(ctx context.Context, username string, req ApplicationCreateRequest) (*ApplicationCreateResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	if !req.DataApproval {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Data approval is required")
	}

	now := time.Now().UTC()
	datePart := now.Format("20060102")

	var count int64
	s.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM applications WHERE DATE(sending_datetime AT TIME ZONE 'UTC') = ?", now.Format("2006-01-02")).Scan(&count)

	applicationNumber := fmt.Sprintf("№ %s/%03d", datePart, count+1)

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	app := models.Application{
		ApplicationNumber: &applicationNumber,
		OrganizationID:    safeDerefInt(req.OrganizationID),
		CompanyID:         req.CompanyID,
		SenderUserID:      user.ID,
		Message:           req.Message,
		DataApproval:      ptrString("true"),
		Status:            ptrString("Непрочитано"),
		Confirmation:      ptrString("Согласование"),
		SendingDatetime:   &now,
	}

	if err := tx.Create(&app).Error; err != nil {
		tx.Rollback()
		slog.Error("Ошибка создания заявки", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating application")
	}

	// Собираем ответственных из организации и компании
	type respUser struct {
		UserID    int
		IsPrimary bool
	}
	var responsibleUsers []respUser
	var primaryResponsibleID *int

	if req.OrganizationID != nil {
		var orgResp []struct {
			UserID    int  `gorm:"column:user_id"`
			IsPrimary bool `gorm:"column:is_primary"`
		}
		tx.Raw("SELECT user_id, COALESCE(is_primary, false) as is_primary FROM organization_users WHERE organization_id = ?", *req.OrganizationID).Scan(&orgResp)
		for _, r := range orgResp {
			responsibleUsers = append(responsibleUsers, respUser{r.UserID, r.IsPrimary})
			if r.IsPrimary {
				primaryResponsibleID = &r.UserID
			}
		}
	}

	if req.CompanyID != nil {
		var compResp []struct {
			UserID    int  `gorm:"column:user_id"`
			IsPrimary bool `gorm:"column:is_primary"`
		}
		tx.Raw("SELECT user_id, COALESCE(is_primary, false) as is_primary FROM companies_users WHERE company_id = ?", *req.CompanyID).Scan(&compResp)
		for _, r := range compResp {
			exists := false
			for _, ru := range responsibleUsers {
				if ru.UserID == r.UserID {
					exists = true
					break
				}
			}
			if !exists {
				responsibleUsers = append(responsibleUsers, respUser{r.UserID, r.IsPrimary})
				if r.IsPrimary && primaryResponsibleID == nil {
					primaryResponsibleID = &r.UserID
				}
			}
		}
	}

	if primaryResponsibleID != nil {
		tx.Exec("UPDATE applications SET responsible_user_id = ? WHERE id = ?", *primaryResponsibleID, app.ID)
	}

	for _, ru := range responsibleUsers {
		tx.Exec(`
			INSERT INTO application_responsible_users (application_id, user_id, is_primary, approval_status, created_at)
			VALUES (?, ?, ?, 'pending', NOW())
			ON CONFLICT (application_id, user_id) DO NOTHING
		`, app.ID, ru.UserID, ru.IsPrimary)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	return &ApplicationCreateResponse{
		Success:           true,
		Message:           "Application created successfully",
		ApplicationID:     app.ID,
		ApplicationNumber: applicationNumber,
	}, nil
}

func (s *applicationService) SubmitCompleteApplication(ctx context.Context, username string, req CompleteApplicationRequest) (*CompleteApplicationResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	if !req.DataApproval {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Data approval is required")
	}
	if len(req.Attachments) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "At least one attachment is required")
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	baseTime := time.Now().UTC()
	historyTime := baseTime
	datePart := baseTime.Format("20060102")

	var count int64
	tx.Raw("SELECT COUNT(*) FROM applications WHERE DATE(sending_datetime AT TIME ZONE 'UTC') = ?", baseTime.Format("2006-01-02")).Scan(&count)
	applicationNumber := fmt.Sprintf("№ %s/%03d", datePart, count+1)

	// Получаем ID организации по имени
	var organizationID *int
	var orgRow struct{ ID int }
	if err := tx.Raw("SELECT id FROM organizations WHERE name = ?", req.Organization).Scan(&orgRow).Error; err == nil && orgRow.ID != 0 {
		organizationID = &orgRow.ID
	}

	// Получаем ID компании по имени
	var companyID *int
	if req.Company != nil {
		var compRow struct{ ID int }
		if err := tx.Raw("SELECT id FROM companies WHERE name = ?", *req.Company).Scan(&compRow).Error; err == nil && compRow.ID != 0 {
			companyID = &compRow.ID
		}
	}

	// Создаём заявку
	var appID int
	err = tx.Raw(`
		INSERT INTO applications (application_number, organization_id, company_id, sender_user_id, message, data_approval, status, confirmation, sending_datetime)
		VALUES (?, ?, ?, ?, ?, ?, 'Непрочитано', 'Согласование', ?)
		RETURNING id
	`, applicationNumber, organizationID, companyID, user.ID, req.Message, fmt.Sprintf("%v", req.DataApproval), baseTime).Scan(&appID).Error
	if err != nil {
		tx.Rollback()
		slog.Error("Ошибка создания заявки", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating application")
	}

	// Записываем создание в историю
	metaCreate, _ := json.Marshal(map[string]string{"confirmation": "Согласование", "status": "Непрочитано"})
	tx.Exec(`
		INSERT INTO application_history (application_id, user_id, action_type, new_value, metadata, created_at)
		VALUES (?, ?, 'create', ?, ?, ?)
	`, appID, user.ID, applicationNumber, string(metaCreate), historyTime)

	// Собираем ответственных
	type respUser struct {
		UserID           int
		IsPrimary        bool
		RequiredApproval bool
	}
	var responsibleUsers []respUser
	var primaryResponsibleID *int

	if organizationID != nil {
		var orgResp []struct {
			UserID           int  `gorm:"column:user_id"`
			IsPrimary        bool `gorm:"column:is_primary"`
			RequiredApproval bool `gorm:"column:required_approval"`
		}
		tx.Raw("SELECT user_id, COALESCE(is_primary, false) as is_primary, required_approval FROM organization_users WHERE organization_id = ?", *organizationID).Scan(&orgResp)
		for _, r := range orgResp {
			responsibleUsers = append(responsibleUsers, respUser{r.UserID, r.IsPrimary, r.RequiredApproval})
			if r.IsPrimary {
				primaryResponsibleID = &r.UserID
			}
		}
	}

	if companyID != nil {
		var compResp []struct {
			UserID           int  `gorm:"column:user_id"`
			IsPrimary        bool `gorm:"column:is_primary"`
			RequiredApproval bool `gorm:"column:required_approval"`
		}
		tx.Raw("SELECT user_id, COALESCE(is_primary, false) as is_primary, required_approval FROM companies_users WHERE company_id = ?", *companyID).Scan(&compResp)
		for _, r := range compResp {
			exists := false
			for _, ru := range responsibleUsers {
				if ru.UserID == r.UserID {
					exists = true
					break
				}
			}
			if !exists {
				responsibleUsers = append(responsibleUsers, respUser{r.UserID, r.IsPrimary, r.RequiredApproval})
				if r.IsPrimary && primaryResponsibleID == nil {
					primaryResponsibleID = &r.UserID
				}
			}
		}
	}

	// Добавляем обязательных из запроса
	if req.RequiredUsers != nil {
		for _, reqUser := range *req.RequiredUsers {
			exists := false
			for i, ru := range responsibleUsers {
				if ru.UserID == reqUser.UserID {
					exists = true
					responsibleUsers[i].RequiredApproval = reqUser.RequiredApproval
					break
				}
			}
			if !exists {
				responsibleUsers = append(responsibleUsers, respUser{reqUser.UserID, false, reqUser.RequiredApproval})
			}
		}
	}

	if primaryResponsibleID != nil {
		tx.Exec("UPDATE applications SET responsible_user_id = ? WHERE id = ?", *primaryResponsibleID, appID)
	}

	for _, ru := range responsibleUsers {
		tx.Exec(`
			INSERT INTO application_responsible_users (application_id, user_id, is_primary, required_approval, approval_status, created_at)
			VALUES (?, ?, ?, ?, 'pending', ?)
			ON CONFLICT (application_id, user_id) DO UPDATE SET is_primary = EXCLUDED.is_primary, required_approval = EXCLUDED.required_approval
		`, appID, ru.UserID, ru.IsPrimary, ru.RequiredApproval, baseTime)

		historyTime = historyTime.Add(time.Millisecond)
		meta, _ := json.Marshal(map[string]interface{}{
			"required_approval": ru.RequiredApproval,
			"is_primary":        ru.IsPrimary,
		})
		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, metadata, created_at)
			VALUES (?, ?, 'assigned_responsible', ?, ?)
		`, appID, ru.UserID, string(meta), historyTime)
	}

	// Создаём вложения
	for _, att := range req.Attachments {
		var attID int
		err := tx.Raw(`
			INSERT INTO attachments (application_id, attachment_type, attachment_name, attachment_display_name, unique_attachment_id, entry_date_from, entry_date_to, entry_time_from, entry_time_to, status)
			VALUES (?, ?, ?, ?, ?, ?::date, ?::date, ?::time, ?::time, 1)
			RETURNING id
		`, appID, att.AttachmentType, att.AttachmentName, att.AttachmentDisplayName, att.UniqueAttachmentID,
			att.EntryDateFrom, att.EntryDateTo, att.EntryTimeFrom, att.EntryTimeTo).Scan(&attID).Error
		if err != nil {
			tx.Rollback()
			slog.Error("Ошибка создания вложения", "error", err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating attachment")
		}

		switch att.AttachmentType {
		case "cars":
			if att.Data.Vehicles != nil {
				for _, v := range *att.Data.Vehicles {
					var carID int
					err := tx.Raw(`
						INSERT INTO cars (attachment_id, car_number, car_brand, unload_place, entry_date_from, entry_time_from, entry_date_to, entry_time_to, status)
						VALUES (?, ?, ?, ?, ?::date, ?::time, ?::date, ?::time, 0)
						RETURNING id
					`, attID, v.CarNumber, v.CarBrand, v.UnloadPlace, att.EntryDateFrom, att.EntryTimeFrom, att.EntryDateTo, att.EntryTimeTo).Scan(&carID).Error
					if err != nil {
						tx.Rollback()
						return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating car")
					}

					carHistoryTime := baseTime.Add(time.Millisecond)
					tx.Exec(`
						INSERT INTO cars_history (car_id, user_id, action_type, comment, created_at)
						VALUES (?, ?, 'create', ?, ?)
					`, carID, user.ID, fmt.Sprintf("Автомобиль %s %s создан", v.CarNumber, v.CarBrand), carHistoryTime)

					for _, placeID := range v.UnloadPlaces {
						tx.Exec("INSERT INTO car_unload_places (car_id, unload_place_id, order_index) VALUES (?, ?, 1)", carID, placeID)
					}
				}
			}

		case "people":
			if att.Data.Employees != nil {
				for _, e := range *att.Data.Employees {
					var empID int
					err := tx.Raw(`
						INSERT INTO employees (attachment_id, last_name, first_name, middle_name, citizenship_id, position, passport_series_number, patent_number, other_permission, status)
						VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
						RETURNING id
					`, attID, e.LastName, e.FirstName, e.MiddleName, e.CitizenshipID, e.Position, e.PassportSeriesNumber, e.PatentNumber, e.OtherPermission).Scan(&empID).Error
					if err != nil {
						tx.Rollback()
						return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating employee")
					}

					for _, tableID := range e.TargetTables {
						tx.Exec("INSERT INTO employee_target_tables (employee_id, table_id, order_index) VALUES (?, ?, 1)", empID, tableID)
					}
				}
			}

		case "items":
			if att.Data.Items != nil {
				for _, item := range *att.Data.Items {
					tx.Exec(`
						INSERT INTO items (attachment_id, name, count, date_created)
						VALUES (?, ?, ?, ?)
					`, attID, item.Name, item.Count, baseTime.Format("2006-01-02"))
				}
			}

		default:
			tx.Rollback()
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment type")
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	return &CompleteApplicationResponse{
		Success:           true,
		Message:           "Application created successfully",
		ApplicationID:     appID,
		ApplicationNumber: applicationNumber,
	}, nil
}

func (s *applicationService) UpdateApplication(ctx context.Context, username string, applicationID int, req ApplicationUpdateRequest) (*ApplicationUpdateResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	setClauses := []string{}
	args := []interface{}{}

	if req.Confirmation != nil {
		setClauses = append(setClauses, "confirmation = ?")
		args = append(args, *req.Confirmation)
		if *req.Confirmation == "Согласовано" || *req.Confirmation == "Не согласовано" {
			setClauses = append(setClauses, "confirmation_datetime = ?")
			args = append(args, now)
		}
	}

	if req.Status != nil {
		setClauses = append(setClauses, "status = ?")
		args = append(args, *req.Status)
		if *req.Status == "В обработке" {
			setClauses = append(setClauses, "reading_datetime = ?")
			args = append(args, now)
		}
	}

	if req.ResponsibleComment != nil {
		setClauses = append(setClauses, "responsible_comment = ?")
		args = append(args, *req.ResponsibleComment)
		setClauses = append(setClauses, "responsible_user_id = ?")
		args = append(args, user.ID)
	}

	if len(setClauses) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "No data to update")
	}

	sqlQuery := fmt.Sprintf("UPDATE applications SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	args = append(args, applicationID)

	result := s.db.WithContext(ctx).Exec(sqlQuery, args...)
	if result.Error != nil {
		slog.Error("Ошибка обновления заявки", "application_id", applicationID, "error", result.Error)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Error updating application: %v", result.Error))
	}

	return &ApplicationUpdateResponse{
		Success:      true,
		Message:      "Application updated successfully",
		RowsAffected: result.RowsAffected,
	}, nil
}

func (s *applicationService) GetApplicationViewers(ctx context.Context, applicationID int) ([]ViewerWithUser, error) {
	viewers := make([]ViewerWithUser, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			av.id,
			av.user_id,
			u.username,
			u.last_name,
			u.first_name,
			u.middle_name,
			u.position,
			av.created_at
		FROM application_viewers av
		JOIN users u ON av.user_id = u.id
		WHERE av.application_id = ?
		ORDER BY u.last_name, u.first_name
	`, applicationID).Scan(&viewers).Error

	if err != nil {
		slog.Error("Ошибка получения просматривающих", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching viewers")
	}

	return viewers, nil
}

func (s *applicationService) GetApplicationAttachments(ctx context.Context, applicationID int) ([]AttachmentInfo, error) {
	attachments := make([]AttachmentInfo, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			a.id,
			a.attachment_type,
			a.attachment_name,
			COALESCE(a.attachment_display_name, '') as attachment_display_name,
			a.entry_date_from,
			a.entry_date_to,
			a.entry_time_from,
			a.entry_time_to,
			a.created_at,
			a.unique_attachment_id,
			ua.title as unique_attachment_title,
			ua.display_name as unique_attachment_display_name
		FROM attachments a
		LEFT JOIN unique_attachments ua ON a.unique_attachment_id = ua.id
		WHERE a.application_id = ?
		ORDER BY ua.title, a.created_at
	`, applicationID).Scan(&attachments).Error

	if err != nil {
		slog.Error("Ошибка получения вложений", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachments")
	}

	return attachments, nil
}

func (s *applicationService) GetAttachmentCars(ctx context.Context, attachmentID int) ([]CarWithPlaces, error) {
	type carRow struct {
		ID            int
		CarNumber     string  `gorm:"column:car_number"`
		CarBrand      string  `gorm:"column:car_brand"`
		UnloadPlace   *string `gorm:"column:unload_place"`
		EntryDateFrom *string `gorm:"column:entry_date_from"`
		EntryTimeFrom *string `gorm:"column:entry_time_from"`
		EntryDateTo   *string `gorm:"column:entry_date_to"`
		EntryTimeTo   *string `gorm:"column:entry_time_to"`
	}
	cars := make([]carRow, 0)
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, car_number, car_brand, unload_place, entry_date_from, entry_time_from, entry_date_to, entry_time_to
		FROM cars WHERE attachment_id = ?
	`, attachmentID).Scan(&cars).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching cars")
	}

	result := make([]CarWithPlaces, 0)
	for _, car := range cars {
		places := make([]UnloadPlaceRef, 0)
		s.db.WithContext(ctx).Raw(`
			SELECT up.id, up.name, up.description
			FROM car_unload_places cup
			JOIN unload_places up ON cup.unload_place_id = up.id
			WHERE cup.car_id = ?
			ORDER BY cup.order_index
		`, car.ID).Scan(&places)

		result = append(result, CarWithPlaces{
			ID:            car.ID,
			CarNumber:     car.CarNumber,
			CarBrand:      car.CarBrand,
			UnloadPlace:   car.UnloadPlace,
			EntryDateFrom: car.EntryDateFrom,
			EntryTimeFrom: car.EntryTimeFrom,
			EntryDateTo:   car.EntryDateTo,
			EntryTimeTo:   car.EntryTimeTo,
			UnloadPlaces:  places,
		})
	}

	return result, nil
}

func (s *applicationService) GetAttachmentEmployees(ctx context.Context, attachmentID int) ([]EmployeeWithTables, error) {
	type empRow struct {
		ID                   int
		LastName             string  `gorm:"column:last_name"`
		FirstName            string  `gorm:"column:first_name"`
		MiddleName           *string `gorm:"column:middle_name"`
		Position             *string `gorm:"column:position"`
		CitizenshipID        *int    `gorm:"column:citizenship_id"`
		PassportSeriesNumber *string `gorm:"column:passport_series_number"`
		PatentNumber         *string `gorm:"column:patent_number"`
		OtherPermission      *string `gorm:"column:other_permission"`
	}
	employees := make([]empRow, 0)
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, last_name, first_name, middle_name, position, citizenship_id, passport_series_number, patent_number, other_permission
		FROM employees WHERE attachment_id = ?
	`, attachmentID).Scan(&employees).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employees")
	}

	result := make([]EmployeeWithTables, 0)
	for _, emp := range employees {
		tables := make([]TableInfoRef, 0)
		s.db.WithContext(ctx).Raw(`
			SELECT st.id, st.name, st.display_name
			FROM employee_target_tables ett
			JOIN system_tables st ON ett.table_id = st.id
			WHERE ett.employee_id = ?
			ORDER BY ett.order_index
		`, emp.ID).Scan(&tables)

		result = append(result, EmployeeWithTables{
			ID:                   emp.ID,
			LastName:             emp.LastName,
			FirstName:            emp.FirstName,
			MiddleName:           emp.MiddleName,
			Position:             emp.Position,
			CitizenshipID:        emp.CitizenshipID,
			PassportSeriesNumber: emp.PassportSeriesNumber,
			PatentNumber:         emp.PatentNumber,
			OtherPermission:      emp.OtherPermission,
			TargetTables:         tables,
		})
	}

	return result, nil
}

func (s *applicationService) GetAttachmentItems(ctx context.Context, attachmentID int) ([]ItemInfo, error) {
	items := make([]ItemInfo, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT id, name, count, date_created
		FROM items WHERE attachment_id = ?
		ORDER BY id
	`, attachmentID).Scan(&items).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching items")
	}
	return items, nil
}
