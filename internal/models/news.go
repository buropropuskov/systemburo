package models

import "time"

type News struct {
	ID          int       `json:"id"`
	Title       string    `gorm:"size:255" json:"title"`
	Description *string   `gorm:"type:text" json:"description"`
	FullText    *string   `gorm:"type:text" json:"full_text"`
	CreatedBy   *int      `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   *int      `json:"updated_by"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
}

func (News) TableName() string { return "news" }

type Announcement struct {
	ID          int        `json:"id"`
	Title       string     `gorm:"size:255" json:"title"`
	Description *string    `gorm:"type:text" json:"description"`
	FullText    *string    `gorm:"type:text" json:"full_text"`
	IsImportant bool       `gorm:"default:false" json:"is_important"`
	IsActive    bool       `gorm:"default:true" json:"is_active"`
	CreatedBy   *int       `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	UpdatedBy   *int       `json:"updated_by"`
	ActivatedAt *time.Time `json:"activated_at"`
	ActivatedBy *int       `json:"activated_by"`
}

// CreateNewsRequest -- запрос на создание новости.
type CreateNewsRequest struct {
	Title       string  `json:"title" validate:"required,max=255"`
	Description *string `json:"description"`
	FullText    *string `json:"full_text"`
	IsActive    *bool   `json:"is_active"`
}

// UpdateNewsRequest -- запрос на обновление новости.
type UpdateNewsRequest struct {
	Title       *string `json:"title" validate:"omitempty,max=255"`
	Description *string `json:"description"`
	FullText    *string `json:"full_text"`
	IsActive    *bool   `json:"is_active"`
}

// NewsWithUser -- новость с именами создателя и обновившего.
type NewsWithUser struct {
	ID            int        `json:"id"`
	Title         string     `json:"title"`
	Description   *string    `json:"description"`
	FullText      *string    `json:"full_text"`
	IsActive      bool       `json:"is_active"`
	CreatedBy     *int       `json:"created_by"`
	CreatedByName *string    `json:"created_by_name"`
	UpdatedBy     *int       `json:"updated_by"`
	UpdatedByName *string    `json:"updated_by_name"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// CreateAnnouncementRequest -- запрос на создание объявления.
type CreateAnnouncementRequest struct {
	Title       string  `json:"title" validate:"required,max=255"`
	Description *string `json:"description"`
	FullText    *string `json:"full_text"`
	IsImportant *bool   `json:"is_important"`
}

// UpdateAnnouncementRequest -- запрос на обновление объявления.
type UpdateAnnouncementRequest struct {
	Title       *string `json:"title" validate:"omitempty,max=255"`
	Description *string `json:"description"`
	FullText    *string `json:"full_text"`
	IsImportant *bool   `json:"is_important"`
}

// SetActiveAnnouncementRequest -- запрос на установку активного объявления.
type SetActiveAnnouncementRequest struct {
	AnnouncementID int `json:"announcement_id" validate:"required,min=1"`
}

// AnnouncementWithUser -- объявление с именами создателя, обновившего и активировавшего.
type AnnouncementWithUser struct {
	ID              int        `json:"id"`
	Title           string     `json:"title"`
	Description     *string    `json:"description"`
	FullText        *string    `json:"full_text"`
	IsImportant     bool       `json:"is_important"`
	IsActive        bool       `json:"is_active"`
	CreatedBy       *int       `json:"created_by"`
	CreatedByName   *string    `json:"created_by_name"`
	UpdatedBy       *int       `json:"updated_by"`
	UpdatedByName   *string    `json:"updated_by_name"`
	ActivatedAt     *time.Time `json:"activated_at"`
	ActivatedBy     *int       `json:"activated_by"`
	ActivatedByName *string    `json:"activated_by_name"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
