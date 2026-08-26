package models

import "time"

// StuckApproval -- строка отчёта «Зависшие согласования» (#1315, S4) для вкладки
// «Обработка заявок»: живая заявка ждёт решения согласующего, чей голос ещё нужен,
// дольше настроенного порога молчания (approval.reminder_first_days). Снимок
// текущего состояния, а не агрегат за период - как журнал решений, он не зависит от
// выбранных дат вкладки.
//
// Отбор зеркалит предикат кворума напоминаний (reminderService.pendingApproverBaseQuery),
// иначе «кому ещё ждать решения» на вкладке и в рассылке разъедется (см. проектный
// урок про список-vs-деталь и аудиторию real-time сигнала).
type StuckApproval struct {
	ApplicationID     int    `json:"application_id"`
	ApplicationNumber string `json:"application_number"`
	ApproverName      string `json:"approver_name"`
	// WaitingDays -- сколько дней согласующий числится на заявке (от назначения,
	// aru.created_at, а не от подачи: его могли добавить позже). Минимум 1.
	WaitingDays int `json:"waiting_days"`
	// ReminderCount -- сколько напоминаний ему уже ушло (0, если рассылка выключена
	// или крон ещё не добежал до порога).
	ReminderCount int `json:"reminder_count"`
	// LastReminderAt -- когда напоминали в последний раз; nil, если ещё ни разу.
	LastReminderAt *time.Time `json:"last_reminder_at"`
}
