package models

import "time"

// Permission -- определение доступного разрешения в системе.
type Permission struct {
	ID          int     `json:"id"`
	Key         string  `gorm:"uniqueIndex;size:255" json:"key"`
	Category    string  `gorm:"size:100;index" json:"category"`
	EntityID    *int    `gorm:"index" json:"entity_id"`
	DisplayName string  `gorm:"size:255" json:"display_name"`
	ParentKey   *string `gorm:"size:255;index" json:"parent_key"`
}

// UserPermission -- legacy таблица привязки разрешения к пользователю.
// Сохранена для обратной совместимости. Новый код должен использовать
// PermissionGroup + UserGroup + UserPermissionOverride.
type UserPermission struct {
	ID            int    `json:"id"`
	UserID        int    `gorm:"index;uniqueIndex:idx_user_perm" json:"user_id"`
	PermissionKey string `gorm:"size:255;uniqueIndex:idx_user_perm" json:"permission_key"`
	Value         string `gorm:"size:50;default:deny" json:"value"`
	GrantedBy     *int   `json:"granted_by"`
}

// PermissionGroup -- именованный набор прав ("Доступ к таблицам", "Управление заявками").
// Группы переиспользуются: можно привязать к роли как default или назначить юзеру явно.
// "Составная группа" в UI -- это >1 запись в UserGroup для одного юзера, в БД отдельной
// сущности нет. Слияние групп создаёт новую simple-группу через эндпоинт merge.
type PermissionGroup struct {
	ID          int       `json:"id"`
	Name        string    `gorm:"size:100" json:"name"`
	Description *string   `gorm:"type:text" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PermissionGroupGrant -- связка "группа -> permission_key" со значением.
// allow выдаёт право, deny явно запрещает (полезно для override на уровне группы).
type PermissionGroupGrant struct {
	GroupID       int             `gorm:"primaryKey;autoIncrement:false" json:"group_id"`
	Group         PermissionGroup `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`
	PermissionKey string          `gorm:"primaryKey;size:255" json:"permission_key"`
	Value         string          `gorm:"size:10;default:'allow'" json:"value"`
	CreatedAt     time.Time       `json:"created_at"`
}

// UserGroup -- явное назначение группы прав пользователю поверх роли.
// Несколько записей у одного юзера = "Составная группа" в UI.
type UserGroup struct {
	UserID    int             `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	User      User            `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	GroupID   int             `gorm:"primaryKey;autoIncrement:false" json:"group_id"`
	Group     PermissionGroup `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`
	GrantedBy *int            `json:"granted_by"`
	GrantedAt time.Time       `json:"granted_at"`
}

// UserPermissionOverride -- точечный override на конкретный permission_key для юзера.
// Применяется ПОСЛЕ объединения прав из роли и групп; deny здесь побеждает любое allow.
type UserPermissionOverride struct {
	UserID        int       `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	User          User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	PermissionKey string    `gorm:"primaryKey;size:255" json:"permission_key"`
	Value         string    `gorm:"size:10" json:"value"`
	GrantedBy     *int      `json:"granted_by"`
	GrantedAt     time.Time `json:"granted_at"`
}

// PermissionGroupResponse -- ответ с группой + её права.
type PermissionGroupResponse struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Keys        []string `json:"keys"`
}

// CreatePermissionGroupRequest -- запрос на создание группы.
type CreatePermissionGroupRequest struct {
	Name        string   `json:"name" validate:"required,min=2,max=100"`
	Description *string  `json:"description"`
	Keys        []string `json:"keys" validate:"required"`
}

// UpdatePermissionGroupRequest -- запрос на обновление группы.
type UpdatePermissionGroupRequest struct {
	Name        string   `json:"name" validate:"required,min=2,max=100"`
	Description *string  `json:"description"`
	Keys        []string `json:"keys" validate:"required"`
}

// MergePermissionGroupsRequest -- запрос на слияние групп юзера в одну новую.
type MergePermissionGroupsRequest struct {
	UserID         int    `json:"user_id" validate:"required,min=1"`
	SourceGroupIDs []int  `json:"source_group_ids" validate:"required,min=2"`
	NewGroupName   string `json:"new_group_name" validate:"required,min=2,max=100"`
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

// AutoGenerateRequest -- запрос на авто-генерацию разрешений для таблицы.
type AutoGenerateRequest struct {
	TableID   int    `json:"table_id" validate:"required"`
	TableName string `json:"table_name" validate:"required"`
}
