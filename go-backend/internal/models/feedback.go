package models

import "time"

type Feedback struct {
	ID                int        `json:"id"`
	UserID            int        `gorm:"index" json:"user_id"`
	User              User       `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Message           string     `gorm:"type:text" json:"message"`
	Status            string     `gorm:"size:20;default:'pending'" json:"status"`
	IsRead            bool       `gorm:"default:false" json:"is_read"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	ResolutionComment *string    `gorm:"type:text" json:"resolution_comment"`
	ResolvedAt        *time.Time `json:"resolved_at"`
}

func (Feedback) TableName() string { return "feedback" }

type FeedbackMessage struct {
	ID                 int        `json:"id"`
	UserID             int        `gorm:"index" json:"user_id"`
	User               User       `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Username           *string    `gorm:"size:100" json:"username"`
	Message            string     `gorm:"type:text" json:"message"`
	Status             string     `gorm:"size:20;default:'pending'" json:"status"` // pending, in_progress, resolved
	CreatedAt          time.Time  `json:"created_at"`
	ResolvedAt         *time.Time `json:"resolved_at"`
	ResolvedBy         *int       `json:"resolved_by"`
	ResolvedByUsername *string    `gorm:"size:100" json:"resolved_by_username"`
}
