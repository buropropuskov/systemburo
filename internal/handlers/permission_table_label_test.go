package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Лейбл права таблицы в каталоге должен использовать человеческое
// display_name таблицы ("КПП №4 тест"), а не системный slug (kpp_test).
func TestPermissions_TableLabel_UsesDisplayName(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	// Таблица, у которой slug != человеческого имени.
	body := `{"name":"kpp_test","display_name":"КПП №4 тест","table_type":"cars"}`
	rec := testutil.POST(t, e, "/system-tables", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	const wantLabel = "КПП №4 тест: Доступ к таблице"

	type catNode struct {
		Key         string `json:"key"`
		DisplayName string `json:"display_name"`
	}

	// /permissions/catalog (его читает модалка прав пользователя).
	rec = testutil.GET(t, e, "/permissions/catalog", h)
	require.Equal(t, http.StatusOK, rec.Code)
	catalog := testutil.ParseResponse[[]catNode](t, rec)
	var catalogLabel string
	for _, n := range catalog {
		if n.Key == "table.kpp_test.view" {
			catalogLabel = n.DisplayName
		}
	}
	require.NotEmpty(t, catalogLabel, "table.kpp_test.view должен быть в каталоге")
	assert.Equal(t, wantLabel, catalogLabel)
	assert.NotContains(t, catalogLabel, "kpp_test")
}
