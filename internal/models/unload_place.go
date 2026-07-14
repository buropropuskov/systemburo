package models

import "time"

type UnloadPlace struct {
	ID          int     `json:"id"`
	Name        string  `gorm:"size:200" json:"name"`
	Description *string `gorm:"type:text" json:"description"`
	// Warning - свободное предупреждение, показывается заявителю всегда при
	// добавлении машины/человека с этим местом (#1183).
	Warning       *string   `gorm:"type:text" json:"warning"`
	IsActive      bool      `gorm:"default:true;index" json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	MapLink       *string   `gorm:"size:500" json:"map_link"`
	StatusComment *string   `gorm:"type:text" json:"status_comment"`
	Status        string    `gorm:"size:20;default:'active'" json:"status"` // active, inactive, maintenance
	UpdatedAt     time.Time `json:"updated_at"`
}

type UnloadPlacePhoto struct {
	ID            int         `json:"id"`
	UnloadPlaceID int         `gorm:"index" json:"unload_place_id"`
	UnloadPlace   UnloadPlace `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	PhotoURL      string      `gorm:"size:500" json:"photo_url"`
	FileName      *string     `gorm:"size:255" json:"file_name"`
	FileSize      *int64      `json:"file_size"`
	MimeType      *string     `gorm:"size:100" json:"mime_type"`
	IsMain        bool        `gorm:"default:false" json:"is_main"`
	UploadedAt    time.Time   `json:"uploaded_at"`
	UploadedBy    *int        `json:"uploaded_by"`
}

type UnloadPlaceTimeSlot struct {
	ID            int         `json:"id"`
	UnloadPlaceID int         `gorm:"index" json:"unload_place_id"`
	UnloadPlace   UnloadPlace `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	DayOfWeek     int         `json:"day_of_week"` // 0-6
	OpenTime      string      `gorm:"size:10" json:"open_time"`
	CloseTime     string      `gorm:"size:10" json:"close_time"`
	IsNextDay     bool        `gorm:"default:false" json:"is_next_day"`
	IsActive      bool        `gorm:"default:true" json:"is_active"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// GetID возвращает идентификатор слота (контракт timeSlotModel для общего стора).
func (s UnloadPlaceTimeSlot) GetID() int { return s.ID }

// UnloadPlaceWarningWindow -- предупреждение по временному окну у места разгрузки.
// Зеркало UnloadPlaceTimeSlot с текстом: показывается заявителю, когда срок заявки
// пересекается с окном (кейс "с 12:00 до 13:00 только малогабарит", #1183).
// DayOfWeek nil = окно на каждый день; TimeFrom/TimeTo nil = весь день.
type UnloadPlaceWarningWindow struct {
	ID            int         `json:"id"`
	UnloadPlaceID int         `gorm:"index" json:"unload_place_id"`
	UnloadPlace   UnloadPlace `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	DayOfWeek     *int        `json:"day_of_week"`              // nil = каждый день, иначе 0-6
	TimeFrom      *string     `gorm:"size:10" json:"time_from"` // nil = весь день
	TimeTo        *string     `gorm:"size:10" json:"time_to"`
	IsNextDay     bool        `gorm:"default:false" json:"is_next_day"`
	Message       string      `gorm:"type:text" json:"message"`
	IsActive      bool        `gorm:"default:true" json:"is_active"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// GetID возвращает идентификатор окна (контракт warningWindowModel для общего стора).
func (w UnloadPlaceWarningWindow) GetID() int { return w.ID }

type OrganizationUnloadPlace struct {
	ID             int          `json:"id"`
	OrganizationID int          `gorm:"index" json:"organization_id"`
	Organization   Organization `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	UnloadPlaceID  int          `gorm:"index" json:"unload_place_id"`
	UnloadPlace    UnloadPlace  `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt      time.Time    `json:"created_at"`
}

type CompaniesUnloadPlace struct {
	ID            int         `json:"id"`
	CompanyID     int         `gorm:"index" json:"company_id"`
	Company       Company     `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	UnloadPlaceID int         `gorm:"index" json:"unload_place_id"`
	UnloadPlace   UnloadPlace `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt     time.Time   `json:"created_at"`
}
