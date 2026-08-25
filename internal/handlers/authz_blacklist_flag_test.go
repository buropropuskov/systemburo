package handlers_test

// Срез 2a аудита доступа: сервер сам помечает записи реестра флагом is_blacklisted,
// чтобы фронту не приходилось выгружать весь чёрный список ПД ради подсветки.
// Флаг обязан совпадать с серверной проверкой (Check) — тот же матч по нормализованному
// ФИО / номеру+марке.

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rowByField(rows []map[string]interface{}, field, want string) map[string]interface{} {
	for _, r := range rows {
		if v, _ := r[field].(string); v == want {
			return r
		}
	}
	return nil
}

func TestUniqueEmployees_IsBlacklistedFlag(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	// В ЧС — Иванов; регистр и лишние пробелы должны нормализоваться при матче.
	_, err := newPersonBlacklistService(db).Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Иванов", FirstName: "Иван", MiddleName: "Иванович", Reason: "нарушение",
	}, getUserID(t, db, "testadmin"))
	require.NoError(t, err)

	blk := fmt.Sprintf(`{"pd_consent":true,"last_name":"  иванов ","first_name":"ИВАН","middle_name":"Иванович","passport_series_number":"1111 111111","organization_id":%d,"company_id":%d}`, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees", blk, h).Code)
	clean := fmt.Sprintf(`{"pd_consent":true,"last_name":"Чистов","first_name":"Пётр","passport_series_number":"2222 222222","organization_id":%d,"company_id":%d}`, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees", clean, h).Code)

	// Ищем по паспорту — точный идентификатор строки (ФИО сервер мог обрезать/нормализовать
	// при сохранении, а матч ЧС всё равно идёт по нормализованному ФИО).
	rows := testutil.ParseSlice(t, testutil.GET(t, e, "/unique-employees?filter_type=all_system", h))
	blkRow := rowByField(rows, "passport_series_number", "1111 111111")
	require.NotNil(t, blkRow, "сотрудник из ЧС должен быть в списке")
	assert.Equal(t, true, blkRow["is_blacklisted"], "сотрудник в ЧС -> is_blacklisted=true (матч без учёта регистра/пробелов)")

	cleanRow := rowByField(rows, "passport_series_number", "2222 222222")
	require.NotNil(t, cleanRow, "чистый сотрудник должен быть в списке")
	assert.Equal(t, false, cleanRow["is_blacklisted"], "сотрудник не из ЧС -> is_blacklisted=false")
}

func TestUniqueCars_IsBlacklistedFlag(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	mark := seedMark(t, db, "BL_Flag_Mark")

	_, err := newVehicleBlacklistService(db).Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "A123AA799", MarkID: mark.ID, Reason: "угон",
	}, getUserID(t, db, "testadmin"))
	require.NoError(t, err)

	blk := fmt.Sprintf(`{"number":"a123aa799","mark":"%s","organization_id":%d,"company_id":%d}`, mark.Name, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-cars", blk, h).Code)
	clean := fmt.Sprintf(`{"number":"B456BB799","mark":"%s","organization_id":%d,"company_id":%d}`, mark.Name, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-cars", clean, h).Code)

	rows := testutil.ParseSlice(t, testutil.GET(t, e, "/unique-cars?filter_type=all_system", h))
	blkRow := rowByField(rows, "number", "a123aa799")
	require.NotNil(t, blkRow, "машина из ЧС должна быть в списке")
	assert.Equal(t, true, blkRow["is_blacklisted"], "машина в ЧС -> is_blacklisted=true (матч без учёта регистра)")

	cleanRow := rowByField(rows, "number", "B456BB799")
	require.NotNil(t, cleanRow, "чистая машина должна быть в списке")
	assert.Equal(t, false, cleanRow["is_blacklisted"], "машина не из ЧС -> is_blacklisted=false")
}
