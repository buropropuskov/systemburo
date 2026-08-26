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

// ReminderService отбирает зависших согласующих и шлёт им напоминания (#1315).
// Тип уведомления -- NotificationTypeApprovalReminder (каталог, notification_catalog.go):
// заявка ждёт решения согласующего дольше настроенного срока молчания.
type ReminderService interface {
	// SendPendingReminders прогоняет отбор и рассылку. Вызывается кроном
	// (cmd/server/main.go) раз в час; no-op, если approval.reminder_enabled выключен.
	SendPendingReminders(ctx context.Context) error

	// ListStuckApprovals возвращает снимок зависших согласований для вкладки
	// «Обработка заявок» (#1315, S4). Не зависит от периода и от reminder_enabled.
	ListStuckApprovals(ctx context.Context) ([]models.StuckApproval, error)
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

	var rows []reminderCandidate
	err := s.pendingApproverBaseQuery(ctx).
		Select("aru.id, aru.user_id, aru.application_id, aru.created_at, a.application_number").
		Where("aru.created_at <= ?", firstBefore).
		Where("aru.last_reminder_at IS NULL OR aru.last_reminder_at <= ?", repeatBefore).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// pendingApproverBaseQuery строит запрос по строкам application_responsible_users,
// чей голос ещё нужен на живой заявке - единый предикат кворума для рассылки
// напоминаний (selectCandidates) и отчёта «Зависшие согласования»
// (ListStuckApprovals). Держим его в одном месте, иначе две модели «кому ещё ждать
// решения» разъедутся (см. selectCandidates про зеркало updateConfirmationBasedOnApprovals).
// Без Select и без временных фильтров - их добавляет вызывающий под свою задачу.
func (s *reminderService) pendingApproverBaseQuery(ctx context.Context) *gorm.DB {
	q := s.db.WithContext(ctx).
		Table("application_responsible_users aru").
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
		`)

	activeCond, activeArgs := activeApplicationCond("a")
	return q.Where(activeCond, activeArgs...)
}

// stuckApprovalRow - сырая строка отчёта до вычисления WaitingDays в Go.
type stuckApprovalRow struct {
	ApplicationID     int
	ApplicationNumber *string
	ApproverName      string
	CreatedAt         time.Time
	ReminderCount     int
	LastReminderAt    *time.Time
}

// ListStuckApprovals возвращает текущий снимок зависших согласований (#1315, S4):
// живые заявки, где согласующий, чей голос ещё нужен, молчит дольше порога
// approval.reminder_first_days. Отбор зеркалит рассылку напоминаний через общий
// pendingApproverBaseQuery, отличается только порогом (без repeat-гейта - показываем
// все зависшие, а не только те, кому прямо сейчас пора слать повтор). От флага
// reminder_enabled не зависит: видимость зависших нужна и при выключенной рассылке.
func (s *reminderService) ListStuckApprovals(ctx context.Context) ([]models.StuckApproval, error) {
	_, firstDays, _ := s.settingsService.GetApprovalReminderSettings(ctx)
	stuckBefore := time.Now().Add(-time.Duration(firstDays) * 24 * time.Hour)

	var rows []stuckApprovalRow
	err := s.pendingApproverBaseQuery(ctx).
		// LEFT JOIN и фолбэк на username - как в разрезе by_approver рейтинга
		// согласующих (approverNameExpr), чтобы имя на вкладке считалось одинаково.
		Joins("LEFT JOIN users u ON u.id = aru.user_id").
		Select("aru.application_id, a.application_number, "+approverNameExpr+" AS approver_name, aru.created_at, aru.reminder_count, aru.last_reminder_at").
		Where("aru.created_at <= ?", stuckBefore).
		Order("aru.created_at ASC"). // дольше всего ждущие - сверху
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list stuck approvals: %w", err)
	}

	out := make([]models.StuckApproval, 0, len(rows))
	for _, r := range rows {
		number := "б/н"
		if r.ApplicationNumber != nil && *r.ApplicationNumber != "" {
			number = *r.ApplicationNumber
		}
		waitingDays := int(time.Since(r.CreatedAt).Hours() / 24)
		if waitingDays < 1 {
			waitingDays = 1
		}
		out = append(out, models.StuckApproval{
			ApplicationID:     r.ApplicationID,
			ApplicationNumber: number,
			ApproverName:      r.ApproverName,
			WaitingDays:       waitingDays,
			ReminderCount:     r.ReminderCount,
			LastReminderAt:    r.LastReminderAt,
		})
	}
	return out, nil
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
