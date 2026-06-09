package models

import "time"

// Типы элементов заявки для ApplicationBlacklistFlag.
const (
	BlacklistElementCar      = "car"
	BlacklistElementEmployee = "employee"
)

// ApplicationBlacklistFlag - per-element предупреждение о возможном обходе ЧС (#481).
//
// Создаётся при сабмите заявки, когда элемент (машина/человек) НЕ точно совпадает с
// активным ЧС (точное совпадение ловит validateBlacklist -> 409), но близок по
// нормализованной форме (FindSimilar: гомоглиф/опечатка/0<->О/без отчества). Подачу
// НЕ блокирует - это мягкое предупреждение; помеченный элемент блокирует согласование
// заявки до явного override (срез 4).
//
// ElementID ссылается на cars.id / employees.id без внешнего ключа - как и история ЧС:
// флаг это снимок момента подачи (matched_value/reason фиксируются), он должен пережить
// изменение/удаление записи ЧС и не каскадиться при удалении элемента.
type ApplicationBlacklistFlag struct {
	ID                 int       `json:"id"`
	ApplicationID      int       `gorm:"index" json:"application_id"`
	ElementType        string    `gorm:"size:20;index:idx_app_blacklist_flag_element,priority:1" json:"element_type"`
	ElementID          int       `gorm:"index:idx_app_blacklist_flag_element,priority:2" json:"element_id"`
	MatchedBlacklistID int       `json:"matched_blacklist_id"`
	MatchedValue       string    `gorm:"size:300" json:"matched_value"`
	MatchedReason      string    `gorm:"size:500" json:"matched_reason"`
	Similarity         float64   `json:"similarity"`
	CreatedAt          time.Time `json:"created_at"`
}
