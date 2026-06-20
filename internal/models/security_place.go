package models

import "time"

// SecurityUserUnloadPlace - прямая привязка аккаунта охранника (тип security) к месту
// разгрузки (#706). Используется фильтром вкладки "Доступные мне": вложение типа cars/items
// доступно охраннику, если место разгрузки вложения пересекается с назначенными ему местами.
// Уникальный индекс (user_id, unload_place_id) делает повторное назначение идемпотентным;
// его левый префикс обслуживает выборку мест охранника, индекс на unload_place_id -
// обратное направление (какие охранники видят место).
type SecurityUserUnloadPlace struct {
	ID            int         `json:"id"`
	UserID        int         `gorm:"uniqueIndex:idx_sec_user_unload,priority:1" json:"user_id"`
	User          User        `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	UnloadPlaceID int         `gorm:"uniqueIndex:idx_sec_user_unload,priority:2;index" json:"unload_place_id"`
	UnloadPlace   UnloadPlace `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt     time.Time   `json:"created_at"`
	CreatedBy     *int        `json:"created_by"`
}

// SecurityUserTable - прямая привязка аккаунта охранника к месту прохода (#706). Место прохода
// это строка system_tables с table_type='people'. Вложение типа people доступно охраннику,
// если место прохода сотрудника (employee_target_tables) пересекается с назначенными местами.
type SecurityUserTable struct {
	ID        int         `json:"id"`
	UserID    int         `gorm:"uniqueIndex:idx_sec_user_table,priority:1" json:"user_id"`
	User      User        `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	TableID   int         `gorm:"uniqueIndex:idx_sec_user_table,priority:2;index" json:"table_id"`
	Table     SystemTable `gorm:"foreignKey:TableID;constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt time.Time   `json:"created_at"`
	CreatedBy *int        `json:"created_by"`
}

// AttachmentUnloadPlace - место разгрузки на уровне ВЛОЖЕНИЯ (#706). Единый источник места для
// фильтра "Доступные мне": для items это единственная привязка места, для cars дублирует
// дедуп-union мест всех машин вложения (per-машина car_unload_places остаётся для read-side
// и истории). Уникальный индекс (attachment_id, unload_place_id) идемпотентен при пересохранении.
type AttachmentUnloadPlace struct {
	ID            int         `json:"id"`
	AttachmentID  int         `gorm:"uniqueIndex:idx_att_unload,priority:1" json:"attachment_id"`
	Attachment    Attachment  `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	UnloadPlaceID int         `gorm:"uniqueIndex:idx_att_unload,priority:2;index" json:"unload_place_id"`
	UnloadPlace   UnloadPlace `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	OrderIndex    *int        `json:"order_index"`
	CreatedAt     time.Time   `json:"created_at"`
}
