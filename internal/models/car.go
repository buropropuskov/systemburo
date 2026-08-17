package models

import "time"

type Car struct {
	ID           int        `json:"id"`
	AttachmentID int        `gorm:"index" json:"attachment_id"`
	Attachment   Attachment `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	// SupplementID - каким дополнением заявки добавлена машина (#1685). NULL - пришла с
	// исходной подачей. По нему принятие дополнения активирует только его строки, а
	// интерфейс выделяет новые. Без FK: дополнения не удаляются, отмена у них - статус.
	SupplementID       *int       `gorm:"index" json:"supplement_id"`
	CarNumber          *string    `gorm:"size:50;index" json:"car_number"`
	CarBrand           *string    `gorm:"size:100" json:"car_brand"` // deprecated: оставлен на N релизов, см. mark_name
	MarkID             *int       `gorm:"index" json:"mark_id,omitempty"`
	MarkName           *string    `gorm:"size:100" json:"mark_name,omitempty"` // snapshot имени марки на момент присвоения
	Mark               *Mark      `gorm:"foreignKey:MarkID" json:"-"`
	UnloadPlace        *string    `gorm:"size:255" json:"unload_place"`
	EntryDateFrom      *string    `gorm:"size:20" json:"entry_date_from"`
	EntryTimeFrom      *string    `gorm:"size:20" json:"entry_time_from"`
	EntryDateTo        *string    `gorm:"size:20" json:"entry_date_to"`
	EntryTimeTo        *string    `gorm:"size:20" json:"entry_time_to"`
	TerritoryEntryTime *time.Time `json:"territory_entry_time"`
	TerritoryStatus    *int       `json:"territory_status"`
	Status             *int       `gorm:"index" json:"status"`
	DateAdded          *time.Time `json:"date_added"`
	DateRemoved        *time.Time `json:"date_removed"`
	// Согласие субъекта на обработку персональных данных: см. Employee.PDConsentAt.
	// У машин поле шаблона по умолчанию выключено (номер и марка субъекта не
	// идентифицируют), колонки заведены для случая, когда администратор его включит.
	PDConsentAt       *time.Time `json:"pd_consent_at"`
	PDConsentByUserID *int       `json:"pd_consent_by_user_id"`
	// IsPurged - финальное удаление из корзины (#186). Запись остаётся в БД для
	// аудита, но скрывается даже из корзины. Восстановление невозможно.
	IsPurged          bool       `gorm:"default:false;index" json:"is_purged"`
	PurgedAt          *time.Time `json:"purged_at,omitempty"`
	PurgedByUserID    *int       `json:"purged_by_user_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	TerritoryExitTime *time.Time `json:"territory_exit_time"`
}

type UniqueCar struct {
	ID             int           `json:"id"`
	Number         *string       `gorm:"size:50;index" json:"number"`
	Mark           *string       `gorm:"size:100" json:"mark"`
	OrganizationID *int          `gorm:"index" json:"organization_id"`
	Organization   *Organization `json:"-"`
	CompanyID      *int          `gorm:"index" json:"company_id"`
	Company        *Company      `json:"-"`
	FormatID       *int          `json:"format_id"`
	UserID         *int          `gorm:"index" json:"user_id"`
	User           *User         `json:"-"`
	// Согласие субъекта на обработку персональных данных: см. Employee.PDConsentAt.
	PDConsentAt       *time.Time `json:"pd_consent_at"`
	PDConsentByUserID *int       `json:"pd_consent_by_user_id"`
	Status            *bool      `gorm:"default:false" json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
}

type CarUnloadPlace struct {
	ID            int     `json:"id"`
	CarID         int     `gorm:"index" json:"car_id"`
	Car           Car     `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	UnloadPlaceID int     `gorm:"index" json:"unload_place_id"`
	OrderIndex    *int    `json:"order_index"`
	PlannedTime   *string `gorm:"size:20" json:"planned_time"`
	Notes         *string `gorm:"type:text" json:"notes"`
}

// CarTargetTable — привязка машины к таблице проходной «Проезд» (#1036). Машина
// показывается только в выбранных cars-таблицах, а не во всех сразу. Зеркало
// EmployeeTargetTable для сотрудников.
type CarTargetTable struct {
	ID         int  `json:"id"`
	CarID      int  `gorm:"index" json:"car_id"`
	TableID    int  `gorm:"index" json:"table_id"`
	OrderIndex *int `json:"order_index"`
	// Source - источник привязки (#1227): "application" (из заявки, дефолт колонки -
	// бэкфиллит existing строки и покрывает сырой submit-INSERT без явного source) или
	// "manual" (bulk-добавление/перенос/ручное добавление - проставляется явно в Create).
	Source string `gorm:"type:varchar(20);not null;default:application" json:"source"`
}
