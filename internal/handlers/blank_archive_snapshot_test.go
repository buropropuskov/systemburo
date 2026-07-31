package handlers_test

import (
	"encoding/json"
	"os"
	"path"
	"testing"
	"time"

	"systemburo/internal/models"

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

	info, err := os.Stat(snapPath)
	require.NoError(t, err)
	assert.WithinDuration(t, old, info.ModTime(), time.Second, "содержимое не изменилось - файл слепка трогать не должны")

	dataAgain, err := os.ReadFile(snapPath)
	require.NoError(t, err)
	assert.Equal(t, data, dataAgain, "слепок обязан быть побайтово стабилен между прогонами на неизменных данных")
}
