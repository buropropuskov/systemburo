package handlers_test

// «Иное разрешение» шифруется с #2351, но правка записи идёт через карту обновления,
// а Updates с картой не проходит через BeforeSave: значение ложилось в базу открытым.
// Рядом же, в том же методе, паспорт и патент шифруются явно - забыли только это поле
// (#2413). Проверяем обе стороны разом: в базе шифротекст, в выдаче читаемое значение.

import (
	"net/http"
	"testing"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUniqueEmployeeUpdate_EncryptsOtherPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees",
		`{"last_name":"Разрешаев","first_name":"Илья","position":"Сварщик","passport_series_number":"4588 030303","pd_consent":true}`, h).Code)
	id := objectionEmployeeID(t, db, "Разрешаев")

	// Ключ включаем здесь, а не в начале: SetupTestApp сбрасывает его в passthrough,
	// и всё, что заведено до этой строки, осталось бы открытым в общей тестовой базе.
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 17)
	}
	crypto.SetGlobalKey(key)
	t.Cleanup(func() { crypto.SetGlobalKey(nil) })

	const permission = "Разрешение на временное проживание 77 №123456"
	rec := testutil.PUT(t, e, "/unique-employees/"+itoa(id),
		`{"other_permission":"`+permission+`"}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Сырой запрос мимо AfterFind: так значение лежит в базе на самом деле.
	var stored string
	require.NoError(t, db.Raw(`SELECT COALESCE(other_permission, '') FROM unique_employees WHERE id = ?`, id).
		Scan(&stored).Error)
	assert.NotEqual(t, permission, stored, "в базе обязан лежать шифротекст, а не открытый текст")
	back, err := crypto.Decrypt(stored, key)
	require.NoError(t, err, "значение должно расшифровываться действующим ключом")
	assert.Equal(t, permission, back)

	// А через модель - читаемое значение, как его видит оператор.
	var employee models.UniqueEmployee
	require.NoError(t, db.First(&employee, id).Error)
	require.NotNil(t, employee.OtherPermission)
	assert.Equal(t, permission, *employee.OtherPermission)
}
