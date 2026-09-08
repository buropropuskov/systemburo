package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Иное разрешение на работу шифруется наравне с патентом (#2351).
//
// Это тот же документ, дающий право работать, просто выданный не по патентной
// схеме. Соседнее поле шифровалось, а это лежало открытым - различие историческое,
// а не осмысленное. Свёртки у него нет намеренно: по нему не ищут.
func TestOtherPermission_StoredEncrypted(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	// Тестовое приложение поднимается без ключа (шифрование в тестах сквозное),
	// поэтому включаем его на время проверки - как в тестах шифрования писем.
	crypto.SetGlobalKey(testPermissionKey())
	t.Cleanup(func() { crypto.SetGlobalKey(nil) })

	const permission = "Разрешение 77-АБ-123456 от 01.03.2026"
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees",
		`{"last_name":"Разрешаев","first_name":"Пётр","position":"Каменщик","passport_series_number":"4600 111222","other_permission":"`+permission+`","pd_consent":true}`, h).Code)

	var id int
	require.NoError(t, db.Model(&models.UniqueEmployee{}).Where("last_name = ?", "Разрешаев").
		Select("id").Row().Scan(&id))

	// В базе - шифротекст: сырым запросом номер разрешения не прочитать.
	var raw string
	require.NoError(t, db.Raw("SELECT other_permission FROM unique_employees WHERE id = ?", id).Row().Scan(&raw))
	assert.NotContains(t, raw, "77-АБ", "номер разрешения не лежит в базе открытым")

	// А через модель читается обратно: шифрование не должно ломать работу с записью.
	var row models.UniqueEmployee
	require.NoError(t, db.First(&row, id).Error)
	require.NotNil(t, row.OtherPermission)
	assert.Equal(t, permission, *row.OtherPermission, "значение расшифровывается при чтении")
}

func testPermissionKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 3)
	}
	return key
}
