package models

import "time"

type Employee struct {
	ID                   int          `json:"id"`
	AttachmentID         int          `gorm:"index" json:"attachment_id"`
	Attachment           Attachment   `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	LastName             *string      `gorm:"size:100" json:"last_name"`
	FirstName            *string      `gorm:"size:100" json:"first_name"`
	MiddleName           *string      `gorm:"size:100" json:"middle_name"`
	CitizenshipID        *int         `gorm:"index" json:"citizenship_id"`
	Citizenship          *Citizenship `json:"-"`
	Position             *string      `gorm:"size:100;column:position" json:"position"`
	PassportSeriesNumber *string      `gorm:"size:50" json:"passport_series_number"`
	PatentNumber         *string      `gorm:"size:50" json:"patent_number"`
	OtherPermission      *string      `gorm:"type:text" json:"other_permission"`
	TerritoryEntryTime   *time.Time   `json:"territory_entry_time"`
	TerritoryStatus      *int         `json:"territory_status"`
	Status               *int         `gorm:"index" json:"status"`
	DateCreated          *time.Time   `json:"date_created"`
	DateDeleted          *time.Time   `json:"date_deleted"`
	CreatedAt            time.Time    `json:"created_at"`
	UpdatedAt            time.Time    `json:"updated_at"`
}

type EmployeeHistory struct {
	ID         int       `json:"id"`
	EmployeeID int       `gorm:"index" json:"employee_id"`
	Employee   Employee  `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	UserID     *int      `gorm:"index" json:"user_id"`
	User       *User     `json:"-"`
	ActionType string    `gorm:"size:50" json:"action_type"`
	FieldName  *string   `gorm:"size:100" json:"field_name"`
	OldValue   *string   `gorm:"type:text" json:"old_value"`
	NewValue   *string   `gorm:"type:text" json:"new_value"`
	Comment    *string   `gorm:"type:text" json:"comment"`
	Metadata   *string   `gorm:"type:jsonb" json:"metadata"`
	CreatedAt  time.Time `json:"created_at"`
	TableID    *int      `json:"table_id"`
}

func (EmployeeHistory) TableName() string { return "employees_history" }

type UniqueEmployee struct {
	ID                   int           `json:"id"`
	LastName             *string       `gorm:"size:100" json:"last_name"`
	FirstName            *string       `gorm:"size:100" json:"first_name"`
	MiddleName           *string       `gorm:"size:100" json:"middle_name"`
	CitizenshipID        *int          `gorm:"index" json:"citizenship_id"`
	Citizenship          *Citizenship  `json:"-"`
	Position             *string       `gorm:"size:100;column:position" json:"position"`
	PassportSeriesNumber *string       `gorm:"size:50" json:"passport_series_number"`
	PatentNumber         *string       `gorm:"size:50" json:"patent_number"`
	OtherPermission      *string       `gorm:"type:text" json:"other_permission"`
	OrganizationID       *int          `gorm:"index" json:"organization_id"`
	Organization         *Organization `json:"-"`
	CompanyID            *int          `gorm:"index" json:"company_id"`
	Company              *Company      `json:"-"`
	UserID               *int          `gorm:"index" json:"user_id"`
	User                 *User         `json:"-"`
	Status               *int          `json:"status"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
}

type ApplicationEmployee struct {
	ID                   int        `json:"id"`
	AttachmentID         int        `gorm:"index" json:"attachment_id"`
	Attachment           Attachment `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	LastName             *string    `gorm:"size:100" json:"last_name"`
	FirstName            *string    `gorm:"size:100" json:"first_name"`
	MiddleName           *string    `gorm:"size:100" json:"middle_name"`
	Position             *string    `gorm:"size:100;column:position" json:"position"`
	CitizenshipID        *int       `json:"citizenship_id"`
	PassportSeriesNumber *string    `gorm:"size:50" json:"passport_series_number"`
	PatentNumber         *string    `gorm:"size:50" json:"patent_number"`
	OtherPermission      *string    `gorm:"type:text" json:"other_permission"`
	OrderIndex           *int       `json:"order_index"`
	CreatedAt            time.Time  `json:"created_at"`
}

type EmployeeFile struct {
	ID         int       `json:"id"`
	EmployeeID int       `gorm:"index" json:"employee_id"`
	Employee   Employee  `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	FilePath   string    `gorm:"size:500" json:"file_path"`
	FileType   *string   `gorm:"size:50" json:"file_type"`
	FileName   *string   `gorm:"size:255" json:"file_name"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type EmployeeTargetTable struct {
	ID         int  `json:"id"`
	EmployeeID int  `gorm:"index" json:"employee_id"`
	TableID    int  `gorm:"index" json:"table_id"`
	OrderIndex *int `json:"order_index"`
}
