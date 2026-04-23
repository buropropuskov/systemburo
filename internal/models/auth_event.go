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
	AuthEventLoginSuccess        = "login_success"
	AuthEventLoginFailed         = "login_failed"
	AuthEventLoginLocked         = "login_locked"
	AuthEventLogout              = "logout"
	AuthEventRefresh             = "refresh"
	AuthEventTokenReuseDetected  = "token_reuse_detected"
	AuthEventAccountLocked       = "account_locked"
)
