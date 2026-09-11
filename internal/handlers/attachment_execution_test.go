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
	assert.Nil(t, before.Attachment.ExecutionMarkedUntil, "до отметки повтор ничем не ограничен")
	require.NotNil(t, before.Attachment.ExecutionMarks, "сводка отметок приходит всегда, даже пустая")
	assert.Zero(t, before.Attachment.ExecutionMarks.TodayCount, "до отметки сегодня отмечено 0 раз")
	assert.Empty(t, before.Attachment.ExecutionMarks.Recent)

	start := time.Now()
	rec := markExecuted(t, h, att, h.guardToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	resp := testutil.ParseResponse[handlers.AttachmentExecutionMarkResponse](t, rec)
	assert.WithinDuration(t, start.Add(5*time.Minute), resp.ExecutionMarkedUntil, 10*time.Second,
		"окно отметки - 5 минут от момента нажатия")

	assert.Equal(t, 1, executionMarkCount(t, h, att), "в журнале одна запись об отметке")

	after := secGetDetail(t, h, att, h.guardToken)
	require.NotNil(t, after.Attachment.ExecutionMarkedUntil, "деталь отражает свежую отметку")
	assert.WithinDuration(t, resp.ExecutionMarkedUntil, *after.Attachment.ExecutionMarkedUntil, time.Second)

	require.Equal(t, 1, after.Attachment.ExecutionMarks.TodayCount, "сводка увидела свежую отметку")
	require.Len(t, after.Attachment.ExecutionMarks.Recent, 1)
	assert.WithinDuration(t, start, after.Attachment.ExecutionMarks.Recent[0].CreatedAt, 10*time.Second)
	require.NotNil(t, after.Attachment.ExecutionMarks.Recent[0].ActorName)
}

// TestMarkAttachmentExecuted_SummaryActorName - автор отметки приходит фамилией с
// инициалами (format_short_name), как отправитель заявки в том же листинге.
func TestMarkAttachmentExecuted_SummaryActorName(t *testing.T) {
	h := setupSecurityHTTP(t)
	att := newMarkableAttachment(t, h)
	require.NoError(t, h.w.db.Table("users").Where("id = ?", h.w.guardID).
		Updates(map[string]interface{}{"last_name": "Иванов", "first_name": "Пётр", "middle_name": "Сергеевич"}).Error)

	rec := markExecuted(t, h, att, h.guardToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	after := secGetDetail(t, h, att, h.guardToken)
	require.Len(t, after.Attachment.ExecutionMarks.Recent, 1)
	require.NotNil(t, after.Attachment.ExecutionMarks.Recent[0].ActorName)
	assert.Equal(t, "Иванов П.С.", *after.Attachment.ExecutionMarks.Recent[0].ActorName)
}

// markSeveralTimes отмечает вложение n раз подряд, состаривая предыдущую отметку
// перед каждым следующим нажатием - иначе окно в 5 минут отклонило бы повтор.
func markSeveralTimes(t *testing.T, h secHTTPWorld, attID, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		rec := markExecuted(t, h, attID, h.guardToken)
		require.Equal(t, http.StatusOK, rec.Code, "отметка %d из %d: %s", i+1, n, rec.Body.String())
		if i < n-1 {
			ageExecutionMark(t, h, attID, 6*time.Minute)
		}
	}
}

// TestMarkAttachmentExecuted_SummaryCountsToday - несколько заездов за день считаются
// все, список несёт их новыми сверху.
func TestMarkAttachmentExecuted_SummaryCountsToday(t *testing.T) {
	h := setupSecurityHTTP(t)
	att := newMarkableAttachment(t, h)

	markSeveralTimes(t, h, att, 3)

	after := secGetDetail(t, h, att, h.guardToken)
	require.Equal(t, 3, after.Attachment.ExecutionMarks.TodayCount, "три заезда - три отметки за сегодня")
	require.Len(t, after.Attachment.ExecutionMarks.Recent, 3)
	for i := 1; i < len(after.Attachment.ExecutionMarks.Recent); i++ {
		assert.False(t, after.Attachment.ExecutionMarks.Recent[i-1].CreatedAt.Before(after.Attachment.ExecutionMarks.Recent[i].CreatedAt),
			"список новыми сверху")
	}
}

// TestMarkAttachmentExecuted_SummaryExcludesYesterday - отметка позавчерашнего дня не
// попадает ни в счётчик "сегодня", ни в список: иначе строка "сегодня отмечено N раз"
// разошлась бы с тем, что реально случилось за сутки.
func TestMarkAttachmentExecuted_SummaryExcludesYesterday(t *testing.T) {
	h := setupSecurityHTTP(t)
	att := newMarkableAttachment(t, h)

	rec := markExecuted(t, h, att, h.guardToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// 25 часов - заведомо больше суток, попадает на предыдущие сутки по Москве
	// независимо от текущего времени прогона теста (в отличие от ровно 24 часов).
	ageExecutionMark(t, h, att, 25*time.Hour)

	rec = markExecuted(t, h, att, h.guardToken)
	require.Equal(t, http.StatusOK, rec.Code, "вчерашняя отметка не блокирует - окно давно закрыто: %s", rec.Body.String())

	after := secGetDetail(t, h, att, h.guardToken)
	assert.Equal(t, 2, executionMarkCount(t, h, att), "в журнале обе отметки - вчерашняя и сегодняшняя")
	require.Equal(t, 1, after.Attachment.ExecutionMarks.TodayCount, "в сводке - только сегодняшняя")
	require.Len(t, after.Attachment.ExecutionMarks.Recent, 1)
}

// TestMarkAttachmentExecuted_SummaryLimit - список обрезается пределом, а счётчик
// "сегодня" остаётся точным даже когда отметок больше предела.
func TestMarkAttachmentExecuted_SummaryLimit(t *testing.T) {
	h := setupSecurityHTTP(t)
	att := newMarkableAttachment(t, h)

	const marksToday = 12 // больше attachmentExecutionRecentLimit (10, см. attachment_execution_service.go)
	markSeveralTimes(t, h, att, marksToday)

	after := secGetDetail(t, h, att, h.guardToken)
	assert.Equal(t, marksToday, after.Attachment.ExecutionMarks.TodayCount,
		"счётчик не подрезан пределом списка")
	assert.Len(t, after.Attachment.ExecutionMarks.Recent, 10,
		"список обрезан пределом - иначе карточка вложения не читается")
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
