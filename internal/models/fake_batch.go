package models

import "time"

// FakeBatch -- партия вымышленных данных, налитая на проверочный стенд командой
// server fake (#1682). Каждая созданная запись перечислена в FakeBatchItem, и
// удаление партии идёт по этому перечню, а не по маске наименования: маска цепляла бы
// данные, заведённые на стенде руками, и не ловила бы номера машин и ФИО.
type FakeBatch struct {
	ID    int    `json:"id"`
	Label string `gorm:"size:100;uniqueIndex" json:"label"`
	// Seed -- источник случайности партии. Повтор команды с тем же значением даёт ту
	// же партию, поэтому найденный на стенде случай воспроизводим.
	Seed      int64     `json:"seed"`
	Profile   string    `gorm:"size:20" json:"profile"`
	CreatedAt time.Time `json:"created_at"`
	// Summary -- сводка созданного по видам сущностей в виде JSON. Её читает человек в
	// выводе -list; разбирать её по полям в коде незачем.
	Summary string `gorm:"type:text" json:"summary"`
}

func (FakeBatch) TableName() string { return "fake_batches" }

// FakeBatchItem -- одна созданная запись партии. Entity берётся из констант
// AuditEntity*: удаление истории идёт по тем же ключам, что и запись в audit_log,
// и лишний словарь соответствий не нужен.
type FakeBatchItem struct {
	ID       int    `json:"id"`
	BatchID  int    `gorm:"index" json:"batch_id"`
	Entity   string `gorm:"size:50;index" json:"entity"`
	EntityID int    `gorm:"index" json:"entity_id"`
}

func (FakeBatchItem) TableName() string { return "fake_batch_items" }
