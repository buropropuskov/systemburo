package models

import "time"

// UserBanHistory -- аудит блокировок/разблокировок пользователя (кто/когда/почему).
// users.is_banned/banned_at/banned_by/ban_reason хранят ТЕКУЩЕЕ состояние для быстрого
// чтения резолвером; история -- отдельная сущность ради 152-ФЗ-аудита и отображения
// хронологии. Action: "ban" | "unban".
type UserBanHistory struct {
	ID        int       `json:"id"`
	UserID    int       `gorm:"index" json:"user_id"`
	Action    string    `gorm:"size:10" json:"action"`
	Reason    *string   `gorm:"type:text" json:"reason"`
	ActorID   *int      `json:"actor_id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}
