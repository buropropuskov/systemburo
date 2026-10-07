package handlers_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"systemburo/internal/handlers"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func uploadHeader(t *testing.T, data []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	p, err := w.CreateFormFile("file", "template.xlsx")
	require.NoError(t, err)
	_, err = p.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	form, err := multipart.NewReader(&body, w.Boundary()).ReadForm(1 << 20)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, form.RemoveAll()) })
	return form.File["file"][0]
}

func TestAttachmentTemplateUploadValidationPreservesActive(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	uaID, oldID := seedListTemplate(t, db, "validation_template", "people", 6, 20)
	var user models.User
	require.NoError(t, db.First(&user).Error)
	dir := t.TempDir()
	book := excelize.NewFile()
	buf, err := book.WriteToBuffer()
	require.NoError(t, err)
	require.NoError(t, book.Close())
	data := buf.Bytes()
	svc := services.NewAttachmentTemplateService(db, dir, int64(len(data)))
	for _, tc := range []struct {
		name     string
		data     []byte
		fakeSize bool
	}{{"text", []byte("not xlsx"), false}, {"broken_zip", []byte("PK\x03\x04broken"), false}, {"over_limit", append(append([]byte(nil), data...), 0), false}, {"actual_over_limit", append(append([]byte(nil), data...), 0), true}} {
		t.Run(tc.name, func(t *testing.T) {
			header := uploadHeader(t, tc.data)
			if tc.fakeSize {
				header.Size = 1
			}
			_, err := svc.Upload(context.Background(), uaID, header, models.CreateTemplateRequest{}, user.ID)
			require.Error(t, err)
			var old models.AttachmentTemplate
			require.NoError(t, db.First(&old, oldID).Error)
			require.True(t, old.IsActive)
			entries, err := os.ReadDir(filepath.Join(dir, "templates"))
			if !os.IsNotExist(err) {
				require.NoError(t, err)
				require.Empty(t, entries)
			}
		})
	}
	newTemplate, err := svc.Upload(context.Background(), uaID, uploadHeader(t, data), models.CreateTemplateRequest{}, user.ID)
	require.NoError(t, err)
	require.True(t, newTemplate.IsActive)
	var old models.AttachmentTemplate
	require.NoError(t, db.First(&old, oldID).Error)
	require.False(t, old.IsActive)
	// Transaction failure restores the old active template and removes the new file.
	before, err := os.ReadDir(filepath.Join(dir, "templates"))
	require.NoError(t, err)
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:template_insert_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "attachment_templates" {
			tx.AddError(errors.New("synthetic insert failure"))
		}
	}))
	defer db.Callback().Create().Remove("test:template_insert_failure")
	_, err = svc.Upload(context.Background(), uaID, uploadHeader(t, data), models.CreateTemplateRequest{}, user.ID)
	require.Error(t, err)
	var active models.AttachmentTemplate
	require.NoError(t, db.First(&active, newTemplate.ID).Error)
	require.True(t, active.IsActive)
	after, err := os.ReadDir(filepath.Join(dir, "templates"))
	require.NoError(t, err)
	require.Len(t, after, len(before))
}

func TestApplicationFileReadPathContainment(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	var user models.User
	require.NoError(t, db.First(&user).Error)
	app := models.Application{OrganizationID: td.OrgID, SenderUserID: user.ID}
	require.NoError(t, db.Create(&app).Error)
	root, outside := t.TempDir(), t.TempDir()
	svc := services.NewApplicationFileService(db, root, nil)
	require.NoError(t, os.Mkdir(svc.Dir(), 0700))
	data := []byte("synthetic content")
	external := filepath.Join(outside, "retained.pdf")
	require.NoError(t, os.WriteFile(external, data, 0600))
	require.NoError(t, os.Symlink(external, filepath.Join(svc.Dir(), "linked.pdf")))
	for _, name := range []string{external, "../retained.pdf", "linked.pdf"} {
		row := models.ApplicationFile{ApplicationID: &app.ID, UploadedBy: user.ID, StoredName: name, FileName: "synthetic.pdf"}
		require.NoError(t, db.Create(&row).Error)
		_, _, err := svc.Locate(context.Background(), app.ID, row.ID)
		require.Error(t, err)
		_, err = svc.ReadContent(context.Background(), row.ID)
		require.Error(t, err)
		svc.DiscardStored(name)
		got, err := os.ReadFile(external)
		require.NoError(t, err)
		require.Equal(t, data, got)
	}
	require.NoError(t, os.WriteFile(filepath.Join(svc.Dir(), "valid.pdf"), data, 0600))
	row := models.ApplicationFile{ApplicationID: &app.ID, UploadedBy: user.ID, StoredName: "valid.pdf", FileName: "synthetic.pdf"}
	require.NoError(t, db.Create(&row).Error)
	got, err := svc.ReadContent(context.Background(), row.ID)
	require.NoError(t, err)
	require.Equal(t, data, got)
	// Storage subdirectory symlink is below the configured root boundary.
	root2 := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(root2, services.ApplicationFilesDir)))
	svc2 := services.NewApplicationFileService(db, root2, nil)
	require.NoError(t, db.Model(&row).Update("stored_name", "retained.pdf").Error)
	_, err = svc2.ReadContent(context.Background(), row.ID)
	require.Error(t, err)
	svc2.DiscardStored("retained.pdf")
	got, err = os.ReadFile(external)
	require.NoError(t, err)
	require.Equal(t, data, got)
}

func TestSystemTablePhotoBatchRollback(t *testing.T) { photoBatchRollback(t, true) }
func TestUnloadPlacePhotoBatchRollback(t *testing.T) { photoBatchRollback(t, false) }

func photoBatchRollback(t *testing.T, system bool) {
	t.Helper()
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	path, body, parentTable, photoTable := "/unload-places", `{"name":"batch"}`, "unload_places", "unload_place_photos"
	if system {
		path, body, parentTable, photoTable = "/system-tables", `{"name":"batch_table","display_name":"Batch","table_type":"passage"}`, "system_tables", "system_table_photos"
	}
	rec := testutil.POST(t, e, path, body, testutil.AuthHeader(token))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	id := int(testutil.ParseMap(t, rec)["id"].(float64))
	var username string
	require.NoError(t, db.Table("users").Select("username").Limit(1).Row().Scan(&username))
	uploadRoot := t.TempDir()
	dir := filepath.Join(uploadRoot, parentTable)
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.png"), pngBytes, 0600))
	var handle func(echo.Context) error
	if system {
		svc := services.NewSystemTableService(db, t.TempDir(), 1024, nil)
		handle = handlers.NewSystemTableHandler(svc, nil, 1024, uploadRoot).UploadPhoto
	} else {
		svc := services.NewUnloadPlaceService(db)
		handle = handlers.NewUnloadPlaceHandler(svc, 1024, uploadRoot).UploadPhoto
	}
	invoke := func(parentID int) error {
		var data bytes.Buffer
		w := multipart.NewWriter(&data)
		for i := 0; i < 2; i++ {
			p, err := w.CreateFormFile("photos", fmt.Sprintf("photo%d.png", i))
			require.NoError(t, err)
			_, err = p.Write(pngBytes)
			require.NoError(t, err)
		}
		require.NoError(t, w.Close())
		r := httptest.NewRequest("POST", "/", &data)
		r.Header.Set("Content-Type", w.FormDataContentType())
		c := echo.New().NewContext(r, httptest.NewRecorder())
		c.SetParamNames("id")
		c.SetParamValues(strconv.Itoa(parentID))
		c.Set("username", username)
		return handle(c)
	}
	require.Error(t, invoke(id+1000000))
	require.NoError(t, db.Table(parentTable).Where("id = ?", id).Update("is_active", false).Error)
	require.Error(t, invoke(id))
	require.NoError(t, db.Table(parentTable).Where("id = ?", id).Update("is_active", true).Error)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	created := 0
	callback := "test:photo_batch_failure"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == photoTable {
			created++
			if created == 2 {
				tx.AddError(errors.New("synthetic second photo insert failure"))
			}
		}
	}))
	require.Error(t, invoke(id))
	require.NoError(t, db.Callback().Create().Remove(callback))
	require.Equal(t, 2, created)
	var count int64
	require.NoError(t, db.Table(photoTable).Count(&count).Error)
	require.Zero(t, count)
	entries, err = os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "existing.png", entries[0].Name())
	require.NoError(t, invoke(id))
	require.NoError(t, db.Table(photoTable).Count(&count).Error)
	require.EqualValues(t, 2, count)
	var mainCount int64
	require.NoError(t, db.Table(photoTable).Where("is_main = ?", true).Count(&mainCount).Error)
	require.EqualValues(t, 1, mainCount)
	created = 0
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == photoTable {
			created++
			if created == 2 {
				tx.AddError(errors.New("synthetic second photo insert failure"))
			}
		}
	}))
	defer db.Callback().Create().Remove(callback)
	require.Error(t, invoke(id))
	require.NoError(t, db.Table(photoTable).Count(&count).Error)
	require.EqualValues(t, 2, count)
	entries, err = os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 3)
}
