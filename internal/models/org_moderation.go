package models

// Статус разбора записи справочника Organization и Company (issue #1437).
// Модерация касается только того, что пришло из формы подачи заявки: организацию,
// которой в справочнике не было, заявка создаёт сразу (иначе заявка осталась бы без
// organization_id и выпала из видимости, согласующих и фильтров), но помечает
// ModerationPending, пока принимающий её не разберёт. Записи, заведённые через
// справочник, и всё, что жило до появления колонки, - ModerationApproved.
const (
	ModerationApproved = "approved"
	ModerationPending  = "pending"
)

// IsValidModerationStatus сообщает, входит ли v в допустимые значения статуса.
func IsValidModerationStatus(v string) bool {
	return v == ModerationApproved || v == ModerationPending
}
