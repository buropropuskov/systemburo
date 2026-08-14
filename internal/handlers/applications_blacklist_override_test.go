package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBlacklistOverride_BlocksApprovalUntilConfirmed: помеченную заявку нельзя согласовать,
// пока ответственный не подтвердит пропуск каждого элемента (override) - тогда разблокируется.
func TestBlacklistOverride_BlocksApprovalUntilConfirmed(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	const orgName = "Test Organization"
	senderToken := testutil.RegisterAndLogin(t, e, "bo_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "bo_sender")
	mark := seedMark(t, db, "BO_Mark")

	_, err := newVehicleBlacklistService(db).Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "C777CC799", MarkID: mark.ID, Reason: "угон",
	}, senderID)
	require.NoError(t, err)

	rec := submitCarApp(t, e, db, senderToken, orgName, "ovr", "C777CC798", mark.ID)
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())
	carID := latestElementID(t, db, "cars", "car_number = ?", "C777CC798")
	flag, ok := blacklistFlagFor(t, db, models.BlacklistElementCar, carID)
	require.True(t, ok, "ожидался флаг похожести")
	appID := flag.ApplicationID

	testutil.RegisterUser(t, e, "bo_appr", "pass123", 1, td.OrgID, td.CompanyID)
	apprID := getUserID(t, db, "bo_appr")
	fwd := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, apprID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwd, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())
	apprToken, _ := testutil.LoginUser(t, e, "bo_appr", "pass123")
	approveBody := fmt.Sprintf(`{"user_id":%d,"status":"approved","comment":"ok"}`, apprID)
	overridePath := fmt.Sprintf("/applications/%d/blacklist-overrides", appID)

	t.Run("согласование заблокировано до override", func(t *testing.T) {
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), approveBody, testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusConflict, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), "чёрн")
	})

	t.Run("прямой PUT confirmation=Согласовано тоже заблокирован", func(t *testing.T) {
		rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d", appID), `{"confirmation":"Согласовано"}`, testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusConflict, rec.Code, "обход гейта через PUT не должен проходить: %s", rec.Body.String())
	})

	t.Run("override без комментария отклоняется", func(t *testing.T) {
		rec := testutil.POST(t, e, overridePath, fmt.Sprintf(`{"flag_id":%d,"comment":""}`, flag.ID), testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("override от не-ответственного запрещён", func(t *testing.T) {
		outsiderToken := testutil.RegisterAndLogin(t, e, "bo_outsider", "pass123", 1, td.OrgID, td.CompanyID)
		rec := testutil.POST(t, e, overridePath, fmt.Sprintf(`{"flag_id":%d,"comment":"чужой"}`, flag.ID), testutil.AuthHeader(outsiderToken))
		require.Equal(t, http.StatusForbidden, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("override фиксирует аудит и разблокирует согласование", func(t *testing.T) {
		rec := testutil.POST(t, e, overridePath, fmt.Sprintf(`{"flag_id":%d,"comment":"проверено лично"}`, flag.ID), testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusOK, rec.Code, "override: %s", rec.Body.String())

		var ovr models.ApplicationBlacklistOverride
		require.NoError(t, db.Where("flag_id = ?", flag.ID).First(&ovr).Error)
		assert.Equal(t, apprID, ovr.OverriddenByUserID)
		assert.Equal(t, "проверено лично", ovr.Comment)
		assert.Equal(t, flag.MatchedValue, ovr.MatchedValue, "снимок совпавшего значения для аудита")

		// Оверлей детали теперь помечает элемент как подтверждённый.
		var attID int
		require.NoError(t, db.Raw("SELECT attachment_id FROM cars WHERE id = ?", carID).Scan(&attID).Error)
		det := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/cars", attID), testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusOK, det.Code, "body: %s", det.Body.String())
		assert.Contains(t, det.Body.String(), `"overridden":true`)

		// После override согласование проходит.
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), approveBody, testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusOK, rec.Code, "approve after override: %s", rec.Body.String())
	})
}

// TestBlacklistOverride_RejectionNotBlocked: отказ ('rejected') помеченной заявки не требует
// override - отклонить подозрительную заявку можно сразу.
func TestBlacklistOverride_RejectionNotBlocked(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	const orgName = "Test Organization"
	senderToken := testutil.RegisterAndLogin(t, e, "br_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "br_sender")
	mark := seedMark(t, db, "BR_Mark")

	_, err := newVehicleBlacklistService(db).Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "C777CC799", MarkID: mark.ID, Reason: "угон",
	}, senderID)
	require.NoError(t, err)

	rec := submitCarApp(t, e, db, senderToken, orgName, "rej", "C777CC798", mark.ID)
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())
	carID := latestElementID(t, db, "cars", "car_number = ?", "C777CC798")
	flag, ok := blacklistFlagFor(t, db, models.BlacklistElementCar, carID)
	require.True(t, ok)
	appID := flag.ApplicationID

	testutil.RegisterUser(t, e, "br_appr", "pass123", 1, td.OrgID, td.CompanyID)
	apprID := getUserID(t, db, "br_appr")
	fwd := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, apprID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwd, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

	apprToken, _ := testutil.LoginUser(t, e, "br_appr", "pass123")
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID),
		fmt.Sprintf(`{"user_id":%d,"status":"rejected","comment":"подозрительно"}`, apprID), testutil.AuthHeader(apprToken))
	require.Equal(t, http.StatusOK, rec.Code, "reject не должен блокироваться: %s", rec.Body.String())
}

// TestGetApplicationDetails_BlacklistGateField: GET /applications/:id/details отдаёт
// has_unoverridden_blacklist_flags - true пока есть помеченный элемент без override,
// false после override (зеркало бэкенд-гейта для блокировки кнопки на фронте, #481).
func TestGetApplicationDetails_BlacklistGateField(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	const orgName = "Test Organization"
	senderToken := testutil.RegisterAndLogin(t, e, "bg_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "bg_sender")
	mark := seedMark(t, db, "BG_Mark")

	_, err := newVehicleBlacklistService(db).Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "C777CC799", MarkID: mark.ID, Reason: "угон",
	}, senderID)
	require.NoError(t, err)

	rec := submitCarApp(t, e, db, senderToken, orgName, "gate", "C777CC798", mark.ID)
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())
	carID := latestElementID(t, db, "cars", "car_number = ?", "C777CC798")
	flag, ok := blacklistFlagFor(t, db, models.BlacklistElementCar, carID)
	require.True(t, ok, "ожидался флаг похожести")
	appID := flag.ApplicationID

	detailsPath := fmt.Sprintf("/applications/%d/details", appID)

	t.Run("true пока флаг не переопределён", func(t *testing.T) {
		det := testutil.GET(t, e, detailsPath, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, det.Code, "body: %s", det.Body.String())
		assert.Contains(t, det.Body.String(), `"has_unoverridden_blacklist_flags":true`)
	})

	t.Run("false после override", func(t *testing.T) {
		testutil.RegisterUser(t, e, "bg_appr", "pass123", 1, td.OrgID, td.CompanyID)
		apprID := getUserID(t, db, "bg_appr")
		fwd := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, apprID)
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwd, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		apprToken, _ := testutil.LoginUser(t, e, "bg_appr", "pass123")
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/blacklist-overrides", appID),
			fmt.Sprintf(`{"flag_id":%d,"comment":"проверено лично"}`, flag.ID), testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusOK, rec.Code, "override: %s", rec.Body.String())

		det := testutil.GET(t, e, detailsPath, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, det.Code, "body: %s", det.Body.String())
		assert.Contains(t, det.Body.String(), `"has_unoverridden_blacklist_flags":false`)
	})
}

// TestGetApplications_BlacklistFlagsCount: списки заявок отдают blacklist_flags_count -
// число непереопределённых помеченных элементов заявки (тот же предикат, что и гейт
// согласования). После override счётчик обнуляется. Питает сводный бейдж "N похожи на ЧС"
// в Центре заявок и "Моих заявках" (#481, срез 6c).
func TestGetApplications_BlacklistFlagsCount(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	const orgName = "Test Organization"
	senderToken := testutil.RegisterAndLogin(t, e, "bc_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "bc_sender")
	mark := seedMark(t, db, "BC_Mark")

	_, err := newVehicleBlacklistService(db).Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "C777CC799", MarkID: mark.ID, Reason: "угон",
	}, senderID)
	require.NoError(t, err)

	rec := submitCarApp(t, e, db, senderToken, orgName, "cnt", "C777CC798", mark.ID)
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())
	carID := latestElementID(t, db, "cars", "car_number = ?", "C777CC798")
	flag, ok := blacklistFlagFor(t, db, models.BlacklistElementCar, carID)
	require.True(t, ok, "ожидался флаг похожести")
	appID := flag.ApplicationID

	// countFor извлекает blacklist_flags_count заявки appID из ответа списка (JSON-числа - float64).
	countFor := func(t *testing.T, rec *httptest.ResponseRecorder) float64 {
		t.Helper()
		require.Equal(t, http.StatusOK, rec.Code, "list: %s", rec.Body.String())
		for _, row := range testutil.ParseSlice(t, rec) {
			if row["id"] == float64(appID) {
				cnt, ok := row["blacklist_flags_count"].(float64)
				require.True(t, ok, "blacklist_flags_count отсутствует или не число: %v", row["blacklist_flags_count"])
				return cnt
			}
		}
		t.Fatalf("заявка %d не найдена в списке", appID)
		return 0
	}

	t.Run("счётчик = 1 во всех трёх списках пока флаг не переопределён", func(t *testing.T) {
		assert.Equal(t, float64(1), countFor(t, testutil.GET(t, e, "/applications", testutil.AuthHeader(senderToken))), "GetApplications")
		assert.Equal(t, float64(1), countFor(t, testutil.GET(t, e, "/applications?per_page=50", testutil.AuthHeader(senderToken))), "GetApplicationsPaginated")
		assert.Equal(t, float64(1), countFor(t, testutil.GET(t, e, "/applications/user", testutil.AuthHeader(senderToken))), "GetUserApplications")
	})

	t.Run("счётчик = 0 после override", func(t *testing.T) {
		testutil.RegisterUser(t, e, "bc_appr", "pass123", 1, td.OrgID, td.CompanyID)
		apprID := getUserID(t, db, "bc_appr")
		fwd := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, apprID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwd, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		apprToken, _ := testutil.LoginUser(t, e, "bc_appr", "pass123")
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/blacklist-overrides", appID),
			fmt.Sprintf(`{"flag_id":%d,"comment":"проверено лично"}`, flag.ID), testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusOK, rec.Code, "override: %s", rec.Body.String())

		assert.Equal(t, float64(0), countFor(t, testutil.GET(t, e, "/applications", testutil.AuthHeader(senderToken))), "GetApplications после override")
		assert.Equal(t, float64(0), countFor(t, testutil.GET(t, e, "/applications?per_page=50", testutil.AuthHeader(senderToken))), "GetApplicationsPaginated после override")
		assert.Equal(t, float64(0), countFor(t, testutil.GET(t, e, "/applications/user", testutil.AuthHeader(senderToken))), "GetUserApplications после override")
	})
}

// TestBlacklistOverride_Delete: отмена подтверждения пропуска (#481, срез C). Снять override
// может ответственный по заявке ИЛИ принимающий, но не посторонний; после отмены согласование
// снова блокируется, а факт фиксируется в истории заявки. Повторная отмена -> 404.
func TestBlacklistOverride_Delete(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	const orgName = "Test Organization"
	senderToken := testutil.RegisterAndLogin(t, e, "bd_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "bd_sender")
	mark := seedMark(t, db, "BD_Mark")

	_, err := newVehicleBlacklistService(db).Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "D777DD799", MarkID: mark.ID, Reason: "розыск",
	}, senderID)
	require.NoError(t, err)

	rec := submitCarApp(t, e, db, senderToken, orgName, "del", "D777DD798", mark.ID)
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())
	carID := latestElementID(t, db, "cars", "car_number = ?", "D777DD798")
	flag, ok := blacklistFlagFor(t, db, models.BlacklistElementCar, carID)
	require.True(t, ok, "ожидался флаг похожести")
	appID := flag.ApplicationID

	testutil.RegisterUser(t, e, "bd_appr", "pass123", 1, td.OrgID, td.CompanyID)
	apprID := getUserID(t, db, "bd_appr")
	fwd := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, apprID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwd, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())
	apprToken, _ := testutil.LoginUser(t, e, "bd_appr", "pass123")

	overridePath := fmt.Sprintf("/applications/%d/blacklist-overrides", appID)
	deletePath := fmt.Sprintf("%s?flag_id=%d", overridePath, flag.ID)
	approveBody := fmt.Sprintf(`{"user_id":%d,"status":"approved","comment":"ok"}`, apprID)

	overrideCount := func() int64 {
		var cnt int64
		db.Model(&models.ApplicationBlacklistOverride{}).Where("flag_id = ?", flag.ID).Count(&cnt)
		return cnt
	}

	// Ответственный подтверждает пропуск (предусловие для отмены).
	rec = testutil.POST(t, e, overridePath, fmt.Sprintf(`{"flag_id":%d,"comment":"проверил лично"}`, flag.ID), testutil.AuthHeader(apprToken))
	require.Equal(t, http.StatusOK, rec.Code, "override: %s", rec.Body.String())
	require.Equal(t, int64(1), overrideCount())

	t.Run("отмена от постороннего запрещена", func(t *testing.T) {
		outsiderToken := testutil.RegisterAndLogin(t, e, "bd_outsider", "pass123", 1, td.OrgID, td.CompanyID)
		rec := testutil.DELETE(t, e, deletePath, testutil.AuthHeader(outsiderToken))
		require.Equal(t, http.StatusForbidden, rec.Code, "body: %s", rec.Body.String())
		assert.Equal(t, int64(1), overrideCount(), "посторонний не должен снять override")
	})

	t.Run("ответственный снимает - гейт снова блокирует, факт в истории", func(t *testing.T) {
		rec := testutil.DELETE(t, e, deletePath, testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusOK, rec.Code, "delete: %s", rec.Body.String())
		assert.Equal(t, int64(0), overrideCount(), "override-строка удалена")

		var histCnt int64
		db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = 'blacklist_override_revoke'", models.AuditEntityApplication, appID).Count(&histCnt)
		assert.Equal(t, int64(1), histCnt, "факт отмены залогирован в историю заявки (audit_log, #870 срез 1.14)")

		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), approveBody, testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusConflict, rec.Code, "согласование снова заблокировано: %s", rec.Body.String())
	})

	t.Run("повторная отмена несуществующего - 404", func(t *testing.T) {
		rec := testutil.DELETE(t, e, deletePath, testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("принимающий (не ответственный по заявке) может подтвердить пропуск", func(t *testing.T) {
		require.Equal(t, int64(0), overrideCount(), "предусловие: подтверждения ещё нет")

		accToken := testutil.RegisterAndLogin(t, e, "bd_acceptor_set", "pass123", 1, td.OrgID, td.CompanyID)
		accID := getUserID(t, db, "bd_acceptor_set")
		db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", accID)

		rec := testutil.POST(t, e, overridePath, fmt.Sprintf(`{"flag_id":%d,"comment":"проверил лично"}`, flag.ID), testutil.AuthHeader(accToken))
		require.Equal(t, http.StatusOK, rec.Code, "override принимающим: %s", rec.Body.String())

		var ovr models.ApplicationBlacklistOverride
		require.NoError(t, db.Where("flag_id = ?", flag.ID).First(&ovr).Error)
		assert.Equal(t, accID, ovr.OverriddenByUserID, "подтверждение записано на принимающего")

		rec = testutil.DELETE(t, e, deletePath, testutil.AuthHeader(accToken))
		require.Equal(t, http.StatusOK, rec.Code, "уборка подтверждения: %s", rec.Body.String())
	})

	t.Run("принимающий (не ответственный по заявке) может снять", func(t *testing.T) {
		rec := testutil.POST(t, e, overridePath, fmt.Sprintf(`{"flag_id":%d,"comment":"снова"}`, flag.ID), testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusOK, rec.Code, "re-override: %s", rec.Body.String())
		require.Equal(t, int64(1), overrideCount())

		accToken := testutil.RegisterAndLogin(t, e, "bd_acceptor", "pass123", 1, td.OrgID, td.CompanyID)
		accID := getUserID(t, db, "bd_acceptor")
		db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", accID)

		rec = testutil.DELETE(t, e, deletePath, testutil.AuthHeader(accToken))
		require.Equal(t, http.StatusOK, rec.Code, "delete принимающим: %s", rec.Body.String())
		assert.Equal(t, int64(0), overrideCount())
	})
}

// TestBlacklistOverride_HistoryAndSuppression: "всё равно пропустить" фиксируется в истории
// заявки И машины (#481, срез C-followup), и гасит повторное предупреждение для той же машины
// в будущих заявках, пока override жив; после отмены override предупреждение снова появляется.
func TestBlacklistOverride_HistoryAndSuppression(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	const orgName = "Test Organization"
	senderToken := testutil.RegisterAndLogin(t, e, "bh_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "bh_sender")
	mark := seedMark(t, db, "BH_Mark")

	_, err := newVehicleBlacklistService(db).Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "H777HH799", MarkID: mark.ID, Reason: "розыск",
	}, senderID)
	require.NoError(t, err)

	rec := submitCarApp(t, e, db, senderToken, orgName, "h1", "H777HH798", mark.ID)
	require.Equal(t, http.StatusOK, rec.Code, "submit1: %s", rec.Body.String())
	carID := latestElementID(t, db, "cars", "car_number = ?", "H777HH798")
	flag, ok := blacklistFlagFor(t, db, models.BlacklistElementCar, carID)
	require.True(t, ok, "ожидался флаг похожести")
	appID := flag.ApplicationID

	testutil.RegisterUser(t, e, "bh_appr", "pass123", 1, td.OrgID, td.CompanyID)
	apprID := getUserID(t, db, "bh_appr")
	fwd := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, apprID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwd, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())
	apprToken, _ := testutil.LoginUser(t, e, "bh_appr", "pass123")

	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/blacklist-overrides", appID),
		fmt.Sprintf(`{"flag_id":%d,"comment":"проверил лично"}`, flag.ID), testutil.AuthHeader(apprToken))
	require.Equal(t, http.StatusOK, rec.Code, "override: %s", rec.Body.String())

	t.Run("override пишет историю заявки и машины", func(t *testing.T) {
		var appHist int64
		db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = 'blacklist_override'", models.AuditEntityApplication, appID).Count(&appHist)
		assert.Equal(t, int64(1), appHist, "запись в истории заявки (audit_log, #870 срез 1.14)")

		var carHist int64
		db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = 'blacklist_override'", models.AuditEntityCar, carID).Count(&carHist)
		assert.Equal(t, int64(1), carHist, "запись в истории машины (audit_log, #870 срез 1.12c)")

		// В комментарии видно, КАКУЮ машину пропустили и причину - иначе в истории непонятно.
		var carComment string
		db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = 'blacklist_override'", models.AuditEntityCar, carID).
			Select("details->>'comment'").Scan(&carComment)
		assert.Contains(t, carComment, "H777HH798", "в истории должен быть номер машины")
		assert.Contains(t, carComment, "проверил лично", "в истории должна быть причина")
	})

	t.Run("после пропуска та же машина в новой заявке не помечается", func(t *testing.T) {
		rec := submitCarApp(t, e, db, senderToken, orgName, "h2", "H777HH798", mark.ID)
		require.Equal(t, http.StatusOK, rec.Code, "submit2: %s", rec.Body.String())
		car2 := latestElementID(t, db, "cars", "car_number = ?", "H777HH798")
		require.NotEqual(t, carID, car2, "ожидалась новая строка машины")
		_, ok := blacklistFlagFor(t, db, models.BlacklistElementCar, car2)
		assert.False(t, ok, "повторное предупреждение должно быть подавлено после override")
	})

	t.Run("после отмены пропуска предупреждение снова появляется", func(t *testing.T) {
		del := testutil.DELETE(t, e, fmt.Sprintf("/applications/%d/blacklist-overrides?flag_id=%d", appID, flag.ID), testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusOK, del.Code, "delete: %s", del.Body.String())

		rec := submitCarApp(t, e, db, senderToken, orgName, "h3", "H777HH798", mark.ID)
		require.Equal(t, http.StatusOK, rec.Code, "submit3: %s", rec.Body.String())
		car3 := latestElementID(t, db, "cars", "car_number = ?", "H777HH798")
		_, ok := blacklistFlagFor(t, db, models.BlacklistElementCar, car3)
		assert.True(t, ok, "после отмены override предупреждение снова появляется")
	})
}
