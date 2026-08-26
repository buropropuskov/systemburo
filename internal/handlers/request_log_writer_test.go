package handlers_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	mw "systemburo/internal/middleware"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Журнал обращений пишется пачками: обработчик кладёт запись в очередь, а фоновый
// писатель отдаёт накопленное одним запросом (#2125). Проверяем оба повода отдать
// пачку - наполнение и остановку сервера. Тест с базой лежит в handlers: это
// единственный пакет, где DB-тесты идут одним бинарём и не дерутся за тестовую БД.

func requestLogEntry(url string) models.RequestLogs {
	method := "GET"
	status := 200
	durationMs := 1
	durationUs := int64(1234)
	return models.RequestLogs{
		Method:         &method,
		URL:            &url,
		ResponseStatus: &status,
		DurationMs:     &durationMs,
		DurationUs:     &durationUs,
		CreatedAt:      time.Now().UTC(),
	}
}

func countLogsWithPrefix(t *testing.T, db *gorm.DB, prefix string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table("request_logs").Where("url LIKE ?", prefix+"%").Count(&n).Error)
	return n
}

// Набралась пачка - записи уходят в базу сами, без остановки сервера и без таймера.
func TestRequestLogWriter_FlushesFullBatch(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Уникальный путь: тесты делят одну базу, и чужие записи журнала не должны
	// попадать в счёт.
	const prefix = "/api/zzz-writer-batch/"
	// Таймер заведомо длиннее теста, чтобы пачку отдало именно наполнение.
	writer := mw.NewRequestLogWriter(db, mw.WithRequestLogBatch(2, time.Hour))
	defer writer.Shutdown(context.Background())

	writer.Enqueue(requestLogEntry(prefix + "1"))
	writer.Enqueue(requestLogEntry(prefix + "2"))

	require.Eventually(t, func() bool {
		return countLogsWithPrefix(t, db, prefix) == 2
	}, 5*time.Second, 50*time.Millisecond, "полная пачка должна лечь в базу без остановки писателя")
}

// Остановка сервера не теряет принятые записи: разбирают обычно как раз те обращения,
// что шли перед падением или деплоем.
func TestRequestLogWriter_ShutdownFlushesRemainder(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	const prefix = "/api/zzz-writer-shutdown/"
	// Пачка заведомо не наберётся, таймер не успеет сработать - слить остаток может
	// только Shutdown.
	writer := mw.NewRequestLogWriter(db, mw.WithRequestLogBatch(100, time.Hour))

	const total = 3
	for i := 1; i <= total; i++ {
		writer.Enqueue(requestLogEntry(fmt.Sprintf("%s%d", prefix, i)))
	}
	require.Equal(t, int64(0), countLogsWithPrefix(t, db, prefix), "до остановки записи ещё в очереди")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	writer.Shutdown(ctx)

	require.Equal(t, int64(total), countLogsWithPrefix(t, db, prefix), "остаток очереди должен лечь в базу при остановке")

	// Повторная остановка не должна ронять процесс: main.go зовёт Shutdown один раз,
	// но закрытие канала дважды - классический способ уронить сервер на выходе.
	writer.Shutdown(context.Background())
}
