package models

import (
	"encoding/json"
	"time"
)

// UserActionType - константы action-типов истории учётных записей.
const (
	UserActionCreated        = "created"
	UserActionUpdated        = "updated"
	UserActionTypeChanged    = "type_changed"
	UserActionOrgChanged     = "org_changed"
	UserActionCompanyChanged = "company_changed"
	UserActionPasswordReset  = "password_reset"
	// Срок действия пароля вышел, и плановая проверка потребовала сменить его при
	// следующем входе. Пароль при этом не менялся - отдельное действие как раз
	// затем, чтобы это было видно в журнале и не читалось как сброс.
	UserActionPasswordExpired = "password_expired"
	UserActionArchived        = "archived"
	UserActionRestored        = "restored"
	UserActionBanned          = "banned"
	UserActionUnbanned        = "unbanned"
	// Согласие на обработку персональных данных: кто и когда его дал или отозвал.
	// Пишется в историю учётной записи, потому что это факт о самом работнике, и
	// администратор должен видеть его там же, где остальные события по нему.
	UserActionConsentGranted = "consent_granted"
	UserActionConsentRevoked = "consent_revoked"
)

// UserHistoryItem - запись истории с именем актора для API (LEFT JOIN users).
type UserHistoryItem struct {
	ID          int             `json:"id"`
	ActionType  string          `json:"action_type"`
	Details     json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	ActorUserID *int            `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	CreatedAt   time.Time       `json:"created_at"`
}
