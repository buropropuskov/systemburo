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

// TestForwardMessages_RoundTrip закрывает #967: сопроводительное сообщение при пересылке
// пишется в comment сводной записи forwarded, отдаётся GET /forward-messages с ФИО автора,
// попадает в историю, видно получателям и закрыто от посторонних. Пустое сообщение записи
// не создаёт.
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

	// Получатель-читатель: видит заявку и должен видеть сообщения пересылки.
	testutil.RegisterUser(t, e, "fwdmsg_viewer", "pass123", 1, td.OrgID, td.CompanyID)
	viewerID := getUserID(t, db, "fwdmsg_viewer")
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

	// Пересылка без сообщения новому получателю: запись сообщения НЕ добавляется.
	testutil.RegisterUser(t, e, "fwdmsg_viewer2", "pass123", 1, td.OrgID, td.CompanyID)
	viewer2ID := getUserID(t, db, "fwdmsg_viewer2")
	emptyBody := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":false,"can_view":true}],"message":"   "}`, viewer2ID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), emptyBody, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward без сообщения: %s", rec.Body.String())

	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/forward-messages", appID), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	afterEmpty := testutil.ParseResponse[[]services.ForwardMessageItem](t, rec)
	assert.Len(t, afterEmpty, 1, "пустое сообщение не должно добавлять запись")
}
