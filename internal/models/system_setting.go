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

// TestMailRequest - запрос на проверочное письмо при настройке почты (#1906).
type TestMailRequest struct {
	To string `json:"to" validate:"required,email"`
}

// PDConsentSettings -- настройки согласия на обработку персональных данных, которое
// запрашивается у пользователя при первом входе (#1567). Хранятся тремя ключами в
// system_settings: legal.pd_consent_text, legal.pd_consent_version,
// legal.pd_consent_required.
//
// Required -- отдельный выключатель, а не производное от "текст непустой": иначе
// первый же ввод в редакторе включал бы запрос согласия до нажатия "Сохранить", а
// первый деплой закрывал бы систему до того, как администратор вставит текст.
type PDConsentSettings struct {
	Text    string `json:"text"`
	Version int    `json:"version"`
	// VersionAt -- когда появилась действующая редакция. Пользователь по номеру
	// «редакция 17» не поймёт ничего, а по дате видит, с какого числа действует то,
	// что ему показывают. Пустая строка у настроек, заведённых до появления поля.
	VersionAt string `json:"version_at"`
	Required  bool   `json:"required"`
}

// UpdatePDConsentTextRequest -- сохранение текста согласия. Пустая строка допустима:
// это очистка текста, при которой запрос согласия перестаёт работать (администратор
// видит предупреждение в интерфейсе).
//
// RequireAgain поднимает редакцию тем же запросом: изменённый текст -- это новая
// редакция согласия, и подтверждать её надо заново. Отдельным вызовом делать нельзя,
// иначе сбой между сохранением и подъёмом оставил бы новый текст со старой редакцией,
// то есть людей, согласившихся не с тем, что им теперь показывают.
type UpdatePDConsentTextRequest struct {
	Text         string `json:"text"`
	RequireAgain bool   `json:"require_again"`
}

// UpdatePDConsentRequiredRequest -- переключение запроса согласия при входе.
// Указатель, чтобы отличить переданное false от отсутствующего поля.
type UpdatePDConsentRequiredRequest struct {
	Required *bool `json:"required" validate:"required"`
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
