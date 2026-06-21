package models

import "time"

// RolePermissionGrant -- собственный точечный grant роли (allow/deny на конкретный ключ).
// Параллелен PermissionGroupGrant, но привязан к роли напрямую: роль -- живой источник
// прав (правка роли сразу отражается на всех её носителях). Резолвер объединяет
// role_permission_grants + role.default_groups + user_groups, затем user overrides.
type RolePermissionGrant struct {
	RoleID        int       `gorm:"primaryKey;autoIncrement:false" json:"role_id"`
	Role          Role      `gorm:"foreignKey:RoleID;constraint:OnDelete:CASCADE" json:"-"`
	PermissionKey string    `gorm:"primaryKey;size:255" json:"permission_key"`
	Value         string    `gorm:"size:10;default:'allow'" json:"value"`
	CreatedAt     time.Time `json:"created_at"`
}
