package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// scanURL -- адрес файла в раздаче статики по имени на диске. Именно так к нему и
// приходят: имя наружу не отдаётся, но оседает в журнале запросов, в резервной
// копии и в пакете выгрузки организации.
func scanURL(storedName string) string {
	return "/api/uploads/application_files/" + storedName
}

// storedNameOf -- имя файла заявки на диске по идентификатору строки.
func storedNameOf(t *testing.T, db *gorm.DB, fileID int) string {
	t.Helper()
	var row models.ApplicationFile
	require.NoError(t, db.First(&row, fileID).Error)
	require.NotEmpty(t, row.StoredName)
	return row.StoredName
}

// getStatic дёргает раздачу /api/uploads так же, как это делает браузер: адрес
// целиком, без подстановок testutil (тому нужен префикс /api и путь без кодировки).
func getStatic(t *testing.T, e *echo.Echo, url string, headers http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	for name, values := range headers {
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// TestApplicationScans_StaticGate: раздача сканов заявок отдаёт файл только тем,
// кому открыта сама заявка (#2465). До этого хватало любого входа в систему -- кто
// узнал имя файла на диске, тот и забирал чужой скан документа мимо всех проверок.
func TestApplicationScans_StaticGate(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	ownerToken := testutil.RegisterAndLogin(t, e, "scanowner", "pass123", 1, td.OrgID, td.CompanyID)
	file := uploadDraftFile(t, e, ownerToken, "паспорт.png", realPNG(t))
	rec := submitWithFiles(t, e, db, ownerToken, "scangate", []int{file.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	stored := storedNameOf(t, db, file.ID)

	strangerToken := testutil.RegisterAndLogin(t, e, "scanstranger", "pass123", 1, 0, 0)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	t.Run("владелец заявки получает свой скан", func(t *testing.T) {
		r := getStatic(t, e, scanURL(stored), testutil.AuthHeader(ownerToken))
		require.Equal(t, http.StatusOK, r.Code, r.Body.String())
		require.NotEmpty(t, r.Body.Bytes(), "файл должен отдаться содержимым")
	})

	t.Run("посторонний не отличает чужой скан от несуществующего", func(t *testing.T) {
		foreign := getStatic(t, e, scanURL(stored), testutil.AuthHeader(strangerToken))
		missing := getStatic(t, e, scanURL("00000000-0000-4000-8000-000000000000.png"),
			testutil.AuthHeader(strangerToken))

		require.Equal(t, http.StatusNotFound, foreign.Code, foreign.Body.String())
		require.Equal(t, http.StatusNotFound, missing.Code, missing.Body.String())
		// 403 подтвердил бы, что файл с таким именем есть, и превратил бы раздачу
		// в оракул подбора имён.
		require.Equal(t, missing.Body.String(), foreign.Body.String(),
			"чужой и несуществующий файл должны отвечать неотличимо")
		require.Equal(t, missing.Header().Get("Content-Type"), foreign.Header().Get("Content-Type"))
	})

	t.Run("суперадминистратор проходит", func(t *testing.T) {
		r := getStatic(t, e, scanURL(stored), testutil.AuthHeader(adminToken))
		require.Equal(t, http.StatusOK, r.Code, r.Body.String())
	})

	t.Run("без входа в систему по-прежнему 401", func(t *testing.T) {
		r := getStatic(t, e, scanURL(stored), nil)
		require.Equal(t, http.StatusUnauthorized, r.Code, r.Body.String())
	})

	t.Run("cookie продления сеанса проверяется так же", func(t *testing.T) {
		_, ownerRefresh := testutil.LoginUser(t, e, "scanowner", "pass123")
		_, strangerRefresh := testutil.LoginUser(t, e, "scanstranger", "pass123")
		require.NotEmpty(t, ownerRefresh)
		require.NotEmpty(t, strangerRefresh)

		byCookie := func(value string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodGet, scanURL(stored), nil)
			req.AddCookie(&http.Cookie{Name: services.RefreshCookieName, Value: value})
			r := httptest.NewRecorder()
			e.ServeHTTP(r, req)
			return r
		}

		require.Equal(t, http.StatusOK, byCookie(ownerRefresh).Code)
		require.Equal(t, http.StatusNotFound, byCookie(strangerRefresh).Code,
			"cookie тоже называет пользователя, значит проверка принадлежности применима")
	})

	t.Run("процентное кодирование не проносит путь мимо гейта", func(t *testing.T) {
		// Раздача echo снимает кодировку сама, уже после выбора маршрута: путь
		// вида a/%2e%2e/application_files/x доезжает до того же файла.
		sneaky := "/api/uploads/unload_places/%2e%2e/application_files/" + stored

		owner := getStatic(t, e, sneaky, testutil.AuthHeader(ownerToken))
		require.Equal(t, http.StatusOK, owner.Code,
			"путь должен вести к тому же файлу, иначе проверка ниже ничего не значит")

		stranger := getStatic(t, e, sneaky, testutil.AuthHeader(strangerToken))
		require.Equal(t, http.StatusNotFound, stranger.Code, stranger.Body.String())
	})
}

// TestApplicationScans_DraftBelongsToUploader: незавершённая подача -- файл уже на
// диске, а заявки ещё нет. Проверять через неё нечего, поэтому черновик открыт
// только загрузившему.
func TestApplicationScans_DraftBelongsToUploader(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	ownerToken := testutil.RegisterAndLogin(t, e, "draftscanowner", "pass123", 1, td.OrgID, td.CompanyID)
	file := uploadDraftFile(t, e, ownerToken, "черновик.png", realPNG(t))

	var row models.ApplicationFile
	require.NoError(t, db.First(&row, file.ID).Error)
	require.Nil(t, row.ApplicationID, "файл должен остаться черновиком")

	strangerToken := testutil.RegisterAndLogin(t, e, "draftscanstranger", "pass123", 1, 0, 0)

	require.Equal(t, http.StatusOK,
		getStatic(t, e, scanURL(row.StoredName), testutil.AuthHeader(ownerToken)).Code)
	require.Equal(t, http.StatusNotFound,
		getStatic(t, e, scanURL(row.StoredName), testutil.AuthHeader(strangerToken)).Code)
}

// TestApplicationScans_OtherUploadDirsUnchanged: гейт срабатывает только на каталоге
// сканов. Фото мест разгрузки персональных данных не содержат, привязки к заявке у
// них нет -- проверка принадлежности сломала бы их показ всем, кроме автора.
func TestApplicationScans_OtherUploadDirsUnchanged(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/unload-places", `{"name":"Scan Gate Regression"}`, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	placeID := int(testutil.ParseMap(t, rec)["id"].(float64))

	body, ctype := multipartPhoto(t, "photos", "place.png")
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/unload-places/%d/photos", placeID), body)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	up := httptest.NewRecorder()
	e.ServeHTTP(up, req)
	require.Equal(t, http.StatusOK, up.Code, up.Body.String())

	details := testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/unload-places/%d", placeID), testutil.AuthHeader(adminToken)))
	photos, ok := details["photos"].([]interface{})
	require.True(t, ok)
	require.Len(t, photos, 1)
	photoURL, _ := photos[0].(map[string]interface{})["photo_url"].(string)
	require.Contains(t, photoURL, "/api/uploads/unload_places/")

	// Фото открыто любому вошедшему, в том числе не заводившему это место.
	strangerToken := testutil.RegisterAndLogin(t, e, "photoviewer", "pass123", 1, 0, 0)
	r := getStatic(t, e, photoURL, testutil.AuthHeader(strangerToken))
	require.Equal(t, http.StatusOK, r.Code, r.Body.String())
	require.NotEmpty(t, r.Body.Bytes())
}
