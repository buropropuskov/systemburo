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

// TestOrgNameNormalized_FilledOnCreate - ключ дедупликации проставляется хуком модели
// при создании через API, без явной записи поля в сервисе (#1437).
func TestOrgNameNormalized_FilledOnCreate(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/organizations", `{"name":"ООО \"Петрушка\"","type":"Подрядчик"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	var org models.Organization
	require.NoError(t, db.Where("name = ?", `ООО "Петрушка"`).First(&org).Error)
	assert.Equal(t, "ооо петрушка", org.NameNormalized)
}

// TestOrgNameNormalized_RejectsOtherWriting - второе написание того же юрлица не
// создаёт вторую запись. Это и есть защита от «ооо петрушка» рядом с «ООО "Петрушка"».
func TestOrgNameNormalized_RejectsOtherWriting(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK,
		testutil.POST(t, e, "/organizations", `{"name":"ООО \"Петрушка\"","type":"Подрядчик"}`, testutil.AuthHeader(token)).Code)

	for _, writing := range []string{`ооо петрушка`, `ООО Петрушка`, `Общество с ограниченной ответственностью "Петрушка"`} {
		body := `{"name":"` + writing + `","type":"Подрядчик"}`
		rec := testutil.POST(t, e, "/organizations", body, testutil.AuthHeader(token))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "написание %q должно упереться в существующую запись", writing)
	}

	var count int64
	require.NoError(t, db.Model(&models.Organization{}).Where("name_normalized = ?", "ооо петрушка").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

// TestOrgNameNormalized_UpdateRecalculates - переименование пересчитывает ключ.
// Update идёт map-обновлением, куда хук модели не достаёт: если поле не записать
// явно, ключ останется от старого имени и дедупликация начнёт врать.
func TestOrgNameNormalized_UpdateRecalculates(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/organizations", `{"name":"ООО \"Старое\"","type":"Подрядчик"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	created := testutil.ParseMap(t, rec)
	id := int(created["id"].(float64))

	require.Equal(t, http.StatusOK,
		testutil.PUT(t, e, "/organizations/"+strconv.Itoa(id), `{"name":"ЗАО \"Новое\"","type":"Подрядчик"}`, testutil.AuthHeader(token)).Code)

	var org models.Organization
	require.NoError(t, db.First(&org, id).Error)
	assert.Equal(t, "зао новое", org.NameNormalized)
}

// TestOrgNameNormalized_Backfill - бэкфилл заполняет ключ у записей, созданных в
// обход хука (прямая вставка), и не трогает уже согласованные строки при повторном
// прогоне. Второе свойство важно: функция вызывается на каждом старте.
func TestOrgNameNormalized_Backfill(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	testutil.SeedTestData(t, db)

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

	// Повторный прогон идемпотентен: ключи те же, ошибок нет.
	require.NoError(t, database.BackfillOrgNameNormalized(db))
	var again models.Organization
	require.NoError(t, db.Where("name = ?", `ООО "Без ключа"`).First(&again).Error)
	assert.Equal(t, org.NameNormalized, again.NameNormalized)
}
