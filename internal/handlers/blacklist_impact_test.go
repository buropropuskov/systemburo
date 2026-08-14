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

// Предпросмотр последствий внесения в чёрный список: администратор до подтверждения
// видит, где человек или машина фигурирует. Отбор обязан совпадать с тем, что потом
// деактивирует внесение, иначе окно обещает не то, что произойдёт.

// seedPeopleTableForImpact - таблица поста для людей, чтобы предпросмотр мог её назвать.
func seedPeopleTableForImpact(t *testing.T, db *gorm.DB) int {
	t.Helper()
	display := "Пост для предпросмотра"
	tbl := models.SystemTable{Name: "impact_people_post", DisplayName: &display, TableType: "people", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	return tbl.ID
}

// Человек из действующей заявки, стоящий на посту: предпросмотр называет и пост, и заявку.
func TestBlacklistImpact_PersonShowsTablesAndApplications(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	senderID := seedAttachSender(t, db, td.OrgID)
	appID, _ := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "IMP111AA777")
	empID := seedAssignEmployee(t, db, appID, "Импактов")
	tableID := seedPeopleTableForImpact(t, db)
	require.NoError(t, db.Exec("INSERT INTO employee_target_tables (employee_id, table_id, source) VALUES (?, ?, 'application')", empID, tableID).Error)

	// Имя работника выставляем полностью: отбор строгий по ФИО.
	require.NoError(t, db.Exec("UPDATE employees SET first_name = 'Импакт', middle_name = 'Импактович' WHERE id = ?", empID).Error)

	rec := testutil.GET(t, e, "/person-blacklist/impact?last_name=Импактов&first_name=Импакт&middle_name=Импактович", h)
	require.Equal(t, http.StatusOK, rec.Code, "предпросмотр: %s", rec.Body.String())

	body := rec.Body.String()
	assert.Contains(t, body, `"matches":1`, "нашёлся один работник")
	assert.Contains(t, body, "Пост для предпросмотра", "назван пост, из которого он уйдёт")
	assert.Contains(t, body, "APP-ASSIGN-IMP111AA777", "названа заявка, где он есть")
}

// Совпадений нет - предпросмотр отвечает нулём, а не ошибкой: так администратор видит,
// что внесение никого не затронет.
func TestBlacklistImpact_PersonNoMatches(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/person-blacklist/impact?last_name=Никого&first_name=Нету", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "предпросмотр: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"matches":0`)
}

// Уже деактивированный работник в предпросмотр не попадает: внесение его не тронет.
func TestBlacklistImpact_SkipsInactive(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	senderID := seedAttachSender(t, db, td.OrgID)
	appID, _ := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "IMP222AA777")
	empID := seedAssignEmployee(t, db, appID, "Выключенный")
	require.NoError(t, db.Exec("UPDATE employees SET first_name = 'Тест', status = 0 WHERE id = ?", empID).Error)

	rec := testutil.GET(t, e, "/person-blacklist/impact?last_name=Выключенный&first_name=Тест", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "предпросмотр: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"matches":0`)
}

// Машина отбирается вместе с маркой: тот же номер под другой маркой не в счёт,
// потому что и внесение в чёрный список его не деактивирует.
func TestBlacklistImpact_VehicleRespectsMark(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	ourMark := seedMark(t, db, "IMPACT_Mark")
	otherMark := seedMark(t, db, "IMPACT_Other")

	senderID := seedAttachSender(t, db, td.OrgID)
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "IMP333AA777")
	require.NoError(t, db.Exec("UPDATE cars SET mark_id = ? WHERE id = ?", ourMark.ID, carID).Error)
	_ = appID

	rec := testutil.GET(t, e, fmt.Sprintf("/vehicle-blacklist/impact?car_number=IMP333AA777&mark_id=%d", ourMark.ID), h)
	require.Equal(t, http.StatusOK, rec.Code, "своя марка: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"matches":1`)

	rec = testutil.GET(t, e, fmt.Sprintf("/vehicle-blacklist/impact?car_number=IMP333AA777&mark_id=%d", otherMark.ID), h)
	require.Equal(t, http.StatusOK, rec.Code, "чужая марка: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"matches":0`, "машина другой марки в предпросмотр не идёт")
}

// Предпросмотр отдаёт ФИО и номера заявок, поэтому закрыт правом на чёрный список.
func TestBlacklistImpact_RequiresBlacklistPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	outsider := testutil.RegisterAndLogin(t, e, "imp_outsider", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/person-blacklist/impact?last_name=Кто&first_name=То", testutil.AuthHeader(outsider))
	require.Equal(t, http.StatusForbidden, rec.Code, "без права нельзя: %s", rec.Body.String())
}
