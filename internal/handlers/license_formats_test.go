package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseFormats_GetAll_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/license-plate-formats", h)
	assert.Equal(t, http.StatusOK, rec.Code)

	list := testutil.ParseSlice(t, rec)
	assert.Empty(t, list)
}

func TestLicenseFormats_GetAll_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/license-plate-formats", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestLicenseFormats_CRUD_Cycle(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// --- Create ---
	body := `{
		"name": "Российский формат",
		"country_code": "RU",
		"icon": "🇷🇺",
		"is_default": true,
		"cells": [
			{
				"cell_order": 1,
				"cell_type": "letter",
				"min_length": 1,
				"max_length": 1,
				"allowed_letters": "АВЕКМНОРСТУХ",
				"alphabet_type": "cyrillic",
				"language": "ru"
			},
			{
				"cell_order": 2,
				"cell_type": "digit",
				"min_length": 3,
				"max_length": 3
			}
		]
	}`
	rec := testutil.POST(t, e, "/license-plate-formats", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	createResp := testutil.ParseMap(t, rec)
	assert.Equal(t, "Формат номеров успешно создан", createResp["message"])
	formatID := int(createResp["id"].(float64))
	assert.Greater(t, formatID, 0)

	// --- Read (verify created) ---
	rec = testutil.GET(t, e, "/license-plate-formats", h)
	require.Equal(t, http.StatusOK, rec.Code)

	list := testutil.ParseSlice(t, rec)
	require.Len(t, list, 1)

	entry := list[0]
	format := entry["format"].(map[string]interface{})
	cells := entry["cells"].([]interface{})

	assert.Equal(t, "Российский формат", format["name"])
	assert.Equal(t, "RU", format["country_code"])
	assert.Equal(t, "🇷🇺", format["icon"])
	assert.Equal(t, true, format["is_default"])
	assert.Len(t, cells, 2)

	// Verify cell details
	cell1 := cells[0].(map[string]interface{})
	assert.Equal(t, float64(1), cell1["cell_order"])
	assert.Equal(t, "letter", cell1["cell_type"])
	assert.Equal(t, "АВЕКМНОРСТУХ", cell1["allowed_letters"])

	cell2 := cells[1].(map[string]interface{})
	assert.Equal(t, float64(2), cell2["cell_order"])
	assert.Equal(t, "digit", cell2["cell_type"])

	// --- Update ---
	updateBody := fmt.Sprintf(`{
		"name": "Обновлённый формат",
		"country_code": "KZ",
		"icon": "🇰🇿",
		"is_default": false,
		"cells": [
			{
				"cell_order": 1,
				"cell_type": "digit",
				"min_length": 3,
				"max_length": 4
			}
		]
	}`)
	rec = testutil.PUT(t, e, fmt.Sprintf("/license-plate-formats/%d", formatID), updateBody, h)
	require.Equal(t, http.StatusOK, rec.Code)

	updateMsg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Формат номеров успешно обновлен", updateMsg)

	// --- Read (verify updated) ---
	rec = testutil.GET(t, e, "/license-plate-formats", h)
	require.Equal(t, http.StatusOK, rec.Code)
	list = testutil.ParseSlice(t, rec)
	require.Len(t, list, 1)

	format = list[0]["format"].(map[string]interface{})
	cells = list[0]["cells"].([]interface{})

	assert.Equal(t, "Обновлённый формат", format["name"])
	assert.Equal(t, "KZ", format["country_code"])
	assert.Equal(t, false, format["is_default"])
	assert.Len(t, cells, 1, "update should replace cells")

	// --- Delete ---
	rec = testutil.DELETE(t, e, fmt.Sprintf("/license-plate-formats/%d", formatID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	deleteMsg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Формат номеров успешно удален", deleteMsg)

	// --- Read (verify gone) ---
	rec = testutil.GET(t, e, "/license-plate-formats", h)
	require.Equal(t, http.StatusOK, rec.Code)
	list = testutil.ParseSlice(t, rec)
	assert.Empty(t, list)
}

func TestLicenseFormats_Delete_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.DELETE(t, e, "/license-plate-formats/99999", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestLicenseFormats_Update_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.PUT(t, e, "/license-plate-formats/99999",
		`{"name":"Ghost","cells":[]}`, h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestLicenseFormats_Create_DefaultClears_Previous(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create first default format
	body1 := `{"name":"Format A","country_code":"AA","is_default":true,"cells":[]}`
	rec := testutil.POST(t, e, "/license-plate-formats", body1, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Create second default format
	body2 := `{"name":"Format B","country_code":"BB","is_default":true,"cells":[]}`
	rec = testutil.POST(t, e, "/license-plate-formats", body2, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify only the second is default
	rec = testutil.GET(t, e, "/license-plate-formats", h)
	require.Equal(t, http.StatusOK, rec.Code)

	list := testutil.ParseSlice(t, rec)
	require.Len(t, list, 2)

	defaultCount := 0
	for _, entry := range list {
		format := entry["format"].(map[string]interface{})
		if format["is_default"] == true {
			defaultCount++
			assert.Equal(t, "Format B", format["name"])
		}
	}
	assert.Equal(t, 1, defaultCount)
}

func TestLicenseFormats_Create_WithCellDefaults(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create format with cell that omits padding_char and padding_side (should default)
	body := `{
		"name": "Default Padding Test",
		"cells": [
			{"cell_order": 1, "cell_type": "digit", "min_length": 3, "max_length": 3}
		]
	}`
	rec := testutil.POST(t, e, "/license-plate-formats", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Read and verify defaults
	rec = testutil.GET(t, e, "/license-plate-formats", h)
	require.Equal(t, http.StatusOK, rec.Code)

	list := testutil.ParseSlice(t, rec)
	require.Len(t, list, 1)

	cells := list[0]["cells"].([]interface{})
	require.Len(t, cells, 1)

	cell := cells[0].(map[string]interface{})
	assert.Equal(t, "0", cell["padding_char"])
	assert.Equal(t, "left", cell["padding_side"])
}
