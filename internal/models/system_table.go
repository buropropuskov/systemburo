package models

import "time"

type SystemTable struct {
	ID                  int       `json:"id"`
	Name                string    `gorm:"size:100" json:"name"`
	DisplayName         *string   `gorm:"size:200" json:"display_name"`
	TableType           string    `gorm:"size:20;default:'cars'" json:"table_type"` // cars, people
	ShowFactTable       bool      `gorm:"default:false" json:"show_fact_table"`
	FactTableHint       *string   `gorm:"size:255" json:"fact_table_hint"`
	IsActive            bool      `gorm:"default:true;index" json:"is_active"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	Instruction         *string   `gorm:"type:text" json:"instruction"`
	MapLink             *string   `gorm:"size:500" json:"map_link"`
	Status              string    `gorm:"size:20;default:'active'" json:"status"` // active, inactive, maintenance
	StatusComment       *string   `gorm:"type:text" json:"status_comment"`
	LocationDescription *string   `gorm:"type:text" json:"location_description"`

	Fields    []TableField            `gorm:"foreignKey:TableID" json:"fields,omitempty"`
	TimeSlots []SystemTableTimeSlot   `gorm:"foreignKey:TableID" json:"time_slots,omitempty"`
	Photos    []SystemTablePhoto      `gorm:"foreignKey:TableID" json:"photos,omitempty"`
}

type SystemTablePhoto struct {
	ID         int         `json:"id"`
	TableID    int         `gorm:"index" json:"table_id"`
	Table      SystemTable `gorm:"foreignKey:TableID;constraint:OnDelete:CASCADE" json:"-"`
	PhotoURL   string      `gorm:"size:500" json:"photo_url"`
	FileName   *string     `gorm:"size:255" json:"file_name"`
	FileSize   *int64      `json:"file_size"`
	MimeType   *string     `gorm:"size:100" json:"mime_type"`
	IsMain     bool        `gorm:"default:false" json:"is_main"`
	UploadedAt time.Time   `json:"uploaded_at"`
	UploadedBy *int        `json:"uploaded_by"`
}

type SystemTableTimeSlot struct {
	ID        int         `json:"id"`
	TableID   int         `gorm:"index" json:"table_id"`
	Table     SystemTable `gorm:"foreignKey:TableID;constraint:OnDelete:CASCADE" json:"-"`
	DayOfWeek int         `json:"day_of_week"` // 0-6
	OpenTime  string      `gorm:"size:10" json:"open_time"`
	CloseTime string      `gorm:"size:10" json:"close_time"`
	IsNextDay bool        `gorm:"default:false" json:"is_next_day"`
	IsActive  bool        `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type OrganizationTable struct {
	ID             int          `json:"id"`
	OrganizationID int          `gorm:"index" json:"organization_id"`
	Organization   Organization `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	TableID        int          `gorm:"index" json:"table_id"`
	Table          SystemTable  `gorm:"foreignKey:TableID;constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt      time.Time    `json:"created_at"`
}

type CompaniesTable struct {
	ID        int         `json:"id"`
	CompanyID int         `gorm:"index" json:"company_id"`
	Company   Company     `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	TableID   int         `gorm:"index" json:"table_id"`
	Table     SystemTable `gorm:"foreignKey:TableID;constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt time.Time   `json:"created_at"`
}

type TableField struct {
	ID           int         `json:"id"`
	TableID      int         `gorm:"index" json:"table_id"`
	Table        SystemTable `gorm:"foreignKey:TableID;constraint:OnDelete:CASCADE" json:"-"`
	FieldName    string      `gorm:"size:100" json:"field_name"`
	FieldType    *string     `gorm:"size:50" json:"field_type"`
	DisplayOrder *int        `json:"display_order"`
	IsVisible    bool        `gorm:"default:true" json:"is_visible"`
	// Width - относительный вес ширины столбца. Используется как flex-grow:
	// браузер делит доступную ширину пропорционально весам видимых столбцов.
	Width     int       `gorm:"default:10" json:"width"`
	CreatedAt time.Time `json:"created_at"`
}

// SystemTableWithDetails -- таблица с полями, слотами, фото и текущим статусом (открыто/закрыто).
type SystemTableWithDetails struct {
	Table         SystemTable          `json:"table"`
	Fields        []TableField         `json:"fields"`
	TimeSlots     []SystemTableTimeSlot `json:"time_slots"`
	Photos        []SystemTablePhoto   `json:"photos"`
	CurrentStatus string               `json:"current_status"`
}

// CreateSystemTableRequest -- запрос на создание системной таблицы.
type CreateSystemTableRequest struct {
	Name                string  `json:"name" validate:"required"`
	DisplayName         string  `json:"display_name" validate:"required"`
	TableType           string  `json:"table_type" validate:"required"`
	ShowFactTable       *bool   `json:"show_fact_table"`
	FactTableHint       *string `json:"fact_table_hint"`
	Instruction         *string `json:"instruction"`
	MapLink             *string `json:"map_link"`
	Status              *string `json:"status"`
	StatusComment       *string `json:"status_comment"`
	LocationDescription *string `json:"location_description"`
}

// UpdateSystemTableRequest -- запрос на обновление системной таблицы (все поля опциональные).
type UpdateSystemTableRequest struct {
	DisplayName         *string `json:"display_name"`
	TableType           *string `json:"table_type"`
	ShowFactTable       *bool   `json:"show_fact_table"`
	FactTableHint       *string `json:"fact_table_hint"`
	Instruction         *string `json:"instruction"`
	MapLink             *string `json:"map_link"`
	Status              *string `json:"status"`
	StatusComment       *string `json:"status_comment"`
	LocationDescription *string `json:"location_description"`
}

// CreateTimeSlotRequest -- запрос на создание временного слота.
type CreateTimeSlotRequest struct {
	DayOfWeek int     `json:"day_of_week" validate:"min=0,max=6"`
	OpenTime  string  `json:"open_time" validate:"required"`
	CloseTime string  `json:"close_time" validate:"required"`
	IsNextDay *bool   `json:"is_next_day"`
	IsActive  *bool   `json:"is_active"`
}

// UpdateTimeSlotRequest -- запрос на обновление временного слота (все поля опциональные).
type UpdateTimeSlotRequest struct {
	DayOfWeek *int    `json:"day_of_week"`
	OpenTime  *string `json:"open_time"`
	CloseTime *string `json:"close_time"`
	IsNextDay *bool   `json:"is_next_day"`
	IsActive  *bool   `json:"is_active"`
}

// FieldVisibilityUpdate -- одиночное обновление видимости, порядка и ширины столбца (#345).
// DisplayOrder и Width опциональны - если не переданы, не меняются.
type FieldVisibilityUpdate struct {
	FieldName    string `json:"field_name" validate:"required"`
	IsVisible    bool   `json:"is_visible"`
	DisplayOrder *int   `json:"display_order"`
	Width        *int   `json:"width"`
}

// UpdateFieldsRequest -- bulk-обновление видимости столбцов системной таблицы.
type UpdateFieldsRequest struct {
	Fields []FieldVisibilityUpdate `json:"fields" validate:"required"`
}
