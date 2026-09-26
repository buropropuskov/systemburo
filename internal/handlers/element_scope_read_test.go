package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// История отдельной машины и сотрудника, их статус на территории и места разгрузки
// сервер отдаёт только в скоупе пользователя (services.ElementScope): пост, заявка,
// своя организация или компания. Раньше любой вошедший читал их по всей системе,
// в том числе историю человека по ФИО.

type elementScopeFixture struct {
	postGateFixture
	appID, orgID int
}

// seedElementScopeFixture достраивает пост из seedPostGateFixture до того, что читают
// проверяемые роуты: марка и имя для объединённой истории, по отметке в журнале машины
// и сотрудника, место разгрузки у машины.
func seedElementScopeFixture(t *testing.T, db *gorm.DB) elementScopeFixture {
	t.Helper()
	fx := seedPostGateFixture(t, db)
	var app models.Application
	require.NoError(t, db.Where("application_number = ?", "POST-GATE-1").First(&app).Error)

	require.NoError(t, db.Exec("UPDATE cars SET car_brand = 'Газель' WHERE id IN (?, ?)", fx.carID, fx.unboundCarID).Error)
	require.NoError(t, db.Exec("UPDATE employees SET first_name = 'Пётр' WHERE id = ?", fx.empID).Error)
	for _, row := range []struct {
		entity string
		id     int
	}{{models.AuditEntityCar, fx.carID}, {models.AuditEntityEmployee, fx.empID}} {
		require.NoError(t, db.Exec(`INSERT INTO audit_log (entity_type, entity_id, action, details, created_at)
			VALUES (?, ?, 'entry', '{"comment": "отметка скоупа"}', NOW())`, row.entity, row.id).Error)
	}
	place := models.UnloadPlace{Name: "Склад скоупа", IsActive: true, Status: "active"}
	require.NoError(t, db.Create(&place).Error)
	require.NoError(t, db.Exec("INSERT INTO car_unload_places (car_id, unload_place_id, order_index) VALUES (?, ?, 1)",
		fx.carID, place.ID).Error)
	return elementScopeFixture{postGateFixture: fx, appID: app.ID, orgID: app.OrganizationID}
}

// elementReads - что пользователь получил по элементам фикстуры с каждого роута.
type elementReads struct {
	carHistory, empHistory         int
	carUnified, empUnified         int
	carStatus, empStatus, carPlace bool
}

func readElements(t *testing.T, e *echo.Echo, h http.Header, fx elementScopeFixture) elementReads {
	t.Helper()
	var r elementReads
	r.carHistory = testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", fx.carID), h).Code
	r.empHistory = testutil.GET(t, e, fmt.Sprintf("/employees/%d/history", fx.empID), h).Code

	r.carUnified = scopeRowCount(t, e, h, "/cars/history/unified?"+url.Values{"car_number": {"Р200ОС77"}, "car_brand": {"Газель"}}.Encode())
	r.empUnified = scopeRowCount(t, e, h, "/employees/history/unified?"+url.Values{"last_name": {"Постовой"}, "first_name": {"Пётр"}}.Encode())

	r.carStatus = scopeHasRow(t, e, h, "/cars/history/current-status", "car_id", fx.carID)
	r.empStatus = scopeHasRow(t, e, h, "/employees/history/current-status", "employee_id", fx.empID)
	r.carPlace = scopeHasRow(t, e, h, "/cars/unload-places", "car_id", fx.carID)
	return r
}

func scopeRowCount(t *testing.T, e *echo.Echo, h http.Header, path string) int {
	t.Helper()
	rec := testutil.GET(t, e, path, h)
	require.Equal(t, http.StatusOK, rec.Code, "%s: %s", path, rec.Body.String())
	return len(testutil.ParseSlice(t, rec))
}

func scopeHasRow(t *testing.T, e *echo.Echo, h http.Header, path, field string, id int) bool {
	t.Helper()
	rec := testutil.GET(t, e, path, h)
	require.Equal(t, http.StatusOK, rec.Code, "%s: %s", path, rec.Body.String())
	for _, row := range testutil.ParseSlice(t, rec) {
		if v, ok := row[field].(float64); ok && int(v) == id {
			return true
		}
	}
	return false
}

func TestElementScope_Outsider(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	fx := seedElementScopeFixture(t, db)
	h := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "scope_outsider", "pass123", 1, td.OrgID, td.CompanyID))

	got := readElements(t, e, h, fx)
	assert.Equal(t, elementReads{carHistory: http.StatusForbidden, empHistory: http.StatusForbidden}, got,
		"пользователь чужой организации без прав не видит ни истории, ни статусов, ни мест разгрузки")

	rec := testutil.GET(t, e, "/cars/999999/history", h)
	assert.Equal(t, http.StatusForbidden, rec.Code, "несуществующий id отвечает так же, как чужой")
}

// Охранник поста видит машину своего поста, но не сотрудника соседнего поста.
func TestElementScope_PostGuard(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	fx := seedElementScopeFixture(t, db)
	h := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "scope_guard", "pass123", 1, td.OrgID, td.CompanyID))
	testutil.GrantTableVerb(t, getUserID(t, db, "scope_guard"), fx.tableName, "view")

	got := readElements(t, e, h, fx)
	assert.Equal(t, elementReads{
		carHistory: http.StatusOK, empHistory: http.StatusForbidden,
		carUnified: 1, carStatus: true, carPlace: true,
	}, got)

	rec := testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", fx.unboundCarID), h)
	assert.Equal(t, http.StatusForbidden, rec.Code, "машина той же заявки без привязки к посту охраннику не видна")
}

// Доступ к заявке (здесь - читатель пересланной заявки) открывает её машины и людей.
func TestElementScope_ApplicationViewer(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	fx := seedElementScopeFixture(t, db)
	h := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "scope_reader", "pass123", 1, td.OrgID, td.CompanyID))
	require.NoError(t, db.Create(&models.ApplicationViewer{ApplicationID: fx.appID, UserID: getUserID(t, db, "scope_reader")}).Error)

	got := readElements(t, e, h, fx)
	assert.Equal(t, elementReads{
		carHistory: http.StatusOK, empHistory: http.StatusOK,
		carUnified: 1, empUnified: 1, carStatus: true, empStatus: true, carPlace: true,
	}, got)
}

// Своя организация видит свои элементы, как видит их в реестре.
func TestElementScope_OwnOrganization(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	fx := seedElementScopeFixture(t, db)
	h := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "scope_colleague", "pass123", 1, fx.orgID, td.CompanyID))

	got := readElements(t, e, h, fx)
	assert.Equal(t, elementReads{
		carHistory: http.StatusOK, empHistory: http.StatusOK,
		carUnified: 1, empUnified: 1, carStatus: true, empStatus: true, carPlace: true,
	}, got)
}

// Раздел чёрного списка ищет историю по всей системе, но только объединённую:
// историю по id и статусы право на ЧС не открывает.
func TestElementScope_BlacklistManager(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	fx := seedElementScopeFixture(t, db)
	h := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "scope_blacklist", "pass123", 1, td.OrgID, td.CompanyID))
	testutil.GrantPermission(t, getUserID(t, db, "scope_blacklist"), services.KeyPageBlacklist)

	got := readElements(t, e, h, fx)
	assert.Equal(t, elementReads{
		carHistory: http.StatusForbidden, empHistory: http.StatusForbidden,
		carUnified: 1, empUnified: 1,
	}, got)
}

func TestElementScope_Admin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	fx := seedElementScopeFixture(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	got := readElements(t, e, h, fx)
	assert.Equal(t, elementReads{
		carHistory: http.StatusOK, empHistory: http.StatusOK,
		carUnified: 1, empUnified: 1, carStatus: true, empStatus: true, carPlace: true,
	}, got)
}

// Принимающий видит все заявки, а с ними и их машины и людей, без привязки к посту.
func TestElementScope_Approver(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	fx := seedElementScopeFixture(t, db)
	h := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "scope_acceptor", "pass123", 1, td.OrgID, td.CompanyID))
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: getUserID(t, db, "scope_acceptor")}).Error)

	got := readElements(t, e, h, fx)
	assert.Equal(t, elementReads{
		carHistory: http.StatusOK, empHistory: http.StatusOK,
		carUnified: 1, empUnified: 1, carStatus: true, empStatus: true, carPlace: true,
	}, got)
}

// Заблокированный не видит ничего, даже элементы своей организации и своего поста.
func TestElementScope_BannedSeesNothing(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	fx := seedElementScopeFixture(t, db)
	userID := seedDatesUser(t, db, "scope_banned", fx.orgID)
	testutil.GrantTableVerb(t, userID, fx.tableName, "view")

	scopes := services.NewElementScopeResolver(db, services.NewPermissionResolver(db))
	scope, err := scopes.Resolve(context.Background(), userID)
	require.NoError(t, err)
	visible, err := scopes.Visible(context.Background(), scope, services.ElementCar, fx.carID)
	require.NoError(t, err)
	require.True(t, visible, "до блокировки машина своего поста и своей организации видна")

	require.NoError(t, db.Exec("UPDATE users SET is_banned = TRUE WHERE id = ?", userID).Error)
	scope, err = services.NewElementScopeResolver(db, services.NewPermissionResolver(db)).Resolve(context.Background(), userID)
	require.NoError(t, err)
	visible, err = scopes.Visible(context.Background(), scope, services.ElementCar, fx.carID)
	require.NoError(t, err)
	assert.False(t, visible, "после блокировки не видна")
}
