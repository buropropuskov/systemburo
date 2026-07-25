package handlers_test

import (
	"net/http"
	"strconv"
	"testing"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrgNameNormalized покрывает ключ дедупликации наименований (#1437) целиком.
// Секции живут на одном SetupTestApp: пакет handlers идёт в CI под -race у самой
// границы go test -timeout, и шесть отдельных тестов с собственными CleanDB и Seed
// уже перебивали её (#1437, следом за уроком про таймаут handlers).
func TestOrgNameNormalized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	auth := testutil.AuthHeader(token)

	t.Run("ключ проставляется при создании", func(t *testing.T) {
		rec := testutil.POST(t, e, "/organizations", `{"name":"ООО \"Петрушка\"","type":"Подрядчик"}`, auth)
		require.Equal(t, http.StatusOK, rec.Code)

		var org models.Organization
		require.NoError(t, db.Where("name = ?", `ООО "Петрушка"`).First(&org).Error)
		assert.Equal(t, "ооо петрушка", org.NameNormalized)
	})

	t.Run("другое написание не создаёт вторую запись", func(t *testing.T) {
		for _, writing := range []string{`ооо петрушка`, `ООО Петрушка`, `Общество с ограниченной ответственностью "Петрушка"`} {
			rec := testutil.POST(t, e, "/organizations", `{"name":"`+writing+`","type":"Подрядчик"}`, auth)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "написание %q должно упереться в существующую запись", writing)
		}

		var count int64
		require.NoError(t, db.Model(&models.Organization{}).Where("name_normalized = ?", "ооо петрушка").Count(&count).Error)
		assert.Equal(t, int64(1), count)
	})

	// Update идёт map-обновлением, куда хук модели не достаёт: если не записать поле
	// явно, ключ останется от старого имени и дедупликация начнёт врать.
	t.Run("переименование пересчитывает ключ", func(t *testing.T) {
		rec := testutil.POST(t, e, "/organizations", `{"name":"ООО \"Старое\"","type":"Подрядчик"}`, auth)
		require.Equal(t, http.StatusOK, rec.Code)
		id := int(testutil.ParseMap(t, rec)["id"].(float64))

		require.Equal(t, http.StatusOK,
			testutil.PUT(t, e, "/organizations/"+strconv.Itoa(id), `{"name":"ЗАО \"Новое\"","type":"Подрядчик"}`, auth).Code)

		var org models.Organization
		require.NoError(t, db.First(&org, id).Error)
		assert.Equal(t, "зао новое", org.NameNormalized)
	})

	// Сервисы организаций и компаний зеркальны, и расхождение между ними тихое:
	// компанию с другим написанием заводит тот же экран справочника.
	t.Run("у компаний дедупликация работает так же", func(t *testing.T) {
		require.Equal(t, http.StatusOK,
			testutil.POST(t, e, "/companies", `{"name":"ООО \"Ромашка\"","type":"Подрядчик"}`, auth).Code)
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/companies", `{"name":"ооо ромашка","type":"Подрядчик"}`, auth).Code)

		var company models.Company
		require.NoError(t, db.Where("name = ?", `ООО "Ромашка"`).First(&company).Error)
		assert.Equal(t, "ооо ромашка", company.NameNormalized)
	})

	// Наименование, от которого после нормализации ничего не остаётся, даёт пустой
	// ключ. По нему несвязанные записи схлопнулись бы в одну, поэтому для них сверка
	// идёт по точной строке - защита не слабее той, что была до введения ключа.
	t.Run("вырожденное наименование сверяется по точной строке", func(t *testing.T) {
		require.Equal(t, http.StatusOK,
			testutil.POST(t, e, "/organizations", `{"name":"\"","type":"Подрядчик"}`, auth).Code)
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/organizations", `{"name":"\"","type":"Подрядчик"}`, auth).Code,
			"то же вырожденное имя - дубль")
		assert.Equal(t, http.StatusOK,
			testutil.POST(t, e, "/organizations", `{"name":"--","type":"Подрядчик"}`, auth).Code,
			"другое вырожденное имя с тем же пустым ключом - отдельная запись")
	})

	// Бэкфилл нужен записям, созданным в обход хука, и прогоняется на каждом старте,
	// поэтому повторный проход обязан быть безобидным.
	t.Run("бэкфилл заполняет ключ и идемпотентен", func(t *testing.T) {
		require.NoError(t, db.Exec(
			`INSERT INTO organizations (name, type, is_active, name_normalized) VALUES (?, ?, true, '')`,
			`ООО "Без ключа"`, models.OrgTypeContractor).Error)
		require.NoError(t, db.Exec(
			`INSERT INTO companies (name, type, is_active, name_normalized) VALUES (?, ?, true, '')`,
			`ЗАО «Тоже без ключа»`, models.OrgTypeContractor).Error)

		require.NoError(t, database.BackfillOrgNameNormalized(db))

		var org models.Organization
		require.NoError(t, db.Where("name = ?", `ООО "Без ключа"`).First(&org).Error)
		assert.Equal(t, "ооо без ключа", org.NameNormalized)

		var company models.Company
		require.NoError(t, db.Where("name = ?", `ЗАО «Тоже без ключа»`).First(&company).Error)
		assert.Equal(t, "зао тоже без ключа", company.NameNormalized)

		require.NoError(t, database.BackfillOrgNameNormalized(db))
		var again models.Organization
		require.NoError(t, db.Where("name = ?", `ООО "Без ключа"`).First(&again).Error)
		assert.Equal(t, org.NameNormalized, again.NameNormalized)
	})
}
