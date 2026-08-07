package models

import "time"

// PushSubscription -- подписка браузера на Web Push (#974): доставка уведомлений, пока
// вкладка системы закрыта. Один браузер = одна строка (уникальный Endpoint) -- повторная
// подписка с тем же endpoint обновляет владельца и ключи вместо дубля, см.
// services.PushService.Subscribe.
type PushSubscription struct {
	ID     int  `json:"id"`
	UserID int  `gorm:"index" json:"user_id"`
	User   User `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	// Endpoint -- URL push-сервиса браузера, выдаётся PushManager.subscribe() и
	// уникален на весь мир. Основа upsert при повторной подписке.
	Endpoint string `gorm:"size:512;uniqueIndex" json:"endpoint"`
	P256dh   string `gorm:"size:255" json:"-"`
	Auth     string `gorm:"size:255" json:"-"`
	// UserAgent -- строка браузера на момент подписки, только чтобы показать человеку
	// на экране настроек "какое это устройство".
	UserAgent *string   `gorm:"size:255" json:"user_agent,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	// LastSuccessAt -- когда push последний раз реально доставился (2xx от сервиса).
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	// FailedCount -- подряд идущие неудачные отправки. Растёт на любой ошибке кроме
	// 404/410 (те подписку удаляют сразу), сбрасывается в 0 на успехе.
	FailedCount int     `gorm:"default:0" json:"-"`
	LastError   *string `gorm:"size:500" json:"-"`
}

func (PushSubscription) TableName() string { return "push_subscriptions" }

// PushDevice -- одна подписка на экране настроек (#974): без endpoint и ключей, они не
// нужны человеку и не должны утекать в API-ответ.
type PushDevice struct {
	ID            int        `json:"id"`
	UserAgent     string     `json:"user_agent"`
	CreatedAt     time.Time  `json:"created_at"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
}

// PushStatusResponse -- ответ GET /notifications/push/status: публичный VAPID-ключ для
// PushManager.subscribe на фронте, признак "push настроен на сервере" (пустые VAPID-ключи
// в параметрах = выключен) и список подписанных устройств текущего пользователя.
type PushStatusResponse struct {
	Enabled   bool         `json:"enabled"`
	PublicKey string       `json:"public_key"`
	Devices   []PushDevice `json:"devices"`
}

// PushSubscribeRequest -- тело POST /notifications/push/subscribe. Форма повторяет
// PushSubscription.toJSON() браузера (endpoint + вложенные keys.p256dh/keys.auth) --
// фронт передаёт объект подписки как есть, без пересборки.
type PushSubscribeRequest struct {
	Endpoint string `json:"endpoint" validate:"required"`
	Keys     struct {
		P256dh string `json:"p256dh" validate:"required"`
		Auth   string `json:"auth" validate:"required"`
	} `json:"keys" validate:"required"`
}

// PushUnsubscribeRequest -- тело DELETE /notifications/push/subscribe, если endpoint не
// передан в query-параметре.
type PushUnsubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}
