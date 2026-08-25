package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Тесты удаления людей и машин из поданной заявки принимающим.
// Мотив функции: элемент, похожий на запись чёрного списка, держит всю заявку, и если
// пропускать его нельзя, заявку было не провести вовсе.

// elementState читает состояние строки. Колонка даты удаления у таблиц разная:
// у машин date_removed, у работников date_deleted.
func elementState(t *testing.T, db *gorm.DB, table string, id int) (status *int, deleted *string) {
	t.Helper()
	column := "date_removed"
	if table == "employees" {
		column = "date_deleted"
	}
	var row struct {
		Status  *int
		Deleted *string
	}
	require.NoError(t, db.Raw(fmt.Sprintf("SELECT status, %s::text AS deleted FROM %s WHERE id = ?", column, table), id).
		Scan(&row).Error)
	return row.Status, row.Deleted
}

func removalPath(appID int) string {
	return fmt.Sprintf("/applications/%d/elements", appID)
}

// Принимающий убирает машину: строка деактивируется, получает дату удаления и
// попадает в историю - и элемента, и самой заявки.
func TestRemoveElements_ApproverRemovesCar(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	senderID := seedAttachSender(t, db, td.OrgID)
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "R111RR777")

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"reason":"числится в розыске, пропускать нельзя"}`, carID)
	rec := testutil.DELETEWithBody(t, e, removalPath(appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "удаление: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"removed":1`)

	status, deleted := elementState(t, db, "cars", carID)
	require.NotNil(t, status)
	assert.Equal(t, 0, *status, "машина деактивирована")
	assert.NotNil(t, deleted, "проставлена дата удаления - строка уходит в корзину")

	var elementEntries int64
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM audit_log WHERE entity_type = ? AND entity_id = ? AND action = 'delete'",
		models.AuditEntityCar, carID).Scan(&elementEntries).Error)
	assert.Equal(t, int64(1), elementEntries, "запись в истории машины")

	var appEntries int64
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM audit_log WHERE entity_type = ? AND entity_id = ? AND action = 'element_removed'",
		models.AuditEntityApplication, appID).Scan(&appEntries).Error)
	assert.Equal(t, int64(1), appEntries, "запись в истории заявки")
}

// Повторный вызов ничего не меняет и не плодит записей в истории: две вкладки
// принимающего дают два запроса на одну строку.
func TestRemoveElements_RepeatIsIdempotent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	senderID := seedAttachSender(t, db, td.OrgID)
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "R222RR777")
	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"reason":"дубль строки"}`, carID)

	rec := testutil.DELETEWithBody(t, e, removalPath(appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "первое удаление: %s", rec.Body.String())

	rec = testutil.DELETEWithBody(t, e, removalPath(appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "повтор: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"removed":0`, "повтор ничего не убирает")

	var appEntries int64
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM audit_log WHERE entity_type = ? AND entity_id = ? AND action = 'element_removed'",
		models.AuditEntityApplication, appID).Scan(&appEntries).Error)
	assert.Equal(t, int64(1), appEntries, "второй записи в истории нет")
}

// Работник убирается так же, как машина.
func TestRemoveElements_ApproverRemovesEmployee(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	senderID := seedAttachSender(t, db, td.OrgID)
	appID, _ := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "R333RR777")
	empID := seedAssignEmployee(t, db, appID, "Removalov")

	body := fmt.Sprintf(`{"element_type":"people","element_ids":[%d],"reason":"нет разрешительных документов"}`, empID)
	rec := testutil.DELETEWithBody(t, e, removalPath(appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "удаление работника: %s", rec.Body.String())

	status, deleted := elementState(t, db, "employees", empID)
	require.NotNil(t, status)
	assert.Equal(t, 0, *status)
	assert.NotNil(t, deleted)
}

// Не принимающий не может убирать элементы, даже будучи автором заявки.
func TestRemoveElements_NonApproverForbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	senderToken := testutil.RegisterAndLogin(t, e, "rm_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "rm_sender")

	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "R444RR777")

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"reason":"хочу убрать"}`, carID)
	rec := testutil.DELETEWithBody(t, e, removalPath(appID), body, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusForbidden, rec.Code, "автор заявки убирать не вправе: %s", rec.Body.String())

	status, _ := elementState(t, db, "cars", carID)
	require.NotNil(t, status)
	assert.Equal(t, 1, *status, "машина осталась активной")
}

// Причина обязательна: пустая строка отклоняется, элемент не трогается.
func TestRemoveElements_ReasonRequired(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	senderID := seedAttachSender(t, db, td.OrgID)
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "R555RR777")

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"reason":"  "}`, carID)
	rec := testutil.DELETEWithBody(t, e, removalPath(appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code, "без причины нельзя: %s", rec.Body.String())

	status, _ := elementState(t, db, "cars", carID)
	require.NotNil(t, status)
	assert.Equal(t, 1, *status)
}

// Элемент чужой заявки не убирается подстановкой идентификатора.
func TestRemoveElements_ForeignElementRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	senderID := seedAttachSender(t, db, td.OrgID)
	appID, _ := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "R666RR777")
	_, foreignCarID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "R777RR777")

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"reason":"чужая машина"}`, foreignCarID)
	rec := testutil.DELETEWithBody(t, e, removalPath(appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code, "чужой элемент: %s", rec.Body.String())

	status, _ := elementState(t, db, "cars", foreignCarID)
	require.NotNil(t, status)
	assert.Equal(t, 1, *status, "чужая машина не тронута")
}

// В закрытой заявке состав уже не меняется.
func TestRemoveElements_ClosedApplicationRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	senderID := seedAttachSender(t, db, td.OrgID)
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusCompleted, "R888RR777")

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"reason":"поздно"}`, carID)
	rec := testutil.DELETEWithBody(t, e, removalPath(appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code, "закрытая заявка: %s", rec.Body.String())

	status, _ := elementState(t, db, "cars", carID)
	require.NotNil(t, status)
	assert.Equal(t, 1, *status)
}
