package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// revertEnv - обстановка, общая для проверок отмены (#2437): пост, охранник с
// правами отметки, активированная машина.
type revertEnv struct {
	e       *echo.Echo
	db      *gorm.DB
	table   models.SystemTable
	guardID int
	token   string
	carID   int
	td      testutil.TestData
}

func setupRevertEnv(t *testing.T, tableName, login string) revertEnv {
	t.Helper()
	e, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	dn := "КПП " + tableName
	table := models.SystemTable{Name: tableName, DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&table).Error)

	token := testutil.RegisterAndLogin(t, e, login, "pass123", 1, td.OrgID, td.CompanyID)
	guardID := getUserID(t, db, login)
	testutil.GrantTableVerb(t, guardID, tableName, "entry")
	testutil.GrantTableVerb(t, guardID, tableName, "exit")

	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	return revertEnv{e: e, db: db, table: table, guardID: guardID, token: token, carID: carID, td: td}
}

// mark ставит отметку прохода тем же запросом, каким её ставит охранник на КПП.
func (env revertEnv) mark(t *testing.T, status int, token string, userID int) {
	t.Helper()
	rec := testutil.PUT(t, env.e, fmt.Sprintf("/cars/%d/territory-status", env.carID),
		fmt.Sprintf(`{"territory_status": %d, "user_id": %d, "table_id": %d}`, status, userID, env.table.ID),
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// revert зовёт отмену последней отметки.
func (env revertEnv) revert(t *testing.T, status int, token, reason string) *httptest.ResponseRecorder {
	t.Helper()
	rec := testutil.PUT(t, env.e, fmt.Sprintf("/cars/%d/territory-status/revert", env.carID),
		fmt.Sprintf(`{"territory_status": %d, "table_id": %d, "reason": %q}`, status, env.table.ID, reason),
		testutil.AuthHeader(token))
	return rec
}

// carRow читает territory_status и время входа прямо из строки машины.
func (env revertEnv) carRow(t *testing.T) (*int, *time.Time) {
	t.Helper()
	var row struct {
		TerritoryStatus    *int
		TerritoryEntryTime *time.Time
	}
	require.NoError(t, env.db.Table("cars").Select("territory_status, territory_entry_time").
		Where("id = ?", env.carID).Scan(&row).Error)
	return row.TerritoryStatus, row.TerritoryEntryTime
}

// ageLastMark состаривает последнюю отметку: окно охранника проверяется по времени
// записи, а ждать пятнадцать минут в тесте нельзя.
func (env revertEnv) ageLastMark(t *testing.T, d time.Duration) {
	t.Helper()
	require.NoError(t, env.db.Exec(`
		UPDATE audit_log SET created_at = created_at - ?::interval
		WHERE id = (SELECT id FROM audit_log WHERE entity_type = ? AND entity_id = ?
			AND action IN ('entry','exit') ORDER BY created_at DESC, id DESC LIMIT 1)`,
		fmt.Sprintf("%d minutes", int(d.Minutes())), models.AuditEntityCar, env.carID).Error)
}

// TestPassageRevert_GuardRevertsOwnFreshMark - основной сценарий: охранник отметил
// не того, спохватился и отменил. Строка возвращается в состояние до ошибки, а в
// журнале остаётся и сама отметка, и событие отмены с причиной.
func TestPassageRevert_GuardRevertsOwnFreshMark(t *testing.T) {
	env := setupRevertEnv(t, "kpp_revert_own", "revguard")

	env.mark(t, 1, env.token, env.guardID)
	status, entryTime := env.carRow(t)
	require.NotNil(t, status)
	require.Equal(t, 1, *status, "после отметки машина на территории")
	require.NotNil(t, entryTime, "время въезда проставлено")

	rec := env.revert(t, 1, env.token, "ошибся строкой")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	status, entryTime = env.carRow(t)
	assert.Nil(t, status, "машина вернулась в состояние «отметок не было»")
	assert.Nil(t, entryTime, "время въезда снято вместе с отметкой")

	var rows []struct {
		Action  string
		Details string
	}
	require.NoError(t, env.db.Raw(`SELECT action, details::text AS details FROM audit_log
		WHERE entity_type = ? AND entity_id = ? AND action IN ('entry', ?)
		ORDER BY created_at`, models.AuditEntityCar, env.carID, models.AuditActionEntryRevert).
		Scan(&rows).Error)
	require.Len(t, rows, 2, "отметка осталась в журнале, рядом легла отмена")
	assert.Equal(t, "entry", rows[0].Action)
	assert.Equal(t, models.AuditActionEntryRevert, rows[1].Action)
	assert.Contains(t, rows[1].Details, "ошибся строкой", "причина обязана попасть в журнал")

	// История карточки: отменённая отметка видна с пометкой, событие отмены рядом.
	histRec := testutil.GET(t, env.e, fmt.Sprintf("/cars/%d/history", env.carID), testutil.AuthHeader(env.token))
	require.Equal(t, http.StatusOK, histRec.Code)
	var hist struct {
		Data []struct {
			ActionType string `json:"action_type"`
			Reverted   bool   `json:"reverted"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(histRec.Body.Bytes(), &hist))
	seen := map[string]bool{}
	for _, it := range hist.Data {
		if it.ActionType == "entry" {
			assert.True(t, it.Reverted, "отменённая отметка обязана прийти с пометкой")
		}
		seen[it.ActionType] = true
	}
	assert.True(t, seen["entry"] && seen[models.AuditActionEntryRevert],
		"в истории видны и отметка, и её отмена: %v", seen)
}

// TestPassageRevert_ExitReturnsToTerritory - отменённый выход возвращает человека
// (здесь машину) на территорию вместе со временем входа. Время берётся от последнего
// действительного входа, а не от отменяемой записи.
func TestPassageRevert_ExitReturnsToTerritory(t *testing.T) {
	env := setupRevertEnv(t, "kpp_revert_exit", "revguardx")

	env.mark(t, 1, env.token, env.guardID)
	_, entryAfterMark := env.carRow(t)
	require.NotNil(t, entryAfterMark)

	env.mark(t, 2, env.token, env.guardID)
	status, _ := env.carRow(t)
	require.NotNil(t, status)
	require.Equal(t, 2, *status)

	rec := env.revert(t, 2, env.token, "выехала другая машина")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	status, entryTime := env.carRow(t)
	require.NotNil(t, status)
	assert.Equal(t, 1, *status, "машина снова на территории")
	require.NotNil(t, entryTime, "время въезда восстановлено")
	assert.WithinDuration(t, *entryAfterMark, *entryTime, time.Second,
		"время въезда - от действительного въезда, а не от отменённого выезда")
}

// TestPassageRevert_ForeignAndStaleNeedAdmin - два ограничения охранника: чужая
// отметка и отметка старше окна. Обе отменяет администратор.
func TestPassageRevert_ForeignAndStaleNeedAdmin(t *testing.T) {
	env := setupRevertEnv(t, "kpp_revert_gate", "revguard2")
	other := testutil.RegisterAndLogin(t, env.e, "revguard_other", "pass123", 1, env.td.OrgID, env.td.CompanyID)
	otherID := getUserID(t, env.db, "revguard_other")
	testutil.GrantTableVerb(t, otherID, env.table.Name, "entry")
	testutil.GrantTableVerb(t, otherID, env.table.Name, "exit")

	// Отметку ставит первый охранник, отменить пробует второй.
	env.mark(t, 1, env.token, env.guardID)
	rec := env.revert(t, 1, other, "не мой проход")
	assert.Equal(t, http.StatusForbidden, rec.Code, "чужую отметку охранник не отменяет")

	// Своя, но состаренная на 20 минут - окно закрылось.
	env.ageLastMark(t, 20*time.Minute)
	rec = env.revert(t, 1, env.token, "спохватился поздно")
	assert.Equal(t, http.StatusForbidden, rec.Code, "за пределами окна охранник не отменяет")

	status, _ := env.carRow(t)
	require.NotNil(t, status)
	require.Equal(t, 1, *status, "отказ не должен ничего откатывать")

	// Администратору окно и авторство не мешают.
	adminToken := testutil.RegisterManager(t, env.e, "revadmin", env.td.OrgID, env.td.CompanyID)
	rec = env.revert(t, 1, adminToken, "разбор смены: отметка ошибочна")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	status, _ = env.carRow(t)
	assert.Nil(t, status, "администратор откатил статус")
}

// TestPassageRevert_Conflicts - отказы, по которым охранник понимает, что произошло:
// направление разошлось, отменять нечего, причина не указана.
func TestPassageRevert_Conflicts(t *testing.T) {
	env := setupRevertEnv(t, "kpp_revert_conflict", "revguard3")

	rec := env.revert(t, 1, env.token, "нечего отменять")
	assert.Equal(t, http.StatusConflict, rec.Code, "без отметок отменять нечего")

	env.mark(t, 1, env.token, env.guardID)

	rec = env.revert(t, 2, env.token, "не то направление")
	assert.Equal(t, http.StatusConflict, rec.Code, "последняя отметка - вход, а отменяют выход")

	rec = env.revert(t, 1, env.token, "   ")
	assert.Equal(t, http.StatusBadRequest, rec.Code, "причина обязательна")

	rec = env.revert(t, 1, env.token, "ошибся строкой")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Повторный заход: действительных отметок больше нет, отменять нечего.
	rec = env.revert(t, 1, env.token, "ещё раз")
	assert.Equal(t, http.StatusConflict, rec.Code, "дважды одну отметку не отменить")
}

// TestPassageRevert_NeedsTablePermission - отмена закрыта тем же правом таблицы, что
// и сама отметка. Права раздаются по постам, и охранник соседнего КПП не должен
// править чужой журнал.
func TestPassageRevert_NeedsTablePermission(t *testing.T) {
	env := setupRevertEnv(t, "kpp_revert_perm", "revguard4")
	env.mark(t, 1, env.token, env.guardID)

	stranger := testutil.RegisterAndLogin(t, env.e, "revstranger", "pass123", 1, env.td.OrgID, env.td.CompanyID)
	rec := env.revert(t, 1, stranger, "чужой пост")
	require.Equal(t, http.StatusForbidden, rec.Code)

	var denied map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &denied))
	assert.Equal(t, "table."+env.table.Name+".entry", denied["required_permission"],
		"отмена гейтится правом отметки того же направления")
}

// TestPassageRevert_ChainRestoresEarlierEntry - цепочка вход-выход-вход. Отменяя
// последний вход, система обязана вернуть не только статус «выехал», но и время
// ПЕРВОГО въезда: у предыдущего события (выезда) своего времени въезда нет, и взять
// его оттуда значит показать на КПП время, которого не было.
func TestPassageRevert_ChainRestoresEarlierEntry(t *testing.T) {
	env := setupRevertEnv(t, "kpp_revert_chain", "revguard5")

	env.mark(t, 1, env.token, env.guardID)
	_, firstEntry := env.carRow(t)
	require.NotNil(t, firstEntry)
	env.ageLastMark(t, 60*time.Minute)

	env.mark(t, 2, env.token, env.guardID)
	env.ageLastMark(t, 30*time.Minute)

	env.mark(t, 1, env.token, env.guardID)

	rec := env.revert(t, 1, env.token, "въехала другая машина")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	status, entryTime := env.carRow(t)
	require.NotNil(t, status)
	assert.Equal(t, 2, *status, "после отмены последнего въезда машина снова за территорией")
	require.NotNil(t, entryTime, "время въезда остаётся от действительного въезда")
	assert.WithinDuration(t, firstEntry.Add(-60*time.Minute), *entryTime, 2*time.Second,
		"время въезда - от первого действительного въезда, а не от предыдущего события")
}

// TestPassageRevert_OtherPostRejected - право на отмену проверяется по посту из тела
// запроса, поэтому охранник соседнего КПП со своим правом мог бы дотянуться до чужой
// отметки. Сверка поста самой отметки это закрывает.
func TestPassageRevert_OtherPostRejected(t *testing.T) {
	env := setupRevertEnv(t, "kpp_revert_post_a", "revguard6")

	dn := "КПП Б"
	other := models.SystemTable{Name: "kpp_revert_post_b", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, env.db.Create(&other).Error)
	testutil.GrantTableVerb(t, env.guardID, other.Name, "entry")

	env.mark(t, 1, env.token, env.guardID)

	rec := testutil.PUT(t, env.e, fmt.Sprintf("/cars/%d/territory-status/revert", env.carID),
		fmt.Sprintf(`{"territory_status": 1, "table_id": %d, "reason": "с соседнего поста"}`, other.ID),
		testutil.AuthHeader(env.token))
	assert.Equal(t, http.StatusConflict, rec.Code, "отметку чужого поста отсюда не отменить")

	status, _ := env.carRow(t)
	require.NotNil(t, status)
	assert.Equal(t, 1, *status, "отказ ничего не откатил")
}

// carStatusRow - строка ответа /cars/history/current-status в объёме, нужном тесту.
type carStatusRow struct {
	CarID           int  `json:"car_id"`
	CanRevert       bool `json:"can_revert"`
	LastMarkTableID *int `json:"last_mark_table_id"`
}

// canRevert спрашивает текущий статус глазами владельца токена.
func (env revertEnv) canRevert(t *testing.T, token string) carStatusRow {
	t.Helper()
	rec := testutil.GET(t, env.e, "/cars/history/current-status", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Data []carStatusRow `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	for _, row := range resp.Data {
		if row.CarID == env.carID {
			return row
		}
	}
	t.Fatalf("машина %d не найдена в текущем статусе", env.carID)
	return carStatusRow{}
}

// TestPassageRevert_CanRevertFlag - таблица не должна сама считать, кому и сколько
// можно отменять: признак приходит из текущего статуса, посчитанный для
// спрашивающего. Иначе окно в пятнадцать минут жило бы копией в трёх компонентах.
func TestPassageRevert_CanRevertFlag(t *testing.T) {
	env := setupRevertEnv(t, "kpp_revert_flag", "revguard7")

	require.False(t, env.canRevert(t, env.token).CanRevert, "отметок нет - отменять нечего")

	env.mark(t, 1, env.token, env.guardID)
	own := env.canRevert(t, env.token)
	assert.True(t, own.CanRevert, "свою свежую отметку отменить можно")
	require.NotNil(t, own.LastMarkTableID)
	assert.Equal(t, env.table.ID, *own.LastMarkTableID, "пост отметки виден таблице")

	stranger := testutil.RegisterAndLogin(t, env.e, "revflag_other", "pass123", 1, env.td.OrgID, env.td.CompanyID)
	assert.False(t, env.canRevert(t, stranger).CanRevert, "чужую отметку рядовой пользователь не отменяет")

	env.ageLastMark(t, 20*time.Minute)
	assert.False(t, env.canRevert(t, env.token).CanRevert, "за пределами окна признак снят")

	adminToken := testutil.RegisterManager(t, env.e, "revflagadmin", env.td.OrgID, env.td.CompanyID)
	assert.True(t, env.canRevert(t, adminToken).CanRevert, "администратору окно не мешает")

	rec := env.revert(t, 1, adminToken, "разбор смены")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.False(t, env.canRevert(t, adminToken).CanRevert, "после отмены отменять снова нечего")
}
