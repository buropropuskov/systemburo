package handlers_test

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestApplicationDraft_InvalidBatchLeavesNoRows(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "draftbatch", "pass123", 1, td.OrgID, td.CompanyID)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, content := range []string{"%PDF-1.4\nexample\n%%EOF", "not an allowed document"} {
		part, err := writer.CreateFormFile("files", "example.pdf")
		require.NoError(t, err)
		_, err = part.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	request := httptest.NewRequest("POST", "/api/applications/files", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	require.Equal(t, 400, response.Code, response.Body.String())
	var files, reservations int64
	require.NoError(t, db.Model(&models.ApplicationFile{}).Count(&files).Error)
	require.Zero(t, files)
	require.NoError(t, db.Model(&models.ApplicationFileReservation{}).Count(&reservations).Error)
	require.Zero(t, reservations)
}
