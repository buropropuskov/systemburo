package models

import "time"

// AuthEvent - аудит аутентификационных событий для incident response и compliance
// (152-ФЗ). Пишется при каждом login/logout/refresh/password_change, а также
// при обнаружении аномалий (reuse detection, account lock).
type AuthEvent struct {
	ID        int       `json:"id"`
	UserID    *int      `gorm:"index" json:"user_id,omitempty"`
	Username  string    `gorm:"size:100;index" json:"username"`
	EventType string    `gorm:"size:40;index" json:"event_type"`
	Success   bool      `json:"success"`
	IPAddress string    `gorm:"size:45" json:"ip_address"`
	UserAgent string    `gorm:"size:255" json:"user_agent"`
	Detail    string    `gorm:"size:255" json:"detail,omitempty"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// AuthEventType - канонические значения event_type для AuthEvent.
const (
	AuthEventLoginSuccess       = "login_success"
	AuthEventLoginFailed        = "login_failed"
	AuthEventLoginLocked        = "login_locked"
	AuthEventLogout             = "logout"
	AuthEventRefresh            = "refresh"
	AuthEventTokenReuseDetected = "token_reuse_detected"
	AuthEventAccountLocked      = "account_locked"
)

// AuthEventCategory - UI-категории фильтра истории входов. Каждая категория
// раскрывается в набор event_type (сырые коды в интерфейсе не показываются).
const (
	AuthCategoryLogin   = "login"   // login_success
	AuthCategoryLogout  = "logout"  // logout
	AuthCategoryFailed  = "failed"  // login_failed
	AuthCategoryLocked  = "locked"  // login_locked, account_locked
	AuthCategorySession = "session" // refresh, token_reuse_detected
)

// AuthEventCategoryTypes раскрывает UI-категорию фильтра в набор event_type.
// Пустая или неизвестная категория -> nil (без фильтра по типу, показать всё).
func AuthEventCategoryTypes(category string) []string {
	switch category {
	case AuthCategoryLogin:
		return []string{AuthEventLoginSuccess}
	case AuthCategoryLogout:
		return []string{AuthEventLogout}
	case AuthCategoryFailed:
		return []string{AuthEventLoginFailed}
	case AuthCategoryLocked:
		return []string{AuthEventLoginLocked, AuthEventAccountLocked}
	case AuthCategorySession:
		return []string{AuthEventRefresh, AuthEventTokenReuseDetected}
	default:
		return nil
	}
}

// AuthEventFilter - параметры выборки истории входов одного пользователя.
// UserID резолвит handler из :username; Category - UI-категория фильтра.
type AuthEventFilter struct {
	UserID   int
	Category string
	From     *time.Time
	To       *time.Time
	Page     int
	Limit    int
}

// AuthEventResponse - событие для UI (сырые поля auth_events без user_id/username:
// история персональная, они избыточны).
type AuthEventResponse struct {
	ID        int       `json:"id"`
	EventType string    `json:"event_type"`
	Success   bool      `json:"success"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// AuthEventPageResponse - страница истории входов с пагинацией.
type AuthEventPageResponse struct {
	Items []AuthEventResponse `json:"items"`
	Total int64               `json:"total"`
	Page  int                 `json:"page"`
	Limit int                 `json:"limit"`
}
