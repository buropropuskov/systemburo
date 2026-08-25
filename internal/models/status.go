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

// Статусы дополнения заявки (#1685). Открытыми считаются pending и approved - на заявке
// может быть только одно такое, это гарантирует партиальный уникальный индекс
// uidx_app_supplement_open. Остальные терминальны и следующий раунд не блокируют.
const (
	// SupplementMerged - дополнение влито в текущий круг согласования. Заявка ещё не была
	// принята в работу, её сущности не активированы, терять на КПП нечего: голоса основного
	// круга сброшены, confirmation пересчитан штатно, отдельного раунда нет.
	SupplementMerged = "merged"
	// SupplementPending - отдельный раунд, ждёт голосов согласующих.
	SupplementPending = "pending"
	// SupplementApproved - согласован, ждёт решения принимающего.
	SupplementApproved = "approved"
	// SupplementRejected - обязательный согласующий отказал.
	SupplementRejected = "rejected"
	// SupplementAccepted - принят: сущности дополнения активированы и видны на КПП.
	SupplementAccepted = "accepted"
	// SupplementRefused - принимающий отказал.
	SupplementRefused = "refused"
	// SupplementCancelled - снят автором либо системой (отзыв заявки, вывод из работы,
	// истечение срока вложений).
	SupplementCancelled = "cancelled"
)

// OpenSupplementStatuses - состояния, в которых дополнение считается незакрытым.
var OpenSupplementStatuses = []string{SupplementPending, SupplementApproved}

// Feedback statuses
const (
	FeedbackOpen     = "Не решено"
	FeedbackResolved = "Решено"
)
