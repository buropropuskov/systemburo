package models

import (
	"encoding/json"
	"time"
)

// GuideSection -- раздел руководства по роли (user|guard|admin), раскладка «Вкладки».
// Текст (lead + items) сидится дефолтным черновиком; PDF-файл загружается админом
// в отдельном будущем срезе, поэтому file-поля до загрузки пустые и в ответе
// отдаётся file:null. Доступ к разделу гейтится правом guide.<role>.
type GuideSection struct {
	ID            int             `json:"id"`
	Role          string          `gorm:"size:20;uniqueIndex:uidx_guide_sections_role" json:"role"`
	Title         string          `gorm:"size:255" json:"title"`
	Lead          string          `gorm:"type:text" json:"lead"`
	Items         json.RawMessage `gorm:"type:jsonb" json:"items"`
	FileName      string          `gorm:"size:255" json:"file_name"`
	StoredName    string          `gorm:"size:255" json:"stored_name"`
	FileExt       string          `gorm:"size:10" json:"file_ext"`
	MimeType      string          `gorm:"size:120" json:"mime_type"`
	FileSize      int64           `json:"file_size"`
	FileUpdatedAt *time.Time      `json:"file_updated_at"`
	SortOrder     int             `gorm:"default:0" json:"sort_order"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// GuideSectionResponse -- раздел руководства для фронта (модалка «Руководство»).
type GuideSectionResponse struct {
	Role  string         `json:"role"`
	Title string         `json:"title"`
	Lead  string         `json:"lead"`
	Items []string       `json:"items"`
	File  *GuideFileInfo `json:"file"`
}

// GuideFileInfo -- метаданные PDF раздела. nil пока файл не загружен.
type GuideFileInfo struct {
	Name        string    `json:"name"`
	Ext         string    `json:"ext"`
	MimeType    string    `json:"mime_type"`
	Size        int64     `json:"size"`
	UpdatedAt   time.Time `json:"updated_at"`
	DownloadURL string    `json:"download_url"`
}
