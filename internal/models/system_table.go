package models

import "time"

// Значения SystemTable.TableType.
const (
	TableTypeCars   = "cars"
	TableTypePeople = "people"
)

type SystemTable struct {
	ID                  int       `json:"id"`
	Name                string    `gorm:"size:100" json:"name"`
	DisplayName         *string   `gorm:"size:200" json:"display_name"`
	TableType           string    `gorm:"size:20;default:'cars'" json:"table_type"` // cars, people
	ShowFactTable       bool      `gorm:"default:false" json:"show_fact_table"`
	FactTableHint       *string   `gorm:"type:text" json:"fact_table_hint"` // форматированный HTML из TextConstructor - длина не лезет в varchar(255)
	IsActive            bool      `gorm:"default:true;index" json:"is_active"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	Instruction         *string   `gorm:"type:text" json:"instruction"`
	MapLink             *string   `gorm:"size:500" json:"map_link"`
	Status              string    `gorm:"size:20;default:'active'" json:"status"` // active, inactive, maintenance
	StatusComment       *string   `gorm:"type:text" json:"status_comment"`
	LocationDescription *string   `gorm:"type:text" json:"location_description"`
	// Warning - свободное предупреждение, показывается заявителю всегда при
	// добавлении машины/человека с этим местом (#1183).
	Warning *string `gorm:"type:text" json:"warning"`

	// Оформление таблицы (#345). FontSize - размер шрифта строк (px, 10-24).
	// RowDensity - плотность строк: compact|normal|spacious.
	FontSize   int    `gorm:"default:14" json:"font_size"`
	RowDensity string `gorm:"size:20;default:'normal'" json:"row_density"`
	// То же самое, но для FactTable (отдельные настройки).
	FontSizeFact   int    `gorm:"default:14" json:"font_size_fact"`
	RowDensityFact string `gorm:"size:20;default:'normal'" json:"row_density_fact"`

	Fields         []TableField               `gorm:"foreignKey:TableID" json:"fields,omitempty"`
	FactFields     []TableFieldFact           `gorm:"foreignKey:TableID" json:"fact_fields,omitempty"`
	TimeSlots      []SystemTableTimeSlot      `gorm:"foreignKey:TableID" json:"time_slots,omitempty"`
	WarningWindows []SystemTableWarningWindow `gorm:"foreignKey:TableID" json:"warning_windows,omitempty"`
	Photos         []SystemTablePhoto         `gorm:"foreignKey:TableID" json:"photos,omitempty"`
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

// GetID возвращает идентификатор слота (контракт timeSlotModel для общего стора).
func (s SystemTableTimeSlot) GetID() int { return s.ID }

// SystemTableWarningWindow -- предупреждение по временному окну у системной таблицы
// (проезда/прохода). Зеркало SystemTableTimeSlot с текстом: показывается заявителю,
// когда срок заявки пересекается с окном (кейс "с 12:00 до 13:00 только малогабарит",
// #1183). DayOfWeek nil = окно на каждый день; TimeFrom/TimeTo nil = весь день.
type SystemTableWarningWindow struct {
	ID        int         `json:"id"`
	TableID   int         `gorm:"index" json:"table_id"`
	Table     SystemTable `gorm:"foreignKey:TableID;constraint:OnDelete:CASCADE" json:"-"`
	DayOfWeek *int        `json:"day_of_week"`              // nil = каждый день, иначе 0-6
	TimeFrom  *string     `gorm:"size:10" json:"time_from"` // nil = весь день
	TimeTo    *string     `gorm:"size:10" json:"time_to"`
	IsNextDay bool        `gorm:"default:false" json:"is_next_day"`
	Message   string      `gorm:"type:text" json:"message"`
	IsActive  bool        `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// GetID возвращает идентификатор окна (контракт warningWindowModel для общего стора).
func (w SystemTableWarningWindow) GetID() int { return w.ID }

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
	Width int `gorm:"default:10" json:"width"`
	// Priority - приоритет столбца для портретного режима (1-5).
	// 1 = всегда виден, 2 = виден на компактных экранах, 3-5 = скрывается в портрете.
	Priority int `gorm:"default:3" json:"priority"`
	// Настройки для режима "Увеличенный" (#345).
	// EnlargedIsVisible - видимость в enlarged (по умолчанию true).
	// EnlargedWidth - вес ширины в enlarged (0 = брать обычный Width).
	// EnlargedFontWeight - жирность шрифта в enlarged (400/500/600/700; 0 = default 500).
	EnlargedIsVisible  bool      `gorm:"default:true" json:"enlarged_is_visible"`
	EnlargedWidth      int       `gorm:"default:0" json:"enlarged_width"`
	EnlargedFontWeight int       `gorm:"default:0" json:"enlarged_font_weight"`
	CreatedAt          time.Time `json:"created_at"`
}

// TableFieldFact - настраиваемые столбцы FactTable. Хранится отдельно от
// TableField, чтобы основная и фактовая таблицы конфигурировались независимо.
// Семантика полей идентична TableField.
type TableFieldFact struct {
	ID           int         `json:"id"`
	TableID      int         `gorm:"index" json:"table_id"`
	Table        SystemTable `gorm:"foreignKey:TableID;constraint:OnDelete:CASCADE" json:"-"`
	FieldName    string      `gorm:"size:100" json:"field_name"`
	FieldType    *string     `gorm:"size:50" json:"field_type"`
	DisplayOrder *int        `json:"display_order"`
	IsVisible    bool        `gorm:"default:true" json:"is_visible"`
	Width        int         `gorm:"default:10" json:"width"`
	Priority     int         `gorm:"default:3" json:"priority"`
	CreatedAt    time.Time   `json:"created_at"`
}

// SystemTableWithDetails -- таблица с полями, слотами, фото и текущим статусом (открыто/закрыто).
type SystemTableWithDetails struct {
	Table      SystemTable           `json:"table"`
	Fields     []TableField          `json:"fields"`
	FactFields []TableFieldFact      `json:"fact_fields"`
	TimeSlots  []SystemTableTimeSlot `json:"time_slots"`
	// WarningWindows -- предупреждения по временным окнам (#1183), показываются
	// заявителю, когда срок заявки пересекается с окном.
	WarningWindows []SystemTableWarningWindow `json:"warning_windows"`
	Photos         []SystemTablePhoto         `json:"photos"`
	CurrentStatus  string                     `json:"current_status"`
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
	Warning             *string `json:"warning"`
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
	Warning             *string `json:"warning"`
	// Оформление таблицы (#345). Валидируется в сервисе: FontSize 10-24,
	// RowDensity in {compact, normal, spacious}.
	FontSize       *int    `json:"font_size"`
	RowDensity     *string `json:"row_density"`
	FontSizeFact   *int    `json:"font_size_fact"`
	RowDensityFact *string `json:"row_density_fact"`
}

// CreateTimeSlotRequest -- запрос на создание временного слота.
type CreateTimeSlotRequest struct {
	DayOfWeek int    `json:"day_of_week" validate:"min=0,max=6"`
	OpenTime  string `json:"open_time" validate:"required"`
	CloseTime string `json:"close_time" validate:"required"`
	IsNextDay *bool  `json:"is_next_day"`
	IsActive  *bool  `json:"is_active"`
}

// UpdateTimeSlotRequest -- запрос на обновление временного слота (все поля опциональные).
type UpdateTimeSlotRequest struct {
	DayOfWeek *int    `json:"day_of_week"`
	OpenTime  *string `json:"open_time"`
	CloseTime *string `json:"close_time"`
	IsNextDay *bool   `json:"is_next_day"`
	IsActive  *bool   `json:"is_active"`
}

// FieldVisibilityUpdate -- одиночное обновление видимости, порядка, ширины и
// приоритета столбца (#345). DisplayOrder, Width, Priority и enlarged-поля
// опциональны - не передаются, если их не меняли.
type FieldVisibilityUpdate struct {
	FieldName          string `json:"field_name" validate:"required"`
	IsVisible          bool   `json:"is_visible"`
	DisplayOrder       *int   `json:"display_order"`
	Width              *int   `json:"width"`
	Priority           *int   `json:"priority"`
	EnlargedIsVisible  *bool  `json:"enlarged_is_visible"`
	EnlargedWidth      *int   `json:"enlarged_width"`
	EnlargedFontWeight *int   `json:"enlarged_font_weight"`
}

// UpdateFieldsRequest -- bulk-обновление видимости столбцов системной таблицы.
type UpdateFieldsRequest struct {
	Fields []FieldVisibilityUpdate `json:"fields" validate:"required"`
}
