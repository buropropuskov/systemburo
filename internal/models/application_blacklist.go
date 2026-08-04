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
	ID            int `json:"id"`
	ApplicationID int `gorm:"index" json:"application_id"`
	// SupplementID - флаг относится к сущностям этого дополнения (#1685), NULL - к исходному
	// составу. Без разделения старый неперекрытый флаг блокировал бы согласование нового
	// раунда, а новый - показывал бы давно согласованный состав как заблокированный.
	SupplementID *int   `gorm:"index" json:"supplement_id"`
	ElementType  string `gorm:"size:20;index:idx_app_blacklist_flag_element,priority:1" json:"element_type"`
	ElementID     int    `gorm:"index:idx_app_blacklist_flag_element,priority:2" json:"element_id"`
	// ElementNormalized - нормализованная форма самого элемента (normalize.Plate / normalize.Name),
	// не записи ЧС. Стабильный ключ "этой машины/человека" между сабмитами (ElementID меняется,
	// т.к. cars/employees создаются заново). По нему + MatchedBlacklistID гасим повторные
	// предупреждения после "всё равно пропустить" (#481, срез C-followup).
	ElementNormalized  string    `gorm:"size:300;index" json:"element_normalized"`
	MatchedBlacklistID int       `gorm:"index" json:"matched_blacklist_id"`
	MatchedValue       string    `gorm:"size:300" json:"matched_value"`
	MatchedReason      string    `gorm:"size:500" json:"matched_reason"`
	Similarity         float64   `json:"similarity"`
	CreatedAt          time.Time `json:"created_at"`
}

// ApplicationBlacklistOverride - аудит явного решения "всё равно пропустить" по помеченному
// элементу заявки (#481). Согласование помеченной заявки заблокировано, пока по каждому
// флагу нет override. Отдельная таблица аудита (НЕ pd_audit, off-limits): фиксирует кто/когда/
// комментарий и снимок совпавшего значения - запись неизменяема и переживает удаление флага.
// FlagID уникален (один override на флаг) и без FK - это исторический след, а не живая связь.
type ApplicationBlacklistOverride struct {
	ID            int    `json:"id"`
	FlagID        int    `gorm:"uniqueIndex" json:"flag_id"`
	ApplicationID int    `gorm:"index" json:"application_id"`
	ElementType   string `gorm:"size:20" json:"element_type"`
	ElementID     int    `json:"element_id"`
	// ElementNormalized + MatchedBlacklistID копируются с флага: служат ключом подавления
	// будущих предупреждений по той же паре "элемент <-> запись ЧС" (#481, срез C-followup).
	// Пока override жив - не предупреждаем; отмена override (DELETE) снова включает предупреждение.
	ElementNormalized  string    `gorm:"size:300;index" json:"element_normalized"`
	MatchedBlacklistID int       `gorm:"index" json:"matched_blacklist_id"`
	MatchedValue       string    `gorm:"size:300" json:"matched_value"`
	OverriddenByUserID int       `json:"overridden_by_user_id"`
	Comment            string    `gorm:"size:1000" json:"comment"`
	CreatedAt          time.Time `json:"created_at"`
}
