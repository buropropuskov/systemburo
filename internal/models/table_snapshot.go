package models

import (
	"encoding/json"
	"time"
)

// Причины снятия слепка таблицы.
const (
	SnapshotReasonScheduled = "scheduled" // дневная джоба перед сбросом статусов в 06:00
	SnapshotReasonManual    = "manual"    // ручная кнопка «Сохранить сейчас»
)

// TableSnapshot — слепок состояния системной таблицы (машины/люди со статусом
// нахождения на территории) на момент TakenAt. Снимается дневной джобой ПЕРЕД
// сбросом территориальных статусов в 06:00 (reason=scheduled) и вручную
// (reason=manual), чтобы сохранить суточное состояние до обнуления. Payload —
// полный слепок строк (тот же набор, что показывает страница таблицы) с их
// статусами; Counts — агрегаты (на территории/выехал/не въезжал/всего).
//
// Это НЕ *_history-таблица в смысле #870 (хранит состояние, а не журнал событий -
// потому и мимо audit_log): толстые JSON-блобы не место в горячем журнале, а чистка
// снимков по возрасту проще из отдельной таблицы. Имя без суффикса History - guard
// TestAllModels_NoLegacyHistoryTables проходит.
type TableSnapshot struct {
	ID          int          `json:"id"`
	TableID     int          `gorm:"index:idx_table_snapshots_table_taken,priority:1;not null" json:"table_id"`
	Table       *SystemTable `gorm:"foreignKey:TableID;constraint:OnDelete:CASCADE" json:"-"`
	TakenAt     time.Time    `gorm:"index:idx_table_snapshots_table_taken,priority:2" json:"taken_at"`
	Reason      string       `gorm:"size:20" json:"reason"` // scheduled | manual
	ActorUserID *int         `json:"actor_user_id,omitempty"`
	Actor       *User        `gorm:"foreignKey:ActorUserID;constraint:OnDelete:SET NULL" json:"-"`
	// Payload — {table_type, rows}: полный набор строк таблицы в форме, которую
	// отдаёт её страница (cars: []TableCarResponse; people: строки сотрудника +
	// territory_status). Бэк его не интерпретирует при чтении - хранит и отдаёт.
	Payload json.RawMessage `gorm:"type:jsonb" json:"payload" swaggerignore:"true"`
	// Counts — агрегаты по статусам на момент снимка (models.SnapshotCounts).
	Counts    json.RawMessage `gorm:"type:jsonb" json:"counts" swaggerignore:"true"`
	CreatedAt time.Time       `json:"created_at"`
}

// SnapshotCounts — агрегаты слепка по территориальному статусу строк.
// territory_status: 1=на территории, 2=выехал, 0/nil=не въезжал.
type SnapshotCounts struct {
	OnTerritory int `json:"on_territory"`
	Exited      int `json:"exited"`
	NotEntered  int `json:"not_entered"`
	Total       int `json:"total"`
}

// SnapshotPayload — обёртка слепка: тип таблицы + сырые строки. Rows хранит
// маршалированный срез DTO строк (cars/people), чтобы просмотр и экспорт версии
// рендерили ровно то, что показывала страница на момент снимка.
type SnapshotPayload struct {
	TableType string          `json:"table_type"`
	Rows      json.RawMessage `json:"rows"`
}
