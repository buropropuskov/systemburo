package models

import "time"

// RefreshToken хранит хеш refresh-JWT с family_id для reuse detection.
// FamilyID - uuid, общий для всех токенов-потомков одной сессии (login -> N refresh).
// При попытке использовать уже revoked токен все токены family_id отзываются
// (Auth0/OWASP паттерн): если и легитимный юзер, и attacker гоняют одну семью,
// первый же conflict ломает всю сессию и заставляет перелогиниться.
type RefreshToken struct {
	ID        int       `json:"id"`
	UserID    int       `gorm:"index" json:"user_id"`
	FamilyID  string    `gorm:"size:36;index" json:"family_id"`
	TokenHash string    `gorm:"uniqueIndex" json:"token_hash"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	IsRevoked bool       `gorm:"default:false" json:"is_revoked"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	IPAddress *string    `gorm:"size:45" json:"ip_address"`
	UserAgent *string    `json:"user_agent"`
}
