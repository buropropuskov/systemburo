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

// ExpiryNotifyRunHour - час рабочей зоны, в который крутится суточная рассылка
// предупреждений. Соседние часы уже заняты: 03:00 сверка файлового архива, 04:00
// сроки паролей, 06:00 сброс территориальных статусов.
const ExpiryNotifyRunHour = 9

// expiryNotifyDaysAhead - за сколько дней до конца срока предупреждать. Заявка
// попадает ровно в один порог за прогон: срок у неё один, а пороги не пересекаются.
var expiryNotifyDaysAhead = []int{3, 1}

// ExpiryNotifyService предупреждает инициаторов заявок о том, что срок действия
// пропуска подходит к концу (#1748, S4) - до сих пор об этом узнавали постфактум,
// когда CheckExpiredAttachments (application_workflow_service.go) уже деактивировал
// вложение и пропуск не работал на проходной.
type ExpiryNotifyService interface {
	// NotifyExpiringSoon отбирает живые заявки, чей срок действия истекает через
	// один из expiryNotifyDaysAhead дней, и шлёт инициатору по одному уведомлению
	// на заявку. Вызывается кроном (cmd/server/main.go) раз в сутки.
	NotifyExpiringSoon(ctx context.Context) error
}

type expiryNotifyService struct {
	db                  *gorm.DB
	notificationService NotificationService
}

// NewExpiryNotifyService создаёт сервис предупреждений об истекающем пропуске.
func NewExpiryNotifyService(db *gorm.DB, notificationService NotificationService) ExpiryNotifyService {
	return &expiryNotifyService{db: db, notificationService: notificationService}
}

// expiringApplication - заявка, чей общий срок действия подходит к концу.
type expiringApplication struct {
	ApplicationID     int
	ApplicationNumber string
	SenderUserID      *int
	EntryDateTo       time.Time
	DaysLeft          int
}

// NotifyExpiringSoon отбирает заявки и шлёт предупреждения. Ошибка одной заявки
// не прерывает рассылку остальным - каждая уведомляется независимо.
func (s *expiryNotifyService) NotifyExpiringSoon(ctx context.Context) error {
	apps, err := s.selectExpiringSoon(ctx)
	if err != nil {
		return fmt.Errorf("failed to select applications expiring soon: %w", err)
	}
	if len(apps) == 0 {
		return nil
	}

	slog.Info("предупреждение об истекающем пропуске", "applications", len(apps))
	for _, app := range apps {
		if err := s.notifyOne(ctx, app); err != nil {
			slog.Warn("не удалось предупредить об истекающем пропуске",
				"application_id", app.ApplicationID, "error", err)
		}
	}
	return nil
}

// selectExpiringSoon отбирает живые заявки (не в ArchivableStatuses) по ОБЩЕМУ сроку
// действия - самой поздней дате среди активных вложений (status=1). Именно её человек
// считает сроком заявки, и именно в этот день CheckExpiredAttachments гасит последнее
// вложение и завершает заявку. Одна строка на заявку: у заявки с тремя машинами и
// пятью сотрудниками срок общий, а не свой у каждого бланка.
func (s *expiryNotifyService) selectExpiringSoon(ctx context.Context) ([]expiringApplication, error) {
	var apps []expiringApplication
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			a.id AS application_id,
			COALESCE(a.application_number, '') AS application_number,
			a.sender_user_id AS sender_user_id,
			MAX(CAST(att.entry_date_to AS DATE)) AS entry_date_to,
			MAX(CAST(att.entry_date_to AS DATE)) - `+moscowTodaySQL+` AS days_left
		FROM applications a
		JOIN attachments att ON att.application_id = a.id
		WHERE att.status = 1
		  AND att.entry_date_to IS NOT NULL
		  AND COALESCE(a.status, '') NOT IN ?
		GROUP BY a.id, a.application_number, a.sender_user_id
		HAVING MAX(CAST(att.entry_date_to AS DATE)) - `+moscowTodaySQL+` IN ?
		ORDER BY a.id
	`, models.ArchivableStatuses, expiryNotifyDaysAhead).Scan(&apps).Error
	if err != nil {
		return nil, err
	}
	return apps, nil
}

// notifyOne шлёт одно предупреждение, если по этой заявке на этом же пороге ещё не
// предупреждали за последние сутки (защита от повторной рассылки при рестартах).
func (s *expiryNotifyService) notifyOne(ctx context.Context, app expiringApplication) error {
	if s.notificationService == nil || app.SenderUserID == nil {
		return nil
	}

	already, err := s.alreadyNotifiedToday(ctx, *app.SenderUserID, app.ApplicationID, app.DaysLeft)
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
		"days_left":          app.DaysLeft,
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal notification data: %w", err)
	}
	payloadStr := string(payload)

	title := "Срок действия пропуска истекает"
	body := expiryNotifyBody(number, app.EntryDateTo, app.DaysLeft)

	if err := s.notificationService.CreateForUser(ctx, *app.SenderUserID, NotificationTypeApplicationExpiring, title, body, &payloadStr); err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}
	return nil
}

// expiryNotifyBody собирает текст предупреждения. Накануне человеку понятнее "завтра",
// чем "через 1 день", поэтому ближний порог назван словом.
func expiryNotifyBody(number string, date time.Time, daysLeft int) string {
	when := fmt.Sprintf("через %s", russianDays(daysLeft))
	if daysLeft == 1 {
		when = "завтра"
	}
	return fmt.Sprintf("Срок действия пропуска по заявке %s истекает %s, %s.", number, when, date.Format("02.01.2006"))
}

// russianDays склоняет "день" под число: 1 день, 3 дня, 5 дней, 11 дней, 21 день.
func russianDays(n int) string {
	word := "дней"
	switch {
	case n%100 >= 11 && n%100 <= 14:
	case n%10 == 1:
		word = "день"
	case n%10 >= 2 && n%10 <= 4:
		word = "дня"
	}
	return fmt.Sprintf("%d %s", n, word)
}

// alreadyNotifiedToday проверяет, не предупреждали ли уже этого пользователя об этой
// заявке на этом пороге за последние сутки. NotificationTypeApplicationExpiring не
// схлопывается (Aggregatable=false в каталоге), поэтому дедупликация - забота
// вызывающего, а не общего механизма агрегации (#1748, S2). Порог входит в ключ:
// иначе предупреждение за три дня съело бы предупреждение накануне, окажись они в
// одних сутках после сдвига расписания.
func (s *expiryNotifyService) alreadyNotifiedToday(ctx context.Context, userID, applicationID, daysLeft int) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Table("notifications").
		Where("user_id = ? AND type = ? AND data->>'application_id' = ? AND data->>'days_left' = ? AND created_at >= ?",
			userID, NotificationTypeApplicationExpiring, strconv.Itoa(applicationID), strconv.Itoa(daysLeft),
			time.Now().Add(-24*time.Hour)).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
