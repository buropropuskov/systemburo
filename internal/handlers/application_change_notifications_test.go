package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/testutil"
)

func TestApplicationChanges_SummaryOwnershipAndAcknowledgement(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	actor := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")
	sender := testutil.RegisterAndLogin(t, e, "changes-sender", "pass123", 1, td.OrgID, td.CompanyID)
	other := testutil.RegisterAndLogin(t, e, "changes-other", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "changes-sender")
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "A101AA777")
	var first models.Car
	require.NoError(t, db.First(&first, carID).Error)
	number := "A102AA777"
	second := models.Car{AttachmentID: first.AttachmentID, CarNumber: &number, Status: first.Status}
	require.NoError(t, db.Create(&second).Error)
	before := seedPlace(t, db, "Разгрузка до")
	after := seedPlace(t, db, "Разгрузка после")
	for _, id := range []int{carID, second.ID} {
		require.NoError(t, db.Create(&models.CarUnloadPlace{CarID: id, UnloadPlaceID: before}).Error)
	}
	body := fmt.Sprintf(`{"car_ids":[%d,%d],"place_ids":[%d],"mode":"replace"}`, carID, second.ID, after)
	path := fmt.Sprintf("/applications/%d/elements/unload-places", appID)
	rec := testutil.PUT(t, e, path, body, testutil.AuthHeader(actor))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var notes []models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?", senderID, "application_elements_changed").Find(&notes).Error)
	require.Len(t, notes, 1, "одна массовая правка — одно уведомление")
	old := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Model(&models.Notification{}).Where("id = ?", notes[0].ID).Update("created_at", old).Error)
	require.NoError(t, db.Model(&models.AuditLog{}).Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appID, "assignments_changed").Update("created_at", old).Error)
	for _, target := range []database.RetentionTarget{database.TargetUnreadNotifications, database.TargetAudit} {
		_, err := database.SweepRetention(context.Background(), db, target, database.SweepOptions{Cutoff: old.AddDate(1, 0, 0), Apply: true})
		require.NoError(t, err)
	}
	var remaining int64
	require.NoError(t, db.Model(&models.Notification{}).Where("id = ?", notes[0].ID).Count(&remaining).Error)
	require.EqualValues(t, 1, remaining, "очистка не снимает неподтверждённую отметку")
	load := func(token string) []models.Notification {
		r := testutil.GET(t, e, fmt.Sprintf("/notifications?application_id=%d", appID), testutil.AuthHeader(token))
		require.Equal(t, 200, r.Code, r.Body.String())
		var envelope struct {
			Data []models.Notification `json:"data"`
		}
		require.NoError(t, json.Unmarshal(r.Body.Bytes(), &envelope))
		return envelope.Data
	}
	rows := load(sender)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Data)
	require.Contains(t, *rows[0].Message, "Разгрузка до → Разгрузка после")
	var data struct {
		Changes []struct {
			Index        int  `json:"index"`
			Acknowledged bool `json:"acknowledged"`
		} `json:"changes"`
	}
	require.NoError(t, json.Unmarshal([]byte(*rows[0].Data), &data))
	require.Len(t, data.Changes, 2)
	require.Empty(t, load(other))
	require.Empty(t, load(actor))
	readPath := fmt.Sprintf("/notifications/%d/read", notes[0].ID)
	r := testutil.PUT(t, e, readPath, `{"is_read":true}`, testutil.AuthHeader(sender))
	require.Equal(t, 200, r.Code, r.Body.String())
	rows = load(sender)
	require.NoError(t, json.Unmarshal([]byte(*rows[0].Data), &data))
	require.False(t, data.Changes[0].Acknowledged, "открытие колокольчика не подтверждает изменение")
	r = testutil.PUT(t, e, readPath, `{"acknowledged_change":0}`, testutil.AuthHeader(other))
	require.NotEqual(t, 200, r.Code)
	r = testutil.PUT(t, e, readPath, `{"acknowledged_change":99}`, testutil.AuthHeader(sender))
	require.Equal(t, 404, r.Code, r.Body.String())
	r = testutil.PUT(t, e, readPath, `{"acknowledged_change":0}`, testutil.AuthHeader(sender))
	require.Equal(t, 200, r.Code, r.Body.String())
	rows = load(sender)
	require.NoError(t, json.Unmarshal([]byte(*rows[0].Data), &data))
	require.True(t, data.Changes[0].Acknowledged)
	require.False(t, data.Changes[1].Acknowledged)
	r = testutil.DELETE(t, e, fmt.Sprintf("/notifications/%d", notes[0].ID), testutil.AuthHeader(sender))
	require.Equal(t, 200, r.Code, r.Body.String())
	rows = load(sender)
	require.Len(t, rows, 1, "удаление из колокольчика не снимает отметки в заявке")
	require.NoError(t, json.Unmarshal([]byte(*rows[0].Data), &data))
	require.False(t, data.Changes[1].Acknowledged)
	r = testutil.PUT(t, e, path, body, testutil.AuthHeader(actor))
	require.Equal(t, 200, r.Code, r.Body.String())
	require.Len(t, load(sender), 1, "no-op не создаёт уведомление")
	body = fmt.Sprintf(`{"car_ids":[%d],"place_ids":[%d],"mode":"replace"}`, carID, before)
	r = testutil.PUT(t, e, path, body, testutil.AuthHeader(actor))
	require.Equal(t, 200, r.Code, r.Body.String())
	rows = load(sender)
	require.Len(t, rows, 2)
	require.NoError(t, json.Unmarshal([]byte(*rows[0].Data), &data))
	require.False(t, data.Changes[0].Acknowledged, "новая правка снова требует внимания")
}

func TestApplicationChanges_TableAssignmentsAndRemoval(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	actor := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")
	token := testutil.RegisterAndLogin(t, e, "changes-initiator", "pass123", 1, td.OrgID, td.CompanyID)
	appID, carID := seedAssignApp(t, db, td.OrgID, getUserID(t, db, "changes-initiator"), models.StatusInWork, "A103AA777")
	var original models.Car
	require.NoError(t, db.First(&original, carID).Error)
	number := "A105AA777"
	second := models.Car{AttachmentID: original.AttachmentID, CarNumber: &number, Status: original.Status}
	require.NoError(t, db.Create(&second).Error)
	employeeID := seedAssignEmployee(t, db, appID, "Учебный")
	post := seedCarsTable(t, db, "changes-post", "Учебный пост")
	for _, item := range []struct {
		kind string
		id   int
	}{{"cars", carID}, {"people", employeeID}} {
		body := fmt.Sprintf(`{"element_type":%q,"element_ids":[%d],"table_ids":[%d],"mode":"replace"}`, item.kind, item.id, post)
		r := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/elements/tables", appID), body, testutil.AuthHeader(actor))
		require.Equal(t, 200, r.Code, r.Body.String())
	}
	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d,%d],"reason":"Учебная проверка"}`, carID, second.ID)
	// Route is the existing accepting workflow, not a direct database mutation.
	r := testutil.DELETEWithBody(t, e, fmt.Sprintf("/applications/%d/elements", appID), body, testutil.AuthHeader(actor))
	require.Equal(t, 200, r.Code, r.Body.String())
	body = fmt.Sprintf(`{"element_type":"people","element_ids":[%d],"reason":"Учебная проверка"}`, employeeID)
	r = testutil.DELETEWithBody(t, e, fmt.Sprintf("/applications/%d/elements", appID), body, testutil.AuthHeader(actor))
	require.Equal(t, 200, r.Code, r.Body.String())
	r = testutil.GET(t, e, fmt.Sprintf("/notifications?application_id=%d", appID), testutil.AuthHeader(token))
	require.Equal(t, 200, r.Code, r.Body.String())
	var envelope struct {
		Data []models.Notification `json:"data"`
	}
	require.NoError(t, json.Unmarshal(r.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data, 4)
	require.Contains(t, *envelope.Data[0].Message, "В заявке → Убран из заявки")
	var mass struct {
		Changes []json.RawMessage `json:"changes"`
	}
	require.NoError(t, json.Unmarshal([]byte(*envelope.Data[1].Data), &mass))
	require.Len(t, mass.Changes, 2, "массовое удаление двух машин — одна сводка")
	require.Contains(t, *envelope.Data[2].Message, "Посты прохода")
	require.Contains(t, *envelope.Data[3].Message, "Посты проезда")
}

func TestApplicationChanges_NotificationFailureRollsBackEdit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	actor := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")
	testutil.RegisterAndLogin(t, e, "changes-rollback", "pass123", 1, td.OrgID, td.CompanyID)
	appID, carID := seedAssignApp(t, db, td.OrgID, getUserID(t, db, "changes-rollback"), models.StatusInWork, "A104AA777")
	place := seedPlace(t, db, "Учебная разгрузка")
	require.NoError(t, db.Exec(`CREATE FUNCTION reject_change_notice_2931() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic notification failure'; END $$`).Error)
	defer db.Exec(`DROP FUNCTION reject_change_notice_2931()`)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_change_notice_2931 BEFORE INSERT ON notifications FOR EACH ROW WHEN (NEW.type = 'application_elements_changed') EXECUTE FUNCTION reject_change_notice_2931()`).Error)
	defer db.Exec(`DROP TRIGGER reject_change_notice_2931 ON notifications`)
	body := fmt.Sprintf(`{"car_ids":[%d],"place_ids":[%d],"mode":"replace"}`, carID, place)
	r := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/elements/unload-places", appID), body, testutil.AuthHeader(actor))
	require.Equal(t, 500, r.Code, r.Body.String())
	var count int64
	require.NoError(t, db.Model(&models.CarUnloadPlace{}).Where("car_id = ?", carID).Count(&count).Error)
	require.Zero(t, count, "правка не сохраняется без записи уведомления")
	require.NoError(t, db.Model(&models.AuditLog{}).Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appID, "assignments_changed").Count(&count).Error)
	require.Zero(t, count, "история той же транзакции также откатилась")
}
