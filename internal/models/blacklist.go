package models

import (
	"encoding/json"
	"time"
)

// VehicleBlacklist - запись чёрного списка автомобилей (#443).
//
// Активная запись (is_active=true) блокирует подачу новых заявок на машину с такими
// номером и маркой и каскадно деактивирует совпадающие cars. Снятие из списка - это
// архивация (is_active=false), а не физическое удаление: история и аудит должны
// пережить запись.
//
// Уникальность (car_number, mark_id) обеспечивается partial unique index-ом
// WHERE is_active = true (создаётся в database.Seed), чтобы архивная запись не мешала
// повторно добавить ту же машину.
type VehicleBlacklist struct {
	ID              int       `json:"id"`
	CarNumber       string    `gorm:"size:50;index" json:"car_number"`
	MarkID          int       `gorm:"index" json:"mark_id"`
	MarkName        string    `gorm:"size:100" json:"mark_name"` // снапшот имени марки на момент добавления
	Reason          string    `gorm:"type:text" json:"reason"`
	IsActive        bool      `gorm:"default:true;index" json:"is_active"`
	CreatedByUserID *int      `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// VehicleBlacklistHistory - аудит действий над записью чёрного списка машин.
//
// EntityID намеренно без FK constraint на vehicle_blacklists: аудит должен пережить
// удаление родителя (как SystemTableHistory). UserID ссылается на users для join-а имени.
type VehicleBlacklistHistory struct {
	ID         int             `json:"id"`
	EntityID   int             `gorm:"index" json:"entity_id"`
	ActionType string          `gorm:"size:30;index" json:"action_type"`
	Details    json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"`
	UserID     *int            `gorm:"index" json:"user_id,omitempty"`
	User       *User           `gorm:"foreignKey:UserID" json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
}

// Action-types для истории чёрного списка (машины и люди используют одни и те же).
const (
	BlacklistActionCreated  = "created"
	BlacklistActionArchived = "archived"
	BlacklistActionRestored = "restored"
)

// CreateVehicleBlacklistRequest - тело POST /vehicle-blacklist.
type CreateVehicleBlacklistRequest struct {
	CarNumber string `json:"car_number" validate:"required,min=1,max=50"`
	MarkID    int    `json:"mark_id" validate:"required"`
	Reason    string `json:"reason" validate:"required,min=1"`
}

// VehicleBlacklistHistoryItem - элемент истории для API (с именем пользователя).
type VehicleBlacklistHistoryItem struct {
	ID         int             `json:"id"`
	EntityID   int             `json:"entity_id"`
	ActionType string          `json:"action_type"`
	Details    json.RawMessage `json:"details,omitempty"`
	UserID     *int            `json:"user_id,omitempty"`
	UserName   string          `json:"user_name"`
	CreatedAt  time.Time       `json:"created_at"`
}

// VehicleBlacklistCheckResult - ответ GET /vehicle-blacklist/check.
type VehicleBlacklistCheckResult struct {
	IsBlacklisted bool   `json:"is_blacklisted"`
	Reason        string `json:"reason,omitempty"`
}
