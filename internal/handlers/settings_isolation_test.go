package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Настройки, изменённые одним тестом, не должны доставаться следующему.
//
// Утечка была неочевидной: служба настроек читает их в кэш процесса один раз, при
// создании, а CleanDB тест зовёт уже ПОСЛЕ SetupTestApp - то есть новая служба
// успевала прочитать то, что оставил предыдущий тест, и восстановление базы этому
// кэшу ничем не помогало. Так проверка политики паролей, включавшая требование
// специального знака, роняла следующий тест на создании пользователя: пароль,
// который тест считал допустимым, переставал проходить политику.

// TestSettings_DoNotLeakIntoNextApp: приложение, собранное после теста, изменившего
// политику паролей, видит посеянные настройки, а не оставленные.
func TestSettings_DoNotLeakIntoNextApp(t *testing.T) {
	// Первое приложение: поднимаем требования к паролю, как это делает тест
	// проверки настроек, и ничего за собой не возвращаем - в этом вся суть.
	e, db, cleanup := testutil.SetupTestApp(t)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/settings/password.require_special", `{"value":"true"}`,
		testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = testutil.PUT(t, e, "/settings/password.min_length", `{"value":"20"}`,
		testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	cleanup()

	// Второе приложение - следующий тест. Пароль без специального знака и короче
	// двадцати символов обязан пройти: настройки к этому моменту посеянные.
	e2, db2, cleanup2 := testutil.SetupTestApp(t)
	defer cleanup2()
	testutil.CleanDB(t, db2)
	td2 := testutil.SeedTestData(t, db2)
	admin2 := testutil.RegisterAdmin(t, e2, td2.OrgID, td2.CompanyID)

	body := fmt.Sprintf(`{"username":"settings_leak_probe","password":"prostoyparol123",`+
		`"organization_id":%d,"type_id":1,"last_name":"Проверка"}`, td2.OrgID)
	rec = testutil.POST(t, e2, "/api/users", body, testutil.AuthHeader(admin2))
	assert.Equal(t, http.StatusOK, rec.Code,
		"пароль обязан пройти политику: требования, поднятые прошлым тестом, не его дело")
}
