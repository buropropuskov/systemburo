package handlers_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Принимающий (строка в application_approvers) о поданной заявке не узнавал вообще
// ничего: уведомления при подаче уходили только заявителю и согласующим, а заявка
// ложилась в Центр и ждала, пока принимающий сам её заметит. Жалоба владельца по итогам
// работы на стенде (#974).

func TestSubmit_NotifiesApproversAboutPendingApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Принимающий и посторонний пользователь: первый в реестре, второй нет.
	testutil.RegisterAndLogin(t, e, "acceptor_notify", "pass123", 1, td.OrgID, td.CompanyID)
	var approver models.User
	require.NoError(t, db.Where("username = ?", "acceptor_notify").First(&approver).Error)
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: approver.ID}).Error)

	var outsider models.User
	testutil.RegisterAndLogin(t, e, "outsider_notify", "pass123", 1, td.OrgID, td.CompanyID)
	require.NoError(t, db.Where("username = ?", "outsider_notify").First(&outsider).Error)

	authorToken := testutil.RegisterAndLogin(t, e, "author_notify", "pass123", 1, td.OrgID, td.CompanyID)
	var author models.User
	require.NoError(t, db.Where("username = ?", "author_notify").First(&author).Error)

	uaID := seedUniqueAttachment(t, db, "cars", "cars_pending_tmpl", "Cars Pending")
	body := fmt.Sprintf(`{
		"message":"<p>Привоз мебели на склад</p>","organization":"Test Organization",
		"responsible_person":"Test","contact_phone":"+79001234567","data_approval":true,
		"attachments":[{"attachment_type":"cars","attachment_name":"cars_tmpl",
			"attachment_display_name":"Cars Template","unique_attachment_id":%d,
			"entry_date_from":"2026-04-01","entry_date_to":"2099-12-31",
			"data":{"vehicles":[{"car_number":"K222KK177","car_brand":"Lada"}]}}]
	}`, uaID)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(authorToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var forApprover []models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?",
		approver.ID, services.NotificationTypeApplicationPendingAcceptance).Find(&forApprover).Error)
	require.Len(t, forApprover, 1, "принимающий должен узнать о заявке, ждущей принятия")
	require.NotNil(t, forApprover[0].Title)
	require.NotNil(t, forApprover[0].Message)
	assert.Equal(t, "Новая заявка", *forApprover[0].Title)
	// Номер первой строкой, без повтора заголовка.
	assert.True(t, strings.HasPrefix(*forApprover[0].Message, "№ "),
		"текст должен начинаться с номера заявки, получено: %q", *forApprover[0].Message)
	// Организация в тексте: в шторке телефона видно две строки, и по одному номеру
	// заявки принимающий не понимает, чья она (#974).
	assert.Contains(t, *forApprover[0].Message, "Test Organization")
	// Отправитель рядом с организацией: принимающему важно, кто именно прислал (#974).
	assert.Contains(t, *forApprover[0].Message, "author_notify")
	// Превью сообщения заявки - без разметки, отдельной строкой.
	assert.Contains(t, *forApprover[0].Message, "Привоз мебели")
	assert.NotContains(t, *forApprover[0].Message, "<p>")
	assert.Contains(t, *forApprover[0].Message, "\n\n", "блоки разделяются пустой строкой")
	require.NotNil(t, forApprover[0].Data, "без data в окне подробностей не будет перехода к заявке")
	assert.Contains(t, *forApprover[0].Data, "application_id")
	assert.Contains(t, *forApprover[0].Data, "Test Organization")
	assert.Contains(t, *forApprover[0].Data, "sender_name")

	var forOutsider int64
	require.NoError(t, db.Model(&models.Notification{}).Where("user_id = ? AND type = ?",
		outsider.ID, services.NotificationTypeApplicationPendingAcceptance).Count(&forOutsider).Error)
	assert.Zero(t, forOutsider, "тот, кто не принимает заявки, уведомление получать не должен")
}

// Принимающий, подавший заявку сам, второго уведомления не получает: «Заявка отправлена»
// ему уже ушло, и звать его принять собственную заявку незачем.
func TestSubmit_ApproverWhoSubmittedIsNotCalledToOwnApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "acceptor_self", "pass123", 1, td.OrgID, td.CompanyID)
	var user models.User
	require.NoError(t, db.Where("username = ?", "acceptor_self").First(&user).Error)
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: user.ID}).Error)

	uaID := seedUniqueAttachment(t, db, "cars", "cars_self_tmpl", "Cars Self")
	body := fmt.Sprintf(`{
		"message":"self submit","organization":"Test Organization",
		"responsible_person":"Test","contact_phone":"+79001234567","data_approval":true,
		"attachments":[{"attachment_type":"cars","attachment_name":"cars_tmpl",
			"attachment_display_name":"Cars Template","unique_attachment_id":%d,
			"entry_date_from":"2026-04-01","entry_date_to":"2099-12-31",
			"data":{"vehicles":[{"car_number":"M333MM177","car_brand":"Lada"}]}}]
	}`, uaID)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var count int64
	require.NoError(t, db.Model(&models.Notification{}).Where("user_id = ? AND type = ?",
		user.ID, services.NotificationTypeApplicationPendingAcceptance).Count(&count).Error)
	assert.Zero(t, count, "автор заявки не должен звать сам себя принять её")
}
