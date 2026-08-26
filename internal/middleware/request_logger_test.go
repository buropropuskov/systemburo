package middleware

import (
	"context"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

// Журнал обращений ведётся ради разбора инцидентов, поэтому машинный шум в него не
// идёт: подсказки поиска срабатывают на каждый введённый символ, проба готовности и
// опрос самой страницы мониторинга повторяются по таймеру. Отказы при этом остаются --
// именно они и нужны при разборе.
func TestSkipRequestLog(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		status int
		want   bool
	}{
		{"успешный поиск не пишем", "/api/search", http.StatusOK, true},
		{"отказ в поиске пишем", "/api/search", http.StatusForbidden, false},
		{"ошибку поиска пишем", "/api/search", http.StatusServiceUnavailable, false},
		{"короткий запрос пишем", "/api/search", http.StatusBadRequest, false},
		{"остальные успешные запросы пишем", "/api/applications", http.StatusOK, false},
		// Соседний адрес с общим началом молчать не должен: иначе записи пропали бы из
		// журнала незаметно.
		{"похожий путь пишем", "/api/search-history", http.StatusOK, false},

		// Проба готовности: 87 тысяч одинаковых записей на стенде, четверть журнала.
		// Упавшая проба - наоборот, самое интересное, что в журнале может быть.
		{"живой healthcheck не пишем", "/health", http.StatusOK, true},
		{"упавший healthcheck пишем", "/health", http.StatusServiceUnavailable, false},

		// Страница мониторинга опрашивает себя сама: лента раз в пять секунд, график и
		// метрики шапки раз в тридцать, с каждой открытой вкладки.
		{"ленту не пишем", "/api/request-logs/realtime", http.StatusOK, true},
		{"график не пишем", "/api/request-logs/timeline", http.StatusOK, true},
		{"метрики шапки не пишем", "/api/request-logs/stats", http.StatusOK, true},
		{"отказ в ленте пишем", "/api/request-logs/realtime", http.StatusForbidden, false},

		// Остальное в разделе человек вызывает руками. Выгрузка журнала обращений тем
		// более должна оставлять след: кто и когда её скачал - это и есть предмет разбора.
		{"список журнала пишем", "/api/request-logs", http.StatusOK, false},
		{"выгрузку пишем", "/api/request-logs/export", http.StatusOK, false},
		{"историю пишем", "/api/request-logs/history", http.StatusOK, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, skipRequestLog(tc.path, tc.status))
		})
	}
}

// Очередь писателя не бездонная, и после остановки в неё уже не пишут. Потерянные записи
// считаются: журнал, потерявший часть обращений молча, хуже отсутствующего.
func TestRequestLogWriterCountsDropped(t *testing.T) {
	// База не нужна: пустая пачка до неё не доходит, а после остановки запись даже не
	// доходит до очереди.
	writer := NewRequestLogWriter(nil, WithRequestLogBatch(100, time.Hour))
	writer.Shutdown(context.Background())

	require.Equal(t, int64(0), writer.Dropped())
	writer.Enqueue(models.RequestLogs{CreatedAt: time.Now()})
	require.Equal(t, int64(1), writer.Dropped(), "запись после остановки должна попасть в счётчик потерь")
}
