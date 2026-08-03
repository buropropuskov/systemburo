package middleware

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Билет даёт доступ к данным без заголовка Authorization, а журнал обращений живёт
// месяцами, читается через интерфейс и выгружается. Значение билета в нём - секрет
// в открытом виде; проверено на живой базе: записи вида /api/events?ticket=... там
// уже лежали. Для файлового архива цена выше: тот же приём открывает выгрузку
// бланков с паспортами (#1615).
func TestMaskSecretQuery(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "билет скачивания архива",
			raw:  "/api/file-archive/download?ticket=Sh0rtL1vedSecret",
			want: "/api/file-archive/download?ticket=%2A%2A%2A",
		},
		{
			name: "билет подписки на события",
			raw:  "/api/events?ticket=msERfBPOf2s9wSHZzfGhiw",
			want: "/api/events?ticket=%2A%2A%2A",
		},
		{
			name: "обычный запрос не трогаем",
			raw:  "/api/applications?page=2&per_page=20",
			want: "/api/applications?page=2&per_page=20",
		},
		{
			name: "адрес без параметров остаётся как есть",
			raw:  "/api/file-archive/stats",
			want: "/api/file-archive/stats",
		},
		{
			name: "соседние параметры сохраняются",
			raw:  "/api/file-archive/download?ticket=secret&debug=1",
			want: "/api/file-archive/download?debug=1&ticket=%2A%2A%2A",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.raw)
			require.NoError(t, err)

			got := maskSecretQuery(u)
			assert.Equal(t, tc.want, got)
			assert.NotContains(t, got, "Sh0rtL1vedSecret")
			assert.NotContains(t, got, "msERfBPOf2s9wSHZzfGhiw")
		})
	}
}

// Факт обращения из журнала пропадать не должен: маскируется значение, а не запись.
func TestMaskSecretQuery_KeepsPathAndFactOfTicket(t *testing.T) {
	u, err := url.Parse("/api/file-archive/download?ticket=abc")
	require.NoError(t, err)

	got := maskSecretQuery(u)
	assert.Contains(t, got, "/api/file-archive/download", "путь обращения обязан остаться")
	assert.Contains(t, got, "ticket=", "видно, что запрос пришёл с билетом")
}

func TestMaskSecretQuery_NilSafe(t *testing.T) {
	assert.Equal(t, "", maskSecretQuery(nil))
}
