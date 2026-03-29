package models

import "time"

type Attachment struct {
	ID                    int               `json:"id"`
	ApplicationID         int               `gorm:"index" json:"application_id"`
	Application           Application       `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	AttachmentType        string            `gorm:"size:20" json:"attachment_type"` // cars, people, items
	AttachmentName        *string           `gorm:"size:255" json:"attachment_name"`
	AttachmentDisplayName *string           `gorm:"size:255" json:"attachment_display_name"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
	EntryDateFrom         *string           `gorm:"size:20" json:"entry_date_from"`
	EntryDateTo           *string           `gorm:"size:20" json:"entry_date_to"`
	EntryTimeFrom         *string           `gorm:"size:20" json:"entry_time_from"`
	EntryTimeTo           *string           `gorm:"size:20" json:"entry_time_to"`
	UniqueAttachmentID    *int              `gorm:"index" json:"unique_attachment_id"`
	UniqueAttachment      *UniqueAttachment `json:"-"`
	Status                *int              `json:"status"`
}

type UniqueAttachment struct {
	ID             int       `json:"id"`
	AttachmentType string    `gorm:"size:20" json:"attachment_type"` // cars, people, items
	Name           *string   `gorm:"size:255" json:"name"`
	DisplayName    *string   `gorm:"size:255" json:"display_name"`
	Title          *string   `gorm:"size:255" json:"title"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Instruction    *string   `gorm:"type:text" json:"instruction"`
	IsActive       bool      `gorm:"default:true" json:"is_active"`
}

// CreateUniqueAttachmentRequest -- тело запроса на создание шаблона вложения.
type CreateUniqueAttachmentRequest struct {
	AttachmentType string  `json:"attachment_type"`
	Name           string  `json:"name"`
	DisplayName    string  `json:"display_name"`
	Title          string  `json:"title"`
	Instruction    *string `json:"instruction"`
}

// UpdateUniqueAttachmentRequest -- тело запроса на обновление шаблона вложения.
type UpdateUniqueAttachmentRequest struct {
	AttachmentType string  `json:"attachment_type"`
	Name           string  `json:"name"`
	DisplayName    string  `json:"display_name"`
	Title          string  `json:"title"`
	Instruction    *string `json:"instruction"`
}

// CreateUniqueAttachmentResponse -- ответ после создания шаблона вложения.
type CreateUniqueAttachmentResponse struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
}
