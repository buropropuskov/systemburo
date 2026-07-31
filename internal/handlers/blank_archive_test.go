package handlers_test

import (
	"fmt"
	"net/http"
	"strings"
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

	first := testutil.PUT(t, e, "/file-archive/settings", `{"quota_bytes":123456789,"warn_percent":70}`, adminH)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	second := testutil.PUT(t, e, "/file-archive/settings", `{"enabled":true,"dir_template":"{год}/{дата} {организация}"}`, adminH)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())

	got := testutil.ParseResponse[models.ArchiveSettings](t, second)
	assert.True(t, got.Enabled)
	assert.Equal(t, "{год}/{дата} {организация}", got.DirTemplate)
	assert.Equal(t, int64(123456789), got.QuotaBytes, "квота из прошлого запроса обязана уцелеть")
	assert.Equal(t, 70, got.WarnPercent, "порог из прошлого запроса обязан уцелеть")

	// Прочитанное следующим запросом совпадает с отданным: настройки читаются из БД,
	// а не из кэша процесса, иначе воркер увидел бы правку только после перезапуска.
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

	cases := []struct{ name, body string }{
		{"неизвестный плейсхолдер", `{"dir_template":"{год}/{номер_машины}"}`},
		{"тип вложения в имени папки", `{"dir_template":"{год}/{тип}"}`},
		{"шаблон папок без уровней", `{"dir_template":"   "}`},
		{"пустой шаблон имени файла", `{"file_template":""}`},
		{"порог предупреждения вне диапазона", `{"warn_percent":0}`},
		{"окно сверки вне диапазона", `{"recheck_days":0}`},
		{"отрицательная квота", `{"quota_bytes":-1}`},
		{"нулевой потолок выгрузки", `{"zip_max_bytes":0}`},
		{"отрицательный срок заморозки", `{"freeze_after_days":-1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := testutil.PUT(t, e, "/file-archive/settings", tc.body, adminH)
			require.Equalf(t, http.StatusBadRequest, rec.Code,
				"%s должен отклоняться, получили %d: %s", tc.name, rec.Code, rec.Body.String())
		})
	}

	// Ни одна отклонённая правка не должна была записаться.
	rec := testutil.GET(t, e, "/file-archive/settings", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	got := testutil.ParseResponse[models.ArchiveSettings](t, rec)
	assert.Equal(t, blankpath.DefaultDirTemplate, got.DirTemplate)
	assert.Equal(t, 80, got.WarnPercent)
}

// Изменение настроек попадает в общий журнал: своей *_history таблицы у архива нет.
func TestFileArchive_Settings_UpdateWritesAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	rec := testutil.PUT(t, e, "/file-archive/settings", `{"enabled":true}`, adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var entries []models.AuditLog
	require.NoError(t, db.Where("entity_type = ?", models.AuditEntityArchiveSettings).Find(&entries).Error)
	require.Len(t, entries, 1, "включение выгрузки обязано попасть в журнал")
	assert.Contains(t, string(entries[0].Details), `"enabled"`)

	// Повторное сохранение того же значения ничего не меняет - и записи о нём нет:
	// журнал «изменено» без изменения врёт при разборе, кто и что настроил.
	repeat := testutil.PUT(t, e, "/file-archive/settings", `{"enabled":true}`, adminH)
	require.Equal(t, http.StatusOK, repeat.Code)
	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ?", models.AuditEntityArchiveSettings).Count(&count).Error)
	assert.EqualValues(t, 1, count, "сохранение без изменений не должно плодить записи")
}

// Превью обязано работать на базе без заявок: раскладку настраивают до первой подачи.
func TestFileArchive_Preview_SyntheticOnEmptyBase(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	rec := testutil.POST(t, e, "/file-archive/preview", `{}`, adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := testutil.ParseResponse[models.ArchivePreviewResponse](t, rec)
	assert.True(t, got.Synthetic, "без заявок превью строится на значениях-образцах")
	assert.NotEmpty(t, got.Levels)
	assert.True(t, strings.HasSuffix(got.FileName, ".xlsx"), "имя файла: %s", got.FileName)
	assert.Contains(t, got.RelPath, got.FileName)
	assert.Empty(t, got.DirProblems)
	assert.Empty(t, got.FileProblems)
}

// Претензии к шаблону отдаются отдельно от ошибки: конструктор подсвечивает
// проблемный плейсхолдер и продолжает показывать путь по остальным.
func TestFileArchive_Preview_ReportsTemplateProblems(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	body := `{"dir_template":"{год}/{выдумка}","file_template":"{тип} {ерунда}"}`
	rec := testutil.POST(t, e, "/file-archive/preview", body, adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := testutil.ParseResponse[models.ArchivePreviewResponse](t, rec)
	require.Len(t, got.DirProblems, 1)
	assert.Equal(t, "выдумка", got.DirProblems[0].Token)
	require.Len(t, got.FileProblems, 1)
	assert.Equal(t, "ерунда", got.FileProblems[0].Token)
	assert.NotEmpty(t, got.Levels, "путь по остальным плейсхолдерам показывается всё равно")
}

// На реальной заявке превью показывает её данные, а слэш из номера не создаёт уровень.
func TestFileArchive_Preview_RealApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	appID := seedArchiveApplication(t, db, td.OrgID, "№ 20260731/001")
	body := fmt.Sprintf(`{"dir_template":"{год}/{дата} №{номер}","file_template":"{тип} - {организация}","application_id":%d}`, appID)
	rec := testutil.POST(t, e, "/file-archive/preview", body, adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := testutil.ParseResponse[models.ArchivePreviewResponse](t, rec)
	assert.False(t, got.Synthetic)
	require.Len(t, got.Levels, 2, "слэш из номера заявки не должен создавать лишний уровень: %v", got.Levels)
	assert.Equal(t, "2026", got.Levels[0])
	assert.Equal(t, "31.07.2026 №20260731-001", got.Levels[1],
		"знак номера не удваивается, а слэш заменяется дефисом")
	assert.Equal(t, "Тип для архива - Test Organization.xlsx", got.FileName)
}

func TestFileArchive_Tokens_ScopeFlags(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	rec := testutil.GET(t, e, "/file-archive/tokens", adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	tokens := testutil.ParseResponse[[]blankpath.Token](t, rec)
	require.NotEmpty(t, tokens)
	byKey := map[string]blankpath.Token{}
	for _, tok := range tokens {
		byKey[tok.Key] = tok
	}
	require.Contains(t, byKey, "тип")
	assert.False(t, byKey["тип"].DirAllowed, "тип вложения в имени папки не имеет смысла")
	assert.True(t, byKey["тип"].FileAllowed)
	require.Contains(t, byKey, "год")
	assert.True(t, byKey["год"].DirAllowed)
}

// Реестр архива не должен допускать две строки, указывающие на один файл. Индекс
// частичный: до первой удачной записи rel_dir пуст, и таких строк в очереди много.
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
