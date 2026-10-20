package handlers_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestApplicationDraft_APIQuotaAndOwnList(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "draftquotaapi", "pass123", 1, td.OrgID, td.CompanyID)
	var owner models.User
	require.NoError(t, db.Where("username = ?", "draftquotaapi").First(&owner).Error)
	rows := make([]models.ApplicationFile, 50)
	for i := range rows {
		rows[i] = models.ApplicationFile{UploadedBy: owner.ID, FileName: "own.pdf", StoredName: "own.pdf", FileSize: 10}
	}
	require.NoError(t, db.Create(&rows).Error)
	res := testutil.GET(t, e, "/applications/files/drafts?per_page=2&user_id=1", testutil.AuthHeader(token))
	require.Equal(t, 200, res.Code, res.Body.String())
	page := testutil.ParseResponse[services.DraftFilesPage](t, res)
	require.Len(t, page.Items, 2)
	require.EqualValues(t, 50, page.Usage.SavedCount)
	require.EqualValues(t, 50, page.Limits.MaxCount)
	require.NotContains(t, res.Body.String(), "stored_name")
	post := func(parts []string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for _, content := range parts {
			part, err := writer.CreateFormFile("files", "example.pdf")
			require.NoError(t, err)
			_, err = part.Write([]byte(content))
			require.NoError(t, err)
		}
		require.NoError(t, writer.Close())
		req := httptest.NewRequest(http.MethodPost, "/api/applications/files", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		result := httptest.NewRecorder()
		e.ServeHTTP(result, req)
		return result
	}
	rejected := post([]string{"%PDF-1.4\nexample\n%%EOF"})
	require.Equal(t, 409, rejected.Code, rejected.Body.String())
	require.Contains(t, rejected.Body.String(), "файлов-черновиков")
	require.Equal(t, 200, testutil.DELETE(t, e, "/applications/files/"+strconv.Itoa(rows[0].ID), testutil.AuthHeader(token)).Code)
	accepted := post([]string{"%PDF-1.4\nexample\n%%EOF"})
	require.Equal(t, 200, accepted.Code, accepted.Body.String())
	var reservations int64
	require.NoError(t, db.Model(&models.ApplicationFileReservation{}).Count(&reservations).Error)
	require.Zero(t, reservations)
	require.Equal(t, 401, testutil.GET(t, e, "/applications/files/drafts", nil).Code)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.SetRequest(c.Request().WithContext(services.WithImpersonator(c.Request().Context(), 1)))
			return next(c)
		}
	})
	require.Equal(t, 403, testutil.GET(t, e, "/applications/files/drafts", testutil.AuthHeader(token)).Code)
}
