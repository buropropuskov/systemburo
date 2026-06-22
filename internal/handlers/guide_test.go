package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Супер-админ видит все засеянные разделы руководства, items развёрнуты в массив,
// file == nil пока PDF не загружен.
func TestGuideSections_SuperAdminSeesAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	// Ключи раздела совпадают с каталогом прав (защита от молчаливого рассинхрона
	// permission_catalog.go). guide.guard в каталоге пока нет (заводит perm-gating).
	require.Equal(t, services.KeyGuideUser, services.GuideKeyForRole("user"))
	require.Equal(t, services.KeyGuideAdmin, services.GuideKeyForRole("admin"))

	token := testutil.RegisterAdmin(t, e, 0, 0)
	rec := testutil.GET(t, e, "/guide/sections", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	sections := testutil.ParseResponse[[]models.GuideSectionResponse](t, rec)
	require.Len(t, sections, 3)

	// Порядок по sort_order: user -> guard -> admin.
	assert.Equal(t, "user", sections[0].Role)
	assert.Equal(t, "guard", sections[1].Role)
	assert.Equal(t, "admin", sections[2].Role)

	assert.Equal(t, "Руководство пользователя", sections[0].Title)
	assert.NotEmpty(t, sections[0].Lead)
	require.NotEmpty(t, sections[0].Items, "items должны разворачиваться из jsonb в массив строк")
	assert.Nil(t, sections[0].File, "file == nil пока PDF не загружен")
}

// Обычный пользователь с правом только на guide.user видит лишь свой раздел;
// download гейтит та же проверка.
func TestGuideSections_GatedByPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	const username, password = "guideuser", "Password123!"
	testutil.RegisterUser(t, e, username, password, 1, 0, 0)

	var u models.User
	require.NoError(t, db.Where("username = ?", username).First(&u).Error)
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        u.ID,
		PermissionKey: services.KeyGuideUser,
		Value:         "allow",
		GrantedAt:     time.Now().UTC(),
	}).Error)

	token, _ := testutil.LoginUser(t, e, username, password)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/guide/sections", h)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	sections := testutil.ParseResponse[[]models.GuideSectionResponse](t, rec)
	require.Len(t, sections, 1, "виден только раздел с выданным правом")
	assert.Equal(t, "user", sections[0].Role)

	// Разрешённый раздел без загруженного файла -> 404.
	rec = testutil.GET(t, e, "/guide/sections/user/download", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// Запрещённый раздел -> 403 (нет права guide.admin).
	rec = testutil.GET(t, e, "/guide/sections/admin/download", h)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Неизвестная роль -> 404.
	rec = testutil.GET(t, e, "/guide/sections/bogus/download", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
