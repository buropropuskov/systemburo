package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Привязка записей реестра машин и сотрудников к организации и компании. Привязка
// даёт доступ: canEditCar/canEditEmployee пускают всех из организации записи.
// Поэтому чужая организация в теле - только с правом подачи от другой организации
// (как у заявки, #1437) или администратору, а владелец (user_id) из тела не читается
// вовсе: иначе запись подбрасывалась в чужой реестр или уводилась туда правкой.

func TestUniqueCars_ForeignBinding_RequiresOverride(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	header, userID := registryOwner(t, e, db, "bind_car_user", "Bind Own Org")
	var user models.User
	require.NoError(t, db.First(&user, userID).Error)
	foreignOrg := models.Organization{Name: "Bind Foreign Org"}
	require.NoError(t, db.Create(&foreignOrg).Error)
	foreignCompany := models.Company{Name: "Bind Foreign Company"}
	require.NoError(t, db.Create(&foreignCompany).Error)

	rec := testutil.POST(t, e, "/unique-cars",
		fmt.Sprintf(`{"number":"BIND001","mark":"Kia","organization_id":%d}`, foreignOrg.ID), header)
	assert.Equal(t, http.StatusForbidden, rec.Code, "чужая организация без права: %s", rec.Body.String())

	rec = testutil.POST(t, e, "/unique-cars",
		fmt.Sprintf(`{"number":"BIND002","mark":"Kia","company_id":%d}`, foreignCompany.ID), header)
	assert.Equal(t, http.StatusForbidden, rec.Code, "чужая компания без права: %s", rec.Body.String())

	var count int64
	require.NoError(t, db.Model(&models.UniqueCar{}).Where("number IN ?", []string{"BIND001", "BIND002"}).Count(&count).Error)
	assert.Zero(t, count, "отказ не должен оставлять запись")

	rec = testutil.POST(t, e, "/unique-cars",
		fmt.Sprintf(`{"number":"BIND003","mark":"Kia","organization_id":%d}`, *user.OrganizationID), header)
	require.Equal(t, http.StatusOK, rec.Code, "своя организация - всегда: %s", rec.Body.String())

	testutil.GrantPermission(t, userID, services.KeyApplicationOrganizationOverride)
	rec = testutil.POST(t, e, "/unique-cars",
		fmt.Sprintf(`{"number":"BIND004","mark":"Kia","organization_id":%d,"company_id":%d}`, foreignOrg.ID, foreignCompany.ID), header)
	require.Equal(t, http.StatusOK, rec.Code, "с правом подачи от другой организации - можно: %s", rec.Body.String())
}

func TestUniqueCars_AdminBindsForeignOrg(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminHeader := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	foreignOrg := models.Organization{Name: "Admin Foreign Org"}
	require.NoError(t, db.Create(&foreignOrg).Error)

	rec := testutil.POST(t, e, "/unique-cars",
		fmt.Sprintf(`{"number":"ADMB001","mark":"Kia","organization_id":%d}`, foreignOrg.ID), adminHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestUniqueCars_UpdateBinding(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminHeader := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	header, userID := registryOwner(t, e, db, "bind_upd_user", "Bind Upd Org")
	var user models.User
	require.NoError(t, db.First(&user, userID).Error)
	ownOrg := *user.OrganizationID
	foreignOrg := models.Organization{Name: "Bind Upd Foreign Org"}
	require.NoError(t, db.Create(&foreignOrg).Error)

	// Машину организации пользователя, но с чужой ему компанией, заводит администратор:
	// пользователь правит её по совпадению организации.
	rec := testutil.POST(t, e, "/unique-cars",
		fmt.Sprintf(`{"number":"UPD001","mark":"Kia","organization_id":%d,"company_id":%d}`, ownOrg, td.CompanyID), adminHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	car := testutil.ParseResponse[services.UniqueCarResponse](t, rec)
	url := fmt.Sprintf("/unique-cars/%d", car.ID)

	rec = testutil.PUT(t, e, url,
		fmt.Sprintf(`{"number":"UPD001","mark":"Lada","organization_id":%d,"company_id":%d}`, ownOrg, td.CompanyID), header)
	require.Equal(t, http.StatusOK, rec.Code, "привязку, которая уже стоит в записи, оставить можно: %s", rec.Body.String())

	rec = testutil.PUT(t, e, url,
		fmt.Sprintf(`{"number":"UPD001","mark":"Lada","organization_id":%d,"company_id":%d}`, foreignOrg.ID, td.CompanyID), header)
	assert.Equal(t, http.StatusForbidden, rec.Code, "увести запись в чужую организацию нельзя: %s", rec.Body.String())

	var stored models.UniqueCar
	require.NoError(t, db.First(&stored, car.ID).Error)
	require.NotNil(t, stored.OrganizationID)
	assert.Equal(t, ownOrg, *stored.OrganizationID, "после отказа привязка прежняя")
	require.NotNil(t, stored.Mark)
	assert.Equal(t, "Lada", *stored.Mark, "разрешённая правка применилась")
}

func TestUniqueCars_UserIDFromBodyIgnored(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	header, userID := registryOwner(t, e, db, "bind_uid_user", "Bind Uid Org")
	_, victimID := registryOwner(t, e, db, "bind_uid_victim", "Bind Uid Victim Org")

	rec := testutil.POST(t, e, "/unique-cars",
		fmt.Sprintf(`{"number":"UID001","mark":"Kia","user_id":%d}`, victimID), header)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	car := testutil.ParseResponse[services.UniqueCarResponse](t, rec)

	var stored models.UniqueCar
	require.NoError(t, db.First(&stored, car.ID).Error)
	require.NotNil(t, stored.UserID)
	assert.Equal(t, userID, *stored.UserID, "владелец при создании - из токена")

	rec = testutil.PUT(t, e, fmt.Sprintf("/unique-cars/%d", car.ID),
		fmt.Sprintf(`{"number":"UID001","mark":"Kia","user_id":%d}`, victimID), header)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, db.First(&stored, car.ID).Error)
	assert.Equal(t, userID, *stored.UserID, "правка владельца не меняет")
}

func TestUniqueCars_DeadRoutesRemoved(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	rec := testutil.POST(t, e, "/unique-cars/batch", `[{"number":"DEAD01","mark":"Kia"}]`, h)
	assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, rec.Code, rec.Body.String())

	var count int64
	require.NoError(t, db.Model(&models.UniqueCar{}).Where("number = ?", "DEAD01").Count(&count).Error)
	assert.Zero(t, count)
}

func TestUniqueEmployees_Binding(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	header, userID := registryOwner(t, e, db, "bind_emp_user", "Bind Emp Org")
	_, victimID := registryOwner(t, e, db, "bind_emp_victim", "Bind Emp Victim Org")
	foreignOrg := models.Organization{Name: "Bind Emp Foreign Org"}
	require.NoError(t, db.Create(&foreignOrg).Error)

	rec := testutil.POST(t, e, "/unique-employees",
		fmt.Sprintf(`{"pd_consent":true,"last_name":"Лесков","first_name":"Николай","organization_id":%d}`, foreignOrg.ID), header)
	assert.Equal(t, http.StatusForbidden, rec.Code, "чужая организация без права: %s", rec.Body.String())

	rec = testutil.POST(t, e, "/unique-employees",
		fmt.Sprintf(`{"pd_consent":true,"last_name":"Лесков","first_name":"Николай","user_id":%d}`, victimID), header)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	emp := testutil.ParseResponse[services.UniqueEmployeeResponse](t, rec)
	var stored models.UniqueEmployee
	require.NoError(t, db.First(&stored, emp.ID).Error)
	require.NotNil(t, stored.UserID)
	assert.Equal(t, userID, *stored.UserID, "владелец при создании - из токена")

	rec = testutil.PUT(t, e, fmt.Sprintf("/unique-employees/%d", emp.ID),
		fmt.Sprintf(`{"pd_consent":true,"last_name":"Лесков","organization_id":%d,"user_id":%d}`, foreignOrg.ID, victimID), header)
	assert.Equal(t, http.StatusForbidden, rec.Code, "увести сотрудника в чужую организацию нельзя: %s", rec.Body.String())

	rec = testutil.PUT(t, e, fmt.Sprintf("/unique-employees/%d", emp.ID),
		fmt.Sprintf(`{"pd_consent":true,"last_name":"Лесков","user_id":%d}`, victimID), header)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, db.First(&stored, emp.ID).Error)
	assert.Equal(t, userID, *stored.UserID, "правка владельца не меняет")

	testutil.GrantPermission(t, userID, services.KeyApplicationOrganizationOverride)
	rec = testutil.POST(t, e, "/unique-employees",
		fmt.Sprintf(`{"pd_consent":true,"last_name":"Гаршин","first_name":"Всеволод","organization_id":%d}`, foreignOrg.ID), header)
	require.Equal(t, http.StatusOK, rec.Code, "с правом подачи от другой организации - можно: %s", rec.Body.String())
}
