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

// Пометка о сходстве - снимок совпавшей записи чёрного списка на момент подачи. Если
// запись потом убрали из списка, предупреждать больше не о чем: пометка должна гаснуть,
// а заявка - согласовываться без подтверждения пропуска.

func TestBlacklistFlag_ReleasedWhenEntryArchived(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	const orgName = "Test Organization"
	senderToken := testutil.RegisterAndLogin(t, e, "bfr_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "bfr_sender")
	mark := seedMark(t, db, "BFR_Mark")

	entry, err := newVehicleBlacklistService(db).Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "F777FF799", MarkID: mark.ID, Reason: "розыск",
	}, senderID)
	require.NoError(t, err)

	rec := submitCarApp(t, e, db, senderToken, orgName, "flagrel", "F777FF798", mark.ID)
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())
	carID := latestElementID(t, db, "cars", "car_number = ?", "F777FF798")
	flag, ok := blacklistFlagFor(t, db, models.BlacklistElementCar, carID)
	require.True(t, ok, "ожидался флаг похожести")
	appID := flag.ApplicationID

	// Согласующий, чей голос обязателен: на нём и виден гейт.
	testutil.RegisterUser(t, e, "bfr_appr", "pass123", 1, td.OrgID, td.CompanyID)
	apprID := getUserID(t, db, "bfr_appr")
	fwd := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, apprID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwd, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())
	apprToken, _ := testutil.LoginUser(t, e, "bfr_appr", "pass123")
	approveBody := fmt.Sprintf(`{"user_id":%d,"status":"approved","comment":"ok"}`, apprID)

	t.Run("пока запрет действует - согласование заблокировано", func(t *testing.T) {
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), approveBody, testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusConflict, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("запись убрана из чёрного списка - пометка не приходит по вложению", func(t *testing.T) {
		require.NoError(t, newVehicleBlacklistService(db).Archive(ctx, entry.ID, senderID))

		rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "детали: %s", rec.Body.String())
		assert.NotContains(t, rec.Body.String(), `"blacklist_similar"`, "пометка снята вместе с запретом")
	})

	t.Run("после снятия запрета согласование проходит без подтверждения пропуска", func(t *testing.T) {
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), approveBody, testutil.AuthHeader(apprToken))
		require.Equal(t, http.StatusOK, rec.Code, "согласование: %s", rec.Body.String())

		var overrides int64
		require.NoError(t, db.Table("application_blacklist_overrides").
			Where("flag_id = ?", flag.ID).Count(&overrides).Error)
		assert.Equal(t, int64(0), overrides, "подтверждение пропуска не понадобилось")
	})

	t.Run("возврат записи в чёрный список снова поднимает пометку", func(t *testing.T) {
		require.NoError(t, newVehicleBlacklistService(db).Restore(ctx, entry.ID, senderID))

		var count int64
		require.NoError(t, db.Table("application_blacklist_flags").
			Where("id = ?", flag.ID).Count(&count).Error)
		assert.Equal(t, int64(1), count, "сама пометка не удалялась - она снимок, а не состояние")

		rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "детали: %s", rec.Body.String())
	})
}
