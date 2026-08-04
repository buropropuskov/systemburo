package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Пороги места и сводка файлового архива (#1615, срез B2): EnforceThresholds,
// остановка очереди, предупреждение администраторам, GET /file-archive/stats.
//
// Сервис поднимается на db из SetupTestApp, а не тестируется в своём пакете:
// DB-backed тест живёт только в handlers (#706) - второй бинарь с базой делит с
// handlers одну тест-БД и роняет чужие тесты чисткой посреди чужого прогона.
//
// Секции живут на одном поднятом приложении: отдельный SetupTestApp на каждую
// перебивал границу go test -timeout у пакета handlers.

// quotaImpossibleFreeBytes - требование к свободному месту, которое не выполнит ни
// один реальный раздел (4 ЭБ). Так жёсткий порог по свободному месту проверяется, не
// заполняя диск: сервис сравнивает настройку с настоящим statfs каталога архива.
const quotaImpossibleFreeBytes = int64(1) << 62

// archiveBlockFlagSettingKey / archiveWarnFlagSettingKey - устойчивые отметки
// «действие перехода выполнено» из blank_export_quota.go. Тест знает их строками:
// именно факт, что отметка НЕ появилась после провала действия, и стережёт порядок
// «сначала действие, потом отметка».
const (
	archiveBlockFlagSettingKey = "archive.blocked_notified"
	archiveWarnFlagSettingKey  = "archive.warn_notified"
)

type quotaWorld struct {
	e         *echo.Echo
	db        *gorm.DB
	adminH    http.Header
	adminID   int
	regularID int
	archive   string
}

func setupQuotaWorld(t *testing.T) quotaWorld {
	t.Helper()
	e, db, archiveRoot, cleanup := testutil.SetupTestAppWithArchive(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	// Обычный пользователь - контроль аудитории предупреждения: права управления
	// архивом у него нет, значит и уведомления быть не должно.
	testutil.RegisterAndLogin(t, e, "quota_regular", "password123", 1, td.OrgID, td.CompanyID)

	return quotaWorld{
		e: e, db: db, adminH: adminH,
		adminID:   secUserIDByUsername(t, db, "testadmin"),
		regularID: secUserIDByUsername(t, db, "quota_regular"),
		archive:   archiveRoot,
	}
}

// newQuota собирает сервис места на той же базе, что и приложение. notifier и
// recorder подменяемые: провал уведомления и провал записи журнала - ровно те
// сбои, после которых отметка перехода не имеет права сохраниться.
func (w quotaWorld) newQuota(notifier services.NotificationService, recorder services.AuditRecorder) *services.BlankExportQuotaService {
	settings := services.NewSettingsService(w.db, &config.Config{PaginationMaxLimit: 100})
	return services.NewBlankExportQuotaService(
		w.db, settings, notifier, services.NewPermissionResolver(w.db), recorder,
		w.archive, w.archive, "")
}

// reset возвращает мир к состоянию «архив включён, пороги не заданы, журнал пуст».
// Пользователей и справочники не трогает: CleanDB отобрал бы у секции токен админа.
func (w quotaWorld) reset(t *testing.T) {
	t.Helper()
	require.NoError(t, w.db.Exec("DELETE FROM blank_exports").Error)
	require.NoError(t, w.db.Where("entity_type = ?", models.AuditEntityArchiveQuota).
		Delete(&models.AuditLog{}).Error)
	require.NoError(t, w.db.Exec("DELETE FROM notifications").Error)
	require.NoError(t, w.db.Exec("DELETE FROM system_settings WHERE key IN (?, ?)",
		archiveBlockFlagSettingKey, archiveWarnFlagSettingKey).Error)
	w.setSettings(t, models.UpdateArchiveSettingsRequest{Enabled: testutil.Ptr(true), MinFreeBytes: testutil.Ptr[int64](0), QuotaBytes: testutil.Ptr[int64](0), WarnPercent: testutil.Ptr(99)})
}

func (w quotaWorld) setSettings(t *testing.T, req models.UpdateArchiveSettingsRequest) {
	t.Helper()
	testutil.SetArchiveSettings(t, w.db, req)
}

func (w quotaWorld) auditRows(t *testing.T, action string) []models.AuditLog {
	t.Helper()
	var rows []models.AuditLog
	require.NoError(t, w.db.Where("entity_type = ? AND action = ?", models.AuditEntityArchiveQuota, action).
		Order("id").Find(&rows).Error)
	return rows
}

func (w quotaWorld) registryRowByID(t *testing.T, id int) models.BlankExport {
	t.Helper()
	var row models.BlankExport
	require.NoError(t, w.db.First(&row, id).Error)
	return row
}

// flagValue отдаёт значение устойчивой отметки порога; пусто, если строки нет.
func (w quotaWorld) flagValue(t *testing.T, key string) string {
	t.Helper()
	var values []string
	require.NoError(t, w.db.Table("system_settings").Where("key = ?", key).Pluck("value", &values).Error)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (w quotaWorld) notificationsOfType(t *testing.T, notifType string) []models.Notification {
	t.Helper()
	var rows []models.Notification
	require.NoError(t, w.db.Where("type = ?", notifType).Order("id").Find(&rows).Error)
	return rows
}

// quotaSeedRow заводит строку реестра: статус, размер и месяц - всё, что читают
// сводка и блокировка очереди.
func quotaSeedRow(t *testing.T, db *gorm.DB, appID, attID int, status string, size int64, bucket time.Time) models.BlankExport {
	t.Helper()
	row := models.BlankExport{
		ApplicationID: appID, AttachmentID: attID,
		BucketDate: bucket, Status: status, SizeBytes: size,
		QueuedAt: time.Now(),
	}
	if status == models.BlankExportOK {
		row.RelDir = fmt.Sprintf("%d/%02d", bucket.Year(), int(bucket.Month()))
		row.FileName = fmt.Sprintf("Бланк-%d-%d.xlsx", appID, attID)
	}
	require.NoError(t, db.Create(&row).Error)
	return row
}

// flakyRecorder ломает запись журнала на заданном действии. Нужен, чтобы
// смоделировать провал самого действия перехода: без него порядок «отметка до
// действия» ничем не отличим от правильного.
type flakyRecorder struct {
	services.AuditRecorder
	failAction string
}

var errQuotaAuditDown = errors.New("audit down")

func (r *flakyRecorder) Record(ctx context.Context, exec *gorm.DB, entityType string, entityID *int, action string, actorID *int, details interface{}) error {
	if action == r.failAction {
		return errQuotaAuditDown
	}
	return r.AuditRecorder.Record(ctx, exec, entityType, entityID, action, actorID, details)
}

func (r *flakyRecorder) Log(ctx context.Context, exec *gorm.DB, entityType string, entityID *int, action string, actorID *int, details interface{}) {
	_ = r.Record(ctx, exec, entityType, entityID, action, actorID, details)
}

// flakyNotifier роняет отправку уведомлений, пока включён fail.
type flakyNotifier struct {
	services.NotificationService
	fail bool
}

var errQuotaNotifierDown = errors.New("notifier down")

func (n *flakyNotifier) CreateForUser(ctx context.Context, userID int, notifType, title, message string, data *string) error {
	if n.fail {
		return errQuotaNotifierDown
	}
	return n.NotificationService.CreateForUser(ctx, userID, notifType, title, message, data)
}

func TestFileArchiveQuota(t *testing.T) {
	w := setupQuotaWorld(t)
	t.Run("нехватка места останавливает очередь", func(t *testing.T) { quotaBlocksQueueSection(t, w) })
	t.Run("объём архива достиг квоты", func(t *testing.T) { quotaExceededSection(t, w) })
	t.Run("провал блокировки повторяется следующим тиком", func(t *testing.T) { quotaRetriesBlockSection(t, w) })
	t.Run("предупреждение уходит носителям права", func(t *testing.T) { quotaWarnsAdminsSection(t, w) })
	t.Run("провал уведомления повторяется следующим тиком", func(t *testing.T) { quotaRetriesWarnSection(t, w) })
	t.Run("выключенный архив не сторожится", func(t *testing.T) { quotaDisabledSection(t, w) })
}

// Свободного места меньше минимума - очередь встаёт, журнал получает запись, а
// второй тик при том же состоянии ничего не повторяет. Возврат места снимает
// блокировку, и следующее пересечение порога снова останавливает очередь.
func quotaBlocksQueueSection(t *testing.T, w quotaWorld) {
	w.reset(t)
	bucket := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	pending := quotaSeedRow(t, w.db, 7101, 6101, models.BlankExportPending, 0, bucket)
	failed := quotaSeedRow(t, w.db, 7102, 6102, models.BlankExportFailed, 0, bucket)
	done := quotaSeedRow(t, w.db, 7103, 6103, models.BlankExportOK, 4096, bucket)
	noTemplate := quotaSeedRow(t, w.db, 7104, 6104, models.BlankExportNoTemplate, 0, bucket)

	w.setSettings(t, models.UpdateArchiveSettingsRequest{MinFreeBytes: testutil.Ptr(quotaImpossibleFreeBytes)})
	quota := w.newQuota(services.NewNotificationService(w.db), services.NewAuditRecorder(w.db))

	res, err := quota.EnforceThresholds(context.Background())
	require.NoError(t, err)
	require.True(t, res.HardTripped, "свободного места меньше минимума - порог обязан сработать")
	assert.EqualValues(t, 2, res.BlockedRows, "в blocked уходят только pending и failed")

	blocked := w.registryRowByID(t, pending.ID)
	assert.Equal(t, models.BlankExportBlocked, blocked.Status)
	assert.Contains(t, blocked.LastError, "insufficient_free_space", "причина остановки видна в строке: %s", blocked.LastError)
	require.NotNil(t, blocked.NextAttemptAt, "заблокированная строка обязана получить время следующей попытки")
	assert.WithinDuration(t, time.Now().Add(15*time.Minute), *blocked.NextAttemptAt, time.Minute)
	assert.Equal(t, models.BlankExportBlocked, w.registryRowByID(t, failed.ID).Status)
	assert.Equal(t, models.BlankExportOK, w.registryRowByID(t, done.ID).Status,
		"уже записанный файл блокировка не трогает: она про то, что ЕЩЁ предстоит записать")
	assert.Equal(t, models.BlankExportNoTemplate, w.registryRowByID(t, noTemplate.ID).Status,
		"строка без бланка ждёт администратора, а не места на диске")

	entries := w.auditRows(t, models.ArchiveQuotaActionBlocked)
	require.Len(t, entries, 1, "остановка очереди обязана попасть в журнал")
	var details struct {
		Reason string `json:"reason"`
		Rows   int64  `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(entries[0].Details, &details))
	assert.Equal(t, "insufficient_free_space", details.Reason)
	assert.EqualValues(t, 2, details.Rows)
	assert.Equal(t, "true", w.flagValue(t, archiveBlockFlagSettingKey), "отметка перехода обязана пережить рестарт процесса")

	// Второй тик при том же состоянии: порог всё ещё нарушен, но перехода не было.
	second, err := quota.EnforceThresholds(context.Background())
	require.NoError(t, err)
	assert.True(t, second.HardTripped)
	assert.Zero(t, second.BlockedRows, "повторный тик не блокирует заново")
	assert.Len(t, w.auditRows(t, models.ArchiveQuotaActionBlocked), 1, "вторая запись в журнале означала бы, что механизм срабатывает на каждом тике")

	// Рестарт процесса: свежий экземпляр читает отметку из system_settings и тоже
	// не должен повторять блокировку.
	restarted := w.newQuota(services.NewNotificationService(w.db), services.NewAuditRecorder(w.db))
	afterRestart, err := restarted.EnforceThresholds(context.Background())
	require.NoError(t, err)
	assert.Zero(t, afterRestart.BlockedRows, "отметка пережила рестарт - блокировать заново нечего")
	assert.Len(t, w.auditRows(t, models.ArchiveQuotaActionBlocked), 1)

	// Место вернулось.
	w.setSettings(t, models.UpdateArchiveSettingsRequest{MinFreeBytes: testutil.Ptr[int64](0)})
	back, err := restarted.EnforceThresholds(context.Background())
	require.NoError(t, err)
	assert.False(t, back.HardTripped)
	assert.Len(t, w.auditRows(t, models.ArchiveQuotaActionUnblocked), 1, "снятие блокировки видно администратору только этой записью")
	assert.Equal(t, "false", w.flagValue(t, archiveBlockFlagSettingKey))
	assert.Equal(t, models.BlankExportBlocked, w.registryRowByID(t, pending.ID).Status,
		"строки снимает с блокировки воркер по next_attempt_at, а не проверка порога")

	// Место снова кончилось - это новый переход, очередь обязана встать опять.
	w.setSettings(t, models.UpdateArchiveSettingsRequest{MinFreeBytes: testutil.Ptr(quotaImpossibleFreeBytes)})
	again, err := restarted.EnforceThresholds(context.Background())
	require.NoError(t, err)
	assert.True(t, again.HardTripped)
	assert.Len(t, w.auditRows(t, models.ArchiveQuotaActionBlocked), 2, "второе пересечение порога - второе событие")
}

// Второй жёсткий порог - объём самого архива. Сравнение нестрогое: архив ровно на
// потолке квоты уже исчерпал её.
func quotaExceededSection(t *testing.T, w quotaWorld) {
	w.reset(t)
	bucket := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	quotaSeedRow(t, w.db, 7201, 6201, models.BlankExportOK, 3000, bucket)
	quotaSeedRow(t, w.db, 7202, 6202, models.BlankExportOK, 2000, bucket)
	// Незаписанная строка в сумму объёма не входит - файла на диске ещё нет.
	pending := quotaSeedRow(t, w.db, 7203, 6203, models.BlankExportPending, 9000, bucket)

	w.setSettings(t, models.UpdateArchiveSettingsRequest{QuotaBytes: testutil.Ptr[int64](5001)})
	quota := w.newQuota(services.NewNotificationService(w.db), services.NewAuditRecorder(w.db))
	below, err := quota.EnforceThresholds(context.Background())
	require.NoError(t, err)
	assert.False(t, below.HardTripped, "5000 байт архива против квоты 5001 - порог не нарушен")
	assert.Equal(t, models.BlankExportPending, w.registryRowByID(t, pending.ID).Status)

	w.setSettings(t, models.UpdateArchiveSettingsRequest{QuotaBytes: testutil.Ptr[int64](5000)})
	atEdge, err := quota.EnforceThresholds(context.Background())
	require.NoError(t, err)
	require.True(t, atEdge.HardTripped, "архив ровно на потолке квоты обязан останавливать очередь")
	assert.EqualValues(t, 1, atEdge.BlockedRows)
	assert.Contains(t, w.registryRowByID(t, pending.ID).LastError, "quota_exceeded")

	entries := w.auditRows(t, models.ArchiveQuotaActionBlocked)
	require.Len(t, entries, 1)
	assert.Contains(t, string(entries[0].Details), "quota_exceeded")
}

// Регресс на порядок «сначала действие, потом отметка» (ревью среза B2). Отметка,
// сохранённая до действия, сказала бы следующему тику «уже сделано», и очередь
// осталась бы незаблокированной при реально переполненном диске - перехода больше
// не случится, пока место не вернётся и снова не кончится.
func quotaRetriesBlockSection(t *testing.T, w quotaWorld) {
	w.reset(t)
	bucket := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	pending := quotaSeedRow(t, w.db, 7301, 6301, models.BlankExportPending, 0, bucket)
	w.setSettings(t, models.UpdateArchiveSettingsRequest{MinFreeBytes: testutil.Ptr(quotaImpossibleFreeBytes)})

	recorder := &flakyRecorder{AuditRecorder: services.NewAuditRecorder(w.db), failAction: models.ArchiveQuotaActionBlocked}
	quota := w.newQuota(services.NewNotificationService(w.db), recorder)

	_, err := quota.EnforceThresholds(context.Background())
	require.Error(t, err, "сорванное действие обязано быть видно вызывающему")
	assert.ErrorIs(t, err, errQuotaAuditDown)
	assert.Equal(t, models.BlankExportPending, w.registryRowByID(t, pending.ID).Status,
		"строки и запись журнала идут одной транзакцией: не записалось - откатились")
	assert.Empty(t, w.auditRows(t, models.ArchiveQuotaActionBlocked))
	assert.NotEqual(t, "true", w.flagValue(t, archiveBlockFlagSettingKey),
		"отметка «очередь остановлена» при провале действия сохраняться не имеет права")

	// Следующий тик того же процесса обязан повторить попытку.
	recorder.failAction = ""
	retry, err := quota.EnforceThresholds(context.Background())
	require.NoError(t, err)
	require.True(t, retry.HardTripped)
	assert.EqualValues(t, 1, retry.BlockedRows, "повтор после сбоя обязан довести блокировку до конца")
	assert.Equal(t, models.BlankExportBlocked, w.registryRowByID(t, pending.ID).Status)
	assert.Len(t, w.auditRows(t, models.ArchiveQuotaActionBlocked), 1)
	assert.Equal(t, "true", w.flagValue(t, archiveBlockFlagSettingKey))
}

// Мягкий порог: предупреждение получают носители права управления архивом, ровно
// один раз на переход через край.
func quotaWarnsAdminsSection(t *testing.T, w quotaWorld) {
	w.reset(t)
	// Порог в 1% заведомо пройден на любом рабочем разделе; если вдруг нет -
	// проверка упадёт на require ниже, а не притворится зелёной.
	w.setSettings(t, models.UpdateArchiveSettingsRequest{WarnPercent: testutil.Ptr(1)})
	quota := w.newQuota(services.NewNotificationService(w.db), services.NewAuditRecorder(w.db))

	res, err := quota.EnforceThresholds(context.Background())
	require.NoError(t, err)
	require.True(t, res.SoftTripped, "раздел заполнен больше чем на 1% - мягкий порог обязан сработать")
	assert.False(t, res.HardTripped, "мягкий порог очередь не трогает")

	sent := w.notificationsOfType(t, services.NotificationTypeArchiveQuotaWarning)
	require.Len(t, sent, 1, "предупреждение уходит только носителям права управления архивом")
	assert.Equal(t, w.adminID, sent[0].UserID)
	assert.NotEqual(t, w.regularID, sent[0].UserID)
	require.NotNil(t, sent[0].Title)
	assert.Equal(t, "Файловый архив заполняется", *sent[0].Title)
	require.NotNil(t, sent[0].Message)
	assert.Contains(t, *sent[0].Message, "порог 1%")
	assert.Equal(t, "true", w.flagValue(t, archiveWarnFlagSettingKey))

	second, err := quota.EnforceThresholds(context.Background())
	require.NoError(t, err)
	assert.True(t, second.SoftTripped)
	assert.Len(t, w.notificationsOfType(t, services.NotificationTypeArchiveQuotaWarning), 1,
		"второе уведомление о том же событии - ровно то, ради чего отметка и заводилась")
}

// Не дошедшее уведомление не считается сказанным: отметка не двигается, и
// следующий тик предупреждает заново.
func quotaRetriesWarnSection(t *testing.T, w quotaWorld) {
	w.reset(t)
	w.setSettings(t, models.UpdateArchiveSettingsRequest{WarnPercent: testutil.Ptr(1)})
	notifier := &flakyNotifier{NotificationService: services.NewNotificationService(w.db), fail: true}
	quota := w.newQuota(notifier, services.NewAuditRecorder(w.db))

	res, err := quota.EnforceThresholds(context.Background())
	require.NoError(t, err, "сбой уведомления не рвёт прогон: жёсткий порог важнее и проверяется следом")
	require.True(t, res.SoftTripped)
	assert.Empty(t, w.notificationsOfType(t, services.NotificationTypeArchiveQuotaWarning))
	assert.NotEqual(t, "true", w.flagValue(t, archiveWarnFlagSettingKey),
		"отметка «уже предупредили» при недоставленном уведомлении сохраняться не имеет права")

	notifier.fail = false
	retry, err := quota.EnforceThresholds(context.Background())
	require.NoError(t, err)
	require.True(t, retry.SoftTripped)
	assert.Len(t, w.notificationsOfType(t, services.NotificationTypeArchiveQuotaWarning), 1,
		"следующий тик обязан повторить попытку предупредить")
	assert.Equal(t, "true", w.flagValue(t, archiveWarnFlagSettingKey))
}

// Выключенный архив не пишет и не копит очередь - сторожить нечего.
func quotaDisabledSection(t *testing.T, w quotaWorld) {
	w.reset(t)
	bucket := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	pending := quotaSeedRow(t, w.db, 7401, 6401, models.BlankExportPending, 0, bucket)
	w.setSettings(t, models.UpdateArchiveSettingsRequest{Enabled: testutil.Ptr(false), MinFreeBytes: testutil.Ptr(quotaImpossibleFreeBytes), WarnPercent: testutil.Ptr(1)})

	quota := w.newQuota(services.NewNotificationService(w.db), services.NewAuditRecorder(w.db))
	res, err := quota.EnforceThresholds(context.Background())
	require.NoError(t, err)
	assert.False(t, res.HardTripped)
	assert.False(t, res.SoftTripped)
	assert.Zero(t, res.BlockedRows)
	assert.Equal(t, models.BlankExportPending, w.registryRowByID(t, pending.ID).Status)
	assert.Empty(t, w.auditRows(t, models.ArchiveQuotaActionBlocked))
	assert.Empty(t, w.notificationsOfType(t, services.NotificationTypeArchiveQuotaWarning))
}

// Сводка GET /file-archive/stats: занятое место и число файлов считаются по
// записанным строкам реестра, разбивка по месяцам - по bucket_date. Строки, файла
// которых на диске нет (очередь, ошибки, тип без бланка), в сводку не входят: иначе
// администратор увидел бы занятое место, которого не существует.
func TestFileArchiveStats_CountsWrittenRegistryOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	july := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	june := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	may := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	quotaSeedRow(t, db, 7501, 6501, models.BlankExportOK, 1000, july)
	quotaSeedRow(t, db, 7502, 6502, models.BlankExportOK, 500, july)
	quotaSeedRow(t, db, 7503, 6503, models.BlankExportOK, 2500, june)
	// Три статуса «файла нет»: очередь, транзиентная ошибка и тип без бланка. У
	// каждого ненулевой size_bytes - если бы сводка считала их, числа разъехались бы.
	quotaSeedRow(t, db, 7504, 6504, models.BlankExportPending, 7777, july)
	quotaSeedRow(t, db, 7505, 6505, models.BlankExportFailed, 8888, june)
	quotaSeedRow(t, db, 7506, 6506, models.BlankExportNoTemplate, 9999, may)

	rec := testutil.GET(t, e, "/file-archive/stats", adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Счётчики по статусам показывают пробелы архива числом: no_template - это не
	// «пусто», а вложение, для которого бланк никто не настроил.
	counted := testutil.ParseResponse[models.ArchiveStats](t, rec)
	assert.Equal(t, int64(3), counted.Statuses[models.BlankExportOK])
	assert.Equal(t, int64(1), counted.Statuses[models.BlankExportNoTemplate])
	assert.Equal(t, int64(1), counted.Statuses[models.BlankExportPending])
	assert.Equal(t, int64(1), counted.Statuses[models.BlankExportFailed])
	assert.Contains(t, counted.Statuses, models.BlankExportBlocked, "известный статус обязан приходить и с нулём")
	assert.Zero(t, counted.Statuses[models.BlankExportBlocked])

	// Форма ответа - общий конверт, а не голый объект: фронт разворачивает data.
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   string          `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope), rec.Body.String())
	assert.True(t, envelope.Success, "ошибка в конверте: %s", envelope.Error)
	require.NotEmpty(t, envelope.Data)

	stats := testutil.ParseResponse[models.ArchiveStats](t, rec)
	assert.EqualValues(t, 4000, stats.UsedBytes, "занято = сумма размеров записанных файлов")
	assert.EqualValues(t, 3, stats.FileCount, "считаются только записанные файлы")
	assert.Positive(t, stats.FreeBytes, "свободное место берётся у настоящего раздела")
	assert.False(t, stats.GeneratedAt.IsZero())

	require.Len(t, stats.Periods, 2, "май состоит из строки без бланка - файлов за него нет: %v", stats.Periods)
	assert.Equal(t, "2026-07", stats.Periods[0].Month, "свежий месяц сверху")
	assert.EqualValues(t, 1500, stats.Periods[0].Bytes)
	assert.EqualValues(t, 2, stats.Periods[0].FileCount)
	assert.Equal(t, "2026-06", stats.Periods[1].Month)
	assert.EqualValues(t, 2500, stats.Periods[1].Bytes)
	assert.EqualValues(t, 1, stats.Periods[1].FileCount)

	assert.EqualValues(t, 4000, stats.Disk.ArchiveBytes, "состав диска берёт архив из той же суммы реестра")
	assert.Equal(t, stats.FreeBytes, stats.Disk.FreeBytes)
	assert.Positive(t, stats.Disk.TotalBytes)
	assert.GreaterOrEqual(t, stats.Disk.OtherBytes, int64(0), "прочее не уходит в минус при рассинхроне снимков")
	require.NotEmpty(t, stats.Disk.Partitions, "хотя бы один раздел процессу виден")
	assert.NotEmpty(t, stats.Disk.Partitions[0].Labels)

	// Сводка кэшируется на 5 минут: обход каталогов и агрегаты не считаются на
	// каждый заход на вкладку.
	quotaSeedRow(t, db, 7507, 6507, models.BlankExportOK, 12345, july)
	repeat := testutil.GET(t, e, "/file-archive/stats", adminH)
	require.Equal(t, http.StatusOK, repeat.Code)
	cached := testutil.ParseResponse[models.ArchiveStats](t, repeat)
	assert.EqualValues(t, 4000, cached.UsedBytes, "второй запрос отдаёт кэш, а не пересчитывает реестр")
	assert.Equal(t, stats.GeneratedAt.UnixNano(), cached.GeneratedAt.UnixNano())
}

// Сводка разбирает число файлов на заявки, бланки и служебные слепки, называет типы
// вложений теми же именами, под которыми файлы легли на диск, и отдаёт момент
// последней записи. Одно число «файлов» отвечало на вопрос, которого администратор
// не задавал: у одной заявки на диске лежит бланк на каждое вложение плюс слепок.
func TestFileArchiveStats_CompositionAndAttachmentTypes(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	july := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	appID := statsSeedApplication(t, db, td.OrgID)

	// Два вложения одной заявки: у первого имя живёт в справочнике, у второго
	// справочника нет вовсе - остаётся копия имени на строке заявки.
	autoID := statsSeedAttachment(t, db, appID, "Автозаявка", true)
	importID := statsSeedAttachment(t, db, appID, "Заявка на ввоз", false)

	blankAuto := quotaSeedRow(t, db, appID, autoID, models.BlankExportOK, 3000, july)
	quotaSeedRow(t, db, appID, importID, models.BlankExportOK, 1000, july)
	// Слепок заявки: строка реестра без вложения. Его нельзя считать бланком - на
	// диске это заявка.json, а не выгруженный шаблон.
	quotaSeedRow(t, db, appID, 0, models.BlankExportOK, 500, july)
	// Бланк, переживший своё вложение: реестр намеренно без внешних ключей, и такой
	// файл обязан остаться в разбивке видимой строкой, а не выпасть из суммы.
	quotaSeedRow(t, db, 7601, 6601, models.BlankExportOK, 700, july)
	// Строка без файла на диске: в состав не входит ни одним числом.
	quotaSeedRow(t, db, 7602, 6602, models.BlankExportFailed, 9999, july)

	written := time.Date(2026, 7, 15, 9, 30, 0, 0, time.UTC)
	require.NoError(t, db.Model(&models.BlankExport{}).Where("status = ?", models.BlankExportOK).
		Update("generated_at", written.Add(-time.Hour)).Error)
	require.NoError(t, db.Model(&models.BlankExport{}).Where("id = ?", blankAuto.ID).
		Update("generated_at", written).Error)

	rec := testutil.GET(t, e, "/file-archive/stats", adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	stats := testutil.ParseResponse[models.ArchiveStats](t, rec)

	assert.EqualValues(t, 4, stats.FileCount, "файлов всего: три у заявки и один осиротевший")
	assert.EqualValues(t, 2, stats.Composition.Applications, "заявок в архиве: своя и осиротевшая")
	assert.EqualValues(t, 3, stats.Composition.Blanks)
	assert.EqualValues(t, 1, stats.Composition.Snapshots)
	assert.EqualValues(t, stats.FileCount, stats.Composition.Blanks+stats.Composition.Snapshots,
		"состав обязан сходиться с числом файлов, иначе разбивка читается как поломка")

	require.Len(t, stats.AttachmentTypes, 3, "три типа: два названных и осиротевший: %v", stats.AttachmentTypes)
	assert.Equal(t, "Автозаявка", stats.AttachmentTypes[0].Name, "тяжёлый тип сверху")
	assert.EqualValues(t, 3000, stats.AttachmentTypes[0].Bytes)
	assert.EqualValues(t, 1, stats.AttachmentTypes[0].FileCount)
	assert.Equal(t, "Заявка на ввоз", stats.AttachmentTypes[1].Name,
		"имя берётся с копии на заявке, когда справочника у вложения нет")
	assert.Equal(t, "Вложение удалено", stats.AttachmentTypes[2].Name)
	assert.EqualValues(t, 700, stats.AttachmentTypes[2].Bytes)

	var typeFiles int64
	for _, at := range stats.AttachmentTypes {
		typeFiles += at.FileCount
	}
	assert.EqualValues(t, stats.Composition.Blanks, typeFiles,
		"разбивка по типам обязана покрывать все бланки, включая осиротевшие")

	require.NotNil(t, stats.LastWrittenAt, "момент последней записи нужен точнее месяца")
	assert.Equal(t, written.UTC(), stats.LastWrittenAt.UTC())
}

// Пустой архив отвечает нулями и отсутствием момента последней записи, а не
// выдуманной датой: «ещё ничего не писали» и «писали давно» - разные сообщения.
func TestFileArchiveStats_EmptyArchiveHasNoLastWrite(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	quotaSeedRow(t, db, 7701, 6701, models.BlankExportPending, 4242,
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))

	rec := testutil.GET(t, e, "/file-archive/stats", adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	stats := testutil.ParseResponse[models.ArchiveStats](t, rec)

	assert.Nil(t, stats.LastWrittenAt)
	assert.Zero(t, stats.Composition.Applications)
	assert.Zero(t, stats.Composition.Blanks)
	assert.Zero(t, stats.Composition.Snapshots)
	assert.Empty(t, stats.AttachmentTypes)
}

// statsSeedApplication - заявка, к которой цепляются вложения разбивки.
func statsSeedApplication(t *testing.T, db *gorm.DB, orgID int) int {
	t.Helper()
	var senderID int
	require.NoError(t, db.Raw(`SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&senderID).Error)
	require.NotZero(t, senderID, "нужен хотя бы один пользователь-отправитель")

	number := fmt.Sprintf("STATS-%d", time.Now().UnixNano())
	sent := time.Now()
	status := "Завершено"
	app := models.Application{
		ApplicationNumber: &number,
		SendingDatetime:   &sent,
		OrganizationID:    orgID,
		SenderUserID:      senderID,
		Status:            &status,
	}
	require.NoError(t, db.Create(&app).Error)
	return app.ID
}

// statsSeedAttachment создаёт вложение с наименованием. fromCatalog=false - случай,
// когда справочник вложению не назначен и имя остаётся только копией на заявке.
func statsSeedAttachment(t *testing.T, db *gorm.DB, appID int, name string, fromCatalog bool) int {
	t.Helper()
	attachment := models.Attachment{
		ApplicationID:  &appID,
		AttachmentType: "cars",
		AttachmentName: &name,
	}
	if fromCatalog {
		unique := models.UniqueAttachment{
			AttachmentType: "cars",
			Name:           &name,
			DisplayName:    &name,
			Title:          &name,
			IsActive:       true,
		}
		require.NoError(t, db.Create(&unique).Error)
		attachment.UniqueAttachmentID = &unique.ID
		// Имя на строке заявки намеренно другое: если сводка возьмёт его вместо
		// справочника, тест это увидит - на диске лежит имя из справочника.
		stale := name + " (старое имя)"
		attachment.AttachmentName = &stale
	}
	require.NoError(t, db.Create(&attachment).Error)
	return attachment.ID
}
