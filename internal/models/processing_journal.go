package models

import "time"

// Роли события сквозной ленты «Обработка заявок» (#1251 S4, расширены в P7).
// Согласование и несогласование - разные роли: до P7 отрицательный голос ехал в
// ленту как «Согласование», и лента врала о том, что произошло.
const (
	ProcessingJournalRoleApproval    = "approval"     // согласующий согласовал
	ProcessingJournalRoleNotApproved = "not_approved" // согласующий не согласовал
	ProcessingJournalRoleAcceptance  = "acceptance"   // принимающий забрал в работу
	ProcessingJournalRoleRejection   = "rejection"    // принимающий отказал (статус «Отказано»)
	ProcessingJournalRoleWithdrawal  = "withdrawal"   // инициатор отозвал заявку
)

// ProcessingJournalRoles - все роли ленты в порядке пути заявки. Список читает
// валидация фильтра (неизвестная роль - 400) и он же задаёт порядок вкладок на
// фронте; новая роль добавляется сюда, а не в перечисление в каждом потребителе.
var ProcessingJournalRoles = []string{
	ProcessingJournalRoleApproval,
	ProcessingJournalRoleNotApproved,
	ProcessingJournalRoleAcceptance,
	ProcessingJournalRoleRejection,
	ProcessingJournalRoleWithdrawal,
}

// ProcessingJournalEntry — событие ленты обработки: кто (ActorName), по какой заявке
// (ApplicationID/Number), в какой роли (Role) и когда (OccurredAt) совершил
// действие, и сколько рабочего времени Бюро на это ушло (WorkingSeconds).
//
// WorkingSeconds nil — момента начала нет или он позже действия: у согласования и
// несогласования это назначение позже голоса на исторических данных, у принятия и
// отказа — заявка закрыта без согласования (нет обязательных согласующих) либо
// действие раньше согласования. У отзыва длительность всегда nil: отзывает
// инициатор, рабочего времени Бюро на это действие не тратится. В этих случаях
// длительность неопределена, а не ноль, поэтому указатель, не 0.
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
