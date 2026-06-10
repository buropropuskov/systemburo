package models

import "time"

// AttachmentTemplate - настройки Excel-бланка для UniqueAttachment (#183).
// Несколько шаблонов на UniqueAttachment, активный определяется IsActive=true.
// file_path - путь к загруженному .xlsx в uploads/templates/.
type AttachmentTemplate struct {
	ID                 int                         `json:"id"`
	UniqueAttachmentID int                         `gorm:"index" json:"unique_attachment_id"`
	IsActive           bool                        `gorm:"default:true;index" json:"is_active"`
	UniqueAttachment   *UniqueAttachment           `gorm:"foreignKey:UniqueAttachmentID" json:"-"`
	FilePath           string                      `gorm:"size:500" json:"file_path"`
	OriginalFileName   string                      `gorm:"size:255" json:"original_file_name"`
	ListStartRow       int                         `json:"list_start_row"`
	ListEndRow         int                         `json:"list_end_row"`
	MaxListRows        int                         `json:"max_list_rows"`
	ConcatSeparator    *string                     `gorm:"size:20" json:"concat_separator,omitempty"`
	UploadedByUserID   *int                        `json:"uploaded_by_user_id,omitempty"`
	CreatedAt          time.Time                   `json:"created_at"`
	UpdatedAt          time.Time                   `json:"updated_at"`
	Mappings           []AttachmentTemplateMapping `gorm:"foreignKey:TemplateID" json:"mappings,omitempty"`
}

// AttachmentTemplateMapping - связь между ячейкой Excel и полем заявки.
// cell_ref - адрес ячейки ("A1", "B12"); field_path - точечный путь
// из словаря (см. attachment_template_fields.go). Для list-полей
// IsListField=true означает что эта пара заполняется по каждой строке списка.
type AttachmentTemplateMapping struct {
	ID          int       `json:"id"`
	TemplateID  int       `gorm:"index" json:"template_id"`
	CellRef     string    `gorm:"size:10" json:"cell_ref"`
	FieldPath   string    `gorm:"size:100" json:"field_path"`
	IsListField bool      `gorm:"default:false" json:"is_list_field"`
	CreatedAt   time.Time `json:"-"`
}

// AttachmentCustomField - дополнительное поле для UniqueAttachment.
// Отображается в форме создания заявки и доступно для маппинга в Excel.
type AttachmentCustomField struct {
	ID                 int       `json:"id"`
	UniqueAttachmentID int       `gorm:"index" json:"unique_attachment_id"`
	Label              string    `gorm:"size:200" json:"label"`
	Placeholder        *string   `gorm:"size:200" json:"placeholder,omitempty"`
	SortOrder          int       `gorm:"default:0" json:"sort_order"`
	IsRequired         bool      `gorm:"default:false" json:"is_required"`
	IsActive           bool      `gorm:"default:true;index" json:"is_active"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// AttachmentFieldConfig - оверрайд видимости/обязательности базового поля
// для конкретного UniqueAttachment (feedback-0608-H / #529).
// Хранит ТОЛЬКО отличия от дефолта реестра (attachment_fields_registry.go):
// нет строки для (unique_attachment_id, field_key) -> поле берёт дефолт реестра.
// FieldKey - стабильный ключ из реестра ("passport", "entry_date_from", ...).
type AttachmentFieldConfig struct {
	ID                 int       `json:"id"`
	UniqueAttachmentID int       `gorm:"uniqueIndex:idx_field_config_ua_key,priority:1;not null" json:"unique_attachment_id"`
	FieldKey           string    `gorm:"uniqueIndex:idx_field_config_ua_key,priority:2;size:64;not null" json:"field_key"`
	Visible            bool      `json:"visible"`
	Required           bool      `json:"required"`
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
	Mappings        []MappingInput `json:"mappings" validate:"required,dive"`
	ConcatSeparator *string        `json:"concat_separator,omitempty"`
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
	IsRequired  bool    `json:"is_required"`
}

// FieldConfigItem - элемент тела PUT /attachments/{id}/field-config:
// оверрайд видимости/обязательности одного базового поля по ключу реестра.
type FieldConfigItem struct {
	Key      string `json:"key" validate:"required,max=64"`
	Visible  bool   `json:"visible"`
	Required bool   `json:"required"`
}

// SaveFieldConfigRequest - bulk-upsert оверрайдов базовых полей вложения.
type SaveFieldConfigRequest struct {
	Base []FieldConfigItem `json:"base" validate:"dive"`
}

// FieldConfigResponse - ответ GET /attachments/{id}/field-config: базовые поля
// (реестр типа, смерженный с оверрайдами) + кастомные поля вложения. Единый
// источник для админ-модалки настройки и формы подачи.
type FieldConfigResponse struct {
	Base   []MergedField           `json:"base"`
	Custom []AttachmentCustomField `json:"custom"`
}

// MergedField - базовое поле вложения, смерженное с оверрайдами для
// конкретного UniqueAttachment. Единый источник для админ-модалки и формы
// подачи: реестр типа вложения + оверрайды из attachment_field_config.
type MergedField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Group    string `json:"group"`
	Visible  bool   `json:"visible"`
	Required bool   `json:"required"`
	// Locked=true: поле не настраивается (дата/время). Админ-модалка не показывает
	// для него тумблеры, форма подачи рендерит как обязательное всегда.
	Locked bool `json:"locked"`
}

// CustomValueInput - значение кастомного поля при создании attachment.
type CustomValueInput struct {
	CustomFieldID int    `json:"custom_field_id" validate:"required"`
	Value         string `json:"value"`
}
