package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// appOrgRefs - организация и компания заявки как они лежат в БД (nullable, поэтому
// указатели). Поля перечислены плоско: gorm молча не маппит анонимно встроенные структуры.
type appOrgRefs struct {
	OrganizationID *int
	CompanyID      *int
}

func readAppOrgRefs(t *testing.T, db *gorm.DB, appID int) appOrgRefs {
	t.Helper()
	var refs appOrgRefs
	require.NoError(t, db.Raw("SELECT organization_id, company_id FROM applications WHERE id = ?", appID).Scan(&refs).Error)
	return refs
}

// submitWithRefs подаёт заявку с произвольным набором полей организации и компании:
// refsJSON - фрагмент тела вида `"organization_id":7` или `"organization":"ООО ..."`.
func submitWithRefs(t *testing.T, e *echo.Echo, token string, uaID int, plate, refsJSON string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{
		"message": "org resolve test",
		%s,
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "orgres_cars",
			"attachment_display_name": "Org Resolve Cars",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {"vehicles": [{"car_number": "%s", "car_brand": "Toyota"}]}
		}]
	}`, refsJSON, uaID, plate)
	return testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
}

func submittedAppID(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())
	return testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID
}

func countOrganizations(t *testing.T, db *gorm.DB, key string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.Organization{}).Where("name_normalized = ?", key).Count(&n).Error)
	return n
}

// TestApplicationOrgResolve покрывает резолв организации и компании при подаче (#1437):
// дедупликацию написаний, создание записи «на проверке» и снятие фолбэка, из-за которого
// незнакомое наименование давало заявку с organization_id = NULL.
// Секции живут на одном SetupTestApp: пакет handlers идёт в CI под -race у самой границы
// go test -timeout, и отдельные тесты со своими CleanDB и Seed её уже перебивали.
func TestApplicationOrgResolve(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "orgres_cars", "Org Resolve Cars")
	token := testutil.RegisterAndLogin(t, e, "orgresolve", "pass123", 1, td.OrgID, td.CompanyID)

	var sender models.User
	require.NoError(t, db.Where("username = ?", "orgresolve").First(&sender).Error)

	orgType := models.OrgTypeContractor
	existing := models.Organization{Name: `ООО "Петрушка"`, Type: &orgType, IsActive: true}
	require.NoError(t, db.Create(&existing).Error)

	// Ровно тот баг из issue: «ооо петрушка» точному сравнению по name не отвечало,
	// заявка уходила без организации и переставала быть видимой коллегам.
	t.Run("другое написание берёт существующую организацию", func(t *testing.T) {
		appID := submittedAppID(t, submitWithRefs(t, e, token, uaID, "A001AA777", `"organization":"ооо петрушка"`))

		refs := readAppOrgRefs(t, db, appID)
		require.NotNil(t, refs.OrganizationID, "организация заявки не должна быть NULL")
		assert.Equal(t, existing.ID, *refs.OrganizationID)
		assert.Equal(t, int64(1), countOrganizations(t, db, "ооо петрушка"), "вторая запись не нужна")
	})

	t.Run("незнакомое наименование заводит запись на проверке", func(t *testing.T) {
		appID := submittedAppID(t, submitWithRefs(t, e, token, uaID, "A002AA777", `"organization_name":"ООО \"Ромашка-Строй\""`))

		refs := readAppOrgRefs(t, db, appID)
		require.NotNil(t, refs.OrganizationID)

		var created models.Organization
		require.NoError(t, db.First(&created, *refs.OrganizationID).Error)
		assert.Equal(t, `ООО "Ромашка-Строй"`, created.Name)
		assert.Equal(t, "ооо ромашка-строй", created.NameNormalized)
		assert.Equal(t, models.ModerationPending, created.ModerationStatus)
		require.NotNil(t, created.Type)
		assert.Equal(t, models.OrgTypeContractor, *created.Type)
		require.NotNil(t, created.CreatedByUserID)
		assert.Equal(t, sender.ID, *created.CreatedByUserID)
		assert.True(t, created.IsActive)
	})

	// Черновая запись участвует в дедупликации так же, как проверенная: иначе каждая
	// следующая заявка на того же подрядчика плодила бы ещё один pending.
	t.Run("повторная подача того же наименования не плодит запись", func(t *testing.T) {
		appID := submittedAppID(t, submitWithRefs(t, e, token, uaID, "A003AA777", `"organization_name":"ооо ромашка - строй"`))

		refs := readAppOrgRefs(t, db, appID)
		require.NotNil(t, refs.OrganizationID)
		assert.Equal(t, int64(1), countOrganizations(t, db, "ооо ромашка-строй"))
	})

	t.Run("organization_id из запроса используется, наименование не смотрится", func(t *testing.T) {
		appID := submittedAppID(t, submitWithRefs(t, e, token, uaID, "A004AA777",
			fmt.Sprintf(`"organization_id":%d,"organization":"ООО \"Мусор\""`, existing.ID)))

		refs := readAppOrgRefs(t, db, appID)
		require.NotNil(t, refs.OrganizationID)
		assert.Equal(t, existing.ID, *refs.OrganizationID)
		assert.Equal(t, int64(0), countOrganizations(t, db, "ооо мусор"), "наименование при заданном id не резолвится")
	})

	t.Run("архивная и несуществующая организация по id отклоняются", func(t *testing.T) {
		// is_active снимаем отдельным UPDATE: у поля есть gorm-тег default:true, и при
		// Create структуры с false gorm вообще опускает колонку, оставляя запись активной.
		archived := models.Organization{Name: `ООО "Архивная"`, Type: &orgType}
		require.NoError(t, db.Create(&archived).Error)
		require.NoError(t, db.Model(&models.Organization{}).Where("id = ?", archived.ID).Update("is_active", false).Error)

		var check models.Organization
		require.NoError(t, db.First(&check, archived.ID).Error)
		require.False(t, check.IsActive, "фикстура должна быть архивной")

		rec := submitWithRefs(t, e, token, uaID, "A005AA777", fmt.Sprintf(`"organization_id":%d`, archived.ID))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "архивная организация: %s", rec.Body.String())

		rec = submitWithRefs(t, e, token, uaID, "A006AA777", `"organization_id":999999`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "несуществующая организация: %s", rec.Body.String())
	})

	// От вырожденного наименования не остаётся ключа дедупликации, поэтому запись из него
	// не заводим: справочник получил бы мусор от опечатки, который ни с чем не схлопнётся.
	t.Run("вырожденное наименование отклоняется и не заводит запись", func(t *testing.T) {
		rec := submitWithRefs(t, e, token, uaID, "A007AA777", `"organization_name":"\"\""`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "вырожденное наименование: %s", rec.Body.String())

		var n int64
		require.NoError(t, db.Model(&models.Organization{}).Where("name_normalized = ?", "").Count(&n).Error)
		assert.Equal(t, int64(0), n)
	})

	// Компании ведёт тот же код, и расхождение между зеркальными сущностями тихое.
	t.Run("компания резолвится так же", func(t *testing.T) {
		appID := submittedAppID(t, submitWithRefs(t, e, token, uaID, "A008AA777",
			`"organization":"","company_name":"ЗАО \"Новая\""`))

		refs := readAppOrgRefs(t, db, appID)
		assert.Nil(t, refs.OrganizationID, "организация не указана - остаётся NULL")
		require.NotNil(t, refs.CompanyID)

		var created models.Company
		require.NoError(t, db.First(&created, *refs.CompanyID).Error)
		assert.Equal(t, "зао новая", created.NameNormalized)
		assert.Equal(t, models.ModerationPending, created.ModerationStatus)
		require.NotNil(t, created.CreatedByUserID)
		assert.Equal(t, sender.ID, *created.CreatedByUserID)
	})

	t.Run("запись справочника проверена по умолчанию", func(t *testing.T) {
		var seeded models.Organization
		require.NoError(t, db.First(&seeded, td.OrgID).Error)
		assert.Equal(t, models.ModerationApproved, seeded.ModerationStatus,
			"колонка добавляется с DEFAULT: записи, жившие до модерации, читаются как проверенные")

		require.Equal(t, http.StatusOK,
			testutil.POST(t, e, "/organizations", `{"name":"ООО \"Из справочника\"","type":"Подрядчик"}`,
				testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))).Code)

		var fromDirectory models.Organization
		require.NoError(t, db.Where("name_normalized = ?", "ооо из справочника").First(&fromDirectory).Error)
		assert.Equal(t, models.ModerationApproved, fromDirectory.ModerationStatus)
		assert.Nil(t, fromDirectory.CreatedByUserID)
	})
}
