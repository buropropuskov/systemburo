package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// Приватность и честность выгрузки журнала обращений (#2125): файл открывается
// таблицей, не уносит наружу поисковые строки с персональными данными, говорит об
// обрезке вслух и оставляет след в аудите.

// exportedSheet разбирает ответ выгрузки в строки листа.
func exportedSheet(t *testing.T, raw []byte) [][]string {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	require.NoError(t, err, "ответ обязан быть читаемой книгой .xlsx")
	defer func() { _ = f.Close() }()

	rows, err := f.GetRows(f.GetSheetName(0))
	require.NoError(t, err)
	return rows
}

// Записи, сделанные до перехода на белый список, лежат в базе с открытым поиском:
// /api/users?search=Тимофей. Выгрузка прогоняет адрес через ту же маску, что и
// запись, иначе файл уносит персональные данные наружу из системы.
func TestRequestLogsExport_MasksStoredQuery(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	now := time.Now().UTC()
	insertRequestLog(t, db, "/api/users?search=Тимофей", now.Add(-time.Minute))
	insertRequestLog(t, db, "/api/request-logs?page=2&per_page=20", now.Add(-2*time.Minute))

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/request-logs/export", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.NotContains(t, body, "Тимофей", "поисковая строка не должна уезжать в файл")

	rows := exportedSheet(t, rec.Body.Bytes())
	var masked, service bool
	for _, r := range rows {
		for _, cell := range r {
			if cell == "/api/users?search=***" {
				masked = true
			}
			if cell == "/api/request-logs?page=2&per_page=20" {
				service = true
			}
		}
	}
	assert.True(t, masked, "адрес обязан остаться с затёртым значением поиска")
	assert.True(t, service, "служебные параметры разбора остаются открытыми")
}

// Файл должен открываться таблицей: до среза метод отдавал сплошной текст с
// рамкой из знаков «=», а документация обещает заказчику электронную таблицу.
func TestRequestLogsExport_XlsxWithHeaderAndCount(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	now := time.Now().UTC()
	insertRequestLog(t, db, "/api/export-one", now.Add(-time.Minute))

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/request-logs/export", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Contains(t, rec.Header().Get("Content-Type"), "spreadsheetml.sheet")
	assert.Contains(t, rec.Header().Get("Content-Disposition"), ".xlsx")
	assert.Equal(t, "false", rec.Header().Get("X-Export-Truncated"))

	rows := exportedSheet(t, rec.Body.Bytes())
	require.NotEmpty(t, rows)
	assert.Equal(t, "Журнал обращений", rows[0][0])

	var header []string
	for _, r := range rows {
		if len(r) > 0 && r[0] == "Дата и время" {
			header = r
		}
	}
	require.NotNil(t, header, "шапка колонок обязана быть в файле")
	assert.Equal(t, []string{"Дата и время", "Метод", "Адрес", "Код ответа", "Длительность, мс", "Пользователь"}, header)
}

// Выгрузка оставляет след: кто и когда унёс сколько строк. Один файл содержит
// адреса обращений сотен пользователей - это снятие данных пачкой, как выгрузка
// реестра заявок, и оно обязано попадать в журнал аудита.
func TestRequestLogsExport_WritesAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	now := time.Now().UTC()
	insertRequestLog(t, db, "/api/audited-one", now.Add(-time.Minute))
	insertRequestLog(t, db, "/api/audited-two", now.Add(-2*time.Minute))

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/request-logs/export?search=Петров&method=GET", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	var entry models.AuditLog
	require.NoError(t, db.Where("entity_type = ?", models.AuditEntityRequestLogExport).
		Order("id DESC").First(&entry).Error, "факт выгрузки обязан попасть в audit_log")

	assert.Equal(t, models.RequestLogExportActionExported, entry.Action)
	require.NotNil(t, entry.ActorUserID, "выгрузка без автора в аудите бесполезна")

	var details map[string]any
	require.NoError(t, json.Unmarshal(entry.Details, &details))
	assert.Equal(t, float64(0), details["rows"], "отбор по несуществующему поиску даёт пустую выгрузку")
	assert.Equal(t, true, details["searched"], "признак поиска в следе нужен")
	assert.Equal(t, "GET", details["method"])
	assert.NotContains(t, string(entry.Details), "Петров", "значение поиска в аудит не переписываем")
}

// Потолок выгрузки остаётся, но перестаёт быть тихим: до среза человек получал
// первые 10 000 записей и считал по ним итоги за период, не зная, что период в
// файл не поместился.
func TestRequestLogsExport_TruncationIsVisible(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		insertRequestLog(t, db, "/api/truncate-me", now.Add(-time.Duration(i+1)*time.Minute))
	}

	svc := services.NewRequestLogsService(db, services.WithRequestLogsExportLimit(2))
	res, err := svc.Export(t.Context(), models.RequestLogsQuery{Search: "/api/truncate-me"})
	require.NoError(t, err)

	assert.Len(t, res.Rows, 2, "в файл идёт ровно потолок строк")
	assert.Equal(t, int64(3), res.Total, "полное число подходящих записей обязано быть известно")
	assert.True(t, res.Truncated, "обрезка должна быть видна вызывающему")
	assert.Equal(t, 2, res.Limit)
}
