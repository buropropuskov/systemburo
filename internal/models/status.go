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
)

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
