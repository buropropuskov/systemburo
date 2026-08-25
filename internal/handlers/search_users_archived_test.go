package handlers_test

// Архивных учётных записей на стенде оказалось 92 из 109, и по распространённой
// фамилии они вытесняли действующего человека из выдачи целиком: раздел отдаёт
// ограниченное число строк, а порядок решал возраст записи. Теперь действующие идут
// первыми, а архивная помечена прямо в подзаголовке - иначе непонятно, почему в
// разделе её не видно.

import (
	"net/http"
	"strings"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func archiveUser(t *testing.T, db *gorm.DB, username string) {
	t.Helper()
	require.NoError(t, db.Table("users").Where("username = ?", username).Update("is_active", false).Error)
}

func TestSearch_Users_ArchivedGoAfterActive(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Действующего заводим ПЕРВЫМ, архивных - следом. Порядок важен: при равной
	// ступени совпадения выдача идёт от свежих записей к старым, поэтому без правки
	// архивные (более новые) встали бы выше - ровно то, что видел человек на стенде.
	testutil.RegisterUser(t, e, "shum_live", "password123", 1, td.OrgID, td.CompanyID)
	require.NoError(t, db.Table("users").Where("username = ?", "shum_live").Update("last_name", "Шумилин").Error)
	for _, name := range []string{"shum_old_a", "shum_old_b"} {
		testutil.RegisterUser(t, e, name, "password123", 1, td.OrgID, td.CompanyID)
		require.NoError(t, db.Table("users").Where("username = ?", name).Update("last_name", "Шумилин").Error)
		archiveUser(t, db, name)
	}

	// Раздел пользователей закрыт правом администратора - выдаём его напрямую.
	testutil.RegisterUser(t, e, "search_admin", "password123", 1, td.OrgID, td.CompanyID)
	require.NoError(t, db.Table("users").Where("username = ?", "search_admin").Update("is_super_admin", true).Error)
	token, _ := testutil.LoginUser(t, e, "search_admin", "password123")

	rec := testutil.GET(t, e, "/search?q=Шумилин", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())
	body := rec.Body.String()

	t.Run("действующая запись идёт раньше архивных", func(t *testing.T) {
		live := strings.Index(body, "shum_live")
		archived := strings.Index(body, "shum_old")
		require.NotEqual(t, -1, live, "действующая учётка обязана быть в выдаче: %s", body)
		require.NotEqual(t, -1, archived, "архивные тоже показываем - их ищут, чтобы восстановить")
		assert.Less(t, live, archived, "архив не должен вытеснять действующие записи: %s", body)
	})

	t.Run("архивная запись помечена в подзаголовке", func(t *testing.T) {
		assert.Contains(t, body, "в архиве", "иначе непонятно, почему записи нет в разделе: %s", body)
	})
}
