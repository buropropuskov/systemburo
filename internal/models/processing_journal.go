package models

import "time"

// Роли события сквозной ленты «Обработка заявок» (#1251 S4).
const (
	ProcessingJournalRoleApproval   = "approval"   // согласующий проголосовал
	ProcessingJournalRoleAcceptance = "acceptance" // принимающий забрал в работу
)

// ProcessingJournalEntry — событие ленты обработки: кто (ActorName), по какой заявке
// (ApplicationID/Number), в какой роли (Role) и когда (OccurredAt) совершил
// действие, и сколько рабочего времени Бюро на это ушло (WorkingSeconds).
//
// WorkingSeconds nil — момента начала нет или он позже действия: у согласования это
// назначение позже голоса на исторических данных, у принятия — заявка принята без
// согласования (нет обязательных согласующих) либо принятие раньше согласования. В
// этих случаях длительность неопределена, а не ноль, поэтому указатель, не 0.
//
// Лента отдаётся GET /statistics/processing-journal, отсортирована по OccurredAt
// убыванием (свежие сверху), глубина ограничена лимитом.
type ProcessingJournalEntry struct {
	ApplicationID     int       `json:"application_id"`
	ApplicationNumber string    `json:"application_number"`
	ActorName         string    `json:"actor_name"`
	Role              string    `json:"role"`
	OccurredAt        time.Time `json:"occurred_at"`
	WorkingSeconds    *int64    `json:"working_seconds"`
}
