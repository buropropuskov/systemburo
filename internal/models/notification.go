package models

import "time"

type Notification struct {
	ID     int     `json:"id"`
	UserID int     `gorm:"index;index:idx_notification_group,priority:1" json:"user_id"`
	User   User    `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Type   *string `gorm:"size:50;index:idx_notification_group,priority:2" json:"type"`
	Title  *string `gorm:"size:255" json:"title"`
	// GroupKey -- ключ схлопывания повторов одного типа в одну запись (#1748).
	// Заполняется агрегацией в следующем срезе; пустой -- уведомление не схлопывается.
	GroupKey *string `gorm:"size:120;index:idx_notification_group,priority:3" json:"group_key,omitempty"`
	Message  *string `gorm:"type:text" json:"message"`
	Data     *string `gorm:"type:jsonb" json:"data"`
	IsRead   bool    `gorm:"default:false;index;index:idx_notification_group,priority:4" json:"is_read"`
	// Count -- сколько событий схлопнуто в эту запись. 1 для обычного, не схлопнутого
	// уведомления.
	Count int `gorm:"default:1" json:"count"`
	// LastEventAt -- момент последнего схлопнутого события. Намеренно НЕ UpdatedAt:
	// gorm трогает UpdatedAt на любой Update (в т.ч. отметку "прочитано"), и уведомление
	// всплывало бы наверх ленты от простого прочтения, а не от нового события.
	LastEventAt *time.Time `json:"last_event_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// MarkNotificationReadRequest -- тело запроса на изменение статуса прочтения уведомления.
type MarkNotificationReadRequest struct {
	IsRead bool `json:"is_read"`
}

// MarkAllReadResponse -- ответ PUT /notifications/read-all: сколько уведомлений отметили
// прочитанными (#1748).
type MarkAllReadResponse struct {
	Updated int64 `json:"updated"`
}

// CreateNotificationRequest -- тело запроса на создание уведомления (admin-only).
type CreateNotificationRequest struct {
	UserID  int     `json:"user_id" validate:"required,min=1"`
	Type    *string `json:"type"`
	Title   *string `json:"title" validate:"required,max=255"`
	Message *string `json:"message"`
	Data    *string `json:"data"`
}

// NotificationListMeta -- meta пагинированной ленты уведомлений (#1748): обычная
// страница плюс unread_count, который считается по ВСЕМ уведомлениям пользователя, а не
// только по текущей странице/фильтру -- бейдж колокольчика должен показывать общее
// непрочитанное количество, а не то, что попало в текущую выборку.
type NotificationListMeta struct {
	PaginationMeta
	UnreadCount int64 `json:"unread_count"`
}

// UserNotificationPreference -- персональное отклонение пользователя от дефолта каталога
// уведомлений (#1748, notification_catalog.go). Хранит ТОЛЬКО отличия: нет строки для
// (user_id, type_code) -> действует NotificationMeta.DefaultEnabled. Пара -- составной
// первичный ключ, дубли исключены самой схемой.
type UserNotificationPreference struct {
	UserID    int       `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	User      User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	TypeCode  string    `gorm:"primaryKey;size:64" json:"type_code"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserNotificationPreference) TableName() string { return "user_notification_preferences" }

// NotificationPreferenceItem -- один тип уведомления на экране настроек: метаданные
// каталога плюс эффективное состояние (учитывает персональный override и Mandatory).
type NotificationPreferenceItem struct {
	TypeCode       string `json:"type_code"`
	Category       string `json:"category"`
	Label          string `json:"label"`
	Description    string `json:"description"`
	Mandatory      bool   `json:"mandatory"`
	DefaultEnabled bool   `json:"default_enabled"`
	Enabled        bool   `json:"enabled"`
}

// NotificationPreferenceCategory -- группа типов уведомлений по категории каталога, для
// рендера экрана настроек секциями.
type NotificationPreferenceCategory struct {
	Category string                       `json:"category"`
	Items    []NotificationPreferenceItem `json:"items"`
}

// NotificationPreferenceItemUpdate -- одна строка батча PUT /notifications/preferences:
// код типа + желаемое состояние переключателя.
type NotificationPreferenceItemUpdate struct {
	TypeCode string `json:"type_code" validate:"required"`
	Enabled  bool   `json:"enabled"`
}

// UpdateNotificationPreferencesRequest -- тело PUT /notifications/preferences.
type UpdateNotificationPreferencesRequest struct {
	Items []NotificationPreferenceItemUpdate `json:"items" validate:"required,min=1,dive"`
}
