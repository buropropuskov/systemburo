package models

import "time"

type RefreshToken struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	TokenHash string    `gorm:"uniqueIndex" json:"token_hash"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	IsRevoked bool      `gorm:"default:false" json:"is_revoked"`
	IPAddress *string   `gorm:"size:45" json:"ip_address"`
	UserAgent *string   `json:"user_agent"`
}
