package models

import "time"

// ApplicationFile -- файл, приложенный к заявке (#1721).
//
// Прикрепить файл можно только при подаче, поэтому загрузка идёт до создания
// заявки: файл ложится на диск и живёт с application_id NULL, пока подача не
// привяжет его к созданной заявке в своей транзакции. Так «только при подаче»
// держится контрактом, а не проверкой статуса, которая проигрывала бы гонку с
// бюро, открывающим заявку в ту же секунду. Непривязанные строки убирает
// суточный уборщик.
type ApplicationFile struct {
	ID            int          `json:"id"`
	ApplicationID *int         `gorm:"index" json:"application_id"`
	Application   *Application `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	FileName      string       `gorm:"size:255" json:"file_name"`
	StoredName    string       `gorm:"size:255" json:"stored_name"`
	MimeType      string       `gorm:"size:120" json:"mime_type"`
	FileSize      int64        `json:"file_size"`
	UploadedBy    int          `gorm:"index" json:"uploaded_by"`
	// Encrypted -- файл на диске зашифрован ключом системы. Флаг, а не проба
	// заголовка: файлы, записанные до появления шифрования, читаются как есть, и
	// признак должен быть явным, а не выводиться из содержимого.
	Encrypted bool      `gorm:"default:false" json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

// ApplicationFileItem -- файл заявки в ответах API. Имя на диске наружу не
// отдаётся: скачивание идёт по id через проверку доступа к заявке, а не по пути.
type ApplicationFileItem struct {
	ID        int       `json:"id"`
	FileName  string    `json:"file_name"`
	MimeType  string    `json:"mime_type"`
	FileSize  int64     `json:"file_size"`
	CreatedAt time.Time `json:"created_at"`
}

// Item -- представление строки для ответа API.
func (f ApplicationFile) Item() ApplicationFileItem {
	return ApplicationFileItem{
		ID:        f.ID,
		FileName:  f.FileName,
		MimeType:  f.MimeType,
		FileSize:  f.FileSize,
		CreatedAt: f.CreatedAt,
	}
}
