package models

import "time"

type Application struct {
	ID                   int          `json:"id"`
	ApplicationNumber    *string      `gorm:"size:50" json:"application_number"`
	Confirmation         *string      `gorm:"size:20" json:"confirmation"`
	SendingDatetime      *time.Time   `json:"sending_datetime"`
	ReadingDatetime      *time.Time   `json:"reading_datetime"`
	ConfirmationDatetime *time.Time   `json:"confirmation_datetime"`
	OrganizationID       int          `gorm:"index" json:"organization_id"`
	Organization         Organization `json:"-"`
	SenderUserID         int          `gorm:"index" json:"sender_user_id"`
	SenderUser           User         `gorm:"foreignKey:SenderUserID" json:"-"`
	Message              *string      `gorm:"type:text" json:"message"`
	Status               *string      `gorm:"size:50;index" json:"status"`
	ResponsibleUserID    *int         `gorm:"index" json:"responsible_user_id"`
	ResponsibleUser      *User        `gorm:"foreignKey:ResponsibleUserID" json:"-"`
	ResponsibleComment   *string      `gorm:"type:text" json:"responsible_comment"`
	DataApproval         *string      `gorm:"type:text" json:"data_approval"`
	CompanyID            *int         `gorm:"index" json:"company_id"`
	Company              *Company     `json:"-"`
}

type ApplicationHistory struct {
	ID            int         `json:"id"`
	ApplicationID int         `gorm:"index" json:"application_id"`
	Application   Application `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	UserID        *int        `gorm:"index" json:"user_id"`
	User          *User       `json:"-"`
	ActionType    string      `gorm:"size:50" json:"action_type"`
	ActionStatus  *string     `gorm:"size:50" json:"action_status"`
	OldValue      *string     `gorm:"type:text" json:"old_value"`
	NewValue      *string     `gorm:"type:text" json:"new_value"`
	Comment       *string     `gorm:"type:text" json:"comment"`
	CreatedAt     time.Time   `json:"created_at"`
	Metadata      *string     `gorm:"type:jsonb" json:"metadata"`
	ActionUserID  *int        `json:"action_user_id"`
}

func (ApplicationHistory) TableName() string { return "application_history" }

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
}

type ApplicationApprover struct {
	ID        int       `json:"id"`
	UserID    int       `gorm:"uniqueIndex" json:"user_id"`
	User      User      `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy *int      `json:"created_by"`
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
	CreatedAt    *time.Time `json:"created_at"`
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
	UserID int `json:"user_id"`
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
