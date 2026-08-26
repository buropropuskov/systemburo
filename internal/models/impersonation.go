package models

import "time"

// ImpersonationTarget - тот, от чьего имени открыт сеанс (#1912). Полоса в
// интерфейсе называет человека, а не идентификатор: администратор должен видеть,
// от чьего имени действует, без похода в справочник.
type ImpersonationTarget struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
}

// ImpersonationResponse - ответ на вход в режим «войти как пользователь».
// Маркер обновления (refresh) намеренно не выдаётся: он остался маркером
// администратора, и именно он возвращает сеанс в свою учётную запись.
type ImpersonationResponse struct {
	Token     string              `json:"token"`
	ExpiresAt time.Time           `json:"expires_at"`
	Target    ImpersonationTarget `json:"target"`
}

// ImpersonationAuditDetails - содержимое записи журнала о входе в режим и выходе
// из него. Поле comment читаемо человеком: ленты историй показывают его как есть.
type ImpersonationAuditDetails struct {
	Comment        string     `json:"comment"`
	ActorUsername  string     `json:"actor_username"`
	TargetUsername string     `json:"target_username"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}
