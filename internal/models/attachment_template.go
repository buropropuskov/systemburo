package models

import "time"

// AttachmentTemplate - настройки Excel-бланка для UniqueAttachment (#183).
// Несколько шаблонов на UniqueAttachment, активный определяется IsActive=true.
// file_path - путь к загруженному .xlsx в uploads/templates/.
type AttachmentTemplate struct {
	ID                  int                          `json:"id"`
	UniqueAttachmentID  int                          `gorm:"index" json:"unique_attachment_id"`
	IsActive            bool                         `gorm:"default:true;index" json:"is_active"`
	UniqueAttachment    *UniqueAttachment            `gorm:"foreignKey:UniqueAttachmentID" json:"-"`
	FilePath            string                       `gorm:"size:500" json:"file_path"`
	OriginalFileName    string                       `gorm:"size:255" json:"original_file_name"`
	ListStartRow        int                          `json:"list_start_row"`
	ListEndRow          int                          `json:"list_end_row"`
	MaxListRows         int                          `json:"max_list_rows"`
	ConcatSeparator     *string                      `gorm:"size:20" json:"concat_separator,omitempty"`
	UploadedByUserID    *int                         `json:"uploaded_by_user_id,omitempty"`
	CreatedAt           time.Time                    `json:"created_at"`
	UpdatedAt           time.Time                    `json:"updated_at"`
	Mappings            []AttachmentTemplateMapping  `gorm:"foreignKey:TemplateID" json:"mappings,omitempty"`
}

// AttachmentTemplateMapping - связь между ячейкой Excel и полем заявки.
// cell_ref - адрес ячейки ("A1", "B12"); field_path - точечный путь
// из словаря (см. attachment_template_fields.go). Для list-полей
// IsListField=true означает что эта пара заполняется по каждой строке списка.
type AttachmentTemplateMapping struct {
	ID          int        `json:"id"`
	TemplateID  int        `gorm:"index" json:"template_id"`
	CellRef     string     `gorm:"size:10" json:"cell_ref"`
	FieldPath   string     `gorm:"size:100" json:"field_path"`
	IsListField bool       `gorm:"default:false" json:"is_list_field"`
	CreatedAt   time.Time  `json:"-"`
}

// AttachmentCustomField - дополнительное поле для UniqueAttachment.
// Отображается в форме создания заявки и доступно для маппинга в Excel.
type AttachmentCustomField struct {
	ID                 int       `json:"id"`
	UniqueAttachmentID int       `gorm:"index" json:"unique_attachment_id"`
	Label              string    `gorm:"size:200" json:"label"`
	Placeholder        *string   `gorm:"size:200" json:"placeholder,omitempty"`
	SortOrder          int       `gorm:"default:0" json:"sort_order"`
	IsActive           bool      `gorm:"default:true;index" json:"is_active"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// AttachmentCustomValue - значение кастомного поля для конкретного экземпляра
// Attachment (привязано к заявке). Каждый Attachment может иметь N значений.
type AttachmentCustomValue struct {
	ID            int       `json:"id"`
	AttachmentID  int       `gorm:"index" json:"attachment_id"`
	CustomFieldID int       `gorm:"index" json:"custom_field_id"`
	Value         string    `gorm:"type:text" json:"value"`
	CreatedAt     time.Time `json:"-"`
	UpdatedAt     time.Time `json:"-"`
}

// CreateTemplateRequest - данные при загрузке/обновлении шаблона
// (multipart form: file + form fields).
type CreateTemplateRequest struct {
	ListStartRow int `form:"list_start_row" json:"list_start_row" validate:"min=1"`
	ListEndRow   int `form:"list_end_row" json:"list_end_row" validate:"min=1"`
	MaxListRows  int `form:"max_list_rows" json:"max_list_rows" validate:"min=0"`
}

// UpdateMappingsRequest - bulk-обновление mappings одним запросом.
type UpdateMappingsRequest struct {
	Mappings []MappingInput `json:"mappings" validate:"required,dive"`
}

// MappingInput - элемент списка mappings (без ID, его выдаст БД).
type MappingInput struct {
	CellRef     string `json:"cell_ref" validate:"required,max=10"`
	FieldPath   string `json:"field_path" validate:"required,max=100"`
	IsListField bool   `json:"is_list_field"`
}

// CreateCustomFieldRequest - создать/обновить custom field.
type CreateCustomFieldRequest struct {
	Label       string  `json:"label" validate:"required,min=1,max=200"`
	Placeholder *string `json:"placeholder" validate:"omitempty,max=200"`
	SortOrder   int     `json:"sort_order"`
}

// CustomValueInput - значение кастомного поля при создании attachment.
type CustomValueInput struct {
	CustomFieldID int    `json:"custom_field_id" validate:"required"`
	Value         string `json:"value"`
}
