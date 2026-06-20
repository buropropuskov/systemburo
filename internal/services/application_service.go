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
	"systemburo/internal/normalize"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

var allowedStatuses = map[string]bool{
	"Непрочитано": true, "В обработке": true, "Принята в работу": true,
	"На согласовании": true, "Не согласовано": true, "Согласовано": true,
	"Отклонена": true, "Завершена": true,
}

var allowedConfirmations = map[string]bool{
	"Согласование": true, "Согласовано": true, "Не согласовано": true,
}

// forwardAttachmentVisible ограничивает агрегаты-теги заявки вложениями, видимыми
// просматривающему с учётом пер-вложенного пересыла (#680): отправитель и пользователь
// без строк forward_attachments видят все вложения, получатель со строками - только
// перечисленные. Три плейсхолдера связываются с id просматривающего; ссылается на att.id
// (вложение тег-подзапроса) и a (заявка внешнего запроса).
const forwardAttachmentVisible = `(
		a.sender_user_id = ?
		OR NOT EXISTS (SELECT 1 FROM forward_attachments fa WHERE fa.application_id = a.id AND fa.recipient_user_id = ?)
		OR EXISTS (SELECT 1 FROM forward_attachments fav WHERE fav.application_id = a.id AND fav.recipient_user_id = ? AND fav.attachment_id = att.id)
	)`

// applicationsListSelect - общий список столбцов для листингов заявок (Центр, пагинация,
// заявки пользователя). Теги has_roof_access/has_free_parking учитывают видимость пересыла
// (forwardAttachmentVisible). Плейсхолдеры (?) связываются через applicationsListSelectArgs.
const applicationsListSelect = `
		a.*,
		COALESCE(o.name, c.name) as organization_name,
		c.name as company_name,
		format_full_name(u.last_name, u.first_name, u.middle_name) as sender_full_name,
		format_short_name(u.last_name, u.first_name, u.middle_name) as sender_name,
		u.is_important as sender_is_important,
		format_full_name(ru.last_name, ru.first_name, ru.middle_name) as responsible_full_name,
		format_short_name(ru.last_name, ru.first_name, ru.middle_name) as responsible_name,
		EXISTS (SELECT 1 FROM attachments att JOIN attachment_templates at2 ON at2.unique_attachment_id = att.unique_attachment_id AND at2.is_active = true WHERE att.application_id = a.id) as has_blank_template,
		EXISTS (SELECT 1 FROM application_reads ar WHERE ar.application_id = a.id AND ar.user_id = ?) as is_read,
		EXISTS (SELECT 1 FROM attachments att WHERE att.application_id = a.id AND att.roof_access = true AND ` + forwardAttachmentVisible + `) as has_roof_access,
		EXISTS (SELECT 1 FROM attachments att WHERE att.application_id = a.id AND att.free_parking = true AND ` + forwardAttachmentVisible + `) as has_free_parking,
		(SELECT COUNT(*) FROM application_blacklist_flags f WHERE f.application_id = a.id AND NOT EXISTS (SELECT 1 FROM application_blacklist_overrides o WHERE o.flag_id = f.id)) as blacklist_flags_count
	`

// applicationsListSelectArgs связывает плейсхолдеры applicationsListSelect: is_read (1)
// всегда по реальному id просматривающего; теги видимости (по три на каждый из двух) -
// по forwardViewerID, который равен 0 для супер-админа (bypass фильтра пересыла, видит
// все теги - симметрично resolveForwardFilter в detail-пути).
func applicationsListSelectArgs(readUserID, forwardViewerID int) []interface{} {
	return []interface{}{
		readUserID,
		forwardViewerID, forwardViewerID, forwardViewerID,
		forwardViewerID, forwardViewerID, forwardViewerID,
	}
}

// forwardViewerID возвращает id для фильтра видимости тегов: 0 (bypass) для супер-админа,
// иначе реальный id - супер-админ видит все теги независимо от пересыла.
func forwardViewerID(user *models.User) int {
	if user.IsSuperAdmin {
		return 0
	}
	return user.ID
}

// ApplicationService определяет интерфейс бизнес-логики для работы с заявками.
type ApplicationService interface {
	// GetApplications возвращает список заявок для Центра заявок с фильтрацией.
	GetApplications(ctx context.Context, username string, filter ApplicationFilter) ([]ApplicationWithDetails, error)

	// GetApplicationsPaginated возвращает страницу заявок с общим количеством.
	GetApplicationsPaginated(ctx context.Context, username string, filter ApplicationFilter, page, perPage int) ([]ApplicationWithDetails, int64, error)

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
	// OverrideBlacklistFlag фиксирует "всё равно пропустить" по помеченному элементу (#481),
	// снимая блокировку согласования по этому флагу. Только ответственный, идемпотентно.
	OverrideBlacklistFlag(ctx context.Context, username string, applicationID int, req OverrideBlacklistFlagRequest) error

	// DeleteBlacklistOverride снимает ранее подтверждённый пропуск по флагу (#481), снова
	// блокируя согласование по нему. Право шире, чем создание: ответственный ИЛИ принимающий.
	DeleteBlacklistOverride(ctx context.Context, username string, applicationID, flagID int) error

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

	// GetApplicationAttachments возвращает вложения заявки, видимые получателю пересылки
	// (viewerUserID; 0 - супер-админ, без фильтра).
	GetApplicationAttachments(ctx context.Context, applicationID, viewerUserID int) ([]AttachmentInfo, error)

	// CanViewAttachment сообщает, доступно ли вложение просматривающему с учётом пересыла.
	CanViewAttachment(ctx context.Context, applicationID, attachmentID, viewerUserID int) (bool, error)

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

	// CanAccessApplication проверяет, имеет ли пользователь доступ к заявке.
	CanAccessApplication(ctx context.Context, applicationID int, username string, isSuperAdmin bool) bool

	// GetApplicationIDByAttachment возвращает ID заявки по ID вложения.
	GetApplicationIDByAttachment(ctx context.Context, attachmentID int) (int, error)

	// IsSecurityUser сообщает, является ли аккаунт типом security (резолв по user_types.code).
	IsSecurityUser(ctx context.Context, userID int) (bool, error)

	// GetAvailableAttachmentsForSecurity возвращает страницу вложений подтверждённых заявок,
	// доступных охраннику по совпадению мест (#706), и общее количество. Супер-админ - без
	// фильтра по местам. filter (BE-S6) опционально сужает выдачу поверх гейта видимости.
	GetAvailableAttachmentsForSecurity(ctx context.Context, userID int, isSuperAdmin bool, filter AvailableAttachmentFilters, page, perPage int) ([]AvailableAttachment, int64, error)

	// CanSecurityViewAttachment сообщает, доступно ли конкретное вложение охраннику по тем же
	// правилам, что и листинг (#706). Для 403 на чужое вложение в детальном эндпоинте.
	CanSecurityViewAttachment(ctx context.Context, userID int, isSuperAdmin bool, attachmentID int) (bool, error)

	// GetAvailableAttachmentByID возвращает заголовок вложения с инфо заявки для детали
	// "Доступные мне" (#706). nil без ошибки - вложение не найдено. Без проверки доступа,
	// вызывать после CanSecurityViewAttachment.
	GetAvailableAttachmentByID(ctx context.Context, attachmentID int) (*AvailableAttachment, error)
}

// --- DTO: запросы ---

// ApplicationFilter параметры фильтрации списка заявок.
type ApplicationFilter struct {
	SearchQuery    *string `query:"search_query"`
	OrganizationID *int    `query:"organization_id"`
	CompanyID      *int    `query:"company_id"`
	Confirmation   *string `query:"confirmation"`
	Status         *string `query:"status"`
	DateFrom       *string `query:"date_from"`
	DateTo         *string `query:"date_to"`
	Archive        *bool   `query:"archive"`
	ActiveToday    *bool   `query:"active_today"`
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
	ResponsiblePerson string               `json:"responsible_person" validate:"required"`
	ContactPhone      string               `json:"contact_phone" validate:"required"`
	DataApproval      bool                 `json:"data_approval"`
	Attachments       []AttachmentData     `json:"attachments"`
	RequiredUsers     *[]RequiredUserInput `json:"required_users"`
}

// AttachmentData данные вложения при создании заявки.
type AttachmentData struct {
	AttachmentType        string  `json:"attachment_type"`
	AttachmentName        string  `json:"attachment_name"`
	AttachmentDisplayName string  `json:"attachment_display_name"`
	UniqueAttachmentID    int     `json:"unique_attachment_id"`
	EntryDateFrom         *string `json:"entry_date_from"`
	EntryDateTo           *string `json:"entry_date_to"`
	EntryTimeFrom         *string `json:"entry_time_from"`
	EntryTimeTo           *string `json:"entry_time_to"`
	RoofAccess            bool    `json:"roof_access"`
	FreeParking           bool    `json:"free_parking"`
	// UnloadPlaces - места разгрузки на уровне вложения (#706). Для items-вложений это
	// единственный источник мест; для cars дублирует дедуп-union мест всех машин вложения.
	UnloadPlaces []int                      `json:"unload_places,omitempty"`
	Data         AttachmentContentData      `json:"data"`
	CustomValues *[]models.CustomValueInput `json:"custom_values,omitempty"`
}

// AttachmentContentData содержимое вложения: машины, сотрудники или ТМЦ.
type AttachmentContentData struct {
	Vehicles  *[]VehicleInput  `json:"vehicles"`
	Employees *[]EmployeeInput `json:"employees"`
	Items     *[]ItemInput     `json:"items"`
}

// VehicleInput данные автомобиля при создании.
type VehicleInput struct {
	CarNumber    string  `json:"car_number"`
	CarBrand     string  `json:"car_brand"`
	MarkID       *int    `json:"mark_id"`
	UnloadPlace  *string `json:"unload_place"`
	UnloadPlaces []int   `json:"unload_places"`
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
// AttachmentIDs - общий для всех получателей список вложений (#680): пустой -> получатели
// видят все вложения заявки; непустой -> в forward_attachments пишется строка на каждого
// получателя x каждое вложение, и при чтении получатель видит только перечисленные.
type ForwardApplicationRequest struct {
	Users         []ForwardUser `json:"users"`
	AttachmentIDs []int         `json:"attachment_ids"`
}

// ForwardUser пользователь для пересылки. required_approval и can_view не могут быть оба true.
type ForwardUser struct {
	UserID           int  `json:"user_id"`
	RequiredApproval bool `json:"required_approval"`
	CanView          bool `json:"can_view"`
}

// UserApprovalRequest тело запроса на согласование заявки.
type UserApprovalRequest struct {
	UserID  int     `json:"user_id" validate:"gte=1"`
	Status  string  `json:"status" validate:"required,oneof=approved rejected"`
	Comment *string `json:"comment"`
}

// TakeToWorkRequest тело запроса на принятие заявки в работу.
type TakeToWorkRequest struct {
	UserID  int     `json:"user_id" validate:"gte=1"`
	Action  string  `json:"action" validate:"required,oneof=accept reject"`
	Comment *string `json:"comment"`
}

// RevokeFromWorkRequest тело запроса на отзыв заявки из работы.
type RevokeFromWorkRequest struct {
	UserID  int     `json:"user_id" validate:"gte=1"`
	Comment *string `json:"comment"`
}

// AddHistoryEntryRequest тело запроса на добавление записи в историю.
type AddHistoryEntryRequest struct {
	ApplicationID int              `json:"application_id" validate:"gte=1"`
	UserID        int              `json:"user_id" validate:"gte=1"`
	ActionType    string           `json:"action_type" validate:"required"`
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
	SenderIsImportant    bool       `json:"sender_is_important"`
	Message              *string    `json:"message"`
	Status               string     `json:"status"`
	ResponsibleUserID    *int       `json:"responsible_user_id"`
	ResponsibleFullName  *string    `json:"responsible_full_name"`
	ResponsibleName      string     `json:"responsible_name"`
	ResponsibleComment   *string    `json:"responsible_comment"`
	DataApproval         bool       `json:"data_approval"`
	HasBlankTemplate     bool       `json:"has_blank_template"`
	IsRead               bool       `json:"is_read"`
	BlacklistFlagsCount  int        `json:"blacklist_flags_count"`
	HasRoofAccess        bool       `json:"has_roof_access"`
	HasFreeParking       bool       `json:"has_free_parking"`
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
	ID                          int                 `json:"id"`
	AttachmentType              string              `json:"attachment_type"`
	AttachmentName              string              `json:"attachment_name"`
	AttachmentDisplayName       string              `json:"attachment_display_name"`
	EntryDateFrom               *string             `json:"entry_date_from"`
	EntryDateTo                 *string             `json:"entry_date_to"`
	EntryTimeFrom               *string             `json:"entry_time_from"`
	EntryTimeTo                 *string             `json:"entry_time_to"`
	RoofAccess                  bool                `json:"roof_access"`
	FreeParking                 bool                `json:"free_parking"`
	CreatedAt                   *time.Time          `json:"created_at"`
	UniqueAttachmentID          *int                `json:"unique_attachment_id"`
	UniqueAttachmentTitle       *string             `json:"unique_attachment_title"`
	UniqueAttachmentDisplayName *string             `json:"unique_attachment_display_name"`
	HasTemplate                 bool                `json:"has_template"`
	CustomValues                []CustomValueDetail `gorm:"-" json:"custom_values,omitempty"`
}

// CustomValueDetail значение кастомного поля для отображения.
type CustomValueDetail struct {
	FieldID int    `json:"field_id"`
	Label   string `json:"label"`
	Value   string `json:"value"`
}

// CarWithPlaces автомобиль с привязанными местами разгрузки.
type CarWithPlaces struct {
	ID             int              `json:"id"`
	CarNumber      string           `json:"car_number"`
	CarBrand       string           `json:"car_brand"`
	UnloadPlace    *string          `json:"unload_place"`
	EntryDateFrom  *string          `json:"entry_date_from"`
	EntryTimeFrom  *string          `json:"entry_time_from"`
	EntryDateTo    *string          `json:"entry_date_to"`
	EntryTimeTo    *string          `json:"entry_time_to"`
	Organization   *string          `json:"organization"`
	OrganizationID *int             `json:"organization_id"`
	Company        *string          `json:"company"`
	CompanyID      *int             `json:"company_id"`
	UnloadPlaces   []UnloadPlaceRef `json:"unload_places"`
	// BlacklistSimilar - предупреждение о возможном обходе ЧС (#481): заполнено, если
	// номер близок к активной записи ЧС (но не точное совпадение). nil - элемент чист.
	BlacklistSimilar *BlacklistFlagInfo `json:"blacklist_similar,omitempty"`
}

// BlacklistFlagInfo - данные per-element предупреждения о возможном обходе ЧС (#481)
// для детали заявки: id флага (для override), что в ЧС похоже, причина, близость [0..1]
// и подтверждён ли уже пропуск (override) - чтобы фронт показал статус и разблокировку.
type BlacklistFlagInfo struct {
	FlagID        int     `json:"flag_id"`
	MatchedValue  string  `json:"matched_value"`
	MatchedReason string  `json:"matched_reason"`
	Similarity    float64 `json:"similarity"`
	Overridden    bool    `json:"overridden"`
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
	CitizenshipName      *string        `json:"citizenship_name"`
	PassportSeriesNumber *string        `json:"passport_series_number"`
	PatentNumber         *string        `json:"patent_number"`
	OtherPermission      *string        `json:"other_permission"`
	EntryDateTo          *string        `json:"entry_date_to"`
	PassTime             *string        `json:"pass_time"`
	Organization         *string        `json:"organization"`
	OrganizationID       *int           `json:"organization_id"`
	Company              *string        `json:"company"`
	CompanyID            *int           `json:"company_id"`
	TargetTables         []TableInfoRef `json:"target_tables"`
	// BlacklistSimilar - предупреждение о возможном обходе ЧС (#481): заполнено, если
	// ФИО близко к активной записи ЧС (но не точное совпадение). nil - элемент чист.
	BlacklistSimilar *BlacklistFlagInfo `json:"blacklist_similar,omitempty"`
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
	db                  *gorm.DB
	permissionService   PermissionService
	notificationService NotificationService
	vehicleBlacklist    VehicleBlacklistService
	personBlacklist     PersonBlacklistService
}

// NewApplicationService создаёт экземпляр сервиса заявок.
func NewApplicationService(db *gorm.DB, permSvc PermissionService, notifSvc NotificationService, vehicleBL VehicleBlacklistService, personBL PersonBlacklistService) ApplicationService {
	return &applicationService{
		db:                  db,
		permissionService:   permSvc,
		notificationService: notifSvc,
		vehicleBlacklist:    vehicleBL,
		personBlacklist:     personBL,
	}
}

// --- Основные методы ---

// GetApplications возвращает список заявок для Центра заявок с фильтрацией.
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
		Select(applicationsListSelect, applicationsListSelectArgs(user.ID, forwardViewerID(user))...).
		Joins("LEFT JOIN organizations o ON a.organization_id = o.id").
		Joins("LEFT JOIN companies c ON a.company_id = c.id").
		Joins("LEFT JOIN users u ON a.sender_user_id = u.id").
		Joins("LEFT JOIN users ru ON a.responsible_user_id = ru.id")

	query = applyApplicationAccessFilter(query, user.ID, isApprover)

	query = applyApplicationFilters(query, filter, false)
	query = query.Order("a.sending_datetime DESC")

	rows := make([]ApplicationWithDetails, 0)
	if err := query.Find(&rows).Error; err != nil {
		slog.Error("Ошибка получения заявок", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	return rows, nil
}

// buildApplicationsBaseQuery строит базовый запрос с джойнами и фильтрами без Select и Order.
func (s *applicationService) buildApplicationsBaseQuery(ctx context.Context, userID int, isApprover bool, filter ApplicationFilter) *gorm.DB {
	query := s.db.WithContext(ctx).Table("applications a").
		Joins("LEFT JOIN organizations o ON a.organization_id = o.id").
		Joins("LEFT JOIN companies c ON a.company_id = c.id").
		Joins("LEFT JOIN users u ON a.sender_user_id = u.id").
		Joins("LEFT JOIN users ru ON a.responsible_user_id = ru.id")

	query = applyApplicationAccessFilter(query, userID, isApprover)

	return applyApplicationFilters(query, filter, false)
}

// GetApplicationsPaginated возвращает страницу заявок с общим количеством.
func (s *applicationService) GetApplicationsPaginated(ctx context.Context, username string, filter ApplicationFilter, page, perPage int) ([]ApplicationWithDetails, int64, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, 0, err
	}
	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	countQuery := s.buildApplicationsBaseQuery(ctx, user.ID, isApprover, filter)
	if err := countQuery.Count(&total).Error; err != nil {
		slog.Error("Ошибка подсчёта заявок", "error", err)
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	offset := (page - 1) * perPage
	dataQuery := s.buildApplicationsBaseQuery(ctx, user.ID, isApprover, filter)
	dataQuery = dataQuery.
		Select(applicationsListSelect, applicationsListSelectArgs(user.ID, forwardViewerID(user))...).
		Order("a.sending_datetime DESC").
		Offset(offset).
		Limit(perPage)

	rows := make([]ApplicationWithDetails, 0)
	if err := dataQuery.Find(&rows).Error; err != nil {
		slog.Error("Ошибка получения заявок (paginated)", "error", err)
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	return rows, total, nil
}

// GetUserApplications возвращает заявки текущего пользователя с фильтрацией.
func (s *applicationService) GetUserApplications(ctx context.Context, username string, filter ApplicationFilter) ([]ApplicationWithDetails, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	query := s.db.WithContext(ctx).Table("applications a").
		Select(applicationsListSelect, applicationsListSelectArgs(user.ID, forwardViewerID(user))...).
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

// GetApplicationByID возвращает заявку по ID с обновлением статуса при первом прочтении.
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
		SenderIsImportant   bool    `gorm:"column:sender_is_important"`
		ResponsibleFullName *string `gorm:"column:responsible_full_name"`
		ResponsibleName     *string `gorm:"column:responsible_name"`
	}

	result := tx.Table("applications a").
		Select(`
			a.*,
			COALESCE(o.name, c.name) as organization_name,
			c.name as company_name,
			format_full_name(u.last_name, u.first_name, u.middle_name) as sender_full_name,
			format_short_name(u.last_name, u.first_name, u.middle_name) as sender_name,
			u.is_important as sender_is_important,
			format_full_name(ru.last_name, ru.first_name, ru.middle_name) as responsible_full_name,
			format_short_name(ru.last_name, ru.first_name, ru.middle_name) as responsible_name
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
		"sender_is_important":   row.SenderIsImportant,
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

// GetApplicationDetails возвращает расширенную информацию о заявке.
func (s *applicationService) GetApplicationDetails(ctx context.Context, applicationID int) (map[string]interface{}, error) {
	var row struct {
		models.Application
		OrganizationName    *string `gorm:"column:organization_name"`
		CompanyName         *string `gorm:"column:company_name"`
		SenderFullName      *string `gorm:"column:sender_full_name"`
		SenderName          *string `gorm:"column:sender_name"`
		SenderIsImportant   bool    `gorm:"column:sender_is_important"`
		ResponsibleFullName *string `gorm:"column:responsible_full_name"`
		ResponsibleName     *string `gorm:"column:responsible_name"`
	}

	result := s.db.WithContext(ctx).Table("applications a").
		Select(`
			a.*,
			COALESCE(o.name, c.name) as organization_name,
			c.name as company_name,
			format_full_name(u.last_name, u.first_name, u.middle_name) as sender_full_name,
			format_short_name(u.last_name, u.first_name, u.middle_name) as sender_name,
			u.is_important as sender_is_important,
			format_full_name(ru.last_name, ru.first_name, ru.middle_name) as responsible_full_name,
			format_short_name(ru.last_name, ru.first_name, ru.middle_name) as responsible_name
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

	// Зеркало гейта согласования (#481): пока есть помеченные элементы без override,
	// фронт держит кнопку "Согласовать" заблокированной. Источник правды - тот же
	// hasUnoverriddenBlacklistFlags, что блокирует согласование на бэке (409).
	blacklistBlocked, err := hasUnoverriddenBlacklistFlags(ctx, s.db, applicationID)
	if err != nil {
		return nil, err
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
		"sender_is_important":   row.SenderIsImportant,
		"message":               row.Message,
		"status":                row.Status,
		"responsible_user_id":   row.ResponsibleUserID,
		"responsible_full_name": row.ResponsibleFullName,
		"responsible_name":      responsibleName,
		"responsible_comment":   row.ResponsibleComment,
		"data_approval":         row.DataApproval,
		"responsible_users":     responsibles,

		"has_unoverridden_blacklist_flags": blacklistBlocked,
	}

	return response, nil
}

// CreateApplication создаёт новую заявку с назначением ответственных.
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

// validateBlacklist проверяет машины и людей заявки против активного ЧС (#443).
// Машины матчатся по номеру + mark_id (как и фронтовый /check); машины без mark_id
// ("по факту"/свободная марка) пропускаем - по mark_id в ЧС они попасть не могут.
// Люди - строгое совпадение ФИО. Возвращает 409 при первом совпадении.
func (s *applicationService) validateBlacklist(ctx context.Context, req CompleteApplicationRequest) error {
	for _, att := range req.Attachments {
		if att.Data.Vehicles != nil {
			for _, v := range *att.Data.Vehicles {
				// Машины из mark-дропдауна приходят с mark_id (строгий матч), выбранные из
				// существующих unique_cars - без mark_id, но с car_brand (имя марки): для них
				// fallback на матч по имени, иначе заблокированная машина прошла бы гард.
				var (
					res models.VehicleBlacklistCheckResult
					err error
				)
				switch {
				case v.MarkID != nil:
					res, err = s.vehicleBlacklist.Check(ctx, v.CarNumber, *v.MarkID)
				case strings.TrimSpace(v.CarBrand) != "":
					res, err = s.vehicleBlacklist.CheckByName(ctx, v.CarNumber, v.CarBrand)
				default:
					continue
				}
				if err != nil {
					return err
				}
				if res.IsBlacklisted {
					return echo.NewHTTPError(http.StatusConflict,
						fmt.Sprintf("Машина %s %s в чёрном списке: %s", v.CarNumber, v.CarBrand, res.Reason))
				}
			}
		}
		if att.Data.Employees != nil {
			for _, e := range *att.Data.Employees {
				// Тихая деградация ЧС (#529): если ФИО скрыто конфигом - данных для
				// совпадения нет, матчить нечем, пропускаем (не падаем, не 500).
				if strings.TrimSpace(e.LastName) == "" && strings.TrimSpace(e.FirstName) == "" {
					continue
				}
				middleName := ""
				if e.MiddleName != nil {
					middleName = *e.MiddleName
				}
				res, err := s.personBlacklist.Check(ctx, e.LastName, e.FirstName, middleName)
				if err != nil {
					return err
				}
				if res.IsBlacklisted {
					fio := strings.TrimSpace(fmt.Sprintf("%s %s %s", e.LastName, e.FirstName, middleName))
					return echo.NewHTTPError(http.StatusConflict,
						fmt.Sprintf("Человек %s в чёрном списке: %s", fio, res.Reason))
				}
			}
		}
	}
	return nil
}

// requiredFieldKeys - ключи полей, которые админ ЯВНО настроил обязательными для
// вложения (override visible=true И required=true). Поля без override не валидируются:
// строгая проверка только для настроенных полей, существующие шаблоны не ломаются
// (решение владельца, #529 H-9).
func requiredFieldKeys(overrides []models.AttachmentFieldConfig) map[string]bool {
	req := make(map[string]bool, len(overrides))
	for _, o := range overrides {
		if o.Visible && o.Required {
			req[o.FieldKey] = true
		}
	}
	return req
}

// employeeFieldPresent сообщает, заполнено ли поле сотрудника с данным ключом реестра.
// Незнакомый ключ -> true (не валидируем то, чего не знаем).
func employeeFieldPresent(e EmployeeInput, key string) bool {
	switch key {
	case "last_name":
		return strings.TrimSpace(e.LastName) != ""
	case "first_name":
		return strings.TrimSpace(e.FirstName) != ""
	case "middle_name":
		return e.MiddleName != nil && strings.TrimSpace(*e.MiddleName) != ""
	case "passport":
		return strings.TrimSpace(e.PassportSeriesNumber) != ""
	case "position":
		return strings.TrimSpace(e.Position) != ""
	case "citizenship":
		return e.CitizenshipID > 0
	case "patent":
		// Патент удовлетворяется номером патента ИЛИ иным разрешением (как на фронте).
		return (e.PatentNumber != nil && strings.TrimSpace(*e.PatentNumber) != "") ||
			(e.OtherPermission != nil && strings.TrimSpace(*e.OtherPermission) != "")
	case "work_permission":
		// Делит поле OtherPermission с patent: если оба настроены required,
		// одно заполненное разрешение удовлетворяет обоим (как на фронте).
		return e.OtherPermission != nil && strings.TrimSpace(*e.OtherPermission) != ""
	case "target_tables":
		return len(e.TargetTables) > 0
	}
	return true
}

// vehicleFieldPresent сообщает, заполнено ли поле машины. "По факту" приходит непустым
// sentinel-ом ("По факту") в car_number/car_brand, поэтому by-fact проходит проверку.
func vehicleFieldPresent(v VehicleInput, key string) bool {
	switch key {
	case "number":
		return strings.TrimSpace(v.CarNumber) != ""
	case "mark":
		return v.MarkID != nil || strings.TrimSpace(v.CarBrand) != ""
	case "unloading_places":
		return len(v.UnloadPlaces) > 0
	}
	return true
}

// itemFieldPresent сообщает, заполнено ли поле ТМЦ.
func itemFieldPresent(i ItemInput, key string) bool {
	switch key {
	case "item_name":
		return strings.TrimSpace(i.Name) != ""
	case "quantity":
		return i.Count >= 1
	}
	return true
}

// validateConfiguredRequiredFields проверяет, что поля, явно настроенные админом
// обязательными (override visible+required), присутствуют в подаче. Скрытые и
// ненастроенные поля не валидируются. Запускается до транзакции (#529 H-9).
func (s *applicationService) validateConfiguredRequiredFields(ctx context.Context, req CompleteApplicationRequest) error {
	for _, att := range req.Attachments {
		var overrides []models.AttachmentFieldConfig
		if err := s.db.WithContext(ctx).
			Where("unique_attachment_id = ?", att.UniqueAttachmentID).
			Find(&overrides).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки настройки полей")
		}
		required := requiredFieldKeys(overrides)
		if len(required) == 0 {
			continue
		}
		labels := fieldDefByKey(att.AttachmentType)
		fail := func(key string) error {
			label := key
			if d, ok := labels[key]; ok {
				label = d.Label
			}
			return echo.NewHTTPError(http.StatusBadRequest,
				fmt.Sprintf("Поле «%s» обязательно для заполнения", label))
		}

		if att.Data.Employees != nil {
			for _, e := range *att.Data.Employees {
				for key := range required {
					if !employeeFieldPresent(e, key) {
						return fail(key)
					}
				}
			}
		}
		if att.Data.Vehicles != nil {
			for _, v := range *att.Data.Vehicles {
				for key := range required {
					if !vehicleFieldPresent(v, key) {
						return fail(key)
					}
				}
			}
		}
		if att.Data.Items != nil {
			for _, item := range *att.Data.Items {
				for key := range required {
					if !itemFieldPresent(item, key) {
						return fail(key)
					}
				}
			}
		}
	}
	return nil
}

// pendingVehicleFlag - вставленная машина заявки, по которой нужно проверить близость к ЧС
// после коммита (id + номер; FindSimilar матчит по нормализованному номеру, марка не нужна).
type pendingVehicleFlag struct {
	carID     int
	carNumber string
}

// pendingEmployeeFlag - вставленный сотрудник заявки для пост-коммит проверки близости к ЧС.
type pendingEmployeeFlag struct {
	empID      int
	lastName   string
	firstName  string
	middleName string
}

// detectBlacklistSimilarity - мягкий слой предупреждения о возможном обходе ЧС (#481).
// Запускается ПОСЛЕ коммита сабмита и намеренно best-effort: это не блокирующая проверка
// (точное совпадение уже отклонено в validateBlacklist -> 409), а предупреждение. Любая
// ошибка поиска/записи флага логируется и проглатывается - неудача warning-слоя НЕ должна
// валить уже созданную заявку. Вне транзакции сабмита: ошибка здесь не отравит и не откатит её.
func (s *applicationService) detectBlacklistSimilarity(ctx context.Context, appID int, vehicles []pendingVehicleFlag, employees []pendingEmployeeFlag) {
	for _, v := range vehicles {
		matches, err := s.vehicleBlacklist.FindSimilar(ctx, v.carNumber)
		if err != nil {
			slog.Warn("blacklist similarity check failed (vehicle)", "err", err, "app_id", appID, "car_id", v.carID)
			continue
		}
		s.saveBlacklistFlag(ctx, appID, models.BlacklistElementCar, v.carID, normalize.Plate(v.carNumber), matches)
	}
	for _, e := range employees {
		matches, err := s.personBlacklist.FindSimilar(ctx, e.lastName, e.firstName, e.middleName)
		if err != nil {
			slog.Warn("blacklist similarity check failed (person)", "err", err, "app_id", appID, "employee_id", e.empID)
			continue
		}
		s.saveBlacklistFlag(ctx, appID, models.BlacklistElementEmployee, e.empID, normalize.Name(e.lastName, e.firstName, e.middleName), matches)
	}
}

// saveBlacklistFlag сохраняет ЛУЧШЕЕ совпадение как флаг элемента: matches приходят
// отсортированными по убыванию близости (контракт FindSimilar). Пустой срез - элемент
// чист, флаг не пишем. Ошибку записи логируем и проглатываем (best-effort warning-слой).
func (s *applicationService) saveBlacklistFlag(ctx context.Context, appID int, elementType string, elementID int, elementNormalized string, matches []models.BlacklistSimilarMatch) {
	if len(matches) == 0 {
		return
	}
	best := matches[0]
	// Если оператор уже подтвердил пропуск этого элемента против этой записи ЧС - не
	// предупреждаем повторно (#481, срез C-followup). Отмена override снова включит флаг.
	if s.isBlacklistSuppressed(ctx, elementType, elementNormalized, best.ID) {
		return
	}
	flag := models.ApplicationBlacklistFlag{
		ApplicationID:      appID,
		ElementType:        elementType,
		ElementID:          elementID,
		ElementNormalized:  elementNormalized,
		MatchedBlacklistID: best.ID,
		MatchedValue:       best.MatchedValue,
		MatchedReason:      best.Reason,
		Similarity:         best.Similarity,
		CreatedAt:          time.Now().UTC(),
	}
	if err := s.db.WithContext(ctx).Create(&flag).Error; err != nil {
		slog.Warn("blacklist flag save failed", "err", err, "app_id", appID, "element_type", elementType, "element_id", elementID)
	}
}

// isBlacklistSuppressed - оператор ранее нажал "всё равно пропустить" по этому элементу
// против этой записи ЧС, и подтверждение ещё действует (override-строка жива) -> повторно
// не предупреждаем (#481, срез C-followup). Ключ - нормализованная форма элемента + id записи
// ЧС, поэтому переживает пересоздание cars/employees между заявками. Best-effort: при пустом
// ключе или ошибке считаем "не подавлено" (лучше лишний раз предупредить, чем тихо пропустить).
func (s *applicationService) isBlacklistSuppressed(ctx context.Context, elementType, elementNormalized string, matchedBlacklistID int) bool {
	if elementNormalized == "" {
		return false
	}
	var cnt int64
	err := s.db.WithContext(ctx).Model(&models.ApplicationBlacklistOverride{}).
		Where("element_type = ? AND element_normalized = ? AND matched_blacklist_id = ?",
			elementType, elementNormalized, matchedBlacklistID).
		Count(&cnt).Error
	if err != nil {
		slog.Warn("blacklist suppression check failed", "err", err, "element_type", elementType)
		return false
	}
	return cnt > 0
}

// SubmitCompleteApplication создаёт полную заявку с вложениями, машинами и сотрудниками.
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
	// Организация и компания взаимозаменяемы: для отправки достаточно одной из двух.
	if strings.TrimSpace(req.Organization) == "" &&
		(req.Company == nil || strings.TrimSpace(*req.Company) == "") {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Укажите организацию или компанию")
	}

	// Серверная валидация настроенных обязательных полей (#529 H-9): поля, помеченные
	// админом required, должны присутствовать; скрытые/ненастроенные пропускаются.
	if err := s.validateConfiguredRequiredFields(ctx, req); err != nil {
		return nil, err
	}

	// Серверный гард ЧС (#443): отклоняем заявку с заблокированной машиной/человеком
	// до старта транзакции - на случай обхода фронтовой проверки.
	if err := s.validateBlacklist(ctx, req); err != nil {
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

	// Вставленные машины/сотрудники для пост-коммит проверки близости к ЧС (#481).
	var pendingVehicleFlags []pendingVehicleFlag
	var pendingEmployeeFlags []pendingEmployeeFlag

	// Создаём вложения
	for _, att := range req.Attachments {
		var attID int
		err := tx.Raw(`
			INSERT INTO attachments (application_id, attachment_type, attachment_name, attachment_display_name, unique_attachment_id, entry_date_from, entry_date_to, entry_time_from, entry_time_to, roof_access, free_parking, status)
			VALUES (?, ?, ?, ?, ?, ?::date, ?::date, ?::time, ?::time, ?, ?, 1)
			RETURNING id
		`, appID, att.AttachmentType, att.AttachmentName, att.AttachmentDisplayName, att.UniqueAttachmentID,
			att.EntryDateFrom, att.EntryDateTo, att.EntryTimeFrom, att.EntryTimeTo, att.RoofAccess, att.FreeParking).Scan(&attID).Error
		if err != nil {
			tx.Rollback()
			slog.Error("Ошибка создания вложения", "error", err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating attachment")
		}

		switch att.AttachmentType {
		case "cars":
			if att.Data.Vehicles != nil {
				// Дедуп-union мест всех машин вложения для attachment_unload_places (#706).
				// car_unload_places продолжаем писать для read-side и истории.
				carPlacesSet := make(map[int]struct{})
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

					pendingVehicleFlags = append(pendingVehicleFlags, pendingVehicleFlag{carID: carID, carNumber: v.CarNumber})

					carHistoryTime := baseTime.Add(time.Millisecond)
					tx.Exec(`
						INSERT INTO cars_history (car_id, user_id, action_type, comment, created_at)
						VALUES (?, ?, 'create', ?, ?)
					`, carID, user.ID, fmt.Sprintf("Автомобиль %s %s создан", v.CarNumber, v.CarBrand), carHistoryTime)

					for _, placeID := range v.UnloadPlaces {
						tx.Exec("INSERT INTO car_unload_places (car_id, unload_place_id, order_index) VALUES (?, ?, 1)", carID, placeID)
						carPlacesSet[placeID] = struct{}{}
					}
				}
				// Пишем дедупированные места в attachment_unload_places (источник для охранника).
				for placeID := range carPlacesSet {
					tx.Exec("INSERT INTO attachment_unload_places (attachment_id, unload_place_id) VALUES (?, ?) ON CONFLICT DO NOTHING", attID, placeID)
				}
			}

		case "people":
			if att.Data.Employees != nil {
				for _, e := range *att.Data.Employees {
					statusZero := 0
					lastName := e.LastName
					firstName := e.FirstName
					citizenshipID := e.CitizenshipID
					position := e.Position
					passportSeriesNumber := e.PassportSeriesNumber
					employee := models.Employee{
						AttachmentID:         &attID,
						LastName:             &lastName,
						FirstName:            &firstName,
						MiddleName:           e.MiddleName,
						CitizenshipID:        &citizenshipID,
						Position:             &position,
						PassportSeriesNumber: &passportSeriesNumber,
						PatentNumber:         e.PatentNumber,
						OtherPermission:      e.OtherPermission,
						Status:               &statusZero,
					}
					if err := tx.Create(&employee).Error; err != nil {
						tx.Rollback()
						return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating employee")
					}
					empID := employee.ID

					empHistoryTime := baseTime.Add(time.Millisecond)
					empMiddle := ""
					if e.MiddleName != nil {
						empMiddle = *e.MiddleName
					}
					pendingEmployeeFlags = append(pendingEmployeeFlags, pendingEmployeeFlag{
						empID: empID, lastName: lastName, firstName: firstName, middleName: empMiddle,
					})
					empComment := fmt.Sprintf("Сотрудник %s создан", strings.TrimSpace(strings.Join([]string{lastName, firstName, empMiddle}, " ")))
					tx.Exec(`
						INSERT INTO employees_history (employee_id, user_id, action_type, comment, created_at)
						VALUES (?, ?, 'create', ?, ?)
					`, empID, user.ID, empComment, empHistoryTime)

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
			// Места разгрузки для items приходят на уровне вложения (#706).
			for _, placeID := range att.UnloadPlaces {
				tx.Exec("INSERT INTO attachment_unload_places (attachment_id, unload_place_id) VALUES (?, ?) ON CONFLICT DO NOTHING", attID, placeID)
			}

		default:
			tx.Rollback()
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment type")
		}

		if att.CustomValues != nil {
			for _, cv := range *att.CustomValues {
				if cv.Value == "" {
					continue
				}
				if err := tx.Create(&models.AttachmentCustomValue{
					AttachmentID:  attID,
					CustomFieldID: cv.CustomFieldID,
					Value:         cv.Value,
				}).Error; err != nil {
					tx.Rollback()
					return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error saving custom field value")
				}
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	// Мягкая проверка возможного обхода ЧС по похожему номеру/ФИО (#481): помечает
	// элементы флагом для предупреждения согласующим. Best-effort, заявку не блокирует.
	// Синхронно (не в горутине) намеренно: флаги должны быть готовы сразу для детали
	// заявки и детерминированны для тестов; сабмит не hot-path, элементов в заявке мало.
	s.detectBlacklistSimilarity(ctx, appID, pendingVehicleFlags, pendingEmployeeFlags)

	// Уведомление отправителю о создании заявки
	if s.notificationService != nil {
		if err := s.notificationService.CreateForUser(
			ctx, user.ID,
			"application_created",
			"Заявка отправлена",
			fmt.Sprintf("Ваша заявка %s отправлена и ожидает согласования.", applicationNumber),
			nil,
		); err != nil {
			slog.Warn("notification create failed", "err", err, "user_id", user.ID, "app_id", appID)
		}
	}

	// Уведомления ответственным (approvers) о новой заявке
	if s.notificationService != nil {
		for _, ru := range responsibleUsers {
			if !ru.RequiredApproval || ru.UserID == user.ID {
				continue
			}
			if err := s.notificationService.CreateForUser(
				ctx, ru.UserID,
				"application_approval_required",
				"Требуется согласование",
				fmt.Sprintf("Поступила новая заявка %s на согласование.", applicationNumber),
				nil,
			); err != nil {
				slog.Warn("notification create failed", "err", err, "user_id", ru.UserID, "app_id", appID)
			}
		}
	}

	return &CompleteApplicationResponse{
		Success:           true,
		Message:           "Application created successfully",
		ApplicationID:     appID,
		ApplicationNumber: applicationNumber,
	}, nil
}

// UpdateApplication обновляет данные заявки (confirmation, status, комментарий).
func (s *applicationService) UpdateApplication(ctx context.Context, username string, applicationID int, req ApplicationUpdateRequest) (*ApplicationUpdateResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	setClauses := []string{}
	args := []interface{}{}

	if req.Confirmation != nil {
		if !allowedConfirmations[*req.Confirmation] {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid confirmation value")
		}
		// Гейт обхода ЧС (#481): прямое выставление "Согласовано" этим путём (минуя
		// поэлементное голосование) тоже блокируем, пока есть помеченные элементы без
		// override - иначе блокировку согласования из ApproveApplicationByUser легко обойти.
		if *req.Confirmation == models.ConfirmationApproved {
			blocked, err := hasUnoverriddenBlacklistFlags(ctx, s.db, applicationID)
			if err != nil {
				return nil, err
			}
			if blocked {
				return nil, echo.NewHTTPError(http.StatusConflict,
					"Заявка содержит элементы, похожие на чёрный список. Подтвердите пропуск каждого ('Всё равно пропустить') перед согласованием")
			}
		}
		setClauses = append(setClauses, "confirmation = ?")
		args = append(args, *req.Confirmation)
		if *req.Confirmation == "Согласовано" || *req.Confirmation == "Не согласовано" {
			setClauses = append(setClauses, "confirmation_datetime = ?")
			args = append(args, now)
		}
	}

	if req.Status != nil {
		if !allowedStatuses[*req.Status] {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Invalid status value")
		}
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
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating application")
	}

	return &ApplicationUpdateResponse{
		Success:      true,
		Message:      "Application updated successfully",
		RowsAffected: result.RowsAffected,
	}, nil
}
