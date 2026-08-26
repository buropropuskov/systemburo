package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// writeCarPassageAudit пишет запись въезда/выезда в общий журнал (audit_log,
// entity_type=car) - тот же формат, что читает carsHistoryUnion. tableID nil
// повторяет записи, сделанные до появления привязки к таблице.
func writeCarPassageAudit(t *testing.T, db *gorm.DB, carID int, action string, tableID *int) {
	t.Helper()
	details := map[string]any{"comment": "тестовый проезд"}
	if tableID != nil {
		details["table_id"] = *tableID
	}
	raw, err := json.Marshal(details)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.AuditLog{
		EntityType: models.AuditEntityCar,
		EntityID:   &carID,
		Action:     action,
		Details:    raw,
		CreatedAt:  time.Now(),
	}).Error)
}

// GET /cars/history/table/:table_id (#1307): кнопка «История» в таблице проходной
// показывает историю ЭТОЙ таблицы. Чужие записи не попадают, а записи без
// проставленного table_id (писались до появления привязки) подбираются по
// текущей привязке машины - иначе две трети журнала исчезли бы из всех таблиц.
func TestGetCarsHistoryByTable_ScopedToOwnTable(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	tableOwn := seedCarsTable(t, db, "hist_own", "Своя таблица")
	tableOther := seedCarsTable(t, db, "hist_other", "Чужая таблица")

	createCarIn := func(number string, tableID int) int {
		body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles": [{"car_number": %q, "car_brand": "Test"}]}`,
			td.OrgID, tableID, number)
		rec := testutil.POST(t, e, "/cars/manual", body, h)
		require.Equal(t, http.StatusOK, rec.Code, "create car: %s", rec.Body.String())
		ids := testutil.ParseMap(t, rec)["car_ids"].([]interface{})
		require.Len(t, ids, 1)
		return int(ids[0].(float64))
	}

	carOwn := createCarIn("H001AA777", tableOwn)
	carOther := createCarIn("H002BB777", tableOther)

	writeCarPassageAudit(t, db, carOwn, "entry", &tableOwn)
	writeCarPassageAudit(t, db, carOther, "entry", &tableOther)
	// Запись без table_id у машины своей таблицы - подбирается по привязке.
	writeCarPassageAudit(t, db, carOwn, "exit", nil)
	// Машина стоит в обеих таблицах, но её проезд через чужой пост остаётся в
	// истории того поста: иначе один проезд попал бы в историю всех таблиц.
	shared := createCarIn("H003CC777", tableOwn)
	require.NoError(t, db.Exec("INSERT INTO car_target_tables (car_id, table_id) VALUES (?, ?)", shared, tableOther).Error)
	writeCarPassageAudit(t, db, shared, "entry", &tableOther)

	rec := testutil.GET(t, e, fmt.Sprintf("/cars/history/table/%d", tableOwn), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	items := testutil.ParseSlice(t, rec)
	carIDs := make([]int, 0, len(items))
	for _, item := range items {
		carIDs = append(carIDs, int(item["car_id"].(float64)))
	}

	assert.Len(t, items, 2, "обе записи своей таблицы: с table_id и без него")
	assert.NotContains(t, carIDs, carOther, "запись чужой таблицы не попадает в выдачу")
	assert.NotContains(t, carIDs, shared, "проезд через чужой пост не попадает, хотя машина есть в обеих таблицах")

	// Контроль: общий журнал по-прежнему отдаёт всё - его читают другие экраны.
	recAll := testutil.GET(t, e, "/cars/history/all", h)
	require.Equal(t, http.StatusOK, recAll.Code)
	assert.Len(t, testutil.ParseSlice(t, recAll), 4, "общая история не отфильтрована")
}
