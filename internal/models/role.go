package models

import "time"

// Role -- бизнес-роль пользователя (арендатор, охранник, руководитель).
// Не путать с группой прав: роль определяет "кто", группа -- "что разрешено".
// К каждой роли можно привязать несколько default-групп прав через RoleDefaultGroup.
type Role struct {
	ID          int     `json:"id"`
	Code        string  `gorm:"uniqueIndex;size:50" json:"code"`
	Name        string  `gorm:"size:100" json:"name"`
	Description *string `gorm:"type:text" json:"description"`
	// IsSystem помечает встроенную роль "Пользователь" (базовый фундамент прав).
	// Системную роль нельзя удалить; новые роли создаются со снимком её grants.
	IsSystem  bool      `gorm:"default:false" json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RoleDefaultGroup -- M:N связь "роль -> default-группа прав".
// Юзеры с этой ролью получают права из всех привязанных групп при создании.
type RoleDefaultGroup struct {
	RoleID    int             `gorm:"primaryKey;autoIncrement:false" json:"role_id"`
	Role      Role            `gorm:"foreignKey:RoleID;constraint:OnDelete:CASCADE" json:"-"`
	GroupID   int             `gorm:"primaryKey;autoIncrement:false" json:"group_id"`
	Group     PermissionGroup `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt time.Time       `json:"created_at"`
}

// CreateRoleRequest -- запрос на создание роли.
type CreateRoleRequest struct {
	Code        string  `json:"code" validate:"required,min=2,max=50"`
	Name        string  `json:"name" validate:"required,min=2,max=100"`
	Description *string `json:"description"`
}

// UpdateRoleRequest -- запрос на обновление роли.
type UpdateRoleRequest struct {
	Name        string  `json:"name" validate:"required,min=2,max=100"`
	Description *string `json:"description"`
}

// SetRoleGroupsRequest -- запрос на установку дефолтных групп для роли.
type SetRoleGroupsRequest struct {
	GroupIDs []int `json:"group_ids" validate:"required"`
}

// SetRolePermissionsRequest -- запрос на установку точечных прав роли (полная замена).
// Пустой/отсутствующий список очищает все прямые гранты роли, поэтому без required.
type SetRolePermissionsRequest struct {
	Keys []string `json:"keys"`
}

// RoleResponse -- ответ с информацией о роли + привязанных группах.
// DirectGrants -- ключи прав, выданных роли напрямую (allow), помимо default-групп.
type RoleResponse struct {
	ID            int                       `json:"id"`
	Code          string                    `json:"code"`
	Name          string                    `json:"name"`
	Description   *string                   `json:"description"`
	DefaultGroups []PermissionGroupResponse `json:"default_groups"`
	DirectGrants  []string                  `json:"direct_grants"`
}
