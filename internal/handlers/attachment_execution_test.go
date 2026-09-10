package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/handlers"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Тесты отметки "вложение исполнено" (#2446, вкладка "Доступные мне"). Реюз secHTTPWorld
// из security_attachments_test.go (тот же пакет handlers_test, единственный DB-тест-бинарь
// проекта).

// markExecuted зовёт эндпоинт отметки вложения исполненным.
func markExecuted(t *testing.T, h secHTTPWorld, attID int, token string) *httptest.ResponseRecorder {
	t.Helper()
	return testutil.POST(t, h.e, fmt.Sprintf("/applications/available-attachments/%d/mark-executed", attID),
		"", testutil.AuthHeader(token))
}

// executionMarkCount считает записи журнала по конкретному вложению - проверяет, что
// повтор внутри окна НЕ пишет вторую строку, а повтор за окном пишет.
func executionMarkCount(t *testing.T, h secHTTPWorld, attID int) int {
	t.Helper()
	var count int64
	require.NoError(t, h.w.db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityAttachment, attID, models.AuditActionAttachmentExecuted).
		Count(&count).Error)
	return int(count)
}

// ageExecutionMark состаривает последнюю отметку вложения: окно проверяется по времени
// записи, а ждать пять минут в тесте нельзя.
func ageExecutionMark(t *testing.T, h secHTTPWorld, attID int, d time.Duration) {
	t.Helper()
	require.NoError(t, h.w.db.Exec(`
		UPDATE audit_log SET created_at = created_at - ?::interval
		WHERE id = (SELECT id FROM audit_log WHERE entity_type = ? AND entity_id = ? AND action = ?
			ORDER BY created_at DESC, id DESC LIMIT 1)`,
		fmt.Sprintf("%d minutes", int(d.Minutes())), models.AuditEntityAttachment, attID,
		models.AuditActionAttachmentExecuted).Error)
}

// newMarkableAttachment - вложение, видимое охраннику h.guardToken: своё место разгрузки
// на подтверждённой заявке в работе.
func newMarkableAttachment(t *testing.T, h secHTTPWorld) int {
	t.Helper()
	w := h.w
	place := w.newUnloadPlace(t, "Склад отметки", true)
	w.assignUnloadPlace(t, place)
	app := w.newApp(t, models.ConfirmationApproved)
	att := w.newAttachment(t, app, "cars")
	w.attachPlace(t, att, place)
	return att
}

// TestMarkAttachmentExecuted_Basic - основной сценарий: охранник открыл вложение и
// отметил его исполненным. Ответ несёт срок действия отметки, деталь вложения его же
// отражает, а в журнале остаётся ровно одна запись.
func TestMarkAttachmentExecuted_Basic(t *testing.T) {
	h := setupSecurityHTTP(t)
	att := newMarkableAttachment(t, h)

	before := secGetDetail(t, h, att, h.guardToken)
	assert.Nil(t, before.ExecutionMarkedUntil, "до отметки повтор ничем не ограничен")

	start := time.Now()
	rec := markExecuted(t, h, att, h.guardToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	resp := testutil.ParseResponse[handlers.AttachmentExecutionMarkResponse](t, rec)
	assert.WithinDuration(t, start.Add(5*time.Minute), resp.ExecutionMarkedUntil, 10*time.Second,
		"окно отметки - 5 минут от момента нажатия")

	assert.Equal(t, 1, executionMarkCount(t, h, att), "в журнале одна запись об отметке")

	after := secGetDetail(t, h, att, h.guardToken)
	require.NotNil(t, after.ExecutionMarkedUntil, "деталь отражает свежую отметку")
	assert.WithinDuration(t, resp.ExecutionMarkedUntil, *after.ExecutionMarkedUntil, time.Second)
}

// TestMarkAttachmentExecuted_RepeatWithinWindow - повторное нажатие в течение окна
// отклоняется и не плодит вторую запись в журнале.
func TestMarkAttachmentExecuted_RepeatWithinWindow(t *testing.T) {
	h := setupSecurityHTTP(t)
	att := newMarkableAttachment(t, h)

	rec := markExecuted(t, h, att, h.guardToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = markExecuted(t, h, att, h.guardToken)
	assert.Equal(t, http.StatusConflict, rec.Code, "повтор внутри окна отклоняется: %s", rec.Body.String())

	assert.Equal(t, 1, executionMarkCount(t, h, att), "вторая попытка не добавила запись")
}

// TestMarkAttachmentExecuted_RepeatAfterWindow - за пределами окна отметка снова
// доступна: за день по вложению может накопиться несколько заездов.
func TestMarkAttachmentExecuted_RepeatAfterWindow(t *testing.T) {
	h := setupSecurityHTTP(t)
	att := newMarkableAttachment(t, h)

	rec := markExecuted(t, h, att, h.guardToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	ageExecutionMark(t, h, att, 6*time.Minute)

	rec = markExecuted(t, h, att, h.guardToken)
	require.Equal(t, http.StatusOK, rec.Code, "за окном отметка снова доступна: %s", rec.Body.String())

	assert.Equal(t, 2, executionMarkCount(t, h, att), "второй заезд оставил вторую запись")
}

// TestMarkAttachmentExecuted_NoAccess - отметить может только тот, кто видит вложение в
// "Доступные мне": ни посторонний пользователь, ни охранник чужого места.
func TestMarkAttachmentExecuted_NoAccess(t *testing.T) {
	h := setupSecurityHTTP(t)
	w := h.w

	myPlace := w.newUnloadPlace(t, "Склад свой", true)
	otherPlace := w.newUnloadPlace(t, "Склад чужой", true)
	w.assignUnloadPlace(t, myPlace)

	app := w.newApp(t, models.ConfirmationApproved)
	foreignAtt := w.newAttachment(t, app, "cars")
	w.attachPlace(t, foreignAtt, otherPlace)

	rec := markExecuted(t, h, foreignAtt, h.userToken)
	assert.Equal(t, http.StatusForbidden, rec.Code, "обычный пользователь не имеет доступа: %s", rec.Body.String())

	rec = markExecuted(t, h, foreignAtt, h.guardToken)
	assert.Equal(t, http.StatusForbidden, rec.Code, "чужое место закрыто и для отметки: %s", rec.Body.String())

	assert.Equal(t, 0, executionMarkCount(t, h, foreignAtt), "отказ не оставил запись в журнале")
}
