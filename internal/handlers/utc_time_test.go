package handlers_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCarHistory_CreatedAtIsValidISO8601 проверяет, что endpoint /cars/{id}/history
// отдаёт created_at в формате ISO 8601 с TZ-маркером, и что это значение можно
// распарсить через time.Parse(time.RFC3339, ...). Issue #184.
func TestCarHistory_CreatedAtIsValidISO8601(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carutc1", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)
	passTbl := seedPassTableGrant(t, db, getUserID(t, db, "carutc1"), "cars")

	// Сделаем действие, чтобы появилась запись истории.
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/territory-status", carID),
		fmt.Sprintf(`{"territory_status": 1, "table_id": %d}`, passTbl), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	history := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, history, "история должна содержать как минимум 1 запись")

	for i, item := range history {
		raw, ok := item["created_at"].(string)
		require.True(t, ok, "запись %d: created_at должно быть строкой", i)
		require.NotEmpty(t, raw, "запись %d: created_at не должно быть пустым", i)

		_, err := time.Parse(time.RFC3339, raw)
		assert.NoErrorf(t, err, "запись %d: created_at=%q должно парситься как RFC3339", i, raw)

		// Маркер таймзоны обязателен: либо "Z" (UTC), либо "+HH:MM"/"-HH:MM".
		hasTZ := strings.HasSuffix(raw, "Z") ||
			strings.Contains(raw[10:], "+") ||
			strings.Count(raw, "-") >= 3
		assert.Truef(t, hasTZ, "запись %d: created_at=%q должно содержать TZ-маркер", i, raw)
	}
}

// TestCarHistory_DeleteActionRecordedAtUTC проверяет ключевой кейс из issue #184:
// удаление машины записывается с моментом времени = time.Now().UTC(), и при отдаче
// через API оно корректно представлено в RFC3339. До фикса хардкод +03:00
// генерировал смещение которое не учитывало локаль пользователя.
func TestCarHistory_DeleteActionRecordedAtUTC(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carutc2", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	beforeUTC := time.Now().UTC().Add(-2 * time.Second)

	// Деактивация (мягкое удаление) -- именно этот сценарий отлажен в issue.
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID),
		`{"status": 2}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	afterUTC := time.Now().UTC().Add(2 * time.Second)

	// Получаем запись из БД напрямую и убеждаемся, что момент в UTC-окне.
	// После cutover (#870, срез 1.12c) delete пишется в audit_log.
	var hist models.AuditLog
	require.NoError(t,
		db.Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityCar, carID, "delete").
			Order("created_at DESC").
			First(&hist).Error)

	createdUTC := hist.CreatedAt.UTC()
	assert.Truef(t,
		!createdUTC.Before(beforeUTC) && !createdUTC.After(afterUTC),
		"created_at=%v должно быть между %v и %v (UTC)", createdUTC, beforeUTC, afterUTC)

	// API должен отдать тот же момент в RFC3339.
	rec = testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	items := testutil.ParseSlice(t, rec)
	var deleteAt string
	for _, item := range items {
		if action, _ := item["action_type"].(string); action == "delete" {
			deleteAt, _ = item["created_at"].(string)
			break
		}
	}
	require.NotEmpty(t, deleteAt, "API должно вернуть запись delete")

	parsed, err := time.Parse(time.RFC3339, deleteAt)
	require.NoError(t, err, "created_at=%q должно парситься", deleteAt)

	// API-значение и БД-значение должны представлять один и тот же момент
	// (с точностью до секунды -- gorm может округлить субсекундные знаки).
	diff := parsed.Sub(createdUTC)
	if diff < 0 {
		diff = -diff
	}
	assert.LessOrEqual(t, diff, time.Second,
		"API created_at=%v и DB created_at=%v должны совпадать с точностью до секунды", parsed, createdUTC)
}

// TestCarsCurrentStatus_TerritoryEntryTimeIsUTC проверяет что endpoint
// /cars/current-status отдаёт territory_entry_time в формате ISO 8601 с TZ-маркером.
// Issue #184.
func TestCarsCurrentStatus_TerritoryEntryTimeIsUTC(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carutc3", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)
	passTbl := seedPassTableGrant(t, db, getUserID(t, db, "carutc3"), "cars")

	// Делаем "въезд", который проставит territory_entry_time через time.Now().UTC().
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/territory-status", carID),
		fmt.Sprintf(`{"territory_status": 1, "table_id": %d}`, passTbl), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, "/cars/history/current-status", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	items := testutil.ParseSlice(t, rec)
	var entryTime string
	for _, item := range items {
		if id, ok := item["car_id"].(float64); ok && int(id) == carID {
			entryTime, _ = item["entry_time"].(string)
			break
		}
	}
	require.NotEmpty(t, entryTime, "entry_time должно быть установлено для активной машины")

	_, err := time.Parse(time.RFC3339, entryTime)
	assert.NoErrorf(t, err, "entry_time=%q должно парситься как RFC3339", entryTime)

	hasTZ := strings.HasSuffix(entryTime, "Z") ||
		strings.Contains(entryTime[10:], "+") ||
		strings.Count(entryTime, "-") >= 3
	assert.Truef(t, hasTZ, "entry_time=%q должно содержать TZ-маркер", entryTime)
}
