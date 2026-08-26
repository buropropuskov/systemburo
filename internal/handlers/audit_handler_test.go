package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type auditResp struct {
	Success bool                  `json:"success"`
	Data    []models.AuditLogItem `json:"data"`
	Meta    models.PaginationMeta `json:"meta"`
}

func parseAudit(t *testing.T, body []byte) auditResp {
	t.Helper()
	var r auditResp
	require.NoError(t, json.Unmarshal(body, &r))
	return r
}

// seedAudit наполняет audit_log детерминированным набором для фильтр-тестов.
func seedAudit(t *testing.T, rec services.AuditRecorder, actorID int) {
	t.Helper()
	ctx := context.Background()
	c1 := 1
	c2 := 2
	car1 := 1
	require.NoError(t, rec.Record(ctx, nil, "citizenship", &c1, "created", &actorID, map[string]any{"name": "РФ"}))
	require.NoError(t, rec.Record(ctx, nil, "citizenship", &c1, "updated", &actorID, map[string]any{"name": map[string]any{"old": "РФ", "new": "Россия"}}))
	require.NoError(t, rec.Record(ctx, nil, "citizenship", &c2, "created", nil, map[string]any{"name": "Беларусь"}))
	require.NoError(t, rec.Record(ctx, nil, "car", &car1, "entry", &actorID, nil))
}

func TestAuditLog_FilterByEntity(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	var admin models.User
	require.NoError(t, db.Where("username = ?", "testadmin").First(&admin).Error)
	seedAudit(t, services.NewAuditRecorder(db), admin.ID)

	// История одной сущности: citizenship #1 - только её 2 записи.
	rec := testutil.GET(t, e, "/audit?entity_type=citizenship&entity_id=1", h)
	require.Equal(t, http.StatusOK, rec.Code)
	resp := parseAudit(t, rec.Body.Bytes())
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, int64(2), resp.Meta.Total)
	for _, it := range resp.Data {
		assert.Equal(t, "citizenship", it.EntityType)
		require.NotNil(t, it.EntityID)
		assert.Equal(t, 1, *it.EntityID)
	}
	// Порядок: новые сверху (id DESC при равном времени) - первой "updated".
	assert.Equal(t, "updated", resp.Data[0].Action)
	// actor_name разрешён из users.
	assert.NotEmpty(t, resp.Data[0].ActorName)

	// Весь тип: 3 записи citizenship.
	rec = testutil.GET(t, e, "/audit?entity_type=citizenship", h)
	resp = parseAudit(t, rec.Body.Bytes())
	assert.Len(t, resp.Data, 3)
}

func TestAuditLog_FilterByAction(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	var admin models.User
	require.NoError(t, db.Where("username = ?", "testadmin").First(&admin).Error)
	seedAudit(t, services.NewAuditRecorder(db), admin.ID)

	// action=created - 2 записи (citizenship #1 и #2).
	rec := testutil.GET(t, e, "/audit?action=created", h)
	require.Equal(t, http.StatusOK, rec.Code)
	resp := parseAudit(t, rec.Body.Bytes())
	assert.Len(t, resp.Data, 2)
	for _, it := range resp.Data {
		assert.Equal(t, "created", it.Action)
	}

	// actor=nil запись (citizenship #2 created) -> actor_name пустой.
	rec = testutil.GET(t, e, "/audit?entity_type=citizenship&entity_id=2", h)
	resp = parseAudit(t, rec.Body.Bytes())
	require.Len(t, resp.Data, 1)
	assert.Nil(t, resp.Data[0].ActorUserID)
	assert.Empty(t, resp.Data[0].ActorName)
}

func TestAuditLog_Pagination(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	var admin models.User
	require.NoError(t, db.Where("username = ?", "testadmin").First(&admin).Error)
	seedAudit(t, services.NewAuditRecorder(db), admin.ID)

	rec := testutil.GET(t, e, "/audit?entity_type=citizenship&entity_id=1&per_page=1&page=1", h)
	resp := parseAudit(t, rec.Body.Bytes())
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, int64(2), resp.Meta.Total)
	assert.Equal(t, 1, resp.Meta.Page)
	assert.Equal(t, 1, resp.Meta.PerPage)
	first := resp.Data[0].ID

	rec = testutil.GET(t, e, "/audit?entity_type=citizenship&entity_id=1&per_page=1&page=2", h)
	resp = parseAudit(t, rec.Body.Bytes())
	require.Len(t, resp.Data, 1)
	assert.NotEqual(t, first, resp.Data[0].ID, "вторая страница - другая запись")
}

func TestAuditLog_EmptyResult(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/audit?entity_type=nonexistent", h)
	require.Equal(t, http.StatusOK, rec.Code)
	resp := parseAudit(t, rec.Body.Bytes())
	assert.Empty(t, resp.Data)
	assert.Equal(t, int64(0), resp.Meta.Total)
}

func TestAuditLog_RequiresAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Без токена -> 401.
	rec := testutil.GET(t, e, "/audit", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Обычный пользователь -> 403 (page.admin).
	token := testutil.RegisterAndLogin(t, e, "regular_audit", "password123", 1, td.OrgID, td.CompanyID)
	rec = testutil.GET(t, e, "/audit", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
