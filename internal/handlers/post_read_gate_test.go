package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Состав поста и журнал его проходов отдаются по праву на этот пост
// (table.<name>.view и table.<name>.history), сводный журнал всех постов - администратору.

func systemTableID(t *testing.T, db *gorm.DB, name string) int {
	t.Helper()
	var id int
	require.NoError(t, db.Raw("SELECT id FROM system_tables WHERE name = ?", name).Scan(&id).Error)
	require.NotZero(t, id, "пост %s", name)
	return id
}

// grantPostView выдаёт пользователю доступ к посту по его id, как охраннику этого поста.
func grantPostView(t *testing.T, db *gorm.DB, username string, tableID int) {
	t.Helper()
	var name string
	require.NoError(t, db.Raw("SELECT name FROM system_tables WHERE id = ?", tableID).Scan(&name).Error)
	require.NotEmpty(t, name, "пост %d", tableID)
	testutil.GrantTableVerb(t, getUserID(t, db, username), name, "view")
}

func TestPostReadGate_OutsiderForbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	fx := seedPostGateFixture(t, db)
	carsID := systemTableID(t, db, fx.tableName)
	peopleID := systemTableID(t, db, "post_gate_people")
	token := testutil.RegisterAndLogin(t, e, "post_read_outsider", "pass123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	paths := []string{
		fmt.Sprintf("/cars/active-for-table/%d", carsID),
		fmt.Sprintf("/cars/fact-for-table/%d", carsID),
		fmt.Sprintf("/employees/active-for-table/%d", peopleID),
		fmt.Sprintf("/cars/history/table/%d", carsID),
		fmt.Sprintf("/cars/history/table/%d/filter-options", carsID),
		fmt.Sprintf("/employees/history/table/%d", peopleID),
		fmt.Sprintf("/employees/history/table/%d/filter-options", peopleID),
		"/cars/history/all",
		fmt.Sprintf("/cars/history/filter-options?table_id=%d", carsID),
		"/employees/history/all",
		fmt.Sprintf("/employees/history/filter-options?table_id=%d", peopleID),
	}
	for _, p := range paths {
		rec := testutil.GET(t, e, p, h)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s: %s", p, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), "Р200ОС77", "%s: номер машины поста в ответе", p)
		assert.NotContains(t, rec.Body.String(), "Постовой", "%s: фамилия с поста в ответе", p)
	}
}

// Право на один пост не открывает соседний, а доступ к составу не открывает журнал.
func TestPostReadGate_ViewAndHistoryArePerPost(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	fx := seedPostGateFixture(t, db)
	carsID := systemTableID(t, db, fx.tableName)
	otherID := systemTableID(t, db, fx.otherTableName)
	token := testutil.RegisterAndLogin(t, e, "post_read_guard", "pass123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)
	guardID := getUserID(t, db, "post_read_guard")
	testutil.GrantTableVerb(t, guardID, fx.tableName, "view")

	rec := testutil.GET(t, e, fmt.Sprintf("/cars/active-for-table/%d", carsID), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Р200ОС77", "свой пост отдаёт свою машину")

	rec = testutil.GET(t, e, fmt.Sprintf("/cars/active-for-table/%d", otherID), h)
	assert.Equal(t, http.StatusForbidden, rec.Code, "соседний пост: %s", rec.Body.String())

	journal := fmt.Sprintf("/cars/history/table/%d", carsID)
	rec = testutil.GET(t, e, journal, h)
	assert.Equal(t, http.StatusForbidden, rec.Code, "журнал без права на журнал: %s", rec.Body.String())

	testutil.GrantTableVerb(t, guardID, fx.tableName, "history")
	rec = testutil.GET(t, e, journal, h)
	assert.Equal(t, http.StatusOK, rec.Code, "журнал своего поста: %s", rec.Body.String())
	rec = testutil.GET(t, e, journal+"/filter-options", h)
	assert.Equal(t, http.StatusOK, rec.Code, "фильтры журнала своего поста: %s", rec.Body.String())

	rec = testutil.GET(t, e, "/cars/history/all", h)
	assert.Equal(t, http.StatusForbidden, rec.Code, "сводный журнал не открывается правом на один пост: %s", rec.Body.String())
}
