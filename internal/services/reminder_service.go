package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// NotificationTypeApprovalReminder -- уведомление согласующему о зависшей заявке
// (#1315): заявка ждёт его решения дольше настроенного срока молчания.
const NotificationTypeApprovalReminder = "application_approval_reminder"

// ReminderService отбирает зависших согласующих и шлёт им напоминания (#1315).
type ReminderService interface {
	// SendPendingReminders прогоняет отбор и рассылку. Вызывается кроном
	// (cmd/server/main.go) раз в час; no-op, если approval.reminder_enabled выключен.
	SendPendingReminders(ctx context.Context) error
}

type reminderService struct {
	db                  *gorm.DB
	notificationService NotificationService
	settingsService     SettingsService
}

// NewReminderService создаёт сервис напоминаний согласующим.
func NewReminderService(db *gorm.DB, notificationService NotificationService, settingsService SettingsService) ReminderService {
	return &reminderService{db: db, notificationService: notificationService, settingsService: settingsService}
}

// reminderCandidate -- строка application_responsible_users, которой пора напомнить,
// вместе с данными заявки для текста уведомления.
type reminderCandidate struct {
	ID                int
	UserID            int
	ApplicationID     int
	ApplicationNumber *string
	CreatedAt         time.Time
}

// SendPendingReminders отбирает зависших согласующих и шлёт напоминания.
func (s *reminderService) SendPendingReminders(ctx context.Context) error {
	enabled, firstDays, repeatDays := s.settingsService.GetApprovalReminderSettings(ctx)
	if !enabled {
		return nil
	}

	candidates, err := s.selectCandidates(ctx, firstDays, repeatDays)
	if err != nil {
		return fmt.Errorf("failed to select reminder candidates: %w", err)
	}

	for _, c := range candidates {
		if err := s.remind(ctx, c); err != nil {
			slog.Error("напоминание согласующему", "responsible_id", c.ID, "user_id", c.UserID,
				"application_id", c.ApplicationID, "error", err)
		}
	}
	return nil
}

// selectCandidates отбирает pending-строки ответственных, которым пора напомнить.
// Кворум зеркалит updateConfirmationBasedOnApprovals (application_helpers.go):
//   - если у заявки есть хоть один обязательный (required_approval=true) - напоминаем
//     ТОЛЬКО обязательным pending (голос необязательного на исход не влияет);
//   - обязательных нет - напоминаем всем pending, но как только кто-то из них
//     проголосовал (в любую сторону), confirmation уже не "Согласование" (тот же
//     updateConfirmationBasedOnApprovals меняет его синхронно с голосом) - фильтр по
//     confirmation закрывает этот случай сам, отдельного EXISTS не нужно.
//
// a.confirmation = "Согласование" исключает уже решённые заявки, но withdraw/отказ
// принимающего меняют только a.status (не confirmation, см. WithdrawApplication) -
// поэтому статус закрытия исключается отдельно (ArchivableStatuses), а не только
// через confirmation. archivedApplicationCond/activeApplicationCond добавляют то же
// самое исключение для архивных заявок вне месячной отсрочки.
func (s *reminderService) selectCandidates(ctx context.Context, firstDays, repeatDays int) ([]reminderCandidate, error) {
	firstBefore := time.Now().Add(-time.Duration(firstDays) * 24 * time.Hour)
	repeatBefore := time.Now().Add(-time.Duration(repeatDays) * 24 * time.Hour)

	query := s.db.WithContext(ctx).
		Table("application_responsible_users aru").
		Select("aru.id, aru.user_id, aru.application_id, aru.created_at, a.application_number").
		Joins("JOIN applications a ON a.id = aru.application_id").
		Where("a.confirmation = ?", models.ConfirmationPending).
		Where("COALESCE(a.status, '') NOT IN ?", models.ArchivableStatuses).
		Where("aru.approval_status IS NULL OR aru.approval_status = 'pending'").
		Where(`
			aru.required_approval = true
			OR NOT EXISTS (
				SELECT 1 FROM application_responsible_users r2
				WHERE r2.application_id = aru.application_id AND r2.required_approval = true
			)
		`).
		Where("aru.created_at <= ?", firstBefore).
		Where("aru.last_reminder_at IS NULL OR aru.last_reminder_at <= ?", repeatBefore)

	activeCond, activeArgs := activeApplicationCond("a")
	query = query.Where(activeCond, activeArgs...)

	var rows []reminderCandidate
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// remind шлёт одно напоминание и отмечает его на строке ответственного. Условие
// approval_status='pending' стоит прямо в UPDATE (не проверяется заранее отдельным
// SELECT) - защита от гонки с ручным согласованием, случившимся между отбором и
// записью отметки: если согласующий успел проголосовать, отметка не пишется (лишнее
// уведомление уже не страшно, а reminder_count не должен расти на решённой заявке).
func (s *reminderService) remind(ctx context.Context, c reminderCandidate) error {
	appNumber := "б/н"
	if c.ApplicationNumber != nil && *c.ApplicationNumber != "" {
		appNumber = *c.ApplicationNumber
	}
	waitingDays := int(time.Since(c.CreatedAt).Hours() / 24)
	if waitingDays < 1 {
		waitingDays = 1
	}

	data := map[string]any{
		"application_id":     c.ApplicationID,
		"application_number": appNumber,
		"waiting_days":       waitingDays,
	}
	payload, _ := json.Marshal(data)
	payloadStr := string(payload)

	message := fmt.Sprintf("Заявка %s ждёт вашего решения уже %d дн. Пожалуйста, рассмотрите её.", appNumber, waitingDays)
	if err := s.notificationService.CreateForUser(ctx, c.UserID, NotificationTypeApprovalReminder,
		"Напоминание о согласовании", message, &payloadStr); err != nil {
		return fmt.Errorf("failed to create reminder notification: %w", err)
	}

	result := s.db.WithContext(ctx).Exec(`
		UPDATE application_responsible_users
		SET last_reminder_at = NOW(), reminder_count = reminder_count + 1
		WHERE id = ? AND approval_status = 'pending'
	`, c.ID)
	if result.Error != nil {
		return fmt.Errorf("failed to mark reminder sent: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		slog.Info("напоминание отправлено, но согласующий уже проголосовал",
			"responsible_id", c.ID, "user_id", c.UserID, "application_id", c.ApplicationID)
	}
	return nil
}
