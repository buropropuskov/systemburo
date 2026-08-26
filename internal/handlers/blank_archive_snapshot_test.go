package handlers_test

import (
	"encoding/json"
	"os"
	"path"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// archiveSnapshotFileName зеркалит константу того же имени в internal/services -
// пакет не экспортирует её, а дублировать значение здесь дешевле, чем расширять
// публичный контракт сервиса ради одного теста.
const archiveSnapshotFileName = "заявка.json"

func snapStrPtr(s string) *string { return &s }

// Машиночитаемый слепок заявки (заявка.json) рядом с бланками (#1615, срез A5c):
// состав полей заявки/людей/машин/ТМЦ/согласований и побайтовая стабильность между
// прогонами без изменения исходных данных.
func archiveSnapshotSection(t *testing.T, w archiveWorld) {
	uaID := w.newExportType(t, "Пропуск слепок", false, true) // без шаблона - бланк не пишется, слепок обязан появиться и без него
	appID, attID := w.newExportApp(t, "20260731/010", uaID, "")

	citizenship := models.Citizenship{Name: "Узбекистан"}
	require.NoError(t, w.db.Create(&citizenship).Error)
	passport := "45 03 123456"
	employee := models.Employee{
		AttachmentID: &attID, LastName: snapStrPtr("Иванов"), FirstName: snapStrPtr("Иван"),
		CitizenshipID: &citizenship.ID, PassportSeriesNumber: &passport,
	}
	require.NoError(t, w.db.Create(&employee).Error)

	carAtt := models.Attachment{ApplicationID: &appID, AttachmentType: "cars"}
	require.NoError(t, w.db.Create(&carAtt).Error)
	carNumber := "А123ВС77"
	require.NoError(t, w.db.Create(&models.Car{AttachmentID: carAtt.ID, CarNumber: &carNumber}).Error)

	itemsAtt := models.Attachment{ApplicationID: &appID, AttachmentType: "items"}
	require.NoError(t, w.db.Create(&itemsAtt).Error)
	itemName, itemCount := "Ноутбук", 2
	require.NoError(t, w.db.Create(&models.Item{AttachmentID: itemsAtt.ID, Name: &itemName, Count: &itemCount}).Error)

	require.NoError(t, w.db.Create(&models.ApplicationResponsibleUser{
		ApplicationID: appID, UserID: w.senderID, RequiredApproval: true,
		ApprovalStatus: snapStrPtr("approved"),
	}).Error)

	res := w.reexport(t, appID)
	require.Equal(t, models.BlankExportOK, res.Snapshot.Status, res.Snapshot.Error)
	assert.True(t, res.Snapshot.Written, "первый прогон обязан записать слепок")
	assert.Equal(t, path.Join(res.RelDir, archiveSnapshotFileName), res.Snapshot.RelPath,
		"путь слепка приходит в ответе - по нему администратор его и ищет")

	snapPath := w.abs(path.Join(res.RelDir, archiveSnapshotFileName))
	data, err := os.ReadFile(snapPath)
	require.NoError(t, err, "слепок заявки не появился на диске рядом с бланками")

	var snap map[string]any
	require.NoError(t, json.Unmarshal(data, &snap))

	app, ok := snap["application"].(map[string]any)
	require.True(t, ok, "в слепке нет раздела application")
	assert.EqualValues(t, appID, app["id"])
	assert.Equal(t, "20260731/010", app["number"])

	attachments, ok := snap["attachments"].([]any)
	require.True(t, ok)
	require.Len(t, attachments, 3, "в заявке три вложения - люди, машины, ТМЦ")

	var employees, cars, items []any
	for _, raw := range attachments {
		a := raw.(map[string]any)
		switch a["type"] {
		case "people":
			employees, _ = a["employees"].([]any)
		case "cars":
			cars, _ = a["cars"].([]any)
		case "items":
			items, _ = a["items"].([]any)
		}
	}
	require.Len(t, employees, 1)
	emp := employees[0].(map[string]any)
	assert.Equal(t, "Иванов", emp["last_name"])
	assert.Equal(t, "Узбекистан", emp["citizenship"])
	assert.Equal(t, passport, emp["passport_series_number"], "паспорт в слепке открытым текстом - в базе он шифруется, а бланк по тем же данным кладёт его читаемым")

	require.Len(t, cars, 1)
	assert.Equal(t, carNumber, cars[0].(map[string]any)["number"])

	require.Len(t, items, 1)
	assert.Equal(t, itemName, items[0].(map[string]any)["name"])
	assert.EqualValues(t, itemCount, items[0].(map[string]any)["count"])

	approvals, ok := snap["approvals"].([]any)
	require.True(t, ok)
	require.Len(t, approvals, 1)
	assert.Equal(t, "approved", approvals[0].(map[string]any)["status"])

	// Повторный прогон без изменения исходных данных не должен трогать файл на
	// диске: дедуп по хэшу слепка работает так же, как у бланков.
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(snapPath, old, old))

	second := w.reexport(t, appID)
	assert.Equal(t, res.RelDir, second.RelDir)
	assert.False(t, second.Snapshot.Written, "содержимое не изменилось - переписывать нечего")

	info, err := os.Stat(snapPath)
	require.NoError(t, err)
	assert.WithinDuration(t, old, info.ModTime(), time.Second, "содержимое не изменилось - файл слепка трогать не должны")

	dataAgain, err := os.ReadFile(snapPath)
	require.NoError(t, err)
	assert.Equal(t, data, dataAgain, "слепок обязан быть побайтово стабилен между прогонами на неизменных данных")
}

// Заморозка защищает уже лежащий слепок от перезаписи, но не отменяет первую его
// запись. Заявка, замороженная до появления заявка.json, и заявка, потерявшая слепок
// на том самом прогоне, где замёрзли бланки, - одно и то же состояние на диске:
// файла нет, а бланки уже документы. Без записи «первого раза» такая заявка осталась
// бы без слепка навсегда - у него нет ни строки реестра, ни ретрая, и ручное
// «пересоздать» упиралось бы в тот же признак заморозки.
func archiveSnapshotFrozenSection(t *testing.T, w archiveWorld) {
	testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{FreezeAfterDays: testutil.Ptr(0)})
	t.Cleanup(func() {
		testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{FreezeAfterDays: testutil.Ptr(30)})
	})

	uaID := w.newExportType(t, "Пропуск слепок заморозка", true, true)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	appID, attID := w.newExportApp(t, "20260731/011", uaID, yesterday)
	require.NoError(t, w.db.Model(&models.Application{}).Where("id = ?", appID).
		Update("status", models.StatusCompleted).Error)

	first := w.reexport(t, appID)
	require.Equal(t, models.BlankExportOK, first.Items[0].Status, first.Items[0].Error)
	require.True(t, first.Items[0].Frozen, "срок вышел - бланки заявки окончательны")
	require.NotNil(t, w.registryRow(t, appID, attID).FrozenAt)

	snapPath := w.abs(path.Join(first.RelDir, archiveSnapshotFileName))
	require.FileExists(t, snapPath)
	require.NoError(t, os.Remove(snapPath))

	second := w.reexport(t, appID)
	assert.Equal(t, models.BlankExportOK, second.Snapshot.Status, second.Snapshot.Error)
	assert.True(t, second.Snapshot.Frozen, "бланки заявки заморожены - ответ обязан это показывать")
	assert.True(t, second.Snapshot.Written, "пропавший слепок замороженной заявки записывается заново")

	data, err := os.ReadFile(snapPath)
	require.NoError(t, err, "заморозка не должна отменять первую запись слепка")

	// Уже лежащий слепок заморозка защищает: правка заявки его не двигает.
	old := time.Now().Add(-3 * time.Hour)
	require.NoError(t, os.Chtimes(snapPath, old, old))
	require.NoError(t, w.db.Model(&models.Application{}).Where("id = ?", appID).
		Update("message", "правка после заморозки").Error)

	third := w.reexport(t, appID)
	assert.Equal(t, models.BlankExportOK, third.Snapshot.Status, third.Snapshot.Error)
	assert.False(t, third.Snapshot.Written, "замороженный слепок не перезаписывается")
	assert.Equal(t, path.Join(first.RelDir, archiveSnapshotFileName), third.Snapshot.RelPath)

	info, err := os.Stat(snapPath)
	require.NoError(t, err)
	assert.WithinDuration(t, old, info.ModTime(), time.Second, "замороженный слепок трогать нельзя: mtime уводит синхронизацию в перекачку")

	kept, err := os.ReadFile(snapPath)
	require.NoError(t, err)
	assert.Equal(t, data, kept, "содержимое замороженного слепка не следует за правками заявки")
}

// Заявка, у которой ни одному типу вложения не настроен бланк, замороженных строк
// реестра не заводит вовсе - там нечего замораживать. Слепок у неё при этом есть, и
// он обязан замереть по сроку самой заявки: иначе единственный документ такой
// заявки переписывался бы вечно, расходясь с уже увезённой копией.
func archiveSnapshotFrozenWithoutBlanksSection(t *testing.T, w archiveWorld) {
	testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{FreezeAfterDays: testutil.Ptr(0)})
	t.Cleanup(func() {
		testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{FreezeAfterDays: testutil.Ptr(30)})
	})

	uaID := w.newExportType(t, "Тип без бланка со слепком", false, true)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	appID, _ := w.newExportApp(t, "20260731/012", uaID, yesterday)
	require.NoError(t, w.db.Model(&models.Application{}).Where("id = ?", appID).
		Update("status", models.StatusCompleted).Error)

	first := w.reexport(t, appID)
	require.Equal(t, models.BlankExportNoTemplate, first.Items[0].Status, "у типа нет бланка - выгружать нечего")
	require.Equal(t, models.BlankExportOK, first.Snapshot.Status, first.Snapshot.Error)
	require.True(t, first.Snapshot.Written, "слепок пишется даже когда бланков у заявки нет")

	snapPath := w.abs(path.Join(first.RelDir, archiveSnapshotFileName))
	before, err := os.ReadFile(snapPath)
	require.NoError(t, err)

	old := time.Now().Add(-4 * time.Hour)
	require.NoError(t, os.Chtimes(snapPath, old, old))
	require.NoError(t, w.db.Model(&models.Application{}).Where("id = ?", appID).
		Update("message", "правка заявки без бланков").Error)

	second := w.reexport(t, appID)
	assert.True(t, second.Snapshot.Frozen, "срок заявки вышел - слепок окончателен и без единого бланка рядом")
	assert.False(t, second.Snapshot.Written, "замороженный слепок не переписывается")

	info, err := os.Stat(snapPath)
	require.NoError(t, err)
	assert.WithinDuration(t, old, info.ModTime(), time.Second, "файл трогать нельзя")

	kept, err := os.ReadFile(snapPath)
	require.NoError(t, err)
	assert.Equal(t, before, kept, "содержимое не следует за правками завершённой заявки")
}
