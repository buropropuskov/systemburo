package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// Журнал обращений ведётся ради разбора инцидентов, поэтому подсказки поиска в него не
// идут: они срабатывают на каждый введённый символ и вытеснили бы всё остальное. Отказы
// при этом остаются -- именно они и нужны при разборе.
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, skipRequestLog(tc.path, tc.status))
		})
	}
}
