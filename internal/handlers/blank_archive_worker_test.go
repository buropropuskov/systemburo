package handlers_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path"
	"testing"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Фоновая обработка файлового архива (#1615, срез B1): разбор очереди, подметатель
// повторов по next_attempt_at и ночная сверка реестра с диском.
//
// Воркер тестируется вызовом его шагов напрямую, а не запуском горутины: у горутины
// нет момента «шаг закончился», и проверка выродилась бы в ожидание с таймаутом.
//
// Сервис поднимается на db из SetupTestAppWithArchive, а не тестируется в своём
// пакете: DB-backed тест живёт только в handlers (#706).

// archiveWorkerBatchLimit - потолок одной выборки подметателя (archiveSweepBatchLimit
// в blank_export_worker.go). Продублирован числом намеренно: тест стережёт именно то,
// что выборка ограничена, и смена константы обязана его разбудить.
const archiveWorkerBatchLimit = 200

// newWorkerExport собирает сервис выгрузки со сторожем места на той же базе и том же
// корне архива, что у приложения.
//
// Своя пара на каждую секцию: у сторожа есть состояние перехода через порог в памяти,
// и общий экземпляр перенёс бы «уже остановили» из одной секции в другую.
func (w archiveWorld) newWorkerExport(t *testing.T) *services.BlankExportService {
	t.Helper()
	writer, err := services.NewArchiveWriter(w.root)
	require.NoError(t, err)

	settings := services.NewSettingsService(w.db, &config.Config{PaginationMaxLimit: 100})
	quota := services.NewBlankExportQuotaService(
		w.db, settings, services.NewNotificationService(w.db),
		services.NewPermissionResolver(w.db), services.NewAuditRecorder(w.db),
		w.root, w.root, "")
	return services.NewBlankExportService(
		w.db, services.NewAttachmentBlankService(w.db),
		services.NewArchivePathService(w.db, time.UTC), writer, settings, quota)
}

// putSettings правит настройки архива тем же путём, каким их правят на самом деле -
// сервисом, с той же проверкой значений. Через веб настройки больше не меняются:
// раскладку и пороги задаёт команда server archive (#1615).
func (w archiveWorld) putSettings(t *testing.T, req models.UpdateArchiveSettingsRequest) {
	t.Helper()
	testutil.SetArchiveSettings(t, w.db, req)
}

// filePath собирает путь на диске по строке реестра.
func (w archiveWorld) filePath(row models.BlankExport) string {
	return w.abs(path.Join(row.RelDir, row.FileName))
}

// parkExistingApplications уводит всё, что накопили соседние секции, за окно сверки.
// Recheck идёт по ВСЕМ заявкам периода, а не по чьей-то выборке, поэтому без этого в
// прогон попали бы чужие заявки этого же файла и счётчик обработанных стал бы
// зависеть от порядка секций.
func (w archiveWorld) parkExistingApplications(t *testing.T) {
	t.Helper()
	old := time.Now().AddDate(0, 0, -90)
	require.NoError(t, w.db.Exec("UPDATE applications SET sending_datetime = ?", old).Error)
	require.NoError(t, w.db.Exec("UPDATE blank_exports SET bucket_date = ?", old).Error)
}

// workerSeedQueueRow заводит строку реестра в заданном состоянии очереди: статус и
// срок следующей попытки - всё, по чему подметатель решает, брать ли её.
func workerSeedQueueRow(t *testing.T, db *gorm.DB, appID, attID int, status string, nextAttempt *time.Time) {
	t.Helper()
	row := models.BlankExport{
		ApplicationID: appID, AttachmentID: attID,
		BucketDate: time.Now(), Status: status, NextAttemptAt: nextAttempt,
		QueueReason: services.BlankExportReasonSubmit, QueuedAt: time.Now(),
	}
	require.NoError(t, db.Create(&row).Error)
}

func TestFileArchiveWorker(t *testing.T) {
	w := setupArchiveWorld(t)
	t.Run("очередь разбирается шагом воркера", func(t *testing.T) { archiveWorkerQueueSection(t, w) })
	t.Run("нехватка места останавливает разбор", func(t *testing.T) { archiveWorkerQuotaStopSection(t, w) })
	t.Run("подметатель берёт просроченные повторы", func(t *testing.T) { archiveWorkerSweepSection(t, w) })
	t.Run("подметатель укладывается в батч-лимит", func(t *testing.T) { archiveWorkerSweepLimitSection(t, w) })
	t.Run("сверка чинит пропавший файл в окне", func(t *testing.T) { archiveWorkerRecheckWindowSection(t, w) })
	t.Run("сверка помечает сироту и оставляет файл", func(t *testing.T) { archiveWorkerRecheckOrphanSection(t, w) })
}

// Шаг воркера выгружает ровно то, что поставили в очередь, и опустошает её: заявка,
// которой никто не касался, файлов и строк реестра не получает.
func archiveWorkerQueueSection(t *testing.T, w archiveWorld) {
	ctx := context.Background()
	svc := w.newWorkerExport(t)
	uaID := w.newExportType(t, "Пропуск очередь", true, true)
	queued, queuedAtt := w.newExportApp(t, "20260801/001", uaID, "")
	untouched, _ := w.newExportApp(t, "20260801/002", uaID, "")

	processed, failed := svc.ProcessQueue(ctx)
	require.Zero(t, failed)
	require.Zero(t, processed, "пустая очередь не должна ничего выгружать")

	// Повторная постановка одной заявки до разбора - одна выгрузка, а не две.
	svc.EnqueueApplication(queued, services.BlankExportReasonSubmit)
	svc.EnqueueApplication(queued, services.BlankExportReasonUpdate)

	processed, failed = svc.ProcessQueue(ctx)
	require.Zero(t, failed)
	assert.Equal(t, 1, processed, "очередь схлопывает повторную постановку одной заявки")

	row := w.registryRow(t, queued, queuedAtt)
	assert.Equal(t, models.BlankExportOK, row.Status, "ошибка выгрузки: %s", row.LastError)
	assert.FileExists(t, w.filePath(row), "шаг воркера обязан положить бланк на диск")
	assert.Equal(t, services.BlankExportReasonUpdate, row.QueueReason,
		"позднейшая причина побеждает раннюю: интересен актуальный повод")

	var untouchedRows int64
	require.NoError(t, w.db.Model(&models.BlankExport{}).
		Where("application_id = ?", untouched).Count(&untouchedRows).Error)
	assert.Zero(t, untouchedRows, "заявка вне очереди фоновым шагом не трогается")

	processed, failed = svc.ProcessQueue(ctx)
	assert.Zero(t, processed, "разбор опустошает очередь - повторный шаг делать нечего")
	assert.Zero(t, failed)
}

// Регресс на интеграцию B1 с B2: пороги места спрашиваются ДО записи, и сработавший
// жёсткий порог пропускает разбор очереди целиком. Без вызова EnforceThresholds из
// шага воркера вся защита от переполнения раздела остаётся мёртвым кодом.
//
// Очередь при отказе НЕ вычерпывается: заявка обязана дождаться прогона, на котором
// писать будет куда, иначе один переполненный диск терял бы её навсегда.
func archiveWorkerQuotaStopSection(t *testing.T, w archiveWorld) {
	ctx := context.Background()
	svc := w.newWorkerExport(t)
	t.Cleanup(func() {
		require.NoError(t, w.db.Exec("DELETE FROM system_settings WHERE key IN (?, ?)",
			archiveBlockFlagSettingKey, archiveWarnFlagSettingKey).Error)
	})

	uaID := w.newExportType(t, "Пропуск место", true, true)
	appID, attID := w.newExportApp(t, "20260801/003", uaID, "")
	svc.EnqueueApplication(appID, services.BlankExportReasonSubmit)

	// Требование к свободному месту, которого не выполнит ни один реальный раздел
	// (то же значение, что и в тестах самих порогов).
	w.putSettings(t, models.UpdateArchiveSettingsRequest{MinFreeBytes: testutil.Ptr(quotaImpossibleFreeBytes)})

	processed, failed := svc.ProcessQueue(ctx)
	assert.Zero(t, processed, "жёсткий порог обязан остановить разбор очереди")
	assert.Zero(t, failed, "остановка по месту - не сбой выгрузки, а отказ начинать её")

	var rows int64
	require.NoError(t, w.db.Model(&models.BlankExport{}).
		Where("application_id = ?", appID).Count(&rows).Error)
	assert.Zero(t, rows, "строки реестра нет: заявку даже не пробовали выгружать")

	// Место вернулось - та же заявка обязана уехать в архив без повторной постановки.
	w.putSettings(t, models.UpdateArchiveSettingsRequest{MinFreeBytes: testutil.Ptr[int64](0)})
	processed, failed = svc.ProcessQueue(ctx)
	require.Zero(t, failed)
	require.Equal(t, 1, processed, "очередь при нехватке места не вычерпывается, а ждёт")

	row := w.registryRow(t, appID, attID)
	assert.Equal(t, models.BlankExportOK, row.Status, "ошибка выгрузки: %s", row.LastError)
	assert.FileExists(t, w.filePath(row))
}

// Подметатель берёт строки, которым подошёл срок повтора, и только их. Остановленные
// нехваткой места (blocked) идут наравне с failed: им обещан повтор через пятнадцать
// минут, а других претендентов на этот повтор в системе нет.
func archiveWorkerSweepSection(t *testing.T, w archiveWorld) {
	ctx := context.Background()
	svc := w.newWorkerExport(t)
	require.NoError(t, w.db.Exec("DELETE FROM blank_exports").Error)

	uaID := w.newExportType(t, "Пропуск подметатель", true, true)
	dueApp, dueAtt := w.newExportApp(t, "20260801/004", uaID, "")
	blockedApp, blockedAtt := w.newExportApp(t, "20260801/005", uaID, "")
	laterApp, laterAtt := w.newExportApp(t, "20260801/006", uaID, "")
	quietApp, quietAtt := w.newExportApp(t, "20260801/007", uaID, "")

	// Второе вложение той же просроченной заявки: выборка группирует по заявкам, и
	// два её пробела обязаны стоить одну выгрузку, а не две.
	secondAtt := models.Attachment{ApplicationID: &dueApp, AttachmentType: "people", UniqueAttachmentID: &uaID}
	require.NoError(t, w.db.Create(&secondAtt).Error)

	past := time.Now().Add(-time.Minute)
	future := time.Now().Add(time.Hour)
	workerSeedQueueRow(t, w.db, dueApp, dueAtt, models.BlankExportFailed, &past)
	workerSeedQueueRow(t, w.db, dueApp, secondAtt.ID, models.BlankExportFailed, &past)
	workerSeedQueueRow(t, w.db, blockedApp, blockedAtt, models.BlankExportBlocked, &past)
	workerSeedQueueRow(t, w.db, laterApp, laterAtt, models.BlankExportFailed, &future)
	workerSeedQueueRow(t, w.db, quietApp, quietAtt, models.BlankExportNoTemplate, nil)

	processed, failed := svc.Sweep(ctx)
	require.Zero(t, failed)
	assert.Equal(t, 2, processed, "две заявки со сроком повтора, а не четыре строки реестра")

	assert.Equal(t, models.BlankExportOK, w.registryRow(t, dueApp, dueAtt).Status)
	assert.Equal(t, models.BlankExportOK, w.registryRow(t, dueApp, secondAtt.ID).Status,
		"выгрузка идёт заявкой целиком, поэтому чинятся обе её строки")
	assert.Equal(t, models.BlankExportOK, w.registryRow(t, blockedApp, blockedAtt).Status,
		"остановленной по месту строке обещан повтор через 15 минут - подобрать её больше некому")
	assert.Equal(t, models.BlankExportFailed, w.registryRow(t, laterApp, laterAtt).Status,
		"срок следующей попытки ещё не подошёл")
	assert.Equal(t, models.BlankExportNoTemplate, w.registryRow(t, quietApp, quietAtt).Status,
		"строка без срока повтора ждёт администратора, а не времени")
}

// Один прогон подметателя ограничен потолком выборки: лавина неудач иначе держала бы
// воркер в многочасовом прогоне и не давала бы отвечать на новые заявки. Остаток
// разбирается следующим прогоном - подметатель идемпотентен.
func archiveWorkerSweepLimitSection(t *testing.T, w archiveWorld) {
	ctx := context.Background()
	svc := w.newWorkerExport(t)
	require.NoError(t, w.db.Exec("DELETE FROM blank_exports").Error)
	t.Cleanup(func() { require.NoError(t, w.db.Exec("DELETE FROM blank_exports").Error) })

	// Строки заведомо несуществующих заявок: выгрузка каждой отваливается на первом
	// же запросе, зато прогон дёшев, а проверить надо ровно потолок выборки.
	// Реестр живёт без внешних ключей, такие строки для него легальны.
	past := time.Now().Add(-time.Minute)
	rows := make([]models.BlankExport, 0, archiveWorkerBatchLimit+1)
	for i := 0; i <= archiveWorkerBatchLimit; i++ {
		rows = append(rows, models.BlankExport{
			ApplicationID: 900000 + i, AttachmentID: 800000 + i,
			BucketDate: time.Now(), Status: models.BlankExportFailed, NextAttemptAt: &past,
			QueueReason: services.BlankExportReasonSweep, QueuedAt: time.Now(),
		})
	}
	require.NoError(t, w.db.Create(&rows).Error)

	// Двести отказов подряд - это двести строк в журнале процесса, из-за которых в
	// выводе прогона не видно ничего другого. Глушим только на время секции.
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(restore)

	processed, failed := svc.Sweep(ctx)
	assert.Zero(t, processed, "заявок под этими строками не существует - выгружать нечего")
	assert.Equal(t, archiveWorkerBatchLimit, failed,
		"прогон обязан упереться в потолок выборки, а не забрать всё разом")
}

// Ночная сверка перепроверяет заявки окна recheck_days и возвращает на диск файл,
// который оттуда пропал. Заявка вне окна в прогон не попадает вовсе.
func archiveWorkerRecheckWindowSection(t *testing.T, w archiveWorld) {
	ctx := context.Background()
	svc := w.newWorkerExport(t)
	w.parkExistingApplications(t)

	uaID := w.newExportType(t, "Пропуск сверка", true, true)
	inWindow, inAtt := w.newExportApp(t, "20260801/008", uaID, "")
	outWindow, outAtt := w.newExportApp(t, "20260801/009", uaID, "")

	svc.EnqueueApplications([]int{inWindow, outWindow}, services.BlankExportReasonSubmit)
	processed, failed := svc.ProcessQueue(ctx)
	require.Zero(t, failed)
	require.Equal(t, 2, processed)

	inFile := w.filePath(w.registryRow(t, inWindow, inAtt))
	outFile := w.filePath(w.registryRow(t, outWindow, outAtt))
	require.FileExists(t, inFile)
	require.FileExists(t, outFile)

	// Одна заявка уходит за окно вместе со своими строками реестра, вторая остаётся.
	old := time.Now().AddDate(0, 0, -90)
	require.NoError(t, w.db.Exec("UPDATE applications SET sending_datetime = ? WHERE id = ?", old, outWindow).Error)
	require.NoError(t, w.db.Exec("UPDATE blank_exports SET bucket_date = ? WHERE application_id = ?", old, outWindow).Error)
	w.putSettings(t, models.UpdateArchiveSettingsRequest{RecheckDays: testutil.Ptr(1)})
	t.Cleanup(func() { w.putSettings(t, models.UpdateArchiveSettingsRequest{RecheckDays: testutil.Ptr(30)}) })

	// Файлы обеих заявок пропали с диска (снесли руками, не доехала синхронизация).
	require.NoError(t, os.Remove(inFile))
	require.NoError(t, os.Remove(outFile))

	processed, failed = svc.Recheck(ctx)
	require.Zero(t, failed)
	assert.Equal(t, 1, processed, "сверка идёт по окну recheck_days, а не по всему реестру")
	assert.FileExists(t, inFile, "пропавший файл заявки в окне обязан вернуться на диск")
	assert.NoFileExists(t, outFile, "заявка вне окна в прогон не попадает")
}

// Вложение убрали из заявки, а её бланк остался лежать на диске. Сверка обязана
// доложить о расхождении статусом строки и НЕ удалять файл: удалять документ по
// расхождению, причину которого никто не разобрал, опаснее, чем хранить лишний файл.
func archiveWorkerRecheckOrphanSection(t *testing.T, w archiveWorld) {
	ctx := context.Background()
	svc := w.newWorkerExport(t)
	w.parkExistingApplications(t)
	w.putSettings(t, models.UpdateArchiveSettingsRequest{RecheckDays: testutil.Ptr(1)})
	t.Cleanup(func() { w.putSettings(t, models.UpdateArchiveSettingsRequest{RecheckDays: testutil.Ptr(30)}) })

	uaID := w.newExportType(t, "Пропуск сирота", true, true)
	appID, keptAtt := w.newExportApp(t, "20260801/010", uaID, "")
	goneAtt := models.Attachment{ApplicationID: &appID, AttachmentType: "people", UniqueAttachmentID: &uaID}
	require.NoError(t, w.db.Create(&goneAtt).Error)

	svc.EnqueueApplication(appID, services.BlankExportReasonSubmit)
	processed, failed := svc.ProcessQueue(ctx)
	require.Zero(t, failed)
	require.Equal(t, 1, processed)

	goneFile := w.filePath(w.registryRow(t, appID, goneAtt.ID))
	require.FileExists(t, goneFile)

	require.NoError(t, w.db.Exec("DELETE FROM attachments WHERE id = ?", goneAtt.ID).Error)

	processed, failed = svc.Recheck(ctx)
	require.Zero(t, failed)
	require.Equal(t, 1, processed)

	orphan := w.registryRow(t, appID, goneAtt.ID)
	assert.Equal(t, models.BlankExportOrphan, orphan.Status,
		"файл без вложения - расхождение, о котором надо доложить")
	assert.Nil(t, orphan.NextAttemptAt, "сироту не ретраят: её ждёт человек, а не время")
	assert.FileExists(t, goneFile, "документ по расхождению не удаляется молча")
	assert.Equal(t, models.BlankExportOK, w.registryRow(t, appID, keptAtt).Status,
		"живое вложение той же заявки сиротой не становится")
}
