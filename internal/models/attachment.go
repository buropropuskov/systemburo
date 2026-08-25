package models

import "time"

type Attachment struct {
	ID                    int               `json:"id"`
	ApplicationID         *int              `gorm:"index" json:"application_id"`
	Application           *Application      `gorm:"constraint:OnDelete:CASCADE" json:"-"`
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
	// Общие булевы параметры заявки (#529): сохраняются на вложении, в просмотре - теги.
	RoofAccess  bool `gorm:"default:false" json:"roof_access"`
	FreeParking bool `gorm:"default:false" json:"free_parking"`
	// Ручное добавление в таблицы (#1049): вложение-сирота без заявки. При ручном
	// добавлении application_id NULL + is_manual=true, а org/company хранятся прямо
	// на вложении (у заявочных берутся из applications).
	OrganizationID  *int          `gorm:"index" json:"organization_id"`
	Organization    *Organization `json:"-"`
	CompanyID       *int          `gorm:"index" json:"company_id"`
	Company         *Company      `json:"-"`
	IsManual        bool          `gorm:"default:false;index" json:"is_manual"`
	CreatedByUserID *int          `json:"created_by_user_id"`
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
	// AutoExport - писать ли бланки этого типа в файловый архив (#1615). Включён по
	// умолчанию: администратор настраивает архив глобальным рубильником, а тумблер
	// здесь нужен, чтобы исключить отдельный тип, а не чтобы перечислять нужные.
	AutoExport bool `gorm:"not null;default:true" json:"auto_export"`
}

// CreateUniqueAttachmentRequest -- тело запроса на создание шаблона вложения.
type CreateUniqueAttachmentRequest struct {
	AttachmentType string  `json:"attachment_type" validate:"required,oneof=cars people items"`
	Name           string  `json:"name" validate:"required,min=1,max=255"`
	DisplayName    string  `json:"display_name" validate:"required,min=1,max=255"`
	Title          string  `json:"title" validate:"required,min=1,max=255"`
	Instruction    *string `json:"instruction"`
	// AutoExport -- указатель, чтобы отличить явное false от отсутствия поля:
	// клиенты, не знающие про файловый архив, должны получать значение по умолчанию.
	AutoExport *bool `json:"auto_export"`
}

// UpdateUniqueAttachmentRequest -- тело запроса на обновление шаблона вложения.
//
// PUT здесь -- полная замена полей, но AutoExport остаётся указателем: запрос без
// этого ключа не трогает тумблер архива. Иначе сохранение имени или инструкции из
// формы, которая про архив не знает, молча выключало бы выгрузку бланков (#1615).
type UpdateUniqueAttachmentRequest struct {
	AttachmentType string  `json:"attachment_type" validate:"required,oneof=cars people items"`
	Name           string  `json:"name" validate:"required,min=1,max=255"`
	DisplayName    string  `json:"display_name" validate:"required,min=1,max=255"`
	Title          string  `json:"title" validate:"required,min=1,max=255"`
	Instruction    *string `json:"instruction"`
	AutoExport     *bool   `json:"auto_export"`
}

// CreateUniqueAttachmentResponse -- ответ после создания шаблона вложения.
type CreateUniqueAttachmentResponse struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
}
