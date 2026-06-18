package models

import (
	"encoding/json"
	"time"
)

// ReportTemplate — сохранённый набор параметров конструктора отчётов (#632).
// Config — непрозрачный для бэка снимок состояния гида (mode/metrics/dimension/
// filters/period preset); сервер его не интерпретирует, только хранит и отдаёт.
// Системные пресеты (IsSystem) сидируются и защищены от правки/удаления. Личные
// привязаны к OwnerUserID; IsShared делает личный шаблон видимым всем.
type ReportTemplate struct {
	ID          int             `json:"id"`
	Name        string          `gorm:"size:200;index" json:"name"`
	Description string          `gorm:"type:text" json:"description,omitempty"`
	Config      json.RawMessage `gorm:"type:jsonb" json:"config" swaggerignore:"true"`
	IsSystem    bool            `gorm:"default:false;index" json:"is_system"`
	IsShared    bool            `gorm:"default:false" json:"is_shared"`
	OwnerUserID *int            `gorm:"index" json:"owner_user_id,omitempty"`
	// Owner с FK OnDelete:CASCADE — личные шаблоны уходят вместе с пользователем,
	// чтобы не оставлять осиротевшие записи. Системные пресеты имеют OwnerUserID=nil.
	Owner     *User     `gorm:"foreignKey:OwnerUserID;constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SaveReportTemplateRequest — тело создания/обновления личного шаблона. Config
// валидируется на непустой JSON в сервисе (validator не умеет json.RawMessage).
type SaveReportTemplateRequest struct {
	Name        string          `json:"name" validate:"required,min=1,max=200"`
	Description string          `json:"description" validate:"max=1000"`
	Config      json.RawMessage `json:"config" swaggerignore:"true"`
	IsShared    *bool           `json:"is_shared"`
}
