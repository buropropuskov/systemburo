package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Права администратора над реестрами сотрудников и машин: он правит и удаляет запись,
// к кому бы она ни была привязана, а обычный пользователь чужую запись не трогает.
//
// Отдельный замок стоит на владельце записи. Update раньше подставлял в user_id того,
// кто правит, когда поля в запросе нет: пока чужие записи были недоступны, это было
// незаметно, а с приходом администратора первая же правка перевела бы сотрудника
// контрагента на бюро - и реестр расползся бы молча, без ошибки в ответе.

// registryOwner заводит обычного пользователя-владельца в отдельной организации и
// возвращает его токен и id. Организация своя у каждого, иначе совпадение по
// organization_id само даст право правки и замок на админство станет вакуумным.
func registryOwner(t *testing.T, e *echo.Echo, db *gorm.DB, username, orgName string) (http.Header, int) {
	t.Helper()
	org := models.Organization{Name: orgName}
	require.NoError(t, db.Create(&org).Error, "seed org %s", orgName)

	token := testutil.RegisterAndLogin(t, e, username, "pass123", 1, org.ID, 0)
	var user models.User
	require.NoError(t, db.Where("username = ?", username).First(&user).Error, "fetch user %s", username)
	return testutil.AuthHeader(token), user.ID
}

func TestUniqueEmployees_AdminEditsForeignRecord_OwnerKept(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	ownerHeader, ownerID := registryOwner(t, e, db, "regowner_emp", "Registry Owner Org")
	adminHeader := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	rec := testutil.POST(t, e, "/unique-employees", `{"pd_consent":true,"last_name":"Пешков","first_name":"Иван","passport_series_number":"4510 111222"}`, ownerHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	created := testutil.ParseResponse[services.UniqueEmployeeResponse](t, rec)

	rec = testutil.PUT(t, e, fmt.Sprintf("/unique-employees/%d", created.ID), `{"pd_consent":true,"last_name":"Пешков","first_name":"Иоанн"}`, adminHeader)
	require.Equal(t, http.StatusOK, rec.Code, "администратор правит сотрудника чужой организации: %s", rec.Body.String())

	var stored models.UniqueEmployee
	require.NoError(t, db.First(&stored, created.ID).Error)
	require.NotNil(t, stored.FirstName)
	assert.Equal(t, "Иоанн", *stored.FirstName, "правка применилась")
	require.NotNil(t, stored.UserID)
	assert.Equal(t, ownerID, *stored.UserID, "запись осталась за прежним владельцем, а не переехала на администратора")
}

func TestUniqueEmployees_ForeignUserCannotEditOrDelete(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	testutil.SeedTestData(t, db)

	ownerHeader, _ := registryOwner(t, e, db, "regowner_emp2", "Registry Owner Org 2")
	strangerHeader, _ := registryOwner(t, e, db, "regstranger_emp", "Registry Stranger Org")

	rec := testutil.POST(t, e, "/unique-employees", `{"pd_consent":true,"last_name":"Одинцов","first_name":"Пётр"}`, ownerHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	created := testutil.ParseResponse[services.UniqueEmployeeResponse](t, rec)

	rec = testutil.PUT(t, e, fmt.Sprintf("/unique-employees/%d", created.ID), `{"pd_consent":true,"last_name":"Одинцов","first_name":"Павел"}`, strangerHeader)
	assert.Equal(t, http.StatusForbidden, rec.Code, "посторонний пользователь не правит чужого сотрудника")

	rec = testutil.DELETE(t, e, fmt.Sprintf("/unique-employees/%d", created.ID), strangerHeader)
	assert.Equal(t, http.StatusForbidden, rec.Code, "посторонний пользователь не удаляет чужого сотрудника")
}

func TestUniqueEmployees_AdminDeletesForeignRecord(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	ownerHeader, _ := registryOwner(t, e, db, "regowner_emp3", "Registry Owner Org 3")
	adminHeader := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	rec := testutil.POST(t, e, "/unique-employees", `{"pd_consent":true,"last_name":"Кротов","first_name":"Семён"}`, ownerHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	created := testutil.ParseResponse[services.UniqueEmployeeResponse](t, rec)

	rec = testutil.DELETE(t, e, fmt.Sprintf("/unique-employees/%d", created.ID), adminHeader)
	require.Equal(t, http.StatusOK, rec.Code, "администратор удаляет сотрудника чужой организации: %s", rec.Body.String())

	var count int64
	require.NoError(t, db.Model(&models.UniqueEmployee{}).Where("id = ?", created.ID).Count(&count).Error)
	assert.Zero(t, count, "запись удалена")
}

func TestUniqueCars_AdminEditsForeignRecord_OwnerKept(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	ownerHeader, ownerID := registryOwner(t, e, db, "regowner_car", "Registry Car Owner Org")
	adminHeader := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	rec := testutil.POST(t, e, "/unique-cars", `{"number":"А111АА777","mark":"Volvo"}`, ownerHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	created := testutil.ParseResponse[services.UniqueCarResponse](t, rec)

	rec = testutil.PUT(t, e, fmt.Sprintf("/unique-cars/%d", created.ID), `{"number":"А111АА777","mark":"Scania"}`, adminHeader)
	require.Equal(t, http.StatusOK, rec.Code, "администратор правит машину чужой организации: %s", rec.Body.String())

	var stored models.UniqueCar
	require.NoError(t, db.First(&stored, created.ID).Error)
	require.NotNil(t, stored.Mark)
	assert.Equal(t, "Scania", *stored.Mark, "правка применилась")
	require.NotNil(t, stored.UserID)
	assert.Equal(t, ownerID, *stored.UserID, "машина осталась за прежним владельцем")
}

func TestUniqueCars_ForeignUserCannotEditOrDelete(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	testutil.SeedTestData(t, db)

	ownerHeader, _ := registryOwner(t, e, db, "regowner_car2", "Registry Car Owner Org 2")
	strangerHeader, _ := registryOwner(t, e, db, "regstranger_car", "Registry Car Stranger Org")

	rec := testutil.POST(t, e, "/unique-cars", `{"number":"В222ВВ777","mark":"MAN"}`, ownerHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	created := testutil.ParseResponse[services.UniqueCarResponse](t, rec)

	rec = testutil.PUT(t, e, fmt.Sprintf("/unique-cars/%d", created.ID), `{"number":"В222ВВ777","mark":"DAF"}`, strangerHeader)
	assert.Equal(t, http.StatusForbidden, rec.Code, "посторонний пользователь не правит чужую машину")

	rec = testutil.DELETE(t, e, fmt.Sprintf("/unique-cars/%d", created.ID), strangerHeader)
	assert.Equal(t, http.StatusForbidden, rec.Code, "посторонний пользователь не удаляет чужую машину")
}

func TestUniqueCars_AdminDeletesForeignRecord(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	ownerHeader, _ := registryOwner(t, e, db, "regowner_car3", "Registry Car Owner Org 3")
	adminHeader := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	rec := testutil.POST(t, e, "/unique-cars", `{"number":"С333СС777","mark":"Iveco"}`, ownerHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	created := testutil.ParseResponse[services.UniqueCarResponse](t, rec)

	rec = testutil.DELETE(t, e, fmt.Sprintf("/unique-cars/%d", created.ID), adminHeader)
	require.Equal(t, http.StatusOK, rec.Code, "администратор удаляет машину чужой организации: %s", rec.Body.String())

	var count int64
	require.NoError(t, db.Model(&models.UniqueCar{}).Where("id = ?", created.ID).Count(&count).Error)
	assert.Zero(t, count, "запись удалена")
}

// Привязку записи к учётной записи видит только администратор: соседям по организации
// знать, чья это запись, незачем. В значении едет ФИО владельца - логин человеку ничего
// не говорит; у владельца без заполненного ФИО остаётся логин с собачкой, как и всюду в
// интерфейсе.
func TestUniqueRegistry_OwnerNameVisibleToAdminOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	ownerHeader, _ := registryOwner(t, e, db, "regowner_name", "Registry Name Org")
	adminHeader := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees", `{"pd_consent":true,"last_name":"Тихонов","first_name":"Лев"}`, ownerHeader).Code)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-cars", `{"number":"Е555ЕЕ777","mark":"Ford"}`, ownerHeader).Code)

	rec := testutil.GET(t, e, "/unique-employees?filter_type=all_system&per_page=50", adminHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	adminEmployees := testutil.ParseResponse[[]services.UniqueEmployeeWithRelations](t, rec)
	require.NotEmpty(t, adminEmployees)
	require.NotNil(t, adminEmployees[0].UserName, "администратор видит, за кем закреплена запись")
	assert.Equal(t, "@regowner_name", *adminEmployees[0].UserName,
		"ФИО у владельца не заполнено - остаётся логин с собачкой")

	rec = testutil.GET(t, e, "/unique-employees?filter_type=user&per_page=50", ownerHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	ownEmployees := testutil.ParseResponse[[]services.UniqueEmployeeWithRelations](t, rec)
	require.Len(t, ownEmployees, 1)
	assert.Nil(t, ownEmployees[0].UserName, "обычному пользователю логин владельца не отдаётся")

	rec = testutil.GET(t, e, "/unique-cars?filter_type=all_system&per_page=50", adminHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	adminCars := testutil.ParseResponse[[]services.UniqueCarWithRelations](t, rec)
	require.NotEmpty(t, adminCars)
	require.NotNil(t, adminCars[0].UserName, "администратор видит владельца машины")

	rec = testutil.GET(t, e, "/unique-cars?filter_type=user&per_page=50", ownerHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	ownCars := testutil.ParseResponse[[]services.UniqueCarWithRelations](t, rec)
	require.Len(t, ownCars, 1)
	assert.Nil(t, ownCars[0].UserName, "обычному пользователю логин владельца машины не отдаётся")
}

// can_manage_all уходит на фронт тем же ответом, по которому вью решает, рисовать ли
// кнопки правки на вкладке «Все в системе». Признак обязан совпадать с серверным
// гейтом, иначе кнопка есть, а ответ 403.
func TestUniqueRegistry_OwnershipInfoCarriesCanManageAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminHeader := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	plainHeader, _ := registryOwner(t, e, db, "regplain_own", "Registry Plain Org")

	for _, path := range []string{"/unique-employees/ownership-info", "/unique-cars/ownership-info"} {
		rec := testutil.GET(t, e, path, adminHeader)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		info := testutil.ParseMap(t, rec)
		assert.Equal(t, true, info["can_manage_all"], "%s: администратор управляет любой записью", path)

		rec = testutil.GET(t, e, path, plainHeader)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		info = testutil.ParseMap(t, rec)
		assert.Equal(t, false, info["can_manage_all"], "%s: обычный пользователь - только свои записи", path)
	}
}

// Значение «за кем закреплена запись» собирается по тем же правилам, что имена во всём
// интерфейсе: заполненное ФИО показывается как есть, а у работника, не давшего согласия
// на обработку своих данных, вместо ФИО стоит его логин с собачкой (#1567).
func TestUniqueRegistry_OwnerNameShowsFullNameAndRespectsConsentMask(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminHeader := testutil.AuthHeader(adminToken)
	ownerHeader, ownerID := registryOwner(t, e, db, "regowner_fio", "Registry FIO Org")
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", ownerID).
		Updates(map[string]interface{}{"last_name": "Пешков", "first_name": "Иван", "middle_name": "Сергеевич"}).Error)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees", `{"pd_consent":true,"last_name":"Тихонов","first_name":"Лев"}`, ownerHeader).Code)

	ownerName := func() string {
		rec := testutil.GET(t, e, "/unique-employees?filter_type=all_system&per_page=50", adminHeader)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		rows := testutil.ParseResponse[[]services.UniqueEmployeeWithRelations](t, rec)
		require.NotEmpty(t, rows)
		require.NotNil(t, rows[0].UserName)
		return *rows[0].UserName
	}

	assert.Equal(t, "Пешков Иван Сергеевич", ownerName(), "показываем ФИО владельца, а не логин")

	// Запрос согласия включён, владелец его не давал - ФИО скрыто общей маской.
	enableConsent(t, e, adminToken, "<p>текст согласия</p>")
	assert.Equal(t, "@regowner_fio", ownerName(), "без согласия владельца вместо ФИО стоит логин")
}
