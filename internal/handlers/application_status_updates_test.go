package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplicationStatusUpdateFlags (#1349): флаг "статус обновился" (has_status_update)
// загорается у участников при реальной смене status/confirmation, не загорается у автора
// действия и гаснет при открытии детали. Один SetupTestApp на все кейсы: пакет handlers -
// единственный DB-бинарь и упирается в CI-таймаут.
func TestApplicationStatusUpdateFlags(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "su_sender", "pass123", 1, td.OrgID, td.CompanyID)

	// Два принимающих: actor совершает переходы, observer только смотрит списки.
	testutil.RegisterUser(t, e, "su_actor", "pass123", 6, td.OrgID, td.CompanyID)
	actorID := getUserID(t, db, "su_actor")
	db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", actorID)
	actorToken, _ := testutil.LoginUser(t, e, "su_actor", "pass123")

	testutil.RegisterUser(t, e, "su_observer", "pass123", 6, td.OrgID, td.CompanyID)
	observerID := getUserID(t, db, "su_observer")
	db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", observerID)
	observerToken, _ := testutil.LoginUser(t, e, "su_observer", "pass123")

	// flagInList находит заявку в листинге и возвращает её has_status_update.
	flagInList := func(t *testing.T, token, path string, appID int) bool {
		t.Helper()
		rec := testutil.GET(t, e, path, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		for _, a := range testutil.ParseSlice(t, rec) {
			if int(a["id"].(float64)) == appID {
				v, _ := a["has_status_update"].(bool)
				return v
			}
		}
		t.Fatalf("заявка %d не найдена в %s", appID, path)
		return false
	}
	statusUpdatedAt := func(t *testing.T, appID int) *time.Time {
		t.Helper()
		var app models.Application
		require.NoError(t, db.First(&app, appID).Error)
		return app.StatusUpdatedAt
	}

	t.Run("take_to_work загорается у читавших и отправителя, не у актора", func(t *testing.T) {
		appID := createSimpleApplication(t, e, senderToken, td.OrgID)
		require.Nil(t, statusUpdatedAt(t, appID), "у свежей заявки флага нет")

		// Observer прочитал заявку ДО смены статуса - непрочитанность его больше не подсветит.
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/read", appID), "", testutil.AuthHeader(observerToken))
		require.Equal(t, http.StatusOK, rec.Code)

		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID),
			fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, actorID), testutil.AuthHeader(actorToken))
		require.Equal(t, http.StatusOK, rec.Code, "take-to-work accept: %s", rec.Body.String())

		assert.True(t, flagInList(t, observerToken, "/applications", appID), "observer видит флаг")
		assert.False(t, flagInList(t, actorToken, "/applications", appID), "актор собственный флаг не видит")
		assert.True(t, flagInList(t, senderToken, "/applications/user", appID),
			"отправитель видит флаг в ЛК без каких-либо reads-строк")
	})

	t.Run("гашение через details и повторная новизна", func(t *testing.T) {
		appID := createSimpleApplication(t, e, senderToken, td.OrgID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID),
			fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, actorID), testutil.AuthHeader(actorToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.True(t, flagInList(t, observerToken, "/applications", appID))

		// Открытие детали гасит флаг только у открывшего.
		rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/details", appID), testutil.AuthHeader(observerToken))
		require.Equal(t, http.StatusOK, rec.Code)
		assert.False(t, flagInList(t, observerToken, "/applications", appID), "после details флаг погас")
		assert.True(t, flagInList(t, senderToken, "/applications/user", appID), "у отправителя флаг остался")

		// Следующий переход зажигает флаг заново - отметка не залипает.
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/revoke-from-work", appID),
			fmt.Sprintf(`{"user_id":%d,"comment":"вернул"}`, actorID), testutil.AuthHeader(actorToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.True(t, flagInList(t, observerToken, "/applications", appID), "новый переход - флаг снова горит")
	})

	t.Run("гашение через deep-link GET /:id", func(t *testing.T) {
		appID := createSimpleApplication(t, e, senderToken, td.OrgID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID),
			fmt.Sprintf(`{"user_id":%d,"action":"reject","comment":"нет"}`, actorID), testutil.AuthHeader(actorToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.True(t, flagInList(t, senderToken, "/applications/user", appID))

		rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code)
		assert.False(t, flagInList(t, senderToken, "/applications/user", appID))
	})

	t.Run("withdraw отправителем: горит у принимающих, не у него", func(t *testing.T) {
		appID := createSimpleApplication(t, e, senderToken, td.OrgID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/withdraw", appID), "", testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		assert.True(t, flagInList(t, observerToken, "/applications", appID))
		assert.False(t, flagInList(t, senderToken, "/applications/user", appID), "актор-отправитель флага не видит")
	})

	t.Run("legacy Непрочитано->В обработке флага не ставит", func(t *testing.T) {
		appID := createSimpleApplication(t, e, senderToken, td.OrgID)
		rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(observerToken))
		require.Equal(t, http.StatusOK, rec.Code)

		var status string
		require.NoError(t, db.Raw("SELECT status FROM applications WHERE id = ?", appID).Scan(&status).Error)
		require.Equal(t, models.StatusProcessing, status, "первое открытие перевело статус")
		assert.Nil(t, statusUpdatedAt(t, appID), "переход от факта открытия - шум, не событие")
	})

	t.Run("confirmation: no-op пересчёт не бампает, реальная смена бампает", func(t *testing.T) {
		appID := createSimpleApplication(t, e, senderToken, td.OrgID)

		respToken := testutil.RegisterAndLogin(t, e, "su_resp", "pass123", 1, td.OrgID, td.CompanyID)
		respID := getUserID(t, db, "su_resp")

		// Forward добавляет согласующего: пересчёт confirmation даёт то же "Согласование" - без бампа.
		body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, respID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())
		assert.Nil(t, statusUpdatedAt(t, appID), "no-op пересчёт confirmation не зажигает флаг")

		// Голос согласующего меняет confirmation -> Согласовано: бамп есть, актор исключён.
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID),
			fmt.Sprintf(`{"user_id":%d,"status":"approved"}`, respID), testutil.AuthHeader(respToken))
		require.Equal(t, http.StatusOK, rec.Code, "approve: %s", rec.Body.String())

		require.NotNil(t, statusUpdatedAt(t, appID))
		assert.True(t, flagInList(t, senderToken, "/applications/user", appID), "отправитель видит смену согласования")
		assert.False(t, flagInList(t, respToken, "/applications/user", appID), "проголосовавший - актор, флага нет")
	})
}

// TestCheckExpiredAttachments_StatusUpdateFlag (#1349): завершение заявки кроном зажигает
// флаг ВСЕМ участникам, включая отправителя (актора-человека нет).
func TestCheckExpiredAttachments_StatusUpdateFlag(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	permSvc := services.NewPermissionService(db)
	notifSvc := services.NewNotificationService(db)
	blRecorder := services.NewAuditRecorder(db)
	vblSvc := services.NewVehicleBlacklistService(db, blRecorder)
	pblSvc := services.NewPersonBlacklistService(db, blRecorder)
	appSvc := services.NewApplicationService(db, permSvc, notifSvc, vblSvc, pblSvc, blRecorder)

	uaID := seedUniqueAttachment(t, db, "cars", "su_exp_cars", "SU Exp Cars")
	senderToken := testutil.RegisterAndLogin(t, e, "su_exp_sender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)

	// Активируем элементы (крон завершает только заявки с активными вложениями) и просрочиваем.
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/update-items-status", appID), "", testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code)
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	require.NoError(t, db.Exec("UPDATE attachments SET entry_date_to = ? WHERE application_id = ?", yesterday, appID).Error)

	require.NoError(t, appSvc.CheckExpiredAttachments(context.Background()))

	var app models.Application
	require.NoError(t, db.First(&app, appID).Error)
	require.NotNil(t, app.Status)
	require.Equal(t, models.StatusCompleted, *app.Status)
	assert.NotNil(t, app.StatusUpdatedAt, "крон-завершение зажигает флаг")

	// Отправителю флаг виден: актор nil, его seen_at не проставлялся. Завершённая вчера
	// заявка ещё не архивная (месяц не прошёл) - в ЛК она есть.
	rec = testutil.GET(t, e, "/applications/user", testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code)
	found := false
	for _, a := range testutil.ParseSlice(t, rec) {
		if int(a["id"].(float64)) == appID {
			found = true
			v, _ := a["has_status_update"].(bool)
			assert.True(t, v, "отправитель видит завершение по сроку")
		}
	}
	require.True(t, found, "завершённая заявка видна в ЛК")
}
