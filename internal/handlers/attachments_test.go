package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachments_GetActive(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/attachments", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	attachments := testutil.ParseSlice(t, rec)
	assert.NotNil(t, attachments)
}

func TestAttachments_GetActive_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/attachments", nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAttachments_GetAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/attachments/all", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	attachments := testutil.ParseSlice(t, rec)
	assert.NotNil(t, attachments)
}

func TestAttachments_Create(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	body := `{
		"attachment_type": "cars",
		"name": "test-attachment",
		"display_name": "Test Attachment",
		"title": "Test Title",
		"instruction": "Some instruction"
	}`
	rec := testutil.POST(t, e, "/attachments", body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := testutil.ParseMap(t, rec)
	assert.NotZero(t, resp["id"])
	assert.Equal(t, "Вложение успешно создано", resp["message"])
}

func TestAttachments_Create_DuplicateName_Fails(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	body := `{
		"attachment_type": "cars",
		"name": "duplicate-name",
		"display_name": "First",
		"title": "First Title"
	}`
	rec1 := testutil.POST(t, e, "/attachments", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec1.Code)

	// Same name should fail
	body2 := `{
		"attachment_type": "people",
		"name": "duplicate-name",
		"display_name": "Second",
		"title": "Second Title"
	}`
	rec2 := testutil.POST(t, e, "/attachments", body2, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusBadRequest, rec2.Code)
}

func TestAttachments_GetByID(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create first
	body := `{
		"attachment_type": "cars",
		"name": "get-by-id-test",
		"display_name": "Get By ID",
		"title": "Title"
	}`
	createRec := testutil.POST(t, e, "/attachments", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	createResp := testutil.ParseMap(t, createRec)
	id := int(createResp["id"].(float64))

	// Get by ID
	rec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d", id), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	att := testutil.ParseMap(t, rec)
	assert.Equal(t, float64(id), att["id"])
	assert.Equal(t, "get-by-id-test", att["name"])
	assert.Equal(t, "Get By ID", att["display_name"])
	assert.Equal(t, "cars", att["attachment_type"])
	assert.Equal(t, true, att["is_active"])
	// Title should be uppercased by service
	assert.Equal(t, "TITLE", att["title"])
}

func TestAttachments_GetByID_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/attachments/99999", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAttachments_Update(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create
	createBody := `{
		"attachment_type": "cars",
		"name": "update-test",
		"display_name": "Original",
		"title": "Original Title"
	}`
	createRec := testutil.POST(t, e, "/attachments", createBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	createResp := testutil.ParseMap(t, createRec)
	id := int(createResp["id"].(float64))

	// Update
	updateBody := `{
		"attachment_type": "people",
		"name": "updated-name",
		"display_name": "Updated Display",
		"title": "Updated Title",
		"instruction": "Updated instruction"
	}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/attachments/%d", id), updateBody, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Вложение успешно обновлено", msg)

	// Verify the update
	getRec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d", id), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, getRec.Code)
	att := testutil.ParseMap(t, getRec)
	assert.Equal(t, "people", att["attachment_type"])
	assert.Equal(t, "updated-name", att["name"])
	assert.Equal(t, "Updated Display", att["display_name"])
	assert.Equal(t, "UPDATED TITLE", att["title"])
}

func TestAttachments_Delete_SoftDelete(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create
	createBody := `{
		"attachment_type": "cars",
		"name": "delete-test",
		"display_name": "Delete Test",
		"title": "Delete Title"
	}`
	createRec := testutil.POST(t, e, "/attachments", createBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	createResp := testutil.ParseMap(t, createRec)
	id := int(createResp["id"].(float64))

	// Delete (soft)
	rec := testutil.DELETE(t, e, fmt.Sprintf("/attachments/%d", id), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Вложение успешно удалено", msg)

	// GetByID should return 404 (only active items)
	getRec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d", id), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, getRec.Code)

	// But it should still appear in /all
	allRec := testutil.GET(t, e, "/attachments/all", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, allRec.Code)
	allAtts := testutil.ParseSlice(t, allRec)

	found := false
	for _, a := range allAtts {
		if int(a["id"].(float64)) == id {
			assert.Equal(t, false, a["is_active"])
			found = true
			break
		}
	}
	assert.True(t, found, "Soft-deleted attachment should appear in /all")
}

func TestAttachments_Delete_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.DELETE(t, e, "/attachments/99999", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAttachments_Restore(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create
	createBody := `{
		"attachment_type": "cars",
		"name": "restore-test",
		"display_name": "Restore Test",
		"title": "Restore Title"
	}`
	createRec := testutil.POST(t, e, "/attachments", createBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	createResp := testutil.ParseMap(t, createRec)
	id := int(createResp["id"].(float64))

	// Delete
	delRec := testutil.DELETE(t, e, fmt.Sprintf("/attachments/%d", id), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, delRec.Code)

	// Verify deleted
	getRec1 := testutil.GET(t, e, fmt.Sprintf("/attachments/%d", id), testutil.AuthHeader(token))
	require.Equal(t, http.StatusNotFound, getRec1.Code)

	// Restore
	restoreRec := testutil.PUT(t, e, fmt.Sprintf("/attachments/%d/restore", id), "", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, restoreRec.Code)
	restoreMsg := testutil.ParseMessage(t, restoreRec)
	assert.Equal(t, "Вложение успешно восстановлено", restoreMsg)

	// Verify restored (GetByID should work again)
	getRec2 := testutil.GET(t, e, fmt.Sprintf("/attachments/%d", id), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, getRec2.Code)
	att := testutil.ParseMap(t, getRec2)
	assert.Equal(t, true, att["is_active"])
}

func TestAttachments_Restore_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/attachments/99999/restore", "", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAttachments_GetActive_OnlyActive(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create two attachments
	body1 := `{"attachment_type":"cars","name":"active-att","display_name":"Active","title":"A"}`
	rec1 := testutil.POST(t, e, "/attachments", body1, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec1.Code)

	body2 := `{"attachment_type":"people","name":"inactive-att","display_name":"Inactive","title":"B"}`
	rec2 := testutil.POST(t, e, "/attachments", body2, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec2.Code)
	createResp2 := testutil.ParseMap(t, rec2)
	id2 := int(createResp2["id"].(float64))

	// Soft-delete the second one
	testutil.DELETE(t, e, fmt.Sprintf("/attachments/%d", id2), testutil.AuthHeader(token))

	// GetActive should only return the active one
	activeRec := testutil.GET(t, e, "/attachments", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, activeRec.Code)
	active := testutil.ParseSlice(t, activeRec)

	for _, a := range active {
		assert.Equal(t, true, a["is_active"], "GetActive should only return active attachments")
	}

	// GetAll should return both
	allRec := testutil.GET(t, e, "/attachments/all", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, allRec.Code)
	all := testutil.ParseSlice(t, allRec)
	assert.GreaterOrEqual(t, len(all), 2)
}

func TestAttachments_Create_TitleUppercased(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	body := `{
		"attachment_type": "cars",
		"name": "title-test",
		"display_name": "Title Test",
		"title": "lowercase title"
	}`
	createRec := testutil.POST(t, e, "/attachments", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	createResp := testutil.ParseMap(t, createRec)
	id := int(createResp["id"].(float64))

	getRec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d", id), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, getRec.Code)
	att := testutil.ParseMap(t, getRec)
	assert.Equal(t, "LOWERCASE TITLE", att["title"])
}

func TestAttachments_InvalidID(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/attachments/abc", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// findHistoryByAction возвращает первую запись истории с указанным action_type (или nil).
// Поиск по action_type, а не по индексу: при близких created_at порядок одинаковых
// меток нестабилен, поэтому тесты проверяют наличие действия, а не его позицию.
func findHistoryByAction(items []map[string]interface{}, action string) map[string]interface{} {
	for _, it := range items {
		if it["action_type"] == action {
			return it
		}
	}
	return nil
}

func TestAttachments_History_Created(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	body := `{"attachment_type":"cars","name":"hist-created","display_name":"История Created","title":"T"}`
	createRec := testutil.POST(t, e, "/attachments", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	id := int(testutil.ParseMap(t, createRec)["id"].(float64))

	rec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/history", id), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	items := testutil.ParseSlice(t, rec)
	require.Len(t, items, 1)
	assert.Equal(t, "created", items[0]["action_type"])
	details, ok := items[0]["details"].(map[string]interface{})
	require.True(t, ok, "created details should be an object")
	assert.Equal(t, "История Created", details["display_name"])
}

func TestAttachments_History_UpdatedDiff(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	createBody := `{"attachment_type":"cars","name":"hist-upd","display_name":"Было","title":"A"}`
	createRec := testutil.POST(t, e, "/attachments", createBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	id := int(testutil.ParseMap(t, createRec)["id"].(float64))

	// Меняем display_name и attachment_type; title и name оставляем прежними.
	updateBody := `{"attachment_type":"people","name":"hist-upd","display_name":"Стало","title":"A"}`
	updRec := testutil.PUT(t, e, fmt.Sprintf("/attachments/%d", id), updateBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, updRec.Code)

	rec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/history", id), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	items := testutil.ParseSlice(t, rec)
	require.Len(t, items, 2)

	upd := findHistoryByAction(items, "updated")
	require.NotNil(t, upd)
	d, ok := upd["details"].(map[string]interface{})
	require.True(t, ok)

	dn, ok := d["display_name"].(map[string]interface{})
	require.True(t, ok, "changed display_name must be in diff")
	assert.Equal(t, "Было", dn["old"])
	assert.Equal(t, "Стало", dn["new"])

	at, ok := d["attachment_type"].(map[string]interface{})
	require.True(t, ok, "changed attachment_type must be in diff")
	assert.Equal(t, "cars", at["old"])
	assert.Equal(t, "people", at["new"])

	_, hasTitle := d["title"]
	assert.False(t, hasTitle, "unchanged title must not appear in diff")
	_, hasName := d["name"]
	assert.False(t, hasName, "unchanged name must not appear in diff")
}

func TestAttachments_History_NoChangeNotLogged(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	createBody := `{"attachment_type":"cars","name":"hist-nochange","display_name":"X","title":"Y"}`
	createRec := testutil.POST(t, e, "/attachments", createBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	id := int(testutil.ParseMap(t, createRec)["id"].(float64))

	// Обновляем теми же значениями - diff пустой, история не должна расти.
	updRec := testutil.PUT(t, e, fmt.Sprintf("/attachments/%d", id), createBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, updRec.Code)

	rec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/history", id), testutil.AuthHeader(token))
	items := testutil.ParseSlice(t, rec)
	require.Len(t, items, 1)
	assert.Equal(t, "created", items[0]["action_type"])
}

func TestAttachments_History_ArchiveRestore(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	createBody := `{"attachment_type":"items","name":"hist-arch","display_name":"Архивный","title":"Z"}`
	createRec := testutil.POST(t, e, "/attachments", createBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	id := int(testutil.ParseMap(t, createRec)["id"].(float64))

	delRec := testutil.DELETE(t, e, fmt.Sprintf("/attachments/%d", id), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, delRec.Code)
	restoreRec := testutil.PUT(t, e, fmt.Sprintf("/attachments/%d/restore", id), "", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, restoreRec.Code)

	rec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/history", id), testutil.AuthHeader(token))
	items := testutil.ParseSlice(t, rec)
	require.Len(t, items, 3)
	assert.NotNil(t, findHistoryByAction(items, "created"))
	require.NotNil(t, findHistoryByAction(items, "archived"))
	require.NotNil(t, findHistoryByAction(items, "restored"))
	// archived/restored без details - omitempty убирает поле из JSON.
	assert.Nil(t, findHistoryByAction(items, "archived")["details"])
	assert.Nil(t, findHistoryByAction(items, "restored")["details"])
}

func TestAttachments_CRUD_FullCycle(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// 1. Create
	createBody := `{
		"attachment_type": "items",
		"name": "full-cycle",
		"display_name": "Full Cycle Test",
		"title": "cycle",
		"instruction": "Step by step"
	}`
	createRec := testutil.POST(t, e, "/attachments", createBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	createResp := testutil.ParseMap(t, createRec)
	id := int(createResp["id"].(float64))

	// 2. Read
	getRec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d", id), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, getRec.Code)
	att := testutil.ParseMap(t, getRec)
	assert.Equal(t, "items", att["attachment_type"])
	assert.Equal(t, "full-cycle", att["name"])
	assert.Equal(t, "Step by step", att["instruction"])

	// 3. Update
	updateBody := `{
		"attachment_type": "cars",
		"name": "full-cycle-updated",
		"display_name": "Updated Full Cycle",
		"title": "updated cycle"
	}`
	updateRec := testutil.PUT(t, e, fmt.Sprintf("/attachments/%d", id), updateBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, updateRec.Code)

	// 4. Delete (soft)
	delRec := testutil.DELETE(t, e, fmt.Sprintf("/attachments/%d", id), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, delRec.Code)

	// 5. Verify deleted
	getRec2 := testutil.GET(t, e, fmt.Sprintf("/attachments/%d", id), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, getRec2.Code)

	// 6. Restore
	restoreRec := testutil.PUT(t, e, fmt.Sprintf("/attachments/%d/restore", id), "", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, restoreRec.Code)

	// 7. Verify restored
	getRec3 := testutil.GET(t, e, fmt.Sprintf("/attachments/%d", id), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, getRec3.Code)
}
