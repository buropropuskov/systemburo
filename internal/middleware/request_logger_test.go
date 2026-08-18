package middleware

import (
	"net/http"
	"testing"

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

		// Страница мониторинга опрашивает себя сама, раз в 5 и 30 секунд с каждой
		// открытой вкладки. Чтение журнала в журнале не нужно, но отказ при чтении нужен.
		{"корень раздела не пишем", "/api/request-logs", http.StatusOK, true},
		{"ленту не пишем", "/api/request-logs/realtime", http.StatusOK, true},
		{"график не пишем", "/api/request-logs/timeline", http.StatusOK, true},
		{"отказ в разделе пишем", "/api/request-logs/export", http.StatusForbidden, false},
		// Тот же случай, что и с поиском: адрес, начинающийся так же, но другой раздел.
		{"похожий раздел пишем", "/api/request-logs-summary", http.StatusOK, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, skipRequestLog(tc.path, tc.status))
		})
	}
}
