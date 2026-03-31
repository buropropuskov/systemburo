package models

// Permission -- определение доступного разрешения в системе.
type Permission struct {
	ID          int     `json:"id"`
	Key         string  `gorm:"uniqueIndex;size:255" json:"key"`
	Category    string  `gorm:"size:100;index" json:"category"`
	EntityID    *int    `gorm:"index" json:"entity_id"`
	DisplayName string  `gorm:"size:255" json:"display_name"`
	ParentKey   *string `gorm:"size:255;index" json:"parent_key"`
}

// UserPermission -- привязка разрешения к пользователю с конкретным значением.
type UserPermission struct {
	ID            int    `json:"id"`
	UserID        int    `gorm:"index;uniqueIndex:idx_user_perm" json:"user_id"`
	PermissionKey string `gorm:"size:255;uniqueIndex:idx_user_perm" json:"permission_key"`
	Value         string `gorm:"size:50;default:deny" json:"value"`
	GrantedBy     *int   `json:"granted_by"`
}

// --- DTOs ---

// UserPermissionResponse -- разрешение пользователя для API-ответа.
type UserPermissionResponse struct {
	Key            string `json:"key"`
	Category       string `json:"category"`
	DisplayName    string `json:"display_name"`
	Value          string `json:"value"`
	GrantedByName  string `json:"granted_by_name,omitempty"`
}

// UpdatePermissionsRequest -- запрос на обновление разрешений пользователя.
type UpdatePermissionsRequest struct {
	Permissions []PermissionUpdate `json:"permissions" validate:"required,dive"`
}

// PermissionUpdate -- одно обновление разрешения.
type PermissionUpdate struct {
	Key   string `json:"key" validate:"required"`
	Value string `json:"value" validate:"required,oneof=allow deny read write"`
}

// PermissionTreeNode -- узел дерева разрешений для админского UI.
type PermissionTreeNode struct {
	Key         string               `json:"key"`
	DisplayName string               `json:"display_name"`
	Category    string               `json:"category"`
	Children    []PermissionTreeNode `json:"children,omitempty"`
}

// AutoGenerateRequest -- запрос на авто-генерацию разрешений для таблицы.
type AutoGenerateRequest struct {
	TableID   int    `json:"table_id" validate:"required"`
	TableName string `json:"table_name" validate:"required"`
}
