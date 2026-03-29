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
