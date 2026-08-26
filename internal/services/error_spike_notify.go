package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"
)

// Уведомление о всплеске серверных ошибок (#2192).
//
// До этого всплеск 5xx был виден только тому, кто сам открыл раздел мониторинга:
// доля ошибок в шапке и столбики по суткам. Пока никто не смотрит - никто не знает,
// а «никто не смотрит» - это ночь, выходные и любой день, когда всё в порядке.
//
// Порог и окно - константы кода, а не параметры окружения. Новый env тянет правку
// приложения Б документации заказчика, а подбирать значение снаружи никто не станет:
// то же решение принято про настройки писателя журнала в #2125.
const (
	// errorSpikeWindow - окно, за которое считается доля. Пять минут - компромисс:
	// на минуте доля скачет от единичной ошибки, на часе всплеск размывается и
	// уведомление приходит, когда всё уже кончилось.
	errorSpikeWindow = 5 * time.Minute

	// errorSpikeMinRequests - ниже этого потока доля не считается вовсе. Ночью за
	// пять минут проходит десяток запросов, и одна ошибка из двух дала бы «50%
	// ошибок» - тревога на ровном месте.
	errorSpikeMinRequests = 50

	// errorSpikeRatePercent - порог доли ответов 5xx в окне.
	errorSpikeRatePercent = 5.0

	// errorSpikeCooldown - пауза между уведомлениями. Всплеск живёт минутами и
	// десятками ошибок в минуту: без паузы каждый прогон слал бы новое уведомление,
	// и центр уведомлений превратился бы в ленту мусора. У Web Push (#974) такой
	// паузы нет вовсе - здесь она сделана сразу.
	errorSpikeCooldown = 30 * time.Minute

	// ErrorSpikeCheckInterval - шаг фоновой проверки.
	ErrorSpikeCheckInterval = 5 * time.Minute
)

// ErrorSpikeNotifyService зовёт тех, кому виден раздел мониторинга, когда доля
// серверных ошибок в окне переходит порог.
type ErrorSpikeNotifyService interface {
	CheckAndNotify(ctx context.Context) error
}

type errorSpikeNotifyService struct {
	db                  *gorm.DB
	notificationService NotificationService
	permissionResolver  *PermissionResolver

	mu           sync.Mutex
	lastNotified time.Time
}

// NewErrorSpikeNotifyService собирает наблюдателя за долей серверных ошибок.
func NewErrorSpikeNotifyService(db *gorm.DB, notificationService NotificationService,
	permissionResolver *PermissionResolver) ErrorSpikeNotifyService {
	return &errorSpikeNotifyService{
		db:                  db,
		notificationService: notificationService,
		permissionResolver:  permissionResolver,
	}
}

// errorSpikeCounts - итог окна.
type errorSpikeCounts struct {
	Total  int64
	Errors int64
}

// CheckAndNotify считает долю за окно и, если она перешла порог, зовёт
// администраторов. Тихий выход - штатная ветка: проверка идёт каждые несколько
// минут и молчит почти всегда.
func (s *errorSpikeNotifyService) CheckAndNotify(ctx context.Context) error {
	counts, err := s.countWindow(ctx, time.Now())
	if err != nil {
		return err
	}
	if !spikeReached(counts) {
		return nil
	}
	if !s.takeCooldownSlot(time.Now()) {
		return nil
	}
	rate := spikeRate(counts)

	audience := s.monitoringAudience(ctx)
	if len(audience) == 0 {
		// Некому сказать - это конфигурация, а не штатная ветка: всплеск есть, а
		// права на раздел нет ни у кого.
		slog.Warn("всплеск серверных ошибок: некому сообщить, носителей права на раздел мониторинга нет",
			"error_rate", rate, "requests", counts.Total)
		return nil
	}

	title := "Всплеск ошибок сервера"
	message := fmt.Sprintf("За последние %d минут доля ответов с кодом 5xx - %.1f%% (%d из %d). "+
		"Подробности в разделе «Мониторинг запросов».",
		int(errorSpikeWindow.Minutes()), rate, counts.Errors, counts.Total)

	payload, _ := json.Marshal(map[string]any{
		"window_minutes": int(errorSpikeWindow.Minutes()),
		"error_rate":     rate,
		"errors":         counts.Errors,
		"requests":       counts.Total,
	})
	payloadStr := string(payload)

	for _, userID := range audience {
		// best-effort: сбой доставки одному не должен отменять остальных.
		if err := s.notificationService.CreateForUser(ctx, userID,
			NotificationTypeErrorSpike, title, message, &payloadStr); err != nil {
			slog.Error("уведомление о всплеске ошибок не создано", "user_id", userID, "error", err)
		}
	}
	slog.Warn("всплеск серверных ошибок", "error_rate", rate, "errors", counts.Errors,
		"requests", counts.Total, "notified", len(audience))
	return nil
}

// spikeReached отвечает, считать ли окно всплеском.
//
// Нижняя граница потока проверяется первой: доля, посчитанная по десятку запросов,
// скачет от одной ошибки, и ночная тишина давала бы тревогу каждую проверку.
func spikeReached(counts errorSpikeCounts) bool {
	if counts.Total < errorSpikeMinRequests {
		return false
	}
	return spikeRate(counts) >= errorSpikeRatePercent
}

// spikeRate - доля серверных ошибок в окне, процентов.
func spikeRate(counts errorSpikeCounts) float64 {
	if counts.Total == 0 {
		return 0
	}
	return float64(counts.Errors) / float64(counts.Total) * 100
}

// countWindow считает запросы и серверные ошибки за окно.
//
// Ошибкой считается ответ 5xx, а не 4xx и выше: 4xx - это отказ по вине самого
// обращения (нет прав, не найдено), и в норме их постоянный фон. Тем же порядком
// заказчик снимает показатель доли ошибок в критериях пилотного периода.
func (s *errorSpikeNotifyService) countWindow(ctx context.Context, now time.Time) (errorSpikeCounts, error) {
	var counts errorSpikeCounts
	// Отбор по created_at отсекает лишние партиции журнала: окно всегда в текущих
	// сутках, полного скана истории здесь не происходит.
	err := s.db.WithContext(ctx).
		Table("request_logs").
		Select("COUNT(*) AS total, COUNT(*) FILTER (WHERE response_status >= 500) AS errors").
		Where("created_at >= ?", now.Add(-errorSpikeWindow)).
		Scan(&counts).Error
	if err != nil {
		return counts, fmt.Errorf("failed to count error spike window: %w", err)
	}
	return counts, nil
}

// takeCooldownSlot отвечает, можно ли сейчас слать уведомление, и занимает паузу.
//
// Состояние живёт в памяти процесса: перезапуск бэкенда во время всплеска даст одно
// повторное уведомление. Хранить отметку в базе ради этого не стоит - всплеск и так
// означает, что с системой что-то не так, и лишнее напоминание в этот момент не вред.
func (s *errorSpikeNotifyService) takeCooldownSlot(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.lastNotified.IsZero() && now.Sub(s.lastNotified) < errorSpikeCooldown {
		return false
	}
	s.lastNotified = now
	return true
}

// monitoringAudience - активные работники, которым виден раздел мониторинга.
//
// Право спрашивается у резолвера - того же источника, что стоит за middleware
// раздела, поэтому «уведомление пришло, а раздел закрыт» не бывает. Администраторы
// проходят через adminAll, личный запрет закрывает раздел и отменяет уведомление.
func (s *errorSpikeNotifyService) monitoringAudience(ctx context.Context) []int {
	if s.permissionResolver == nil {
		slog.Warn("всплеск серверных ошибок: аудиторию не собрать, резолвер прав не подключен")
		return nil
	}

	var candidates []int
	if err := s.db.WithContext(ctx).Table("users").
		Where("is_active = ?", true).
		Order("id").
		Pluck("id", &candidates).Error; err != nil {
		slog.Error("всплеск серверных ошибок: список учётных записей не прочитан", "error", err)
		return nil
	}

	audience := make([]int, 0, len(candidates))
	for _, userID := range candidates {
		set, err := s.permissionResolver.Resolve(ctx, userID)
		if err != nil {
			// best-effort: сбой резолва одного сужает круг, но не отменяет рассылку.
			slog.Warn("всплеск серверных ошибок: резолв прав не удался", "user_id", userID, "error", err)
			continue
		}
		if set.Has(KeyPageAdminMonitoring) {
			audience = append(audience, userID)
		}
	}
	return audience
}
