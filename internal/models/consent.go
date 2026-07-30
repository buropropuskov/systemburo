package models

import "time"

type PDConsent struct {
	ID          int        `json:"id"`
	UserID      int        `gorm:"index;not null" json:"user_id"`
	User        User       `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	ConsentType string     `gorm:"size:50;not null" json:"consent_type"`
	Granted     bool       `gorm:"not null" json:"granted"`
	IPAddress   string     `gorm:"size:45" json:"ip_address"`
	UserAgent   string     `gorm:"type:text" json:"user_agent"`
	GrantedAt   time.Time  `json:"granted_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
	CreatedAt   time.Time  `json:"created_at"`

	// DocumentVersion -- редакция текста согласия, с которой согласился человек
	// (#1567). Ставит СЕРВЕР из настроек; в запросе такого поля нет намеренно, иначе
	// клиент прислал бы заведомо большое число и освободился от всех будущих
	// переподтверждений. Существующие записи получают 1 через default колонки.
	DocumentVersion int `gorm:"not null;default:1;index" json:"document_version"`

	// DocumentHash -- sha256 принятого HTML. Версия отвечает на вопрос "какая
	// редакция", хэш -- "какой именно текст": администратор может поправить
	// формулировку, не двигая версию, и без хэша доказать было бы нечем.
	DocumentHash string `gorm:"size:64" json:"document_hash"`
}

func (PDConsent) TableName() string { return "pd_consents" }

type GrantConsentRequest struct {
	ConsentType string `json:"consent_type" validate:"required,oneof=pd_processing pd_transfer"`
}

// PDConsentGateState -- что показать пользователю на входе (#1567). Required уже
// учитывает исключения (супер-админ) и то, задан ли текст, поэтому фронту не нужно
// повторять эти правила у себя.
type PDConsentGateState struct {
	Required bool `json:"required"`
	Version  int  `json:"version"`
	// VersionAt -- с какого числа действует показываемая редакция. Номер редакции
	// человеку ничего не говорит, дата говорит.
	VersionAt string                  `json:"version_at"`
	Text      string                  `json:"text"`
	Document  *DataProcessingDocument `json:"document"`
}

// PDConsentCollection -- как идёт сбор согласий по текущей редакции (#1567).
//
// Счёт ведётся ТОЙ ЖЕ меркой, что и гейт: в знаменателе только те, кого он реально
// закрывает, а согласившимся считается тот, чья действующая редакция не ниже
// требуемой. Иначе сводка разошлась бы с тем, кого система пускает.
type PDConsentCollection struct {
	// Active -- запрос согласия сейчас реально работает (тумблер включён И текст
	// задан). Пока он выключен, гейт не закрывает никого, а подтвердить согласие
	// невозможно в принципе: счёт всегда «0 из N», и показывать его как ход сбора
	// значит врать администратору.
	Active bool `json:"active"`
	// Version -- редакция, по которой считали. Сводка по прежней редакции
	// бессмысленна: после подъёма номера состав согласившихся меняется.
	Version  int `json:"version"`
	Total    int `json:"total"`
	Accepted int `json:"accepted"`
	Pending  int `json:"pending"`
	// PendingUsers -- кто ещё не подтвердил. Это рабочий список для администратора:
	// кому напомнить и кого искать. По умолчанию урезан: после подъёма редакции сюда
	// попадают ВСЕ работники разом, и на крупной установке это тысячи строк в разметке.
	PendingUsers []PDConsentPendingUser `json:"pending_users"`
	// Truncated -- список показан не целиком. Полный отдаётся по запросу и уходит в
	// выгрузку: молча показать часть значит соврать, что остальных нет.
	Truncated bool `json:"truncated"`
}

// PDConsentPendingUser -- строка списка не подтвердивших.
type PDConsentPendingUser struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	FullName     string `json:"full_name"`
	Organization string `json:"organization"`
}
