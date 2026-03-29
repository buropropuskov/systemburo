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

// --- Вспомогательные методы ---

// getUserByUsername возвращает пользователя по username.
func (s *applicationService) getUserByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusUnauthorized, "User not found")
		}
		slog.Error("Ошибка получения пользователя", "username", username, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return &user, nil
}

// isApprover проверяет, является ли пользователь принимающим.
func (s *applicationService) isApprover(ctx context.Context, userID int) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.ApplicationApprover{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		slog.Error("Ошибка проверки approver", "user_id", userID, "error", err)
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return count > 0, nil
}

// formatFullName формирует полное ФИО.
func formatFullName(lastName, firstName, middleName *string) string {
	parts := []string{}
	if lastName != nil && *lastName != "" {
		parts = append(parts, *lastName)
	}
	if firstName != nil && *firstName != "" {
		parts = append(parts, *firstName)
	}
	if middleName != nil && *middleName != "" {
		parts = append(parts, *middleName)
	}
	return strings.Join(parts, " ")
}

// formatShortName формирует сокращённое ФИО (Фамилия И. О.).
func formatShortName(lastName, firstName, middleName *string) string {
	result := ""
	if lastName != nil && *lastName != "" {
		result = *lastName
	}
	if firstName != nil && *firstName != "" {
		result += " " + string([]rune(*firstName)[:1]) + "."
	}
	if middleName != nil && *middleName != "" {
		result += " " + string([]rune(*middleName)[:1]) + "."
	}
	return strings.TrimSpace(result)
}

// updateConfirmationBasedOnApprovals пересчитывает confirmation заявки по голосам ответственных.
// Правила:
// 1. Обязательный ответственный отказал -> "Не согласовано"
// 2. Все обязательные согласовали -> "Согласовано"
// 3. Нет обязательных и хотя бы один обычный согласовал (без отказов) -> "Согласовано"
// 4. Нет обязательных и есть отказ -> "Не согласовано"
// 5. Иначе -> "Согласование"
func (s *applicationService) updateConfirmationBasedOnApprovals(tx *gorm.DB, applicationID int) error {
	var responsibles []models.ApplicationResponsibleUser
	if err := tx.Where("application_id = ?", applicationID).Find(&responsibles).Error; err != nil {
		slog.Error("Ошибка получения ответственных", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching responsible users")
	}

	if len(responsibles) == 0 {
		return nil
	}

	var required, nonRequired []models.ApplicationResponsibleUser
	for _, r := range responsibles {
		if r.RequiredApproval {
			required = append(required, r)
		} else {
			nonRequired = append(nonRequired, r)
		}
	}

	newConfirmation := "Согласование"

	hasRequiredRejected := false
	for _, r := range required {
		if r.ApprovalStatus != nil && *r.ApprovalStatus == "rejected" {
			hasRequiredRejected = true
			break
		}
	}

	if hasRequiredRejected {
		newConfirmation = "Не согласовано"
	} else if len(required) > 0 {
		allApproved := true
		for _, r := range required {
			if r.ApprovalStatus == nil || *r.ApprovalStatus != "approved" {
				allApproved = false
				break
			}
		}
		if allApproved {
			newConfirmation = "Согласовано"
		}
	} else if len(nonRequired) > 0 {
		hasAnyApproved := false
		hasAnyRejected := false
		for _, r := range nonRequired {
			if r.ApprovalStatus != nil && *r.ApprovalStatus == "approved" {
				hasAnyApproved = true
			}
			if r.ApprovalStatus != nil && *r.ApprovalStatus == "rejected" {
				hasAnyRejected = true
			}
		}
		if hasAnyApproved && !hasAnyRejected {
			newConfirmation = "Согласовано"
		} else if hasAnyRejected {
			newConfirmation = "Не согласовано"
		}
	}

	result := tx.Exec(`
		UPDATE applications
		SET confirmation = ?,
		    confirmation_datetime = CASE
		        WHEN ? != 'Согласование' AND confirmation_datetime IS NULL THEN NOW()
		        ELSE confirmation_datetime
		    END
		WHERE id = ?
	`, newConfirmation, newConfirmation, applicationID)

	if result.Error != nil {
		slog.Error("Ошибка обновления confirmation", "application_id", applicationID, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating application confirmation")
	}

	return nil
}

// activateApplicationItems активирует/деактивирует машины и сотрудников заявки.
func (s *applicationService) activateApplicationItems(tx *gorm.DB, applicationID int, activate bool) error {
	newStatus := 0
	if activate {
		newStatus = 1
	}

	type attachmentRow struct {
		ID             int
		AttachmentType string
	}
	var attachments []attachmentRow
	if err := tx.Raw("SELECT id, attachment_type FROM attachments WHERE application_id = ?", applicationID).Scan(&attachments).Error; err != nil {
		slog.Error("Ошибка получения вложений", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachments")
	}

	for _, att := range attachments {
		switch att.AttachmentType {
		case "cars":
			if err := tx.Exec("UPDATE cars SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ?", newStatus, att.ID).Error; err != nil {
				slog.Error("Ошибка обновления статуса машин", "attachment_id", att.ID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating cars status")
			}
		case "people":
			if err := tx.Exec("UPDATE employees SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ?", newStatus, att.ID).Error; err != nil {
				slog.Error("Ошибка обновления статуса сотрудников", "attachment_id", att.ID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating employees status")
			}
		}
	}

	return nil
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

// applyApplicationFilters накладывает фильтры на запрос заявок.
func applyApplicationFilters(query *gorm.DB, filter ApplicationFilter, includeUserSearch bool) *gorm.DB {
	if filter.SearchQuery != nil && *filter.SearchQuery != "" {
		pattern := "%" + *filter.SearchQuery + "%"
		if includeUserSearch {
			query = query.Where(`
				a.application_number ILIKE ? OR
				COALESCE(o.name, c.name, '') ILIKE ? OR
				c.name ILIKE ? OR
				a.message ILIKE ? OR
				a.status ILIKE ? OR
				a.confirmation ILIKE ? OR
				u.last_name ILIKE ? OR u.first_name ILIKE ? OR u.middle_name ILIKE ? OR
				ru.last_name ILIKE ? OR ru.first_name ILIKE ? OR ru.middle_name ILIKE ?
			`, pattern, pattern, pattern, pattern, pattern, pattern,
				pattern, pattern, pattern, pattern, pattern, pattern)
		} else {
			query = query.Where(`
				a.application_number ILIKE ? OR
				COALESCE(o.name, c.name, '') ILIKE ? OR
				c.name ILIKE ? OR
				a.message ILIKE ? OR
				a.status ILIKE ? OR
				a.confirmation ILIKE ?
			`, pattern, pattern, pattern, pattern, pattern, pattern)
		}
	}

	if filter.OrganizationID != nil {
		query = query.Where("a.organization_id = ?", *filter.OrganizationID)
	}
	if filter.CompanyID != nil {
		query = query.Where("a.company_id = ?", *filter.CompanyID)
	}
	if filter.Confirmation != nil {
		query = query.Where("a.confirmation = ?", *filter.Confirmation)
	}
	if filter.Status != nil {
		query = query.Where("a.status = ?", *filter.Status)
	}
	if filter.DateFrom != nil && *filter.DateFrom != "" {
		query = query.Where("a.sending_datetime >= ?", *filter.DateFrom+" 00:00:00")
	}
	if filter.DateTo != nil && *filter.DateTo != "" {
		query = query.Where("a.sending_datetime <= ?", *filter.DateTo+" 23:59:59")
	}

	return query
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

// fetchResponsibleUsers возвращает ответственных пользователей заявки с информацией о согласовании.
func (s *applicationService) fetchResponsibleUsers(db *gorm.DB, applicationID int) ([]ResponsibleUserInfo, error) {
	responsibles := make([]ResponsibleUserInfo, 0)
	err := db.Raw(`
		SELECT
			u.id,
			u.username,
			u.last_name,
			u.first_name,
			u.middle_name,
			u.position,
			COALESCE(aru.is_primary, false) as is_primary,
			COALESCE(aru.required_approval, false) as required_approval,
			aru.approval_status,
			aru.approval_comment,
			aru.approval_datetime
		FROM application_responsible_users aru
		JOIN users u ON aru.user_id = u.id
		WHERE aru.application_id = ?
		ORDER BY aru.is_primary DESC, u.last_name, u.first_name
	`, applicationID).Scan(&responsibles).Error

	if err != nil {
		slog.Error("Ошибка получения ответственных пользователей", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching responsible users")
	}

	if responsibles == nil {
		responsibles = []ResponsibleUserInfo{}
	}
	return responsibles, nil
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

func (s *applicationService) ForwardApplication(ctx context.Context, username string, applicationID int, req ForwardApplicationRequest) error {
	var user struct {
		ID         int
		LastName   *string
		FirstName  *string
		MiddleName *string
	}
	if err := s.db.WithContext(ctx).Raw("SELECT id, last_name, first_name, middle_name FROM users WHERE username = ?", username).Scan(&user).Error; err != nil || user.ID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}
	currentUserName := formatFullName(user.LastName, user.FirstName, user.MiddleName)

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Проверяем существование заявки
	var exists bool
	tx.Raw("SELECT EXISTS(SELECT 1 FROM applications WHERE id = ?)", applicationID).Scan(&exists)
	if !exists {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}

	// Проверяем права на пересылку
	var canForward bool
	tx.Raw(`
		SELECT EXISTS(
			SELECT 1 FROM applications a WHERE a.id = ? AND (
				a.sender_user_id = ?
				OR EXISTS(SELECT 1 FROM application_responsible_users aru WHERE aru.application_id = a.id AND aru.user_id = ?)
			)
		)
	`, applicationID, user.ID, user.ID).Scan(&canForward)
	if !canForward {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusForbidden, "You don't have permission to forward this application")
	}

	// Сохраняем старый confirmation
	var oldConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&oldConfirmation)

	baseTime := time.Now().UTC()
	historyTime := baseTime

	type addedResp struct {
		UserID           int
		RequiredApproval bool
	}
	var addedResponsibleUsers []addedResp
	var addedViewers []int

	for _, fu := range req.Users {
		// Проверяем существование пользователя
		var userExists bool
		tx.Raw("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", fu.UserID).Scan(&userExists)
		if !userExists {
			continue
		}

		if fu.RequiredApproval && fu.CanView {
			continue
		}

		if fu.RequiredApproval || !fu.CanView {
			// Ответственный пользователь
			var alreadyAdded bool
			tx.Raw("SELECT EXISTS(SELECT 1 FROM application_responsible_users WHERE application_id = ? AND user_id = ?)", applicationID, fu.UserID).Scan(&alreadyAdded)

			if alreadyAdded {
				tx.Exec("UPDATE application_responsible_users SET required_approval = ?, created_by = ? WHERE application_id = ? AND user_id = ?",
					fu.RequiredApproval, user.ID, applicationID, fu.UserID)
			} else {
				tx.Exec(`
					INSERT INTO application_responsible_users (application_id, user_id, required_approval, approval_status, created_at, created_by, is_primary)
					VALUES (?, ?, ?, 'pending', ?, ?, false)
				`, applicationID, fu.UserID, fu.RequiredApproval, baseTime, user.ID)
			}
			addedResponsibleUsers = append(addedResponsibleUsers, addedResp{fu.UserID, fu.RequiredApproval})
		} else {
			// Просматривающий
			var alreadyAdded bool
			tx.Raw("SELECT EXISTS(SELECT 1 FROM application_viewers WHERE application_id = ? AND user_id = ?)", applicationID, fu.UserID).Scan(&alreadyAdded)

			if !alreadyAdded {
				tx.Exec(`
					INSERT INTO application_viewers (application_id, user_id, created_at, created_by)
					VALUES (?, ?, ?, ?)
				`, applicationID, fu.UserID, baseTime, user.ID)
			}
			addedViewers = append(addedViewers, fu.UserID)
		}
	}

	// Записываем историю назначений ответственных
	for _, resp := range addedResponsibleUsers {
		historyTime = historyTime.Add(time.Millisecond)
		meta, _ := json.Marshal(map[string]interface{}{
			"required_approval": resp.RequiredApproval,
			"is_primary":        false,
			"forwarded_by":      currentUserName,
			"type":              "responsible",
		})
		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, metadata, created_at)
			VALUES (?, ?, 'assigned_responsible', ?, ?)
		`, applicationID, resp.UserID, string(meta), historyTime)
	}

	// Записываем историю назначений просматривающих
	for _, viewerID := range addedViewers {
		historyTime = historyTime.Add(time.Millisecond)
		meta, _ := json.Marshal(map[string]interface{}{
			"forwarded_by": currentUserName,
			"type":         "viewer",
		})
		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, metadata, created_at)
			VALUES (?, ?, 'assigned_viewer', ?, ?)
		`, applicationID, viewerID, string(meta), historyTime)
	}

	// Обновляем confirmation если были добавлены ответственные
	if len(addedResponsibleUsers) > 0 {
		if err := s.updateConfirmationBasedOnApprovals(tx, applicationID); err != nil {
			tx.Rollback()
			return err
		}
	}

	// Проверяем изменение confirmation
	var newConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&newConfirmation)

	if (oldConfirmation == nil) != (newConfirmation == nil) || (oldConfirmation != nil && newConfirmation != nil && *oldConfirmation != *newConfirmation) {
		statusChangeTime := historyTime.Add(time.Millisecond)
		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, old_value, new_value, created_at)
			VALUES (?, ?, 'confirmation_change', ?, ?, ?)
		`, applicationID, user.ID, oldConfirmation, newConfirmation, statusChangeTime)
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	return nil
}

func (s *applicationService) ApproveApplicationByUser(ctx context.Context, username string, applicationID int, req UserApprovalRequest) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	if req.UserID != user.ID {
		return echo.NewHTTPError(http.StatusForbidden, "You can only approve for yourself")
	}

	if req.Status != "approved" && req.Status != "rejected" {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid status. Must be 'approved' or 'rejected'")
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Проверяем, что пользователь -- ответственный
	var responsible struct {
		ID               int
		ApprovalStatus   *string
		RequiredApproval bool
	}
	result := tx.Raw(`
		SELECT id, approval_status, required_approval
		FROM application_responsible_users
		WHERE application_id = ? AND user_id = ?
	`, applicationID, req.UserID).Scan(&responsible)
	if result.Error != nil || responsible.ID == 0 {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusForbidden, "You are not responsible for this application")
	}

	if responsible.ApprovalStatus == nil || *responsible.ApprovalStatus != "pending" {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusBadRequest, "You have already voted on this application")
	}

	// Сохраняем старый confirmation
	var oldConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&oldConfirmation)

	nowUTC := time.Now().UTC()
	historyTime := nowUTC

	// Обновляем голос
	tx.Exec(`
		UPDATE application_responsible_users
		SET approval_status = ?, approval_comment = ?, approval_datetime = ?
		WHERE application_id = ? AND user_id = ?
	`, req.Status, req.Comment, nowUTC, applicationID, req.UserID)

	// Записываем действие в историю
	actionType := "approve"
	if req.Status == "rejected" {
		actionType = "reject"
	}
	meta, _ := json.Marshal(map[string]interface{}{"required_approval": responsible.RequiredApproval})
	tx.Exec(`
		INSERT INTO application_history (application_id, user_id, action_type, comment, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, applicationID, user.ID, actionType, req.Comment, string(meta), historyTime)

	// Пересчитываем confirmation
	if err := s.updateConfirmationBasedOnApprovals(tx, applicationID); err != nil {
		tx.Rollback()
		return err
	}

	// Проверяем изменение confirmation
	var newConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&newConfirmation)

	if (oldConfirmation == nil) != (newConfirmation == nil) || (oldConfirmation != nil && newConfirmation != nil && *oldConfirmation != *newConfirmation) {
		statusChangeTime := historyTime.Add(time.Millisecond)
		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, old_value, new_value, created_at)
			VALUES (?, ?, 'confirmation_change', ?, ?, ?)
		`, applicationID, user.ID, oldConfirmation, newConfirmation, statusChangeTime)
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	return nil
}

func (s *applicationService) CheckApprovalStatus(ctx context.Context, applicationID int) (*ApprovalStatusResponse, error) {
	var app struct {
		Confirmation *string
		Status       *string
	}
	result := s.db.WithContext(ctx).Raw("SELECT confirmation, status FROM applications WHERE id = ?", applicationID).Scan(&app)
	if result.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if result.RowsAffected == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}

	return &ApprovalStatusResponse{
		Confirmation: app.Confirmation,
		Status:       app.Status,
	}, nil
}

func (s *applicationService) TakeApplicationToWork(ctx context.Context, username string, applicationID int, req TakeToWorkRequest) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return err
	}
	if !isApprover {
		return echo.NewHTTPError(http.StatusForbidden, "User is not an approver")
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var app struct {
		Status *string
	}
	result := tx.Raw("SELECT status FROM applications WHERE id = ?", applicationID).Scan(&app)
	if result.Error != nil || result.RowsAffected == 0 {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}
	oldStatus := app.Status

	if req.Action == "accept" {
		if oldStatus != nil && *oldStatus == "В работе" {
			tx.Rollback()
			return echo.NewHTTPError(http.StatusBadRequest, "Application is already in work")
		}

		tx.Exec("UPDATE applications SET status = 'В работе', responsible_user_id = ?, responsible_comment = ? WHERE id = ?",
			user.ID, req.Comment, applicationID)

		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, old_value, new_value, comment, created_at)
			VALUES (?, ?, 'take_to_work', ?, 'В работе', ?, NOW())
		`, applicationID, user.ID, oldStatus, req.Comment)

		if err := s.activateApplicationItems(tx, applicationID, true); err != nil {
			tx.Rollback()
			return err
		}
	} else if req.Action == "reject" {
		if oldStatus != nil && *oldStatus == "Отказано" {
			tx.Rollback()
			return echo.NewHTTPError(http.StatusBadRequest, "Application is already rejected")
		}

		tx.Exec("UPDATE applications SET status = 'Отказано', responsible_user_id = ?, responsible_comment = ? WHERE id = ?",
			user.ID, req.Comment, applicationID)

		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, old_value, new_value, comment, created_at)
			VALUES (?, ?, 'reject', ?, 'Отказано', ?, NOW())
		`, applicationID, user.ID, oldStatus, req.Comment)

		if err := s.activateApplicationItems(tx, applicationID, false); err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	return nil
}

func (s *applicationService) RevokeApplicationFromWork(ctx context.Context, username string, applicationID int, req RevokeFromWorkRequest) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return err
	}
	if !isApprover {
		return echo.NewHTTPError(http.StatusForbidden, "Only approver can revoke the application")
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var app struct{ Status *string }
	result := tx.Raw("SELECT status FROM applications WHERE id = ?", applicationID).Scan(&app)
	if result.Error != nil || result.RowsAffected == 0 {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}

	tx.Exec("UPDATE applications SET status = 'В обработке', responsible_user_id = NULL, responsible_comment = NULL WHERE id = ?", applicationID)

	tx.Exec(`
		INSERT INTO application_history (application_id, user_id, action_type, old_value, new_value, comment, created_at)
		VALUES (?, ?, 'revoke_from_work', ?, 'В обработке', ?, NOW())
	`, applicationID, user.ID, app.Status, req.Comment)

	if err := s.activateApplicationItems(tx, applicationID, false); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	return nil
}

func (s *applicationService) RestoreApplicationToWork(ctx context.Context, username string, applicationID int, req RevokeFromWorkRequest) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return err
	}
	if !isApprover {
		return echo.NewHTTPError(http.StatusForbidden, "Only approver can restore the application")
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var app struct{ Status *string }
	result := tx.Raw("SELECT status FROM applications WHERE id = ?", applicationID).Scan(&app)
	if result.Error != nil || result.RowsAffected == 0 {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}

	tx.Exec("UPDATE applications SET status = 'В обработке', responsible_user_id = NULL, responsible_comment = NULL WHERE id = ?", applicationID)

	tx.Exec(`
		INSERT INTO application_history (application_id, user_id, action_type, old_value, new_value, comment, created_at)
		VALUES (?, ?, 'restore_to_work', ?, 'В обработке', ?, NOW())
	`, applicationID, user.ID, app.Status, req.Comment)

	if err := s.activateApplicationItems(tx, applicationID, false); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	return nil
}

func (s *applicationService) GetApplicationResponsibleUsers(ctx context.Context, applicationID int) ([]ResponsibleUserInfo, error) {
	return s.fetchResponsibleUsers(s.db.WithContext(ctx), applicationID)
}

func (s *applicationService) GetApplicationHistory(ctx context.Context, applicationID int) ([]ApplicationHistoryItem, error) {
	items := make([]ApplicationHistoryItem, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			h.id,
			h.application_id,
			h.user_id,
			CONCAT(COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
			) as user_name,
			u.last_name,
			u.first_name,
			u.middle_name,
			h.action_type,
			h.action_status,
			h.old_value,
			h.new_value,
			h.comment,
			h.created_at,
			h.metadata
		FROM application_history h
		JOIN users u ON h.user_id = u.id
		WHERE h.application_id = ?
		ORDER BY h.created_at DESC
	`, applicationID).Scan(&items).Error

	if err != nil {
		slog.Error("Ошибка получения истории заявки", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching application history")
	}

	return items, nil
}

func (s *applicationService) AddHistoryEntry(ctx context.Context, req AddHistoryEntryRequest) error {
	result := s.db.WithContext(ctx).Exec(`
		INSERT INTO application_history (application_id, user_id, action_type, action_status, old_value, new_value, comment, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, req.ApplicationID, req.UserID, req.ActionType, req.ActionStatus, req.OldValue, req.NewValue, req.Comment, req.Metadata)

	if result.Error != nil {
		slog.Error("Ошибка добавления записи истории", "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error adding history entry")
	}

	return nil
}

func (s *applicationService) RevokeApproval(ctx context.Context, username string, applicationID int, req RevokeApprovalRequest) (*RevokeApprovalResponse, error) {
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

	var responsible struct {
		ApprovalStatus   *string
		RequiredApproval bool
	}
	result := tx.Raw("SELECT approval_status, required_approval FROM application_responsible_users WHERE application_id = ? AND user_id = ?",
		applicationID, user.ID).Scan(&responsible)
	if result.Error != nil || result.RowsAffected == 0 {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusForbidden, "You are not responsible for this application")
	}

	if responsible.ApprovalStatus == nil || *responsible.ApprovalStatus == "pending" {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusBadRequest, "You haven't voted yet")
	}

	var oldConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&oldConfirmation)

	nowUTC := time.Now().UTC()
	historyTime := nowUTC

	tx.Exec(`
		UPDATE application_responsible_users
		SET approval_status = 'pending', approval_comment = NULL, approval_datetime = NULL
		WHERE application_id = ? AND user_id = ?
	`, applicationID, user.ID)

	tx.Exec(`
		INSERT INTO application_history (application_id, user_id, action_type, comment, created_at)
		VALUES (?, ?, 'revoke_approval', ?, ?)
	`, applicationID, user.ID, req.Comment, historyTime)

	if err := s.updateConfirmationBasedOnApprovals(tx, applicationID); err != nil {
		tx.Rollback()
		return nil, err
	}

	var newConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&newConfirmation)

	if (oldConfirmation == nil) != (newConfirmation == nil) || (oldConfirmation != nil && newConfirmation != nil && *oldConfirmation != *newConfirmation) {
		statusChangeTime := historyTime.Add(time.Millisecond)
		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, old_value, new_value, created_at)
			VALUES (?, ?, 'confirmation_change', ?, ?, ?)
		`, applicationID, user.ID, oldConfirmation, newConfirmation, statusChangeTime)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	// Получаем обновлённый статус
	var updatedApp struct {
		Confirmation *string
		Status       *string
	}
	s.db.WithContext(ctx).Raw("SELECT confirmation, status FROM applications WHERE id = ?", applicationID).Scan(&updatedApp)

	return &RevokeApprovalResponse{
		Success:      true,
		Message:      "Approval revoked successfully",
		Confirmation: updatedApp.Confirmation,
		Status:       updatedApp.Status,
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

func (s *applicationService) UpdateApplicationItemsStatus(ctx context.Context, applicationID int) error {
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	type attachmentRow struct {
		ID             int
		AttachmentType string
	}
	var attachments []attachmentRow
	if err := tx.Raw("SELECT id, attachment_type FROM attachments WHERE application_id = ?", applicationID).Scan(&attachments).Error; err != nil {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachments")
	}

	for _, att := range attachments {
		switch att.AttachmentType {
		case "cars":
			tx.Exec("UPDATE cars SET status = 1, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ?", att.ID)
		case "people":
			tx.Exec("UPDATE employees SET status = 1, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ?", att.ID)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	return nil
}

func (s *applicationService) CheckExpiredAttachments(ctx context.Context) error {
	slog.Info("Проверка истекших вложений...")

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	type expiredRow struct {
		ID            int
		ApplicationID int
	}
	var expired []expiredRow
	tx.Raw(`
		SELECT id, application_id FROM attachments
		WHERE status = 1 AND (
			(entry_date_to IS NOT NULL AND entry_date_to < CURRENT_DATE)
			OR (entry_date_to IS NOT NULL AND entry_time_to IS NOT NULL
			    AND ((entry_date_to + entry_time_to) AT TIME ZONE 'Europe/Moscow') < CURRENT_TIMESTAMP)
		)
	`).Scan(&expired)

	if len(expired) == 0 {
		tx.Rollback()
		slog.Info("Истекших вложений не найдено")
		return nil
	}

	slog.Info("Найдено истекших вложений", "count", len(expired))

	attachmentIDs := make([]int, len(expired))
	appIDs := make([]int, len(expired))
	for i, e := range expired {
		attachmentIDs[i] = e.ID
		appIDs[i] = e.ApplicationID
	}

	// Получаем машины для истории
	type carDeactivate struct {
		ID        int
		CarNumber string
		CarBrand  string
	}
	var cars []carDeactivate
	tx.Raw("SELECT id, car_number, car_brand FROM cars WHERE attachment_id IN ?", attachmentIDs).Scan(&cars)

	tx.Exec("UPDATE attachments SET status = 0 WHERE id IN ?", attachmentIDs)
	tx.Exec("UPDATE cars SET status = 0 WHERE attachment_id IN ?", attachmentIDs)
	tx.Exec("UPDATE employees SET status = 0 WHERE attachment_id IN ?", attachmentIDs)

	for _, car := range cars {
		tx.Exec(`
			INSERT INTO cars_history (car_id, user_id, action_type, comment, created_at)
			VALUES (?, NULL, 'deactivate', ?, NOW())
		`, car.ID, fmt.Sprintf("Срок действия заявки на автомобиль %s %s истёк", car.CarNumber, car.CarBrand))
	}

	// Завершаем заявки, у которых все вложения неактивны
	uniqueAppIDs := make(map[int]bool)
	for _, id := range appIDs {
		uniqueAppIDs[id] = true
	}
	for appID := range uniqueAppIDs {
		var activeCount int64
		tx.Raw("SELECT COUNT(*) FROM attachments WHERE application_id = ? AND status = 1", appID).Scan(&activeCount)
		if activeCount == 0 {
			tx.Exec("UPDATE applications SET status = 'Завершено' WHERE id = ?", appID)
			slog.Info("Заявка завершена", "application_id", appID)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	slog.Info("Проверка истекших вложений завершена")
	return nil
}

// --- Утилиты ---

func ptrString(s string) *string { return &s }

func safeDerefInt(p *int) int {
	if p != nil {
		return *p
	}
	return 0
}
