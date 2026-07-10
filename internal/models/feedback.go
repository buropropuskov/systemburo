package models

import "time"

// Feedback -- модель таблицы feedback.
// IsRead -- legacy-колонка глобального прочтения (оставлена для GetMy заявителя).
// Админское прочтение теперь персональное - через таблицу feedback_reads.
// Flagged -- общий флажок "важное / взять в работу", виден всем администраторам.
type Feedback struct {
	ID                int        `json:"id"`
	UserID            int        `gorm:"index" json:"user_id"`
	User              User       `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Message           string     `gorm:"type:text" json:"message"`
	Status            string     `gorm:"size:20;default:'Не решено'" json:"status"`
	IsRead            bool       `gorm:"default:false" json:"is_read"`
	Flagged           bool       `gorm:"default:false;index" json:"flagged"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	ResolutionComment *string    `gorm:"type:text" json:"resolution_comment"`
	ResolvedAt        *time.Time `json:"resolved_at"`
}

func (Feedback) TableName() string { return "feedback" }

// FeedbackRead фиксирует прочтение обращения администратором (персонально).
// Наличие записи (feedback_id, user_id) = обращение прочитано этим админом.
// Эталон - application_reads (ON CONFLICT DO NOTHING, unread через NOT EXISTS).
type FeedbackRead struct {
	ID         int       `json:"id"`
	FeedbackID int       `gorm:"uniqueIndex:idx_feedback_user_read" json:"feedback_id"`
	UserID     int       `gorm:"uniqueIndex:idx_feedback_user_read" json:"user_id"`
	ReadAt     time.Time `gorm:"autoCreateTime" json:"read_at"`
}

func (FeedbackRead) TableName() string { return "feedback_reads" }

// FeedbackWithUser -- обращение с именем пользователя (для администраторов).
// IsRead вычисляется персонально для запрашивающего админа (feedback_reads),
// Flagged -- общий флажок обращения.
type FeedbackWithUser struct {
	ID                int        `json:"id"`
	UserID            int        `json:"user_id"`
	UserName          string     `json:"user_name"`
	Message           string     `json:"message"`
	Status            string     `json:"status"`
	IsRead            bool       `json:"is_read"`
	Flagged           bool       `json:"flagged"`
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

// SetFlagRequest -- запрос на установку/снятие общего флажка обращения.
type SetFlagRequest struct {
	Flagged bool `json:"flagged"`
}
