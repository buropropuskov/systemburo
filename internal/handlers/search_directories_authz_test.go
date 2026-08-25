package handlers_test

// Справочники общесистемные: сужать их по владельцу нечем и не нужно, разграничение
// даёт само право page.admin.directories. Поэтому проверяем ровно две вещи -- без права
// раздела нет вовсе, с правом записи находятся, а архивные в подсказки не идут.
//
// Каждый тест ищет по собственному токену со временем в имени: marks не входят в список
// очистки CleanDB и копятся между прогонами, а на имени действующей марки висит
// уникальный индекс. Счётчика uniq() тут мало -- он начинается заново при каждом запуске
// и на втором прогоне даёт то же имя, что осталось в базе от первого.

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// searchDirToken -- токен поиска, уникальный и внутри прогона, и между прогонами.
func searchDirToken(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano()%1_000_000_000)
}

// dropTestMarks убирает марки, оставшиеся от прошлых прогонов: таблица не входит в
// список очистки CleanDB и копится. Одного уникального номера в имени мало: с нечётким
// сравнением похожая старая запись находится по новому токену -- у них общее начало.
// Поэтому у каждого теста и корень слова свой, и чистка своего префикса.
func dropTestMarks(t *testing.T, db *gorm.DB, prefix string) {
	t.Helper()
	require.NoError(t, db.Exec("DELETE FROM marks WHERE name LIKE ?", prefix+"%").Error)
}

func grantPermission(t *testing.T, db *gorm.DB, username, key string) {
	t.Helper()
	userID := userIDByName(t, db, username)
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        userID,
		PermissionKey: key,
		Value:         "allow",
	}).Error)
}

// Без права справочники не показываются, с правом -- находятся.
func TestSearch_Directories_GatedByPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	dropTestMarks(t, db, "Кряквамотор")
	markName := searchDirToken("Кряквамотор")
	require.NoError(t, db.Create(&models.Mark{Name: markName}).Error)

	testutil.RegisterUser(t, e, "dir_plain", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "dir_plain")
	testutil.RegisterUser(t, e, "dir_admin", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "dir_admin")
	grantPermission(t, db, "dir_admin", "page.admin.directories")

	t.Run("без права раздела нет", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "dir_plain", "password123")
		rec := testutil.GET(t, e, "/search?q="+markName, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

		_, found := groupByType(decodeSearch(t, rec.Body.String()), "directories")
		assert.False(t, found, "справочники закрыты правом: %s", rec.Body.String())
	})

	t.Run("с правом марка находится", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "dir_admin", "password123")
		rec := testutil.GET(t, e, "/search?q="+markName, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

		count, found := groupByType(decodeSearch(t, rec.Body.String()), "directories")
		require.True(t, found, "справочник должен находиться: %s", rec.Body.String())
		assert.Equal(t, 1, count)
	})
}

// Архивная запись в подсказки не идёт: восстанавливают её в разделе, а в поиске она
// только сбивает с толку -- выглядит действующей.
func TestSearch_Directories_ArchivedHidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Архивируем отдельным Update: при Create поле с false -- нулевое значение, и gorm
	// подставляет вместо него default:true, из-за чего запись остаётся действующей.
	dropTestMarks(t, db, "Пингвинархив")
	archivedName := searchDirToken("Пингвинархив")
	archived := models.Mark{Name: archivedName}
	require.NoError(t, db.Create(&archived).Error)
	require.NoError(t, db.Model(&archived).Update("is_active", false).Error)

	testutil.RegisterUser(t, e, "dir_arch", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "dir_arch")
	grantPermission(t, db, "dir_arch", "page.admin.directories")

	token, _ := testutil.LoginUser(t, e, "dir_arch", "password123")
	rec := testutil.GET(t, e, "/search?q="+archivedName, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	_, found := groupByType(decodeSearch(t, rec.Body.String()), "directories")
	assert.False(t, found, "архивная запись не должна попадать в подсказки: %s", rec.Body.String())
}

// Одним запросом накрываются разные справочники, и каждый ведёт на свою страницу --
// поэтому у элементов разный код сущности в target.
func TestSearch_Directories_DifferentKindsHaveOwnTargets(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	dropTestMarks(t, db, "Зюзюкаобщий")
	token := searchDirToken("Зюзюкаобщий")
	require.NoError(t, db.Create(&models.Mark{Name: token + "марка"}).Error)
	require.NoError(t, db.Create(&models.Organization{Name: token + "транс", IsActive: true}).Error)

	testutil.RegisterUser(t, e, "dir_kinds", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "dir_kinds")
	grantPermission(t, db, "dir_kinds", "page.admin.directories")

	authToken, _ := testutil.LoginUser(t, e, "dir_kinds", "password123")
	rec := testutil.GET(t, e, "/search?q="+token, testutil.AuthHeader(authToken))
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	body := rec.Body.String()
	assert.Contains(t, body, `"entity":"mark"`, "марка должна вести на свою страницу: %s", body)
	assert.Contains(t, body, `"entity":"organization"`, "организация должна вести на свою страницу: %s", body)
}
