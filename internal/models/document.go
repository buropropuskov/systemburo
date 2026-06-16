package models

import "time"

// DocumentGroup -- группа документов (раздел/категория).
type DocumentGroup struct {
	ID        int       `json:"id"`
	Name      string    `gorm:"size:255;uniqueIndex:uidx_document_groups_name" json:"name"`
	SortOrder int       `gorm:"default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy *int      `json:"created_by"`
	UpdatedBy *int      `json:"updated_by"`
}

// Document -- документ (файл) с метаданными.
type Document struct {
	ID          int        `json:"id"`
	GroupID     *int       `gorm:"index" json:"group_id"`
	Title       string     `gorm:"size:255" json:"title"`
	Description *string    `gorm:"type:text" json:"description"`
	FileName    string     `gorm:"size:255" json:"file_name"`
	StoredName  string     `gorm:"size:255" json:"stored_name"`
	FileExt     string     `gorm:"size:10" json:"file_ext"`
	MimeType    string     `gorm:"size:120" json:"mime_type"`
	FileSize    int64      `json:"file_size"`
	PublishedAt time.Time  `json:"published_at"`
	IsVisible   bool       `gorm:"default:true" json:"is_visible"`
	SortOrder   int        `gorm:"default:0" json:"sort_order"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedBy   *int       `json:"created_by"`
	UpdatedBy   *int       `json:"updated_by"`
}

// -- Request/Response DTO --

// CreateDocumentGroupRequest -- запрос на создание группы.
type CreateDocumentGroupRequest struct {
	Name string `json:"name" validate:"required,max=255"`
}

// UpdateDocumentGroupRequest -- запрос на переименование группы.
type UpdateDocumentGroupRequest struct {
	Name string `json:"name" validate:"required,max=255"`
}

// ReorderDocumentGroupsRequest -- запрос на изменение порядка групп.
type ReorderDocumentGroupsRequest struct {
	IDs []int `json:"ids" validate:"required,min=1"`
}

// DocumentGroupWithCount -- группа с количеством документов.
type DocumentGroupWithCount struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	Count     int64     `json:"count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy *int      `json:"created_by"`
	UpdatedBy *int      `json:"updated_by"`
}

// UploadDocumentRequest -- поля multipart-формы при загрузке документа.
type UploadDocumentRequest struct {
	Title       string  `form:"title" validate:"required,max=255"`
	Description *string `form:"description"`
	GroupID     *int    `form:"group_id"`
	PublishedAt *string `form:"published_at"`
	SortOrder   int     `form:"sort_order"`
}

// UpdateDocumentMetaRequest -- запрос на обновление метаданных документа.
type UpdateDocumentMetaRequest struct {
	Title       *string `json:"title" validate:"omitempty,max=255"`
	Description *string `json:"description"`
	GroupID     *int    `json:"group_id"`
	PublishedAt *string `json:"published_at"`
	IsVisible   *bool   `json:"is_visible"`
}

// ReorderDocumentsRequest -- запрос на изменение порядка документов внутри группы.
type ReorderDocumentsRequest struct {
	GroupID *int  `json:"group_id"`
	IDs     []int `json:"ids" validate:"required,min=1"`
}

// DocumentListItem -- документ для списка в админке.
type DocumentListItem struct {
	ID          int        `json:"id"`
	GroupID     *int       `json:"group_id"`
	GroupName   *string    `json:"group_name"`
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	FileName    string     `json:"file_name"`
	FileExt     string     `json:"file_ext"`
	FileSize    int64      `json:"file_size"`
	PublishedAt time.Time  `json:"published_at"`
	IsVisible   bool       `json:"is_visible"`
	SortOrder   int        `json:"sort_order"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedBy   *int       `json:"created_by"`
	UpdatedBy   *int       `json:"updated_by"`
}

// PublicDocumentGroup -- группа с документами для публичного эндпоинта.
type PublicDocumentGroup struct {
	ID        int               `json:"id"`
	Name      string            `json:"name"`
	SortOrder int               `json:"sort_order"`
	Documents []PublicDocument  `json:"documents"`
}

// PublicDocument -- документ для публичного эндпоинта (без скрытых).
type PublicDocument struct {
	ID          int       `json:"id"`
	GroupID     *int      `json:"group_id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	FileName    string    `json:"file_name"`
	FileExt     string    `json:"file_ext"`
	FileSize    int64     `json:"file_size"`
	PublishedAt time.Time `json:"published_at"`
	SortOrder   int       `json:"sort_order"`
}
