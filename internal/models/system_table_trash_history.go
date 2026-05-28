package models

import (
	"encoding/json"
	"time"
)

// SystemTableTrashHistory логирует массовые действия с корзиной таблицы (#186):
// очистка (cleared) и массовое восстановление (bulk_restored). Действия с
// отдельными элементами логируются в cars_history/employees_history.
type SystemTableTrashHistory struct {
	ID            int             `json:"id"`
	SystemTableID int             `gorm:"index" json:"system_table_id"`
	ActionType    string          `gorm:"size:30;index" json:"action_type"` // cleared | bulk_restored | purged_one
	AffectedCount int             `json:"affected_count"`
	Details       json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"` // [{id,label}] затронутых элементов
	UserID        *int            `gorm:"index" json:"user_id,omitempty"`
	User          *User           `gorm:"foreignKey:UserID" json:"-"`
	CreatedAt     time.Time `json:"created_at"`
}

// TrashActionType - константы для SystemTableTrashHistory.ActionType.
const (
	TrashActionCleared      = "cleared"
	TrashActionBulkRestored = "bulk_restored"
	TrashActionPurgedOne    = "purged_one"
)

// TrashItem - универсальный элемент корзины для API (car или employee).
// Type определяет какие поля заполнены (для cars: car_number, mark_name;
// для employees: last_name, first_name, middle_name).
type TrashItem struct {
	ID                int        `json:"id"`
	Type              string     `json:"type"` // car | employee
	ApplicationNumber *string    `json:"application_number,omitempty"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"`
	DeletedByName     string     `json:"deleted_by_name,omitempty"`
	Organization  string  `json:"organization,omitempty"`
	Company       string  `json:"company,omitempty"`
	EntryDateTo   *string `json:"entry_date_to,omitempty"`
	EntryTimeFrom *string `json:"entry_time_from,omitempty"`
	EntryTimeTo   *string `json:"entry_time_to,omitempty"`
	// Cars-only
	CarNumber    *string         `json:"car_number,omitempty"`
	MarkName     *string         `json:"mark_name,omitempty"`
	UnloadPlaces json.RawMessage `json:"unload_places,omitempty" gorm:"type:jsonb" swaggerignore:"true"`
	// Employees-only
	LastName        *string         `json:"last_name,omitempty"`
	FirstName       *string         `json:"first_name,omitempty"`
	MiddleName      *string         `json:"middle_name,omitempty"`
	Position        *string         `json:"position,omitempty"`
	CitizenshipName string          `json:"citizenship_name,omitempty"`
	PassPlaces      json.RawMessage `json:"pass_places,omitempty" gorm:"type:jsonb" swaggerignore:"true"`
}

// TrashHistoryItem - запись лога корзины с именем пользователя для API.
type TrashHistoryItem struct {
	ID            int             `json:"id"`
	ActionType    string          `json:"action_type"`
	AffectedCount int             `json:"affected_count"`
	Details       json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	UserName      string          `json:"user_name,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// TrashDetail - затронутый элемент в логе корзины (для деталей восстановления/очистки).
type TrashDetail struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// RestoreTrashRequest - тело запроса POST /system-tables/:id/trash/restore.
type RestoreTrashRequest struct {
	IDs []int `json:"ids" validate:"required,min=1"`
}

// TrashFilter - фильтры списка корзины.
type TrashFilter struct {
	Search         string `query:"search"`
	OrganizationID int    `query:"organization_id"`
	DateFrom       string `query:"date_from"`
	DateTo         string `query:"date_to"`
}
