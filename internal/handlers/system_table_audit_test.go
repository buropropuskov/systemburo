package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSystemTables_WriteFlip_ColumnsUpdatedToAuditLog проверяет что оба endpoint-а
// обновления колонок (main и fact) пишут action='columns_updated' в audit_log (#870).
// UpdateFields -> PUT /system-tables/:id/fields (variant=main).
// UpdateFactFields -> PUT /system-tables/:id/fact-fields (variant=fact).
func TestSystemTables_WriteFlip_ColumnsUpdatedToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	t.Run("main fields", func(t *testing.T) {
		rec := testutil.POST(t, e, "/system-tables",
			`{"name":"colupd_main","display_name":"ColUpdMain","table_type":"cars"}`, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

		body := `{"fields":[{"field_name":"status","is_visible":false},{"field_name":"car_number","is_visible":true}]}`
		rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d/fields", tableID), body, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var n int64
		require.NoError(t, db.Model(&models.AuditLog{}).
			Where("entity_type = ? AND entity_id = ? AND action = ?",
				models.AuditEntitySystemTable, tableID, models.SystemTableActionColumnsUpdated).
			Count(&n).Error)
		assert.GreaterOrEqual(t, n, int64(1), "PUT /fields должен писать columns_updated в audit_log")
	})

	t.Run("fact fields", func(t *testing.T) {
		// show_fact_table:true создаёт fact-поля по умолчанию (как в TestSystemTables_UpdateFactFields_PersistsVisibility).
		rec := testutil.POST(t, e, "/system-tables",
			`{"name":"colupd_fact","display_name":"ColUpdFact","table_type":"cars","show_fact_table":true}`, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

		body := `{"fields":[{"field_name":"organization","is_visible":false},{"field_name":"car_number","is_visible":true}]}`
		rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d/fact-fields", tableID), body, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var n int64
		require.NoError(t, db.Model(&models.AuditLog{}).
			Where("entity_type = ? AND entity_id = ? AND action = ?",
				models.AuditEntitySystemTable, tableID, models.SystemTableActionColumnsUpdated).
			Count(&n).Error)
		assert.GreaterOrEqual(t, n, int64(1), "PUT /fact-fields должен писать columns_updated в audit_log")
	})
}
