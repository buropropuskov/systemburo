package models

import "time"

type Item struct {
	ID           int        `json:"id"`
	AttachmentID int        `gorm:"index" json:"attachment_id"`
	Attachment   Attachment `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	// SupplementID - каким дополнением заявки добавлена позиция ТМЦ (#1685). NULL - пришла
	// с исходной подачей. У items, в отличие от машин и сотрудников, нет поля status:
	// невидимость непринятых держится только фильтрами читателей по этой колонке.
	SupplementID *int       `gorm:"index" json:"supplement_id"`
	Name         *string    `gorm:"size:255" json:"name"`
	Count        *int       `json:"count"`
	DateCreated  *time.Time `json:"date_created"`
	DateDeleted  *time.Time `json:"date_deleted"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ApplicationItem struct {
	ID           int        `json:"id"`
	AttachmentID int        `gorm:"index" json:"attachment_id"`
	Attachment   Attachment `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	ItemName     *string    `gorm:"size:255" json:"item_name"`
	Quantity     *int       `json:"quantity"`
	Description  *string    `gorm:"type:text" json:"description"`
	OrderIndex   *int       `json:"order_index"`
	CreatedAt    time.Time  `json:"created_at"`
}
