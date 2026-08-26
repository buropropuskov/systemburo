package middleware

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// lastSeenThrottleWindow - минимальный интервал между записями last_seen одного
// пользователя в БД. Защищает горячий путь: без троттлинга каждый authenticated
// запрос делал бы UPDATE users, что недопустимо под нагрузкой.
const lastSeenThrottleWindow = 60 * time.Second

// seenThrottle решает, пора ли снова писать last_seen в БД для пользователя.
// In-memory, по процессу: при рестарте сбрасывается (тогда первый запрос юзера
// после рестарта снова запишет - это безвредно). Потокобезопасен.
type seenThrottle struct {
	mu     sync.Mutex
	last   map[int]time.Time
	window time.Duration
}

func newSeenThrottle(window time.Duration) *seenThrottle {
	return &seenThrottle{
		last:   make(map[int]time.Time),
		window: window,
	}
}

// shouldWrite возвращает true, если с последней записи для userID прошло не
// меньше window. При true атомарно фиксирует now как момент последней записи,
// поэтому два конкурентных запроса одного юзера дают ровно одну запись.
func (t *seenThrottle) shouldWrite(userID int, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	last, ok := t.last[userID]
	if ok && now.Sub(last) < t.window {
		return false
	}
	t.last[userID] = now
	return true
}

// LastSeen обновляет users.last_seen=now для аутентифицированного пользователя
// с троттлингом (не чаще раза в lastSeenThrottleWindow на юзера).
//
// Ставится ПОСЛЕ JWTAuth (нужен user_id в context). Запись в БД выполняется
// асинхронно в отдельной горутине, чтобы не добавлять латентность ответа на
// горячем пути; метка времени фиксируется в троттле синхронно до запуска
// горутины, поэтому всплеск запросов одного юзера не плодит конкурентные UPDATE.
func LastSeen(db *gorm.DB) echo.MiddlewareFunc {
	throttle := newSeenThrottle(lastSeenThrottleWindow)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, _ := c.Get("user_id").(int)
			if userID == 0 {
				return next(c)
			}
			now := time.Now().UTC()
			if throttle.shouldWrite(userID, now) {
				go writeLastSeen(db, userID, now)
			}
			return next(c)
		}
	}
}

// writeLastSeen пишет last_seen в БД вне жизненного цикла запроса.
// Собственный таймаут-контекст: ctx запроса к этому моменту может быть уже
// отменён (ответ отправлен), а апдейт всё равно нужно довести.
func writeLastSeen(db *gorm.DB, userID int, now time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("last_seen", now).Error; err != nil {
		slog.Warn("last_seen: update failed", "user_id", userID, "error", err)
	}
}
