package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Бэкфилл за период (#1615, срез B4): ручка ставит в очередь заявки диапазона, по
// желанию суженные типом вложения, а фактическую запись на диск подтверждает шаг
// фонового воркера на отдельном экземпляре сервиса - тем же приёмом, что и B1
// (blank_archive_worker_test.go), потому что очередь живёт в памяти конкретного
// экземпляра и HTTP-хендлер её не отдаёт наружу.

func TestFileArchiveBackfill(t *testing.T) {
	w := setupArchiveWorld(t)
	t.Run("период отбирает заявки", func(t *testing.T) { archiveBackfillPeriodSection(t, w) })
	t.Run("тип сужает выборку", func(t *testing.T) { archiveBackfillTypeSection(t, w) })
	t.Run("выключенная выгрузка", func(t *testing.T) { archiveBackfillDisabledSection(t, w) })
	t.Run("некорректные даты", func(t *testing.T) { archiveBackfillBadDatesSection(t, w) })
	t.Run("два вложения одного типа - одна запись", func(t *testing.T) { archiveBackfillTwinAttachmentsSection(t, w) })
}

// backfillSetDate переносит заявку на конкретный день - иначе все заявки секции
// получают "сейчас" от newExportApp и период одной секции захватывал бы заявки
// соседних.
func (w archiveWorld) backfillSetDate(t *testing.T, appID int, day time.Time) {
	t.Helper()
	require.NoError(t, w.db.Model(&models.Application{}).Where("id = ?", appID).
		Update("sending_datetime", day).Error)
}

// Диапазон дат отбирает ровно заявки внутри него, включая крайний день, и оставляет
// снаружи соседнюю заявку - без этого администратор не смог бы доверять счётчику
// «queued» из ответа.
func archiveBackfillPeriodSection(t *testing.T, w archiveWorld) {
	uaID := w.newExportType(t, "Пропуск бэкфилл период", true, true)
	inApp, inAtt := w.newExportApp(t, "20240310/001", uaID, "")
	outApp, _ := w.newExportApp(t, "20240310/002", uaID, "")
	w.backfillSetDate(t, inApp, time.Date(2024, 3, 10, 12, 0, 0, 0, time.UTC))
	w.backfillSetDate(t, outApp, time.Date(2024, 5, 10, 12, 0, 0, 0, time.UTC))

	rec := testutil.POST(t, w.e, "/file-archive/backfill",
		`{"date_from":"2024-03-01","date_to":"2024-03-31"}`, w.adminH)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	res := testutil.ParseResponse[models.ArchiveBackfillResponse](t, rec)
	assert.Equal(t, 1, res.Queued, "заявка вне периода не должна попасть в очередь")

	// Реальная запись на диск - отдельным экземпляром сервиса той же базы: HTTP-ответ
	// подтверждает только счётчик отбора, а не то, что отобранные заявки действительно
	// выгружаются.
	svc := w.newWorkerExport(t)
	queued, err := svc.Backfill(context.Background(),
		time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC), nil)
	require.NoError(t, err)
	require.Equal(t, 1, queued)
	processed, failed := svc.ProcessQueue(context.Background())
	require.Zero(t, failed)
	require.Equal(t, 1, processed)

	row := w.registryRow(t, inApp, inAtt)
	assert.Equal(t, models.BlankExportOK, row.Status, "ошибка выгрузки: %s", row.LastError)
	assert.FileExists(t, w.filePath(row))

	var outsideRows int64
	require.NoError(t, w.db.Model(&models.BlankExport{}).Where("application_id = ?", outApp).Count(&outsideRows).Error)
	assert.Zero(t, outsideRows, "заявка вне периода бэкфилла не должна получить строку реестра")
}

// unique_attachment_id сужает бэкфилл до заявок с вложением конкретного типа - тем
// же запросом пользуется «пересоздать бланки этого типа» после правки маппингов.
func archiveBackfillTypeSection(t *testing.T, w archiveWorld) {
	targetType := w.newExportType(t, "Пропуск бэкфилл цель", true, true)
	otherType := w.newExportType(t, "Пропуск бэкфилл сосед", true, true)
	targetApp, targetAtt := w.newExportApp(t, "20240715/001", targetType, "")
	otherApp, _ := w.newExportApp(t, "20240715/002", otherType, "")
	day := time.Date(2024, 7, 15, 12, 0, 0, 0, time.UTC)
	w.backfillSetDate(t, targetApp, day)
	w.backfillSetDate(t, otherApp, day)

	body := fmt.Sprintf(`{"date_from":"2024-07-01","date_to":"2024-07-31","unique_attachment_id":%d}`, targetType)
	rec := testutil.POST(t, w.e, "/file-archive/backfill", body, w.adminH)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	res := testutil.ParseResponse[models.ArchiveBackfillResponse](t, rec)
	assert.Equal(t, 1, res.Queued, "тип должен сузить выборку до одной заявки")

	uaID := targetType
	svc := w.newWorkerExport(t)
	queued, err := svc.Backfill(context.Background(),
		time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC), &uaID)
	require.NoError(t, err)
	require.Equal(t, 1, queued)
	processed, failed := svc.ProcessQueue(context.Background())
	require.Zero(t, failed)
	require.Equal(t, 1, processed)

	row := w.registryRow(t, targetApp, targetAtt)
	assert.Equal(t, models.BlankExportOK, row.Status, "ошибка выгрузки: %s", row.LastError)

	var otherRows int64
	require.NoError(t, w.db.Model(&models.BlankExport{}).Where("application_id = ?", otherApp).Count(&otherRows).Error)
	assert.Zero(t, otherRows, "соседний тип не должен попасть в бэкфилл")
}

// Выключенная выгрузка отвечает причиной, а не пустым «queued: 0» - администратор,
// нажавший «пересобрать период», должен понять, почему ничего не поставилось.
func archiveBackfillDisabledSection(t *testing.T, w archiveWorld) {
	testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{Enabled: testutil.Ptr(false)})
	t.Cleanup(func() {
		testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{Enabled: testutil.Ptr(true)})
	})

	res := testutil.POST(t, w.e, "/file-archive/backfill",
		`{"date_from":"2024-01-01","date_to":"2024-01-31"}`, w.adminH)
	require.Equal(t, http.StatusConflict, res.Code, res.Body.String())
}

// Некорректный ввод отвечает 400 с понятной причиной, а не 500 или тихим "queued: 0".
func archiveBackfillBadDatesSection(t *testing.T, w archiveWorld) {
	cases := []struct {
		name, body string
	}{
		{"битый date_from", `{"date_from":"вчера","date_to":"2024-01-31"}`},
		{"битый date_to", `{"date_from":"2024-01-01","date_to":"31.01.2024"}`},
		{"date_to раньше date_from", `{"date_from":"2024-02-01","date_to":"2024-01-01"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := testutil.POST(t, w.e, "/file-archive/backfill", tc.body, w.adminH)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

// У заявки бывает два вложения одного типа - в очередь она обязана попасть один раз.
// JOIN вместо EXISTS дал бы две записи, и воркер прогнал бы одну заявку дважды,
// переписав её файлы вторым проходом впустую.
func archiveBackfillTwinAttachmentsSection(t *testing.T, w archiveWorld) {
	uaID := w.newExportType(t, "Пропуск бэкфилл близнецы", true, true)
	appID, _ := w.newExportApp(t, "20240710/001", uaID, "")
	twin := models.Attachment{ApplicationID: &appID, AttachmentType: "people", UniqueAttachmentID: &uaID}
	require.NoError(t, w.db.Create(&twin).Error)
	w.backfillSetDate(t, appID, time.Date(2024, 7, 10, 12, 0, 0, 0, time.UTC))

	body := fmt.Sprintf(`{"date_from":"2024-07-01","date_to":"2024-07-31","unique_attachment_id":%d}`, uaID)
	rec := testutil.POST(t, w.e, "/file-archive/backfill", body, w.adminH)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())

	res := testutil.ParseResponse[models.ArchiveBackfillResponse](t, rec)
	assert.Equal(t, 1, res.Queued, "заявка с двумя вложениями искомого типа ставится в очередь один раз")
}
