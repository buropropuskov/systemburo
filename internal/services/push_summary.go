package services

import (
	"context"
	"fmt"
	"time"

	"systemburo/internal/models"
)

// Сводка Web Push для админского раздела статистики (#974): показывает, окупается ли
// канал и не пора ли заводить запасной для iOS. Не личная настройка - смотрит на всю
// активную базу пользователей, поэтому живёт отдельным файлом от
// Subscribe/Unsubscribe/Send (push_service.go) и гейтится page.statistics, а не личным
// доступом группы /notifications (см. router.go).

// GetSummary считает: сколько активных пользователей всего, у скольких есть хотя бы одна
// живая подписка, разрез живых подписок по платформе браузера и разрез ПОЛЬЗОВАТЕЛЕЙ по
// платформе последнего успешного входа - независимо от того, подключили они push или нет.
// Вторая цифра и есть ответ на "сколько людей вообще на iOS", а не только тех, кто уже
// подписался - на iOS push работает только у установленного на экран "Домой" приложения,
// и разрыв между этими двумя цифрами показывает масштаб ограничения.
func (s *pushService) GetSummary(ctx context.Context) (*models.PushSummary, error) {
	summary := &models.PushSummary{}

	if err := s.db.WithContext(ctx).Table("users").
		Where("is_active = ?", true).
		Count(&summary.ActiveUsersTotal).Error; err != nil {
		return nil, fmt.Errorf("push summary: count active users: %w", err)
	}

	// Подписки считаются только у активных пользователей: архивный аккаунт с забытой
	// подпиской не должен искажать картину адопции текущей базы.
	var subs []struct {
		UserID        int
		Username      string
		UserAgent     string
		CreatedAt     time.Time
		LastSuccessAt *time.Time
		FailedCount   int
		LastError     *string
	}
	if err := s.db.WithContext(ctx).
		Table("push_subscriptions ps").
		Joins("JOIN users u ON u.id = ps.user_id AND u.is_active = true").
		Select(`ps.user_id AS user_id, u.username AS username, COALESCE(ps.user_agent, '') AS user_agent,
			ps.created_at AS created_at, ps.last_success_at AS last_success_at,
			ps.failed_count AS failed_count, ps.last_error AS last_error`).
		Order("ps.created_at DESC").
		Scan(&subs).Error; err != nil {
		return nil, fmt.Errorf("push summary: read subscriptions: %w", err)
	}
	withPush := make(map[int]bool, len(subs))
	summary.Delivery = make([]models.PushDeliveryState, 0, len(subs))
	for _, sub := range subs {
		// Пользователь с двумя устройствами считается один раз в UsersWithPush, но
		// каждое его устройство - отдельная строка в разрезе по платформам: подписка,
		// не человек, единица учёта адопции канала на устройство.
		withPush[sub.UserID] = true
		platform := DetectPlatform(sub.UserAgent)
		addPlatformCount(&summary.SubscriptionsByPlatform, platform)
		summary.Delivery = append(summary.Delivery, models.PushDeliveryState{
			UserID:        sub.UserID,
			Username:      sub.Username,
			Platform:      platform,
			CreatedAt:     sub.CreatedAt,
			LastSuccessAt: sub.LastSuccessAt,
			FailedCount:   sub.FailedCount,
			LastError:     sub.LastError,
		})
	}
	summary.UsersWithPush = int64(len(withPush))
	summary.UsersWithoutPush = summary.ActiveUsersTotal - summary.UsersWithPush

	// Платформа последнего успешного входа каждого активного пользователя - DISTINCT ON
	// берёт самую свежую строку auth_events на юзера одним запросом, без N+1.
	var logins []struct {
		UserID    int
		UserAgent string
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (ae.user_id) ae.user_id AS user_id, ae.user_agent AS user_agent
		FROM auth_events ae
		JOIN users u ON u.id = ae.user_id AND u.is_active = true
		WHERE ae.user_id IS NOT NULL AND ae.event_type = ?
		ORDER BY ae.user_id, ae.created_at DESC`, models.AuthEventLoginSuccess).
		Scan(&logins).Error; err != nil {
		return nil, fmt.Errorf("push summary: read last logins: %w", err)
	}
	loggedIn := make(map[int]bool, len(logins))
	for _, l := range logins {
		loggedIn[l.UserID] = true
		addPlatformCount(&summary.UsersByLastLoginPlatform, DetectPlatform(l.UserAgent))
	}
	// Пользователь без единого успешного входа (например, заведён администратором и
	// ещё не заходил) - платформа неизвестна, но всё равно должен войти в общий счёт,
	// а не пропасть из разреза молча.
	neverLoggedIn := summary.ActiveUsersTotal - int64(len(loggedIn))
	if neverLoggedIn > 0 {
		summary.UsersByLastLoginPlatform.Unknown += neverLoggedIn
	}

	return summary, nil
}
