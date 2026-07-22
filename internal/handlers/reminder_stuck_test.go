package handlers_test

// Интеграционные тесты отчёта «Зависшие согласования» (#1315 S4): снимок живых
// заявок, ждущих решения согласующего дольше порога молчания. Отбор зеркалит
// рассылку напоминаний (общий pendingApproverBaseQuery), поэтому те же исключения
// (необязательный при наличии обязательного, отозванные/архивные, свежие) обязаны
// работать и здесь. Помощники (newReminderOrg/User/App/Responsible, newReminderServices,
// cleanupReminderFixture) переиспользуются из reminder_service_test.go.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/handlers"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findStuck ищет строку отчёта по applicationID.
func findStuck(rows []models.StuckApproval, appID int) *models.StuckApproval {
	for i := range rows {
		if rows[i].ApplicationID == appID {
			return &rows[i]
		}
	}
	return nil
}

// TestListStuckApprovals_ReportsSilentApprover — обязательный согласующий молчит
// дольше порога (дефолт 3 дня): попадает в отчёт с числом дней ожидания и именем;
// напоминаний ещё нет (крон не прогонялся).
func TestListStuckApprovals_ReportsSilentApprover(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc, _ := newReminderServices(db)

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	approverID := newReminderUser(t, db, "approver")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusProcessing, models.ConfirmationPending)
	newResponsible(t, db, appID, approverID, true, pendingStatus(), 5)
	defer cleanupReminderFixture(db, []int{appID}, []int{senderID, approverID}, orgID)

	rows, err := svc.ListStuckApprovals(context.Background())
	require.NoError(t, err)

	row := findStuck(rows, appID)
	require.NotNil(t, row, "зависшая заявка должна попасть в отчёт")
	assert.GreaterOrEqual(t, row.WaitingDays, 5, "ждёт минимум 5 дней")
	assert.Contains(t, row.ApproverName, "approver", "имя согласующего из users, не user_id")
	assert.Zero(t, row.ReminderCount, "крон не прогонялся - напоминаний нет")
	assert.Nil(t, row.LastReminderAt)
}

// TestListStuckApprovals_ReflectsRemindersSent — после реального прогона рассылки
// счётчик напоминаний и время последнего в отчёте отражают отправленное.
func TestListStuckApprovals_ReflectsRemindersSent(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc, _ := newReminderServices(db)
	ctx := context.Background()

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	approverID := newReminderUser(t, db, "approver")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusProcessing, models.ConfirmationPending)
	newResponsible(t, db, appID, approverID, true, pendingStatus(), 5)
	defer cleanupReminderFixture(db, []int{appID}, []int{senderID, approverID}, orgID)

	require.NoError(t, svc.SendPendingReminders(ctx))

	rows, err := svc.ListStuckApprovals(ctx)
	require.NoError(t, err)
	row := findStuck(rows, appID)
	require.NotNil(t, row)
	assert.Equal(t, 1, row.ReminderCount, "одно напоминание уже ушло")
	require.NotNil(t, row.LastReminderAt, "время последнего напоминания проставлено")
}

// TestListStuckApprovals_Exclusions — отчёт не показывает: свежих (моложе порога),
// необязательного согласующего при наличии обязательного, отозванные заявки.
func TestListStuckApprovals_Exclusions(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc, _ := newReminderServices(db)

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	freshID := newReminderUser(t, db, "fresh")
	optionalID := newReminderUser(t, db, "optional")
	requiredID := newReminderUser(t, db, "required")
	withdrawnApproverID := newReminderUser(t, db, "wdapprover")

	// Свежий: назначен день назад при пороге 3 - ещё не завис.
	freshApp := newReminderApp(t, db, orgID, senderID, models.StatusProcessing, models.ConfirmationPending)
	newResponsible(t, db, freshApp, freshID, true, pendingStatus(), 1)

	// Необязательный при наличии обязательного: его голос на исход не влияет.
	mixedApp := newReminderApp(t, db, orgID, senderID, models.StatusProcessing, models.ConfirmationPending)
	newResponsible(t, db, mixedApp, requiredID, true, pendingStatus(), 5)
	newResponsible(t, db, mixedApp, optionalID, false, pendingStatus(), 5)

	// Отозванная заявка: confirmation ещё "Согласование", но статус закрывающий.
	withdrawnApp := newReminderApp(t, db, orgID, senderID, models.StatusWithdrawn, models.ConfirmationPending)
	newResponsible(t, db, withdrawnApp, withdrawnApproverID, true, pendingStatus(), 5)

	defer cleanupReminderFixture(db, []int{freshApp, mixedApp, withdrawnApp},
		[]int{senderID, freshID, optionalID, requiredID, withdrawnApproverID}, orgID)

	rows, err := svc.ListStuckApprovals(context.Background())
	require.NoError(t, err)

	assert.Nil(t, findStuck(rows, freshApp), "свежая заявка не зависла")
	assert.Nil(t, findStuck(rows, withdrawnApp), "отозванная заявка исключена")

	// mixedApp виден, но ТОЛЬКО через обязательного - необязательный не отдельная строка.
	var mixedApprovers []string
	for _, r := range rows {
		if r.ApplicationID == mixedApp {
			mixedApprovers = append(mixedApprovers, r.ApproverName)
		}
	}
	require.Len(t, mixedApprovers, 1, "по mixedApp зависает только обязательный")
	assert.Contains(t, mixedApprovers[0], "required")
}

// TestGetStuckApprovals_HTTP — контракт эндпоинта: envelope, статус и имена полей
// JSON, которые фронт (api/statistics.js) читает as-is.
func TestGetStuckApprovals_HTTP(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc, _ := newReminderServices(db)

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	approverID := newReminderUser(t, db, "approver")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusProcessing, models.ConfirmationPending)
	newResponsible(t, db, appID, approverID, true, pendingStatus(), 5)
	defer cleanupReminderFixture(db, []int{appID}, []int{senderID, approverID}, orgID)

	h := handlers.NewReminderHandler(svc)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/statistics/stuck-approvals", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, h.GetStuckApprovals(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool                   `json:"success"`
		Data    []models.StuckApproval `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)

	// Имена полей строки в сыром JSON: фронт читает именно их.
	var raw struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.NotEmpty(t, raw.Data, "в отчёте есть зависшая заявка")
	for _, key := range []string{"application_id", "application_number", "approver_name", "waiting_days", "reminder_count", "last_reminder_at"} {
		assert.Contains(t, raw.Data[0], key, "поле строки отчёта %q", key)
	}
}
