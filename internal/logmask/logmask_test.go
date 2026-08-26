package logmask

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Журнал обращений живёт месяцами, читается через интерфейс и выгружается файлом.
// Билет даёт доступ к данным без заголовка Authorization (у файлового архива - к
// бланкам с паспортами, #1615), а поисковая строка несёт ФИО и номера заявок:
// проверено на живой базе, записи вида /api/users?search=Тимофей там уже лежали.
// Значения затираются по умолчанию, открытыми остаются только служебные параметры.
func TestQuery(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "билет скачивания архива",
			raw:  "/api/file-archive/download?ticket=Sh0rtL1vedSecret",
			want: "/api/file-archive/download?ticket=***",
		},
		{
			name: "поиск по имени",
			raw:  "/api/users?search=%D0%A2%D0%B8%D0%BC%D0%BE%D1%84%D0%B5%D0%B9",
			want: "/api/users?search=***",
		},
		{
			name: "номер заявки в поиске",
			raw:  "/api/applications?search=20260812%2F003&page=2",
			want: "/api/applications?page=2&search=***",
		},
		{
			name: "новый параметр затирается без правки списка",
			raw:  "/api/employees?full_name=%D0%98%D0%B2%D0%B0%D0%BD%D0%BE%D0%B2",
			want: "/api/employees?full_name=***",
		},
		{
			name: "служебные параметры разбора остаются",
			raw:  "/api/request-logs?page=2&per_page=20&sort=duration&order=desc&status_min=400",
			want: "/api/request-logs?page=2&per_page=20&sort=duration&order=desc&status_min=400",
		},
		{
			name: "адрес без параметров остаётся как есть",
			raw:  "/api/file-archive/stats",
			want: "/api/file-archive/stats",
		},
		{
			name: "безопасный сосед секрета уцелел",
			raw:  "/api/file-archive/download?ticket=secret&page=3",
			want: "/api/file-archive/download?page=3&ticket=***",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.raw)
			require.NoError(t, err)

			got := Query(u)
			assert.Equal(t, tc.want, got)
			assert.NotContains(t, got, "Sh0rtL1vedSecret")
			assert.NotContains(t, got, "Тимофей")
			assert.NotContains(t, got, "Иванов")
		})
	}
}

// Факт обращения из журнала пропадать не должен: затирается значение, а не запись.
func TestQuery_KeepsPathAndKeys(t *testing.T) {
	u, err := url.Parse("/api/file-archive/download?ticket=abc&search=%D0%9F%D0%B5%D1%82%D1%80%D0%BE%D0%B2")
	require.NoError(t, err)

	got := Query(u)
	assert.Contains(t, got, "/api/file-archive/download", "путь обращения обязан остаться")
	assert.Contains(t, got, "ticket=", "видно, что запрос пришёл с билетом")
	assert.Contains(t, got, "search=", "видно, что искали, но не что именно")
}

func TestQuery_NilSafe(t *testing.T) {
	assert.Equal(t, "", Query(nil))
}

// Записи, сделанные до перехода на белый список, лежат в базе с открытыми
// значениями. Выгрузка прогоняет их через ту же маску, иначе файл уносит наружу
// то, чего в свежих записях уже нет.
func TestRawURL(t *testing.T) {
	cases := []struct{ name, raw, want string }{
		{"старая запись с поиском", "/api/users?search=Тимофей", "/api/users?search=***"},
		{"без параметров", "/api/health", "/api/health"},
		{"пустой адрес", "", ""},
		{"служебные параметры", "/api/request-logs?page=2", "/api/request-logs?page=2"},
		{"неразбираемый адрес", "/api/users?search=%zz", "/api/users?***"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RawURL(tc.raw)
			assert.Equal(t, tc.want, got)
			assert.NotContains(t, got, "Тимофей")
		})
	}
}
