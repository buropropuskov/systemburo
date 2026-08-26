package models

import "time"

// AttachmentTemplate - настройки Excel-бланка для UniqueAttachment (#183).
// Несколько шаблонов на UniqueAttachment, активный определяется IsActive=true.
// file_path - путь к загруженному .xlsx в uploads/templates/.
type AttachmentTemplate struct {
	ID                 int               `json:"id"`
	UniqueAttachmentID int               `gorm:"index" json:"unique_attachment_id"`
	IsActive           bool              `gorm:"default:true;index" json:"is_active"`
	UniqueAttachment   *UniqueAttachment `gorm:"foreignKey:UniqueAttachmentID" json:"-"`
	FilePath           string            `gorm:"size:500" json:"file_path"`
	OriginalFileName   string            `gorm:"size:255" json:"original_file_name"`
	ListStartRow       int               `json:"list_start_row"`
	ListEndRow         int               `json:"list_end_row"`
	MaxListRows        int               `json:"max_list_rows"`
	// Вторая таблица бланка - ТМЦ «Заявок на ввоз» этой же заявки. Строки списка
	// принадлежат собственному типу вложения (у заявки на работы - сотрудникам),
	// поэтому ввозимый товар идёт отдельной таблицей.
	//
	// Настраивается ОДНИМ числом - сколько строк отведено под таблицу в бланке; ноль
	// означает, что таблицы нет. Строку начала задавать руками не нужно: её определяет
	// ячейка, в которую админ привязал поля группы «Имущество (список)».
	// StartRow/EndRow остались от первой версии настройки и заполняются сервисом как
	// снимок посчитанных границ - генерация их не читает.
	ItemsListStartRow int                         `json:"items_list_start_row"`
	ItemsListEndRow   int                         `json:"items_list_end_row"`
	ItemsMaxListRows  int                         `json:"items_max_list_rows"`
	ConcatSeparator   *string                     `gorm:"size:20" json:"concat_separator,omitempty"`
	UploadedByUserID  *int                        `json:"uploaded_by_user_id,omitempty"`
	CreatedAt         time.Time                   `json:"created_at"`
	UpdatedAt         time.Time                   `json:"updated_at"`
	Mappings          []AttachmentTemplateMapping `gorm:"foreignKey:TemplateID" json:"mappings,omitempty"`
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

// UpdateTemplateParamsRequest - границы строк списка без перезагрузки файла.
// MaxListRows=0 означает "посчитать по диапазону" (как при загрузке шаблона).
// Границы таблицы ТМЦ необязательны: нули означают "таблицы в бланке нет".
type UpdateTemplateParamsRequest struct {
	ListStartRow int `json:"list_start_row" validate:"min=1"`
	ListEndRow   int `json:"list_end_row" validate:"min=1"`
	MaxListRows  int `json:"max_list_rows" validate:"min=0"`
	// ItemsMaxListRows - сколько строк бланка отведено под таблицу ТМЦ заявки. Ноль -
	// таблицы нет. Строку начала не передаём: её задаёт ячейка привязки полей ТМЦ.
	ItemsMaxListRows int `json:"items_max_list_rows" validate:"min=0"`
}

// UpdateMappingsRequest - bulk-обновление mappings одним запросом.
type UpdateMappingsRequest struct {
	Mappings        []MappingInput `json:"mappings" validate:"required,dive"`
	ConcatSeparator *string        `json:"concat_separator,omitempty"`
}

// CopyMappingsRequest - перенос привязок с другого шаблона. Настраивая новый тип
// вложения, админ набивал те же пары ячейка-поле заново.
// Replace - заменить привязки цели (иначе добавить к текущим, дубли пропускаются).
// CopyParams - перенести ещё и границы списка с разделителем совмещённых полей.
type CopyMappingsRequest struct {
	SourceTemplateID int  `json:"source_template_id" validate:"required,min=1"`
	Replace          bool `json:"replace"`
	CopyParams       bool `json:"copy_params"`
}

// CopyMappingsResult - что получилось перенести. Пропуски перечислены отдельно,
// чтобы интерфейс объяснил, почему привязок стало меньше, чем у источника.
type CopyMappingsResult struct {
	Copied int `json:"copied"`
	// SkippedForeignList - привязки списка чужой группы: у цели другой тип вложения,
	// заполнять их нечем (см. fillListSection).
	SkippedForeignList int `json:"skipped_foreign_list"`
	// SkippedCustom - кастомные поля источника, которых нет у цели: id таких полей
	// принадлежат своему типу вложения, переносить их некуда.
	SkippedCustom int `json:"skipped_custom"`
	// RemappedCustom - кастомные поля, сопоставленные по названию с полями цели.
	RemappedCustom int `json:"remapped_custom"`
	// SkippedDuplicates - пары ячейка-поле, которые у цели уже были (режим добавления).
	SkippedDuplicates int  `json:"skipped_duplicates"`
	ParamsCopied      bool `json:"params_copied"`
}

// TemplateSource - шаблон-кандидат в источники привязок для выпадающего списка.
type TemplateSource struct {
	TemplateID         int    `json:"template_id"`
	UniqueAttachmentID int    `json:"unique_attachment_id"`
	AttachmentName     string `json:"attachment_name"`
	AttachmentType     string `json:"attachment_type"`
	OriginalFileName   string `json:"original_file_name"`
	MappingsCount      int    `json:"mappings_count"`
	IsActive           bool   `json:"is_active"`
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
	// Requirable=false для булевых чекбоксов (крыша/парковка/уведомление): тумблер
	// "обязательно" для них бессмыслен. Админ-модалка по нему прячет этот тумблер.
	Requirable bool `json:"requirable"`
	// Locked=true: поле не настраивается (дата/время). Админ-модалка не показывает
	// для него тумблеры, форма подачи рендерит как обязательное всегда.
	Locked bool `json:"locked"`
}

// CustomValueInput - значение кастомного поля при создании attachment.
type CustomValueInput struct {
	CustomFieldID int    `json:"custom_field_id" validate:"required"`
	Value         string `json:"value"`
}
