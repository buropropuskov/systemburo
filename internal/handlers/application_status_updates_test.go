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
// действия и гаснет при открытии детали. ЕДИНСТВЕННЫЙ SetupTestApp/CleanDB и сквозные
// сценарии на минимуме заявок: пакет handlers - единственный DB-бинарь и упирается в
// CI -timeout 300s, каждый лишний сетап/HTTP-вызов приближает панику по таймауту.
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

	// --- Сквозной сценарий на одной заявке: accept -> details -> revoke -> deep-link -> withdraw ---
	appA := createSimpleApplication(t, e, senderToken, td.OrgID)

	t.Run("take_to_work загорается у читавших и отправителя, не у актора", func(t *testing.T) {
		require.Nil(t, statusUpdatedAt(t, appA), "у свежей заявки флага нет")

		// Observer прочитал заявку ДО смены статуса - непрочитанность его больше не подсветит.
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/read", appA), "", testutil.AuthHeader(observerToken))
		require.Equal(t, http.StatusOK, rec.Code)

		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appA),
			fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, actorID), testutil.AuthHeader(actorToken))
		require.Equal(t, http.StatusOK, rec.Code, "take-to-work accept: %s", rec.Body.String())

		assert.True(t, flagInList(t, observerToken, "/applications", appA), "observer видит флаг")
		assert.False(t, flagInList(t, actorToken, "/applications", appA), "актор собственный флаг не видит")
		assert.True(t, flagInList(t, senderToken, "/applications/user", appA),
			"отправитель видит флаг в ЛК без каких-либо reads-строк")
	})

	t.Run("гашение через details и повторная новизна", func(t *testing.T) {
		// Открытие детали гасит флаг только у открывшего.
		rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/details", appA), testutil.AuthHeader(observerToken))
		require.Equal(t, http.StatusOK, rec.Code)
		assert.False(t, flagInList(t, observerToken, "/applications", appA), "после details флаг погас")
		assert.True(t, flagInList(t, senderToken, "/applications/user", appA), "у отправителя флаг остался")

		// Следующий переход зажигает флаг заново - отметка не залипает.
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/revoke-from-work", appA),
			fmt.Sprintf(`{"user_id":%d,"comment":"вернул"}`, actorID), testutil.AuthHeader(actorToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.True(t, flagInList(t, observerToken, "/applications", appA), "новый переход - флаг снова горит")
	})

	t.Run("гашение через deep-link GET /:id", func(t *testing.T) {
		require.True(t, flagInList(t, senderToken, "/applications/user", appA))
		rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d", appA), testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code)
		assert.False(t, flagInList(t, senderToken, "/applications/user", appA))
	})

	t.Run("withdraw отправителем: горит у принимающих, не у него", func(t *testing.T) {
		// Observer гасит флаг деталью, чтобы withdraw-бамп был виден как НОВОЕ событие.
		rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/details", appA), testutil.AuthHeader(observerToken))
		require.Equal(t, http.StatusOK, rec.Code)

		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/withdraw", appA), "", testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		assert.True(t, flagInList(t, observerToken, "/applications", appA))
		assert.False(t, flagInList(t, senderToken, "/applications/user", appA), "актор-отправитель флага не видит")
	})

	// --- Отдельная заявка: confirmation-флоу ---
	t.Run("confirmation: no-op пересчёт не бампает, реальная смена бампает", func(t *testing.T) {
		appB := createSimpleApplication(t, e, senderToken, td.OrgID)

		respToken := testutil.RegisterAndLogin(t, e, "su_resp", "pass123", 1, td.OrgID, td.CompanyID)
		respID := getUserID(t, db, "su_resp")

		// Forward добавляет согласующего: пересчёт confirmation даёт то же "Согласование" - без бампа.
		body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, respID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appB), body, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())
		assert.Nil(t, statusUpdatedAt(t, appB), "no-op пересчёт confirmation не зажигает флаг")

		// Голос согласующего меняет confirmation -> Согласовано: бамп есть, актор исключён.
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appB),
			fmt.Sprintf(`{"user_id":%d,"status":"approved"}`, respID), testutil.AuthHeader(respToken))
		require.Equal(t, http.StatusOK, rec.Code, "approve: %s", rec.Body.String())

		require.NotNil(t, statusUpdatedAt(t, appB))
		assert.True(t, flagInList(t, senderToken, "/applications/user", appB), "отправитель видит смену согласования")
		assert.False(t, flagInList(t, respToken, "/applications/user", appB), "проголосовавший - актор, флага нет")
	})

	// --- Отдельная заявка: legacy-переход при первом открытии ---
	t.Run("legacy Непрочитано->В обработке флага не ставит", func(t *testing.T) {
		appC := createSimpleApplication(t, e, senderToken, td.OrgID)
		rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d", appC), testutil.AuthHeader(observerToken))
		require.Equal(t, http.StatusOK, rec.Code)

		var status string
		require.NoError(t, db.Raw("SELECT status FROM applications WHERE id = ?", appC).Scan(&status).Error)
		require.Equal(t, models.StatusProcessing, status, "первое открытие перевело статус")
		assert.Nil(t, statusUpdatedAt(t, appC), "переход от факта открытия - шум, не событие")
	})

	// --- Крон завершения: актора-человека нет, флаг всем, включая отправителя ---
	t.Run("крон-завершение зажигает флаг всем", func(t *testing.T) {
		permSvc := services.NewPermissionService(db)
		notifSvc := services.NewNotificationService(db)
		blRecorder := services.NewAuditRecorder(db)
		appSvc := services.NewApplicationService(db, permSvc, notifSvc,
			services.NewVehicleBlacklistService(db, blRecorder), services.NewPersonBlacklistService(db, blRecorder), blRecorder)

		uaID := seedUniqueAttachment(t, db, "cars", "su_exp_cars", "SU Exp Cars")
		appD := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)

		// Активируем элементы (крон завершает только заявки с активными вложениями) и просрочиваем.
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/update-items-status", appD), "", testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code)
		yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
		require.NoError(t, db.Exec("UPDATE attachments SET entry_date_to = ? WHERE application_id = ?", yesterday, appD).Error)

		require.NoError(t, appSvc.CheckExpiredAttachments(context.Background()))

		var app models.Application
		require.NoError(t, db.First(&app, appD).Error)
		require.NotNil(t, app.Status)
		require.Equal(t, models.StatusCompleted, *app.Status)
		assert.NotNil(t, app.StatusUpdatedAt, "крон-завершение зажигает флаг")

		// Завершённая вчера заявка ещё не архивная (месяц не прошёл) - в ЛК она есть,
		// и отправитель видит флаг: актор nil, его seen_at не проставлялся.
		assert.True(t, flagInList(t, senderToken, "/applications/user", appD), "отправитель видит завершение по сроку")
	})
}
