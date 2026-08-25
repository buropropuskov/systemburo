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

// PushPlatformCounts -- разрез по грубой платформе браузера (#974). Технические ключи в
// JSON короткие (ios/android/desktop/unknown); "ios" покрывает и iPhone, и iPad -
// ограничение Apple на push вне установленного на экран "Домой" приложения одинаковое
// для обоих, отдельной группы iPhone нет. Человекочитаемая подпись группы ios -
// "iOS (iPhone, iPad)".
type PushPlatformCounts struct {
	IOS     int64 `json:"ios"`
	Android int64 `json:"android"`
	Desktop int64 `json:"desktop"`
	Unknown int64 `json:"unknown"`
}

// PushSummary -- ответ GET /notifications/push/summary: сводка использования Web Push
// для админского раздела статистики (#974), не личная настройка. UsersByLastLoginPlatform
// считается по платформе ПОСЛЕДНЕГО успешного входа каждого активного пользователя,
// независимо от того, подключил он push или нет, - это и есть ответ на "сколько людей
// вообще на iOS", а не только тех, кто оформил подписку.
type PushSummary struct {
	ActiveUsersTotal         int64              `json:"active_users_total"`
	UsersWithPush            int64              `json:"users_with_push"`
	UsersWithoutPush         int64              `json:"users_without_push"`
	SubscriptionsByPlatform  PushPlatformCounts `json:"subscriptions_by_platform"`
	UsersByLastLoginPlatform PushPlatformCounts `json:"users_by_last_login_platform"`
	// Delivery -- состояние доставки по каждому живому устройству. Причина отказа до
	// этого жила только в журнале приложения, а до журнала на сервере доступа нет ни у
	// администратора, ни при разборе жалобы «уведомление не пришло» (#974): молчание
	// push-службы и отказ push-службы выглядели одинаково - никак.
	Delivery []PushDeliveryState `json:"delivery"`
}

// PushDeliveryState -- строка разбора доставки. Ни endpoint, ни ключей шифрования здесь
// нет и быть не должно: по endpoint можно слать уведомления в чужой браузер, а раздел
// открыт всем носителям права на статистику.
type PushDeliveryState struct {
	UserID        int        `json:"user_id"`
	Username      string     `json:"username"`
	Platform      string     `json:"platform"`
	CreatedAt     time.Time  `json:"created_at"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	FailedCount   int        `json:"failed_count"`
	LastError     *string    `json:"last_error,omitempty"`
}
