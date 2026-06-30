package models

// Application statuses
const (
	StatusUnread     = "Непрочитано"
	StatusProcessing = "В обработке"
	StatusApproval   = "Согласование"
	StatusApproved   = "Согласовано"
	StatusRejected   = "Не согласовано"
	StatusInWork     = "В работе"
	StatusCompleted  = "Завершено"
	StatusRefused    = "Отказано"
	StatusWithdrawn  = "Отозвана"
)

// ArchivableStatuses — статусы заявки, при которых она считается закрытой и
// подлежит переносу в архив (после того как срок действия вложений истёк
// более месяца назад). Заявки "В работе"/"В обработке"/"Непрочитано" остаются
// активными независимо от срока - они ещё действуют либо не обработаны.
// "Отозвана" - отзыв отправителем (#951): заявка закрыта и обратного пути нет.
var ArchivableStatuses = []string{StatusCompleted, StatusRejected, StatusRefused, StatusWithdrawn}

// Application confirmations
const (
	ConfirmationPending  = "Согласование"
	ConfirmationApproved = "Согласовано"
	ConfirmationRejected = "Не согласовано"
)

// Feedback statuses
const (
	FeedbackOpen     = "Не решено"
	FeedbackResolved = "Решено"
)
