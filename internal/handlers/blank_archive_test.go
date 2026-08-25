package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/blankpath"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Настройки файлового архива и превью раскладки (#1615, срез A4).

func TestFileArchive_Settings_DefaultsOnEmptyBase(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	rec := testutil.GET(t, e, "/file-archive/settings", adminH)
	require.Equal(t, http.StatusOK, rec.Code, "раздел должен открываться до первой настройки: %s", rec.Body.String())

	got := testutil.ParseResponse[models.ArchiveSettings](t, rec)
	assert.False(t, got.Enabled, "выгрузка по умолчанию выключена: включение начинает писать ПД на диск")
	assert.Equal(t, blankpath.DefaultDirTemplate, got.DirTemplate)
	assert.Equal(t, blankpath.DefaultFileTemplate, got.FileTemplate)
	assert.Equal(t, 80, got.WarnPercent)
	assert.Equal(t, 30, got.RecheckDays)
	assert.Equal(t, 30, got.FreezeAfterDays)
	assert.Equal(t, int64(2<<30), got.MinFreeBytes)
	assert.Equal(t, int64(2<<30), got.ZipMaxBytes)
	assert.Zero(t, got.QuotaBytes, "квота по умолчанию не ограничивает")
}

// Правка одного поля не должна сбрасывать соседние: форма настроек сохраняет их по
// одному, и отсутствие ключа в запросе означает «не трогать».
func TestFileArchive_Settings_PartialUpdateKeepsOtherFields(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	testutil.SetArchiveSettings(t, db, models.UpdateArchiveSettingsRequest{QuotaBytes: testutil.Ptr[int64](123456789), WarnPercent: testutil.Ptr(70)})

	testutil.SetArchiveSettings(t, db, models.UpdateArchiveSettingsRequest{Enabled: testutil.Ptr(true), DirTemplate: testutil.Ptr("{год}/{дата} {организация}")})

	rec := testutil.GET(t, e, "/file-archive/settings", adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := testutil.ParseResponse[models.ArchiveSettings](t, rec)
	assert.True(t, got.Enabled)
	assert.Equal(t, "{год}/{дата} {организация}", got.DirTemplate)
	assert.Equal(t, int64(123456789), got.QuotaBytes, "квота из прошлого запроса обязана уцелеть")
	assert.Equal(t, 70, got.WarnPercent, "порог из прошлого запроса обязан уцелеть")

	// Раздел читает настройки из БД, а не из кэша процесса: иначе правка командой на
	// сервере доехала бы до интерфейса только после перезапуска.
	after := testutil.GET(t, e, "/file-archive/settings", adminH)
	require.Equal(t, http.StatusOK, after.Code)
	assert.Equal(t, got, testutil.ParseResponse[models.ArchiveSettings](t, after))
}

func TestFileArchive_Settings_RejectsBrokenTemplatesAndNumbers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	cases := []struct {
		name string
		req  models.UpdateArchiveSettingsRequest
	}{
		{"неизвестный плейсхолдер", models.UpdateArchiveSettingsRequest{DirTemplate: testutil.Ptr("{год}/{номер_машины}")}},
		{"тип вложения в имени папки", models.UpdateArchiveSettingsRequest{DirTemplate: testutil.Ptr("{год}/{тип}")}},
		{"шаблон папок без уровней", models.UpdateArchiveSettingsRequest{DirTemplate: testutil.Ptr("   ")}},
		{"пустой шаблон имени файла", models.UpdateArchiveSettingsRequest{FileTemplate: testutil.Ptr("")}},
		{"порог предупреждения вне диапазона", models.UpdateArchiveSettingsRequest{WarnPercent: testutil.Ptr(0)}},
		{"окно сверки вне диапазона", models.UpdateArchiveSettingsRequest{RecheckDays: testutil.Ptr(0)}},
		{"отрицательная квота", models.UpdateArchiveSettingsRequest{QuotaBytes: testutil.Ptr[int64](-1)}},
		{"нулевой потолок выгрузки", models.UpdateArchiveSettingsRequest{ZipMaxBytes: testutil.Ptr[int64](0)}},
		{"отрицательный срок заморозки", models.UpdateArchiveSettingsRequest{FreezeAfterDays: testutil.Ptr(-1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Errorf(t, testutil.TryArchiveSettings(db, tc.req),
				"%s должен отклоняться", tc.name)
		})
	}

	// Ни одна отклонённая правка не должна была записаться.
	rec := testutil.GET(t, e, "/file-archive/settings", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	got := testutil.ParseResponse[models.ArchiveSettings](t, rec)
	assert.Equal(t, blankpath.DefaultDirTemplate, got.DirTemplate)
	assert.Equal(t, 80, got.WarnPercent)
}

func TestBlankExport_PathUniquenessIsPartial(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	queued := []models.BlankExport{
		{ApplicationID: 9001, AttachmentID: 8001, BucketDate: time.Now(), Status: models.BlankExportPending, QueuedAt: time.Now()},
		{ApplicationID: 9002, AttachmentID: 8002, BucketDate: time.Now(), Status: models.BlankExportPending, QueuedAt: time.Now()},
	}
	for i := range queued {
		require.NoError(t, db.Create(&queued[i]).Error, "строки без пути не конфликтуют между собой")
	}

	written := models.BlankExport{
		ApplicationID: 9003, AttachmentID: 8003, BucketDate: time.Now(),
		Status: models.BlankExportOK, QueuedAt: time.Now(),
		RelDir: "2026/7 ИЮЛЬ 2026/31.07.2026", FileName: "Заявка.xlsx",
	}
	require.NoError(t, db.Create(&written).Error)

	duplicate := models.BlankExport{
		ApplicationID: 9004, AttachmentID: 8004, BucketDate: time.Now(),
		Status: models.BlankExportOK, QueuedAt: time.Now(),
		RelDir: written.RelDir, FileName: written.FileName,
	}
	require.Error(t, db.Create(&duplicate).Error, "две строки на один файл диска недопустимы")
}

// seedArchiveApplication заводит заявку с одним вложением напрямую в базе: превью
// читает готовые данные, и прогон подачи целиком тут ничего бы не проверил.
func seedArchiveApplication(t *testing.T, db *gorm.DB, orgID int, number string) int {
	t.Helper()

	var senderID int
	require.NoError(t, db.Raw(`SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&senderID).Error)
	require.NotZero(t, senderID, "нужен хотя бы один пользователь-отправитель")

	sent := time.Date(2026, 7, 31, 9, 15, 0, 0, time.UTC)
	status := "В работе"
	app := models.Application{
		ApplicationNumber: &number,
		SendingDatetime:   &sent,
		OrganizationID:    orgID,
		SenderUserID:      senderID,
		Status:            &status,
	}
	require.NoError(t, db.Create(&app).Error)

	name := "Тип для архива"
	unique := models.UniqueAttachment{AttachmentType: "cars", Name: &name, DisplayName: &name, Title: &name, IsActive: true}
	require.NoError(t, db.Create(&unique).Error)

	attachment := models.Attachment{
		ApplicationID:      &app.ID,
		AttachmentType:     "cars",
		AttachmentName:     &name,
		UniqueAttachmentID: &unique.ID,
	}
	require.NoError(t, db.Create(&attachment).Error)
	return app.ID
}

// Настройку архива из веба поменять нельзя: раскладка каталогов и пороги места
// задаются командой server archive на сервере (#1615). Роут записи удалён совсем,
// а не закрыт правом - захваченная сессия администратора не должна быть способом
// увести файлы в другой каталог или снять ограничение объёма. Гвард против того,
// чтобы роут вернули «для удобства».
func TestFileArchive_SettingsAreReadOnlyOverHTTP(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPut, "/file-archive/settings", `{"enabled":true}`},
		{http.MethodPost, "/file-archive/preview", `{}`},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var rec *httptest.ResponseRecorder
			if tc.method == http.MethodPut {
				rec = testutil.PUT(t, e, tc.path, tc.body, adminH)
			} else {
				rec = testutil.POST(t, e, tc.path, tc.body, adminH)
			}
			assert.Equal(t, http.StatusNotFound, rec.Code,
				"роут записи настроек обязан отсутствовать, а не отвечать %d", rec.Code)
		})
	}

	// Чтение остаётся: дежурный видит, куда пишутся файлы, не заходя на сервер.
	rec := testutil.GET(t, e, "/file-archive/settings", adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := testutil.ParseResponse[models.ArchiveSettings](t, rec)
	assert.NotEmpty(t, got.DirTemplate, "раздел показывает действующую раскладку")
}
