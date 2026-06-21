package models

type SystemSetting struct {
	ID    int    `json:"id"`
	Key   string `gorm:"uniqueIndex;size:100" json:"key"`
	Value string `gorm:"type:text" json:"value"`
	Type  string `gorm:"size:20" json:"type"`
}

func (SystemSetting) TableName() string { return "system_settings" }

type UpdateSettingRequest struct {
	Value string `json:"value" validate:"required"`
}

// DataProcessingDocument -- метаданные загруженного документа согласия на обработку
// персональных данных. Хранится как JSON в system_settings под ключом
// legal.data_processing_document; сам файл лежит на диске под StoredName.
type DataProcessingDocument struct {
	StoredName string `json:"stored_name"`
	FileName   string `json:"file_name"`
	MimeType   string `json:"mime_type"`
	Ext        string `json:"ext"`
	UploadedAt string `json:"uploaded_at"`
}
