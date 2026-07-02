package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForwardMessages_RoundTrip закрывает #967: ветка заявки. Сопроводительное сообщение
// при пересылке пишется в comment сводной записи forwarded, отдаётся GET /forward-messages
// с ФИО автора и получателей (recipients), попадает в историю, видно получателям и закрыто
// от посторонних. Пересылка без текста тоже входит в ветку (message пустой), порядок
// хронологический (старые сверху).
func TestForwardMessages_RoundTrip(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Отправитель (он же пересылающий = автор сообщения). Проставляем ФИО, чтобы
	// author_name в ответе был детерминированным.
	senderToken := testutil.RegisterAndLogin(t, e, "fwdmsg_sender", "pass123", 1, td.OrgID, td.CompanyID)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "fwdmsg_sender").
		Updates(map[string]interface{}{"last_name": "Петров", "first_name": "Пётр", "middle_name": "Петрович"}).Error)
	const wantAuthor = "Петров Пётр Петрович"

	// Получатель-читатель: видит заявку и должен видеть ветку. ФИО проставляем, чтобы
	// recipients в ответе был детерминированным.
	testutil.RegisterUser(t, e, "fwdmsg_viewer", "pass123", 1, td.OrgID, td.CompanyID)
	viewerID := getUserID(t, db, "fwdmsg_viewer")
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "fwdmsg_viewer").
		Updates(map[string]interface{}{"last_name": "Иванов", "first_name": "Иван", "middle_name": "Иванович"}).Error)
	const wantRecipient = "Иванов Иван Иванович"
	viewerToken, _ := testutil.LoginUser(t, e, "fwdmsg_viewer", "pass123")

	// Посторонний: доступа к заявке нет -> 403.
	testutil.RegisterUser(t, e, "fwdmsg_outsider", "pass123", 1, td.OrgID, td.CompanyID)
	outsiderToken, _ := testutil.LoginUser(t, e, "fwdmsg_outsider", "pass123")

	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	// Пересылка с сопроводительным сообщением.
	const wantMessage = "Прошу дополнительно согласовать заявку с вами"
	fwdBody := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":false,"can_view":true}],"message":%q}`, viewerID, wantMessage)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwdBody, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward с сообщением: %s", rec.Body.String())

	// GET /forward-messages от автора: одна запись с верным ФИО и текстом.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/forward-messages", appID), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	msgs := testutil.ParseResponse[[]services.ForwardMessageItem](t, rec)
	require.Len(t, msgs, 1, "должна быть одна запись сообщения пересылки")
	assert.Equal(t, wantMessage, msgs[0].Message)
	assert.Equal(t, wantAuthor, msgs[0].AuthorName)
	assert.Equal(t, []string{wantRecipient}, msgs[0].Recipients, "recipients должны нести ФИО получателя")
	assert.True(t, msgs[0].Whole, "переслана вся заявка (без выбора вложений)")
	assert.Empty(t, msgs[0].Attachments, "у пересылки всей заявки нет перечня вложений")

	// История заявки: запись forwarded несёт comment с тем же текстом.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	hist := testutil.ParseSlice(t, rec)
	fwd := findHistoryEntry(t, hist, "forwarded")
	assert.Equal(t, wantMessage, fwd["comment"], "forwarded в истории должен нести comment")

	// Получатель-читатель видит сообщение.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/forward-messages", appID), testutil.AuthHeader(viewerToken))
	require.Equal(t, http.StatusOK, rec.Code, "получатель должен видеть сообщения: %s", rec.Body.String())
	viewerMsgs := testutil.ParseResponse[[]services.ForwardMessageItem](t, rec)
	require.Len(t, viewerMsgs, 1)
	assert.Equal(t, wantMessage, viewerMsgs[0].Message)

	// Посторонний -> 403.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/forward-messages", appID), testutil.AuthHeader(outsiderToken))
	assert.Equal(t, http.StatusForbidden, rec.Code, "посторонний не должен видеть сообщения пересылки")

	// Пересылка без сообщения новому получателю: тоже входит в ветку (message пустой).
	testutil.RegisterUser(t, e, "fwdmsg_viewer2", "pass123", 1, td.OrgID, td.CompanyID)
	viewer2ID := getUserID(t, db, "fwdmsg_viewer2")
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "fwdmsg_viewer2").
		Updates(map[string]interface{}{"last_name": "Сидоров", "first_name": "Сидор", "middle_name": "Сидорович"}).Error)
	const wantRecipient2 = "Сидоров Сидор Сидорович"
	emptyBody := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":false,"can_view":true}],"message":"   "}`, viewer2ID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), emptyBody, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward без сообщения: %s", rec.Body.String())

	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/forward-messages", appID), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	afterEmpty := testutil.ParseResponse[[]services.ForwardMessageItem](t, rec)
	require.Len(t, afterEmpty, 2, "пересылка без текста тоже входит в ветку")
	// Порядок хронологический (старые сверху): первая - с текстом, вторая - пустая.
	assert.Equal(t, wantMessage, afterEmpty[0].Message, "первая пересылка сверху")
	assert.Empty(t, afterEmpty[1].Message, "пересылка без текста несёт пустое сообщение")
	assert.Equal(t, []string{wantRecipient2}, afterEmpty[1].Recipients, "recipients пустой пересылки")
}

// TestForwardMessages_AttachmentSubset: пересылка конкретных вложений (#967, обогащение
// ветки) даёт в ветке действие whole=false с перечнем вложений. Проверяет, что
// GetForwardMessages извлекает whole/attachments из живого Postgres - каст
// metadata->>'whole'::boolean на реальном false и разбор metadata.attachments.
func TestForwardMessages_AttachmentSubset(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "fwdatt_sender", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "fwdatt_viewer", "pass123", 1, td.OrgID, td.CompanyID)
	viewerID := getUserID(t, db, "fwdatt_viewer")

	uaID := seedUniqueAttachment(t, db, "cars", "cars_fwdatt", "Cars Template")
	appID := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)

	var attID int
	require.NoError(t, db.Raw("SELECT id FROM attachments WHERE application_id = ? LIMIT 1", appID).Scan(&attID).Error)
	require.NotZero(t, attID, "у заявки должно быть вложение")

	// Пересылка ТОЛЬКО этого вложения (subset), без сопроводительного текста.
	fwdBody := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":false,"can_view":true}],"attachment_ids":[%d]}`, viewerID, attID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwdBody, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward subset: %s", rec.Body.String())

	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/forward-messages", appID), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	msgs := testutil.ParseResponse[[]services.ForwardMessageItem](t, rec)
	require.Len(t, msgs, 1)
	assert.False(t, msgs[0].Whole, "переслана не вся заявка, а выбранное вложение")
	assert.Equal(t, []string{"Cars Template"}, msgs[0].Attachments, "перечень вложений в ветке")
}
