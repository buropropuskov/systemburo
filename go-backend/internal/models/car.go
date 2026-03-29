package models

import "time"

type Car struct {
	ID                 int        `json:"id"`
	AttachmentID       int        `gorm:"index" json:"attachment_id"`
	Attachment         Attachment `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	CarNumber          *string    `gorm:"size:50;index" json:"car_number"`
	CarBrand           *string    `gorm:"size:100" json:"car_brand"`
	UnloadPlace        *string    `gorm:"size:255" json:"unload_place"`
	EntryDateFrom      *string    `gorm:"size:20" json:"entry_date_from"`
	EntryTimeFrom      *string    `gorm:"size:20" json:"entry_time_from"`
	EntryDateTo        *string    `gorm:"size:20" json:"entry_date_to"`
	EntryTimeTo        *string    `gorm:"size:20" json:"entry_time_to"`
	TerritoryEntryTime *time.Time `json:"territory_entry_time"`
	TerritoryStatus    *int       `json:"territory_status"`
	Status             *int       `gorm:"index" json:"status"`
	DateAdded          *time.Time `json:"date_added"`
	DateRemoved        *time.Time `json:"date_removed"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	TerritoryExitTime  *time.Time `json:"territory_exit_time"`
}

type CarHistory struct {
	ID         int       `json:"id"`
	CarID      int       `gorm:"index" json:"car_id"`
	Car        Car       `gorm:"constraint:OnDelete:CASCADE" json:"-"`
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

func (CarHistory) TableName() string { return "cars_history" }

type UniqueCar struct {
	ID             int           `json:"id"`
	Number         *string       `gorm:"size:50;index" json:"number"`
	Mark           *string       `gorm:"size:100" json:"mark"`
	OrganizationID *int          `gorm:"index" json:"organization_id"`
	Organization   *Organization `json:"-"`
	CompanyID      *int          `gorm:"index" json:"company_id"`
	Company        *Company      `json:"-"`
	FormatID       *int          `json:"format_id"`
	UserID         *int          `gorm:"index" json:"user_id"`
	User           *User         `json:"-"`
	Status         *int          `json:"status"`
	CreatedAt      time.Time     `json:"created_at"`
}

type CarUnloadPlace struct {
	ID            int     `json:"id"`
	CarID         int     `gorm:"index" json:"car_id"`
	Car           Car     `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	UnloadPlaceID int     `gorm:"index" json:"unload_place_id"`
	OrderIndex    *int    `json:"order_index"`
	PlannedTime   *string `gorm:"size:20" json:"planned_time"`
	Notes         *string `gorm:"type:text" json:"notes"`
}
