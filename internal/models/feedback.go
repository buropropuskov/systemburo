package models

import "time"

// Feedback -- модель таблицы feedback.
type Feedback struct {
	ID                int        `json:"id"`
	UserID            int        `gorm:"index" json:"user_id"`
	User              User       `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Message           string     `gorm:"type:text" json:"message"`
	Status            string     `gorm:"size:20;default:'Не решено'" json:"status"`
	IsRead            bool       `gorm:"default:false" json:"is_read"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	ResolutionComment *string    `gorm:"type:text" json:"resolution_comment"`
	ResolvedAt        *time.Time `json:"resolved_at"`
}

func (Feedback) TableName() string { return "feedback" }

// FeedbackWithUser -- обращение с именем пользователя (для администраторов).
type FeedbackWithUser struct {
	ID                int        `json:"id"`
	UserID            int        `json:"user_id"`
	UserName          string     `json:"user_name"`
	Message           string     `json:"message"`
	Status            string     `json:"status"`
	IsRead            bool       `json:"is_read"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	ResolutionComment *string    `json:"resolution_comment"`
	ResolvedAt        *time.Time `json:"resolved_at"`
}

// MyFeedback -- обращение текущего пользователя.
type MyFeedback struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FeedbackStats -- статистика по обращениям.
type FeedbackStats struct {
	Total      int64 `json:"total"`
	Resolved   int64 `json:"resolved"`
	Unresolved int64 `json:"unresolved"`
	Unread     int64 `json:"unread"`
}

// CreateFeedbackRequest -- запрос на создание обращения.
type CreateFeedbackRequest struct {
	Message string `json:"message" validate:"required,min=10,max=1000"`
}

// UpdateFeedbackStatusRequest -- запрос на обновление статуса обращения.
// Comment -- необязательный ответ заявителю; сохраняется при переводе в "Решено".
type UpdateFeedbackStatusRequest struct {
	Status  string  `json:"status" validate:"required,oneof='Не решено' 'Решено'"`
	Comment *string `json:"comment" validate:"omitempty,max=1000"`
}

// MarkAsReadRequest -- запрос на отметку обращения прочитанным/непрочитанным.
type MarkAsReadRequest struct {
	IsRead bool `json:"is_read"`
}
