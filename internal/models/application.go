package models

import "time"

type Application struct {
	ID                   int        `json:"id"`
	ApplicationNumber    *string    `gorm:"size:50" json:"application_number"`
	Confirmation         *string    `gorm:"size:20" json:"confirmation"`
	SendingDatetime      *time.Time `json:"sending_datetime"`
	ReadingDatetime      *time.Time `json:"reading_datetime"`
	ConfirmationDatetime *time.Time `json:"confirmation_datetime"`
	// AcceptedAt - момент ПЕРВОГО принятия заявки в работу (#1240). Пишется COALESCE'ом:
	// revoke/restore возвращают заявку в "В обработке", и повторное принятие не перетирает
	// исходный момент - иначе длительность обработки считалась бы от последней попытки.
	AcceptedAt *time.Time `gorm:"index" json:"accepted_at"`
	// CompletedAt - момент завершения заявки по истечении срока вложений
	// (CheckExpiredAttachments, #1240). У завершённых до появления колонки остаётся NULL:
	// момента завершения в прошлом не зафиксировано, восстанавливать неоткуда.
	CompletedAt *time.Time `gorm:"index" json:"completed_at"`
	// WithdrawnAt - момент отзыва заявки отправителем. От него отсчитывается месяц
	// до архивации: отзыв гасит вложения сразу, поэтому их сроки для архива уже
	// ничего не значат. У отозванных до появления колонки остаётся NULL - такие
	// заявки архивируются по старому правилу (срокам вложений).
	WithdrawnAt *time.Time `gorm:"index" json:"withdrawn_at"`
	// StatusUpdatedAt - момент последней РЕАЛЬНОЙ смены status/confirmation (#1349).
	// Сравнивается с per-user application_status_views.seen_at: заявка подсвечивается
	// тем участникам, кто смену ещё не видел. NULL - переходов после внедрения не было.
	StatusUpdatedAt    *time.Time   `gorm:"index" json:"status_updated_at"`
	OrganizationID     int          `gorm:"index" json:"organization_id"`
	Organization       Organization `json:"-"`
	SenderUserID       int          `gorm:"index" json:"sender_user_id"`
	SenderUser         User         `gorm:"foreignKey:SenderUserID" json:"-"`
	Message            *string      `gorm:"type:text" json:"message"`
	Status             *string      `gorm:"size:50;index" json:"status"`
	ResponsibleUserID  *int         `gorm:"index" json:"responsible_user_id"`
	ResponsibleUser    *User        `gorm:"foreignKey:ResponsibleUserID" json:"-"`
	ResponsibleComment *string      `gorm:"type:text" json:"responsible_comment"`
	DataApproval       *string      `gorm:"type:text" json:"data_approval"`
	// InitiatorName и ContactPhone - «Инициатор заявки» и «Телефон» из шапки подачи
	// (#1454). Раньше форма их требовала, а бэк отбрасывал: сохранять было некуда, и в
	// бланк попадали только профильные данные отправителя, даже если заявитель указал
	// другого человека.
	InitiatorName *string `gorm:"size:255" json:"initiator_name"`
	ContactPhone  *string `gorm:"size:50" json:"contact_phone"`
	CompanyID          *int         `gorm:"index" json:"company_id"`
	Company            *Company     `json:"-"`
}

type ApplicationStatusHistory struct {
	ID              int         `json:"id"`
	ApplicationID   int         `gorm:"index" json:"application_id"`
	Application     Application `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	OldStatus       *string     `gorm:"size:50" json:"old_status"`
	NewStatus       *string     `gorm:"size:50" json:"new_status"`
	ChangedByUserID *int        `json:"changed_by_user_id"`
	Comment         *string     `gorm:"type:text" json:"comment"`
	ChangedAt       time.Time   `json:"changed_at"`
}

func (ApplicationStatusHistory) TableName() string { return "application_status_history" }

type ApplicationResponsibleUser struct {
	ID               int         `json:"id"`
	ApplicationID    int         `gorm:"uniqueIndex:idx_app_resp_user" json:"application_id"`
	Application      Application `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	UserID           int         `gorm:"uniqueIndex:idx_app_resp_user" json:"user_id"`
	User             User        `json:"-"`
	CreatedAt        time.Time   `json:"created_at"`
	IsPrimary        bool        `gorm:"default:false" json:"is_primary"`
	RequiredApproval bool        `gorm:"default:false" json:"required_approval"`
	ApprovalStatus   *string     `gorm:"size:20;default:'pending'" json:"approval_status"`
	ApprovalComment  *string     `gorm:"type:text" json:"approval_comment"`
	ApprovalDatetime *time.Time  `json:"approval_datetime"`
	CreatedBy        *int        `json:"created_by"`
	// LastReminderAt/ReminderCount -- отметки автонапоминаний зависшим согласующим
	// (#1315, ReminderService). LastReminderAt=NULL означает "напоминаний не было".
	LastReminderAt *time.Time `json:"last_reminder_at"`
	ReminderCount  int        `gorm:"default:0" json:"reminder_count"`
}

type ApplicationApprover struct {
	ID     int  `json:"id"`
	UserID int  `gorm:"uniqueIndex" json:"user_id"`
	User   User `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	// DisplayName - маска ФИО принимающего для заявитель-видимых мест ("Принял", история
	// заявки). Пусто/NULL - показывается реальное ФИО из users. Само реальное ФИО не хранится.
	DisplayName *string   `gorm:"size:255" json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   *int      `json:"created_by"`
}

// ApplicationApproverWithUser — ответ GET /application-approvers с данными пользователя.
type ApplicationApproverWithUser struct {
	ID           int        `json:"id"`
	UserID       int        `json:"user_id"`
	Username     string     `json:"username"`
	LastName     *string    `json:"last_name"`
	FirstName    *string    `json:"first_name"`
	MiddleName   *string    `json:"middle_name"`
	Position     *string    `json:"position"`
	Organization *string    `json:"organization"`
	Company      *string    `json:"company"`
	DisplayName  *string    `json:"display_name"`
	CreatedAt    *time.Time `json:"created_at"`
}

// ApplicationRecipient — принимающий в списке получателей заявки. Отдаётся любому
// работнику: заявитель должен видеть, кому уйдёт заявка. Поэтому здесь только
// отображаемое имя - маска, если администратор её задал, иначе ФИО. Ни организации,
// ни должности, ни контактов: полный состав с этими сведениями отдаёт GetAll, и он
// закрыт правом администратора.
type ApplicationRecipient struct {
	UserID int    `json:"user_id"`
	Name   string `json:"name"`
	Masked bool   `json:"masked"`
}

// AvailableApproverUser — пользователь, доступный для назначения утверждающим.
type AvailableApproverUser struct {
	ID           int     `json:"id"`
	Username     string  `json:"username"`
	LastName     *string `json:"last_name"`
	FirstName    *string `json:"first_name"`
	MiddleName   *string `json:"middle_name"`
	Position     *string `json:"position"`
	Organization *string `json:"organization"`
	Company      *string `json:"company"`
}

// CreateApproverRequest — запрос POST /application-approvers.
type CreateApproverRequest struct {
	UserID int `json:"user_id" validate:"gte=1"`
}

// UpdateApproverRequest — запрос PATCH /application-approvers/:id: задать/снять маску
// отображаемого имени. null или пустая строка снимают маску (показывается реальное ФИО).
type UpdateApproverRequest struct {
	DisplayName *string `json:"display_name"`
}

type ApplicationViewer struct {
	ID            int         `json:"id"`
	ApplicationID int         `gorm:"index" json:"application_id"`
	Application   Application `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	UserID        int         `gorm:"index" json:"user_id"`
	User          User        `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt     time.Time   `json:"created_at"`
	CreatedBy     *int        `json:"created_by"`
}
