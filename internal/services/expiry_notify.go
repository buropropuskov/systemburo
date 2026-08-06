package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// ExpiryNotifyService предупреждает инициаторов заявок о том, что срок действия
// пропуска истекает завтра (#1748, S4) - до сих пор об этом узнавали постфактум,
// когда CheckExpiredAttachments (application_workflow_service.go) уже деактивировал
// вложение и пропуск не работал на проходной.
type ExpiryNotifyService interface {
	// NotifyExpiringTomorrow отбирает живые заявки, у которых активное вложение
	// истекает завтра, и шлёт инициатору по одному уведомлению на заявку. Вызывается
	// кроном (cmd/server/main.go) раз в сутки.
	NotifyExpiringTomorrow(ctx context.Context) error
}

type expiryNotifyService struct {
	db                  *gorm.DB
	notificationService NotificationService
}

// NewExpiryNotifyService создаёт сервис предупреждений об истекающем завтра пропуске.
func NewExpiryNotifyService(db *gorm.DB, notificationService NotificationService) ExpiryNotifyService {
	return &expiryNotifyService{db: db, notificationService: notificationService}
}

// expiringApplication - заявка с активным вложением, истекающим завтра.
type expiringApplication struct {
	ApplicationID     int
	ApplicationNumber string
	SenderUserID      *int
	EntryDateTo       time.Time
}

// NotifyExpiringTomorrow отбирает заявки и шлёт предупреждения. Ошибка одной заявки
// не прерывает рассылку остальным - каждая уведомляется независимо.
func (s *expiryNotifyService) NotifyExpiringTomorrow(ctx context.Context) error {
	apps, err := s.selectExpiringTomorrow(ctx)
	if err != nil {
		return fmt.Errorf("failed to select applications expiring tomorrow: %w", err)
	}
	if len(apps) == 0 {
		return nil
	}

	slog.Info("предупреждение об истекающем завтра пропуске", "applications", len(apps))
	for _, app := range apps {
		if err := s.notifyOne(ctx, app); err != nil {
			slog.Warn("не удалось предупредить об истекающем завтра пропуске",
				"application_id", app.ApplicationID, "error", err)
		}
	}
	return nil
}

// selectExpiringTomorrow зеркалит выборку CheckExpiredAttachments
// (application_workflow_service.go) на день вперёд: активное вложение (status=1),
// у живой заявки (не в ArchivableStatuses), чей entry_date_to выпадает на завтра.
// Одна строка на заявку (DISTINCT ON + берём минимальный срок), даже если у неё
// несколько машин/сотрудников с одинаковой датой окончания - уведомление шлётся на
// заявку, не на вложение.
func (s *expiryNotifyService) selectExpiringTomorrow(ctx context.Context) ([]expiringApplication, error) {
	var apps []expiringApplication
	err := s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (a.id)
			a.id AS application_id,
			COALESCE(a.application_number, '') AS application_number,
			a.sender_user_id AS sender_user_id,
			CAST(att.entry_date_to AS DATE) AS entry_date_to
		FROM applications a
		JOIN attachments att ON att.application_id = a.id
		WHERE att.status = 1
		  AND att.entry_date_to IS NOT NULL
		  AND CAST(att.entry_date_to AS DATE) = CURRENT_DATE + INTERVAL '1 day'
		  AND COALESCE(a.status, '') NOT IN ?
		ORDER BY a.id
	`, models.ArchivableStatuses).Scan(&apps).Error
	if err != nil {
		return nil, err
	}
	return apps, nil
}

// notifyOne шлёт одно предупреждение, если по этой заявке ещё не предупреждали
// за последние сутки (защита от повторной рассылки при рестартах - задача крутится
// раз в сутки, но при рестарте сервиса может отработать чаще).
func (s *expiryNotifyService) notifyOne(ctx context.Context, app expiringApplication) error {
	if s.notificationService == nil || app.SenderUserID == nil {
		return nil
	}

	already, err := s.alreadyNotifiedToday(ctx, *app.SenderUserID, app.ApplicationID)
	if err != nil {
		return fmt.Errorf("failed to check existing notification: %w", err)
	}
	if already {
		return nil
	}

	number := app.ApplicationNumber
	if number == "" {
		number = fmt.Sprintf("№ %d", app.ApplicationID)
	}

	data := map[string]any{
		"application_id":     app.ApplicationID,
		"application_number": number,
		"entry_date_to":      app.EntryDateTo.Format("2006-01-02"),
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal notification data: %w", err)
	}
	payloadStr := string(payload)

	title := "Срок действия пропуска истекает"
	body := fmt.Sprintf("Срок действия пропуска по заявке %s истекает завтра, %s.", number, app.EntryDateTo.Format("02.01.2006"))

	if err := s.notificationService.CreateForUser(ctx, *app.SenderUserID, NotificationTypeApplicationExpiring, title, body, &payloadStr); err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}
	return nil
}

// alreadyNotifiedToday проверяет, не предупреждали ли уже этого пользователя об этой
// заявке за последние сутки. NotificationTypeApplicationExpiring не схлопывается
// (Aggregatable=false в каталоге), поэтому дедупликация - забота вызывающего, а не
// общего механизма агрегации (#1748, S2).
func (s *expiryNotifyService) alreadyNotifiedToday(ctx context.Context, userID, applicationID int) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Table("notifications").
		Where("user_id = ? AND type = ? AND data->>'application_id' = ? AND created_at >= ?",
			userID, NotificationTypeApplicationExpiring, strconv.Itoa(applicationID), time.Now().Add(-24*time.Hour)).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
