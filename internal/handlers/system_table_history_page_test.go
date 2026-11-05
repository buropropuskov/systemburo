package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestSystemTableHistoryBoundedSnapshotFiltersAndConsent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	headers := testutil.AuthHeader(token)
	const tableID = 782001
	privateName := "SyntheticPrivateHistoryActor"
	actor := models.User{Username: "history_page_actor", Password: "x", TypeID: 1, IsActive: true, LastName: &privateName}
	require.NoError(t, db.Create(&actor).Error)
	require.NoError(t, db.Exec("INSERT INTO system_settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value", "legal.pd_consent_required", "false").Error)
	when := time.Date(2030, 1, 2, 21, 30, 0, 0, time.UTC) // 3 January in Moscow.
	rows := make([]models.AuditLog, 205)
	for i := range rows {
		rows[i] = models.AuditLog{EntityType: models.AuditEntitySystemTable, EntityID: func() *int { id := tableID; return &id }(),
			Action: "updated", ActorUserID: &actor.ID, Details: json.RawMessage(`{"display_name":"Synthetic value","table_type":"cars"}`), CreatedAt: when}
	}
	require.NoError(t, db.Create(&rows).Error)
	otherTableID := tableID + 1
	require.NoError(t, db.Create(&models.AuditLog{EntityType: models.AuditEntitySystemTable, EntityID: &otherTableID, Action: "updated", ActorUserID: &actor.ID, CreatedAt: when}).Error)
	base := fmt.Sprintf("/system-tables/%d/history", tableID)
	type response struct {
		Data []models.SystemTableHistoryItem `json:"data"`
		Meta services.SystemTableHistoryMeta `json:"meta"`
	}
	read := func(query string) response {
		t.Helper()
		rec := testutil.GET(t, e, base+query, headers)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body response
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		return body
	}
	first := read("")
	require.Len(t, first.Data, 50)
	require.EqualValues(t, 205, first.Meta.Total)
	require.Equal(t, rows[204].ID, first.Data[0].ID)
	require.Equal(t, rows[155].ID, first.Data[49].ID, "same timestamps use the ID tiebreaker")
	require.NotEmpty(t, first.Meta.NameMaskRevision)
	newer := models.AuditLog{EntityType: models.AuditEntitySystemTable, EntityID: rows[0].EntityID, Action: "created", ActorUserID: &actor.ID, CreatedAt: when.Add(time.Hour)}
	require.NoError(t, db.Create(&newer).Error)
	second := read(fmt.Sprintf("?page=2&snapshot_max_id=%d", first.Meta.SnapshotMaxID))
	require.EqualValues(t, 205, second.Meta.Total)
	require.Equal(t, rows[154].ID, second.Data[0].ID)
	bounded := read("?per_page=999999")
	require.Len(t, bounded.Data, 200)
	require.Equal(t, 200, bounded.Meta.PerPage)
	filtered := read("?search=" + url.QueryEscape("Изменены данные таблицы") + "&date_from=2030-01-03&date_to=2030-01-03&order=asc")
	require.EqualValues(t, 205, filtered.Meta.Total)
	require.Equal(t, rows[0].ID, filtered.Data[0].ID)
	require.Zero(t, read("?date_to=2030-01-02").Meta.Total)
	require.EqualValues(t, 205, read("?search="+url.QueryEscape("Машины")).Meta.Total)
	require.EqualValues(t, 205, read("?search="+url.QueryEscape("Наименование")).Meta.Total)
	require.Zero(t, read("?search=%25").Meta.Total, "LIKE wildcards are literal search text")
	for _, query := range []string{"?date_from=bad", "?snapshot_max_id=-1", "?snapshot_max_id=bad", "?filter_options=2", "?page=9223372036854775807&per_page=200"} {
		rec := testutil.GET(t, e, base+query, headers)
		require.Equal(t, http.StatusBadRequest, rec.Code, query)
	}

	// Consent is re-read even when the caller retains an older audit snapshot.
	for _, setting := range [][2]string{{"legal.pd_consent_required", "true"}, {"legal.pd_consent_text", "<p>Synthetic consent</p>"}} {
		require.NoError(t, db.Exec("INSERT INTO system_settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value", setting[0], setting[1]).Error)
	}
	svc := services.NewSystemTableService(db, "", 0, nil)
	ctx := context.Background()
	q := services.SystemTableHistoryQuery{SnapshotMaxID: &first.Meta.SnapshotMaxID}
	masked, maskedMeta, err := svc.GetHistoryPage(ctx, tableID, q)
	require.NoError(t, err)
	require.NotEmpty(t, masked)
	require.NotContains(t, masked[0].UserName, privateName)
	require.Contains(t, masked[0].UserName, actor.Username)
	require.NotEqual(t, first.Meta.NameMaskRevision, maskedMeta.NameMaskRevision)
	q.Search = privateName
	_, searchMeta, err := svc.GetHistoryPage(ctx, tableID, q)
	require.NoError(t, err)
	require.Zero(t, searchMeta.Total, "masked names cannot be inferred through counts")
	q.Search = actor.Username
	_, searchMeta, err = svc.GetHistoryPage(ctx, tableID, q)
	require.NoError(t, err)
	require.EqualValues(t, 205, searchMeta.Total)
	options, optionMeta, err := svc.GetHistoryUsers(ctx, tableID, q)
	require.NoError(t, err)
	require.Len(t, options, 1)
	require.EqualValues(t, 1, optionMeta.Total)
	require.NotContains(t, options[0].Name, privateName)
	legacy, err := svc.GetHistory(ctx, tableID)
	require.NoError(t, err)
	require.Len(t, legacy, 50, "direct callers also receive a bounded default")
}

func TestSystemTableHistoryActorOptionsArePagedAcrossAllActors(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	headers := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	tableID := 782002
	when := time.Date(2030, 1, 2, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		actor := models.User{Username: fmt.Sprintf("history_option_actor_%d", i), Password: "x", TypeID: 1, IsActive: true}
		require.NoError(t, db.Create(&actor).Error)
		for j := 0; j < 2; j++ {
			require.NoError(t, db.Create(&models.AuditLog{EntityType: models.AuditEntitySystemTable, EntityID: &tableID, Action: "updated", ActorUserID: &actor.ID, CreatedAt: when}).Error)
		}
	}
	var first struct {
		Data []services.PassageFilterUser    `json:"data"`
		Meta services.SystemTableHistoryMeta `json:"meta"`
	}
	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/history?filter_options=1&per_page=2", tableID), headers)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &first))
	require.Len(t, first.Data, 2)
	require.EqualValues(t, 3, first.Meta.Total)
	var second struct {
		Data []services.PassageFilterUser `json:"data"`
	}
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/history?filter_options=1&per_page=2&page=2&snapshot_max_id=%d", tableID, first.Meta.SnapshotMaxID), headers)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &second))
	require.Len(t, second.Data, 1)
	require.Greater(t, second.Data[0].ID, first.Data[1].ID)
}
