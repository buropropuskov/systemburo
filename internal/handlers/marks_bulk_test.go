package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Групповая архивация/восстановление марок: полный успех, дедуп, частичный успех
// (несуществующий id в errors), пустой список -> 400.
func TestMarks_BulkArchiveRestore(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	// Создаём две марки через HTTP, забираем их id из списка.
	require.Equal(t, http.StatusCreated, testutil.POST(t, e, "/marks", `{"name":"BulkMarkA"}`, h).Code)
	require.Equal(t, http.StatusCreated, testutil.POST(t, e, "/marks", `{"name":"BulkMarkB"}`, h).Code)

	markID := func(name string) int {
		for _, m := range testutil.ParseSlice(t, testutil.GET(t, e, "/marks?include_archived=true", h)) {
			if m["name"] == name {
				return int(m["id"].(float64))
			}
		}
		return 0
	}
	isActive := func(name string) interface{} {
		for _, m := range testutil.ParseSlice(t, testutil.GET(t, e, "/marks?include_archived=true", h)) {
			if m["name"] == name {
				return m["is_active"]
			}
		}
		return nil
	}
	idA, idB := markID("BulkMarkA"), markID("BulkMarkB")
	require.Greater(t, idA, 0)
	require.Greater(t, idB, 0)

	t.Run("bulk archive полный успех", func(t *testing.T) {
		rec := testutil.POST(t, e, "/marks/bulk/archive", fmt.Sprintf(`{"ids":[%d,%d]}`, idA, idB), h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		res := testutil.ParseMap(t, rec)
		assert.Equal(t, float64(2), res["success_count"])
		assert.Equal(t, float64(0), res["error_count"])
		assert.Equal(t, false, isActive("BulkMarkA"))
		assert.Equal(t, false, isActive("BulkMarkB"))
	})

	t.Run("дубли id дедуплицируются", func(t *testing.T) {
		dup := testutil.ParseMap(t, testutil.POST(t, e, "/marks/bulk/restore", fmt.Sprintf(`{"ids":[%d,%d]}`, idA, idA), h))
		assert.Equal(t, float64(1), dup["success_count"])
		assert.Equal(t, true, isActive("BulkMarkA"))
	})

	t.Run("несуществующий id -> в errors (207)", func(t *testing.T) {
		rec := testutil.POST(t, e, "/marks/bulk/archive", fmt.Sprintf(`{"ids":[%d,999999]}`, idB), h)
		require.Equal(t, http.StatusMultiStatus, rec.Code)
		res := testutil.ParseMap(t, rec)
		assert.Equal(t, float64(1), res["success_count"])
		assert.Equal(t, float64(1), res["error_count"])
		errs := res["errors"].([]interface{})
		require.Len(t, errs, 1)
		assert.Equal(t, float64(999999), errs[0].(map[string]interface{})["id"])
	})

	t.Run("пустой список -> 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/marks/bulk/archive", `{"ids":[]}`, h).Code)
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/marks/bulk/restore", `{"ids":[]}`, h).Code)
	})
}
