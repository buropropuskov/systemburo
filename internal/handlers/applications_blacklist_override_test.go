package handlers_test

import (
	"context"
	"fmt"
	"net/http"
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
