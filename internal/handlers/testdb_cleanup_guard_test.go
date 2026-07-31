package handlers_test

import (
	"regexp"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

// Гвард изоляции тестов: каждая таблица базы обязана либо чиститься между тестами,
// либо иметь причину, по которой её чистить не надо.
//
// Дефект, ради которого гвард написан, за один день всплыл трижды: таблица без
// внешних ключей (blank_exports, feedback_reads, request_logs_daily) не попадает в
// чистку, потому что удаление родительских строк её не касается. Идентификаторы при
// этом начинаются заново на каждом прогоне, и рано или поздно новая строка встречает
// чужое состояние прошлого запуска: обращение приходит в тест уже прочитанным,
// заявка - уже выгруженной и замороженной, суточный агрегат - с накопленным
// счётчиком. Красный при этом появляется в чужом тесте и читается как своя
// регрессия, а стоит каждый раз полчаса разбирательства.
//
// Партиции журналов исключены по маске: их создаёт планировщик по датам, перечислять
// такое в списке бессмысленно, а чистится партиционированная таблица через родителя.
var logPartition = regexp.MustCompile(`^(request_logs_\d{4}_\d{2}_\d{2}|pd_audit_logs_\d{4}_\d{2})$`)

func TestTestDB_EveryTableIsCleanedOrExempt(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	var present []string
	require.NoError(t, db.Raw(`
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r'
		ORDER BY c.relname`).Scan(&present).Error)
	require.NotEmpty(t, present, "в тестовой базе не нашлось ни одной таблицы - проверь подключение")

	cleaned := make(map[string]struct{}, len(testutil.CleanupTables()))
	for _, name := range testutil.CleanupTables() {
		cleaned[name] = struct{}{}
	}

	var forgotten []string
	for _, name := range present {
		if logPartition.MatchString(name) {
			continue
		}
		if _, ok := cleaned[name]; ok {
			continue
		}
		if _, ok := testutil.CleanupExempt[name]; ok {
			continue
		}
		forgotten = append(forgotten, name)
	}

	require.Empty(t, forgotten,
		"таблицы не чистятся между тестами: %v.\n"+
			"Добавь их в testutil.tables (дочерние строки - выше родительских) либо в "+
			"testutil.CleanupExempt с причиной. Иначе состояние прошлого прогона придёт "+
			"в следующий и покрасит чужой тест.", forgotten)
}

// Исключение без причины - забытая строка, а не решение.
func TestTestDB_ExemptionsHaveReasons(t *testing.T) {
	for table, reason := range testutil.CleanupExempt {
		require.NotEmpty(t, reason, "исключение %q обязано объяснять, почему таблицу не чистят", table)
	}
}
