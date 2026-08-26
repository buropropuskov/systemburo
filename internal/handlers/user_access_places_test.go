package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// HTTP-тесты CRUD привязки мест доступа к охраннику (#706, срез BE-S5).
// DB-backed: живут в пакете handlers_test - единственном DB-использующем тест-бинаре.

type accessWorld struct {
	e             *echo.Echo
	db            *gorm.DB
	guardUsername string
	userUsername  string
	guardToken    string
	userToken     string
	adminToken    string
	up1           int
	up2           int
	tbl1          int
}

func setupAccessWorld(t *testing.T) accessWorld {
	t.Helper()

	e, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	secTypeID := secUserTypeIDByCode(t, db, "security")
	userTypeID := secUserTypeIDByCode(t, db, "user")

	const (
		guardUsername = "access_guard_user"
		userUsername  = "access_regular_user"
		pwd           = "password_long_enough"
	)

	guardToken := testutil.RegisterAndLogin(t, e, guardUsername, pwd, secTypeID, td.OrgID, 0)
	userToken := testutil.RegisterAndLogin(t, e, userUsername, pwd, userTypeID, td.OrgID, 0)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, 0)

	p1 := models.UnloadPlace{Name: "Склад-Доступ-1", IsActive: true}
	require.NoError(t, db.Create(&p1).Error)
	p2 := models.UnloadPlace{Name: "Склад-Доступ-2", IsActive: true}
	require.NoError(t, db.Create(&p2).Error)

	st := models.SystemTable{Name: "Таблица-Доступ-1", TableType: "people", IsActive: true}
	require.NoError(t, db.Create(&st).Error)

	return accessWorld{
		e:             e,
		db:            db,
		guardUsername: guardUsername,
		userUsername:  userUsername,
		guardToken:    guardToken,
		userToken:     userToken,
		adminToken:    adminToken,
		up1:           p1.ID,
		up2:           p2.ID,
		tbl1:          st.ID,
	}
}

func TestUserAccessPlaces_UnloadPlaces_RoundTrip(t *testing.T) {
	w := setupAccessWorld(t)

	body := fmt.Sprintf(`{"unload_place_ids":[%d,%d]}`, w.up1, w.up2)
	putURL := fmt.Sprintf("/users/%s/unload-places", w.guardUsername)
	rec := testutil.PUT(t, w.e, putURL, body, testutil.AuthHeader(w.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	getURL := fmt.Sprintf("/users/%s/unload-places", w.guardUsername)
	rec = testutil.GET(t, w.e, getURL, testutil.AuthHeader(w.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	places := testutil.ParseResponse[[]map[string]interface{}](t, rec)
	require.Len(t, places, 2, "должны вернуться оба назначенных места")
}

func TestUserAccessPlaces_UnloadPlaces_ReplaceNotAppend(t *testing.T) {
	w := setupAccessWorld(t)
	putURL := fmt.Sprintf("/users/%s/unload-places", w.guardUsername)

	// Первое назначение: два места
	body := fmt.Sprintf(`{"unload_place_ids":[%d,%d]}`, w.up1, w.up2)
	rec := testutil.PUT(t, w.e, putURL, body, testutil.AuthHeader(w.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Второе назначение: только одно место — должно ЗАМЕНИТЬ, а не добавить
	body = fmt.Sprintf(`{"unload_place_ids":[%d]}`, w.up1)
	rec = testutil.PUT(t, w.e, putURL, body, testutil.AuthHeader(w.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	getURL := fmt.Sprintf("/users/%s/unload-places", w.guardUsername)
	rec = testutil.GET(t, w.e, getURL, testutil.AuthHeader(w.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	places := testutil.ParseResponse[[]map[string]interface{}](t, rec)
	require.Len(t, places, 1, "второй SET заменяет первый, а не добавляет")
}

func TestUserAccessPlaces_UnloadPlaces_EmptyClears(t *testing.T) {
	w := setupAccessWorld(t)
	putURL := fmt.Sprintf("/users/%s/unload-places", w.guardUsername)

	// Назначить место
	body := fmt.Sprintf(`{"unload_place_ids":[%d]}`, w.up1)
	rec := testutil.PUT(t, w.e, putURL, body, testutil.AuthHeader(w.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Пустой slice снимает все привязки
	rec = testutil.PUT(t, w.e, putURL, `{"unload_place_ids":[]}`, testutil.AuthHeader(w.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	getURL := fmt.Sprintf("/users/%s/unload-places", w.guardUsername)
	rec = testutil.GET(t, w.e, getURL, testutil.AuthHeader(w.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	places := testutil.ParseResponse[[]map[string]interface{}](t, rec)
	require.Empty(t, places, "пустой SET снимает все привязки")
}

func TestUserAccessPlaces_Tables_RoundTrip(t *testing.T) {
	w := setupAccessWorld(t)

	body := fmt.Sprintf(`{"table_ids":[%d]}`, w.tbl1)
	putURL := fmt.Sprintf("/users/%s/tables", w.guardUsername)
	rec := testutil.PUT(t, w.e, putURL, body, testutil.AuthHeader(w.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	getURL := fmt.Sprintf("/users/%s/tables", w.guardUsername)
	rec = testutil.GET(t, w.e, getURL, testutil.AuthHeader(w.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	tables := testutil.ParseResponse[[]map[string]interface{}](t, rec)
	require.Len(t, tables, 1, "место прохода должно вернуться в списке")
	require.EqualValues(t, w.tbl1, tables[0]["id"], "id должен совпадать с назначенным")
}

func TestUserAccessPlaces_Tables_ReplaceNotAppend(t *testing.T) {
	w := setupAccessWorld(t)

	// Создаём второе место прохода в той же БД
	st2 := models.SystemTable{Name: "Таблица-Доступ-2", TableType: "people", IsActive: true}
	require.NoError(t, w.db.Create(&st2).Error)

	putURL := fmt.Sprintf("/users/%s/tables", w.guardUsername)

	// Первое назначение: два места
	body := fmt.Sprintf(`{"table_ids":[%d,%d]}`, w.tbl1, st2.ID)
	rec := testutil.PUT(t, w.e, putURL, body, testutil.AuthHeader(w.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Второе назначение: только одно
	body = fmt.Sprintf(`{"table_ids":[%d]}`, w.tbl1)
	rec = testutil.PUT(t, w.e, putURL, body, testutil.AuthHeader(w.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	getURL := fmt.Sprintf("/users/%s/tables", w.guardUsername)
	rec = testutil.GET(t, w.e, getURL, testutil.AuthHeader(w.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	tables := testutil.ParseResponse[[]map[string]interface{}](t, rec)
	require.Len(t, tables, 1, "второй SET заменяет первый")
}

func TestUserAccessPlaces_NonAdmin_Forbidden(t *testing.T) {
	w := setupAccessWorld(t)

	putURL := fmt.Sprintf("/users/%s/unload-places", w.guardUsername)
	rec := testutil.PUT(t, w.e, putURL, fmt.Sprintf(`{"unload_place_ids":[%d]}`, w.up1), testutil.AuthHeader(w.userToken))
	require.Equal(t, http.StatusForbidden, rec.Code, "обычный пользователь не может назначать места: %s", rec.Body.String())

	getURL := fmt.Sprintf("/users/%s/unload-places", w.guardUsername)
	rec = testutil.GET(t, w.e, getURL, testutil.AuthHeader(w.userToken))
	require.Equal(t, http.StatusForbidden, rec.Code, "обычный пользователь не может читать места: %s", rec.Body.String())

	putURL = fmt.Sprintf("/users/%s/tables", w.guardUsername)
	rec = testutil.PUT(t, w.e, putURL, fmt.Sprintf(`{"table_ids":[%d]}`, w.tbl1), testutil.AuthHeader(w.userToken))
	require.Equal(t, http.StatusForbidden, rec.Code, "обычный пользователь не может назначать таблицы: %s", rec.Body.String())

	getURL = fmt.Sprintf("/users/%s/tables", w.guardUsername)
	rec = testutil.GET(t, w.e, getURL, testutil.AuthHeader(w.userToken))
	require.Equal(t, http.StatusForbidden, rec.Code, "обычный пользователь не может читать таблицы: %s", rec.Body.String())
}
