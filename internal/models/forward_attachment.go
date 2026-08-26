package models

import "time"

// ForwardAttachment связывает получателя пересылки с конкретным вложением заявки (#680).
// Семантика чтения (срез be-read): есть строки для пары (заявка, получатель) - получатель
// видит только перечисленные вложения; нет строк - видит все (обратная совместимость).
// Уникальный индекс по (application_id, recipient_user_id, attachment_id) делает повторную
// пересылку тех же вложений идемпотентной; его левый префикс обслуживает фильтр чтения.
type ForwardAttachment struct {
	ID              int         `json:"id"`
	ApplicationID   int         `gorm:"uniqueIndex:idx_forward_att,priority:1" json:"application_id"`
	Application     Application `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	RecipientUserID int         `gorm:"uniqueIndex:idx_forward_att,priority:2" json:"recipient_user_id"`
	RecipientUser   User        `gorm:"foreignKey:RecipientUserID;constraint:OnDelete:CASCADE" json:"-"`
	AttachmentID    int         `gorm:"uniqueIndex:idx_forward_att,priority:3" json:"attachment_id"`
	Attachment      Attachment  `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt       time.Time   `json:"created_at"`
	CreatedBy       *int        `json:"created_by"`
}
