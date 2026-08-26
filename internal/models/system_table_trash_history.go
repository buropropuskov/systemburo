package models

import (
	"encoding/json"
	"time"
)

// TrashActionType - константы action-типов массовых действий с корзиной таблицы.
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
	ApplicationID     *int       `json:"application_id,omitempty"`
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
	Position             *string         `json:"position,omitempty"`
	CitizenshipName      string          `json:"citizenship_name,omitempty"`
	PassportSeriesNumber *string         `json:"passport_series_number,omitempty"`
	PatentNumber         *string         `json:"patent_number,omitempty"`
	OtherPermission      *string         `json:"other_permission,omitempty"`
	PassPlaces           json.RawMessage `json:"pass_places,omitempty" gorm:"type:jsonb" swaggerignore:"true"`
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
//
// OrganizationIDs - мультивыбор организаций (#1398): comma-список id -> IN. Живёт рядом
// с одиночным OrganizationID, а не вместо него: параметр публичный, сторонние интеграции
// могут слать старую форму. Тип string, а не *string как у ApplicationFilter: структуру
// не биндит echo, handler собирает её поле за полем через c.QueryParam.
type TrashFilter struct {
	Search          string `query:"search"`
	OrganizationID  int    `query:"organization_id"`
	OrganizationIDs string `query:"organization_ids"`
	DateFrom        string `query:"date_from"`
	DateTo          string `query:"date_to"`
}
