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
	AuthEventLoginSuccess = "login_success"
	AuthEventLoginFailed  = "login_failed"
	// AuthEventLoginError - вход сорвался по вине системы (база недоступна, пул
	// исчерпан, запрос отвалился по таймауту), а не по вине вводившего. Отдельный
	// тип нужен, чтобы авария не читалась в журнале как «люди путают пароли».
	//
	// В личной истории входов не показывается и ни в одну категорию фильтра не
	// входит намеренно: пишется он там, где пользователя опознать не удалось, с
	// user_id = NULL, а история отбирается строго по user_id (AuthEventReader).
	// Смотрят такие записи выборкой по username при разборе инцидента.
	AuthEventLoginError = "login_error"
	// AuthEventRefreshError - продление сессии сорвалось по вине системы (та же
	// причина, что у AuthEventLoginError), а не из-за недействительного токена.
	// Пишется и когда пользователь уже опознан (user_id известен) - в отличие от
	// LoginError, Detail тут НЕ содержит текст ошибки драйвера: запись видна в
	// личной истории входов (ListForUser фильтрует по user_id), а туда нельзя
	// пересылать "pq: sorry, too many clients already" и подобное. Только стадия
	// (user lookup/token lookup/token rotation/family invalidation); причина
	// целиком - в системном логе (#2016).
	AuthEventRefreshError = "refresh_error"
	// AuthEventLoginBadHash - вход сорвался из-за ПОВРЕЖДЁННОЙ записи пароля этой
	// конкретной учётной записи: строка в users.password не разбирается как
	// Argon2id PHC (обрезана, обнулена, записана другим форматом/алгоритмом).
	// Отдельный от AuthEventLoginError тип: там причина в недоступности базы
	// (чинить нужно инфраструктуру), здесь - дефект данных ОДНОЙ учётки (чинить
	// нужно её, обычно принудительным сбросом пароля). См. #2017.
	//
	// Как и AuthEventLoginError, пишется с user_id = NULL и не входит ни в одну
	// категорию фильтра личной истории - иначе деталь события (сырой текст ошибки
	// разбора) утекала бы владельцу учётки. Разбирают такие записи выборкой по
	// username, как и аварию базы.
	AuthEventLoginBadHash       = "login_bad_hash"
	AuthEventLoginLocked        = "login_locked"
	AuthEventLogout             = "logout"
	AuthEventRefresh            = "refresh"
	AuthEventTokenReuseDetected = "token_reuse_detected"
	AuthEventAccountLocked      = "account_locked"
	AuthEventLockoutReset       = "lockout_reset"
	AuthEventPasswordChanged    = "password_changed"
)

// AuthEventCategory - UI-категории фильтра истории входов. Каждая категория
// раскрывается в набор event_type (сырые коды в интерфейсе не показываются).
const (
	AuthCategoryLogin    = "login"    // login_success
	AuthCategoryLogout   = "logout"   // logout
	AuthCategoryFailed   = "failed"   // login_failed
	AuthCategoryLocked   = "locked"   // login_locked, account_locked, lockout_reset
	AuthCategorySession  = "session"  // refresh, token_reuse_detected
	AuthCategoryPassword = "password" // password_changed
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
		return []string{AuthEventLoginLocked, AuthEventAccountLocked, AuthEventLockoutReset}
	case AuthCategorySession:
		return []string{AuthEventRefresh, AuthEventTokenReuseDetected}
	case AuthCategoryPassword:
		return []string{AuthEventPasswordChanged}
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
