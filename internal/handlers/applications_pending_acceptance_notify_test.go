package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	ctx := context.Background()
	resolver := services.NewPermissionResolver(db)
	permissions, err := resolver.ResolveFresh(ctx, approver.ID)
	require.NoError(t, err)
	require.False(t, permissions.Has(services.KeyPageCenter), "Type1/application_approvers alone must not grant %s: %+v", services.KeyPageCenter, permissions)
	centerAccess := models.UserPermissionOverride{UserID: approver.ID, PermissionKey: services.KeyPageCenter, Value: "allow"}
	require.NoError(t, db.Create(&centerAccess).Error)
	permissions, err = resolver.ResolveFresh(ctx, approver.ID)
	require.NoError(t, err)
	require.True(t, permissions.Has(services.KeyPageCenter), "positive projection fixture requires %s: %+v", services.KeyPageCenter, permissions)

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
	rec := testutil.POST(t, e, "/applications/submit-complete-application", withApplicationConfirmation(t, db, body), testutil.AuthHeader(authorToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var forApprover []models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?",
		approver.ID, services.NotificationTypeApplicationPendingAcceptance).Find(&forApprover).Error)
	require.Len(t, forApprover, 1, "принимающий должен узнать о заявке, ждущей принятия")
	require.NotNil(t, forApprover[0].Title)
	require.NotNil(t, forApprover[0].Message)
	assert.Equal(t, "Есть обновление", *forApprover[0].Title)
	assert.Equal(t, "Есть обновление", *forApprover[0].Message)
	require.NotNil(t, forApprover[0].ProjectionRef)
	require.NotNil(t, forApprover[0].Data)
	var technical map[string]any
	require.NoError(t, json.Unmarshal([]byte(*forApprover[0].Data), &technical))
	require.Contains(t, technical, "application_id")
	assert.NotContains(t, technical, "sender_name")
	assert.NotContains(t, *forApprover[0].Data, "Test Organization")
	assert.NotContains(t, *forApprover[0].Data, "author_notify")
	assert.NotContains(t, *forApprover[0].Data, "Привоз мебели")
	assert.NotContains(t, *forApprover[0].ProjectionRef, "Test Organization")
	assert.NotContains(t, *forApprover[0].ProjectionRef, "author_notify")
	assert.NotContains(t, *forApprover[0].ProjectionRef, "Привоз мебели")
	applicationID, ok := technical["application_id"].(float64)
	require.True(t, ok, "technical application_id is a numeric identifier")
	var app models.Application
	require.NoError(t, db.Select("id,application_number").First(&app, int(applicationID)).Error)
	require.NotNil(t, app.ApplicationNumber)
	feed, err := services.NewNotificationService(db).GetByUserID(context.Background(), approver.ID)
	require.NoError(t, err)
	var projected *models.Notification
	for i := range feed {
		if feed[i].ID == forApprover[0].ID {
			projected = &feed[i]
			break
		}
	}
	require.NotNil(t, projected)
	require.NotNil(t, projected.Title)
	require.NotNil(t, projected.Message)
	assert.Equal(t, "Новая заявка", *projected.Title)
	assert.Contains(t, *projected.Message, *app.ApplicationNumber)
	assert.Contains(t, *projected.Message, author.Username, "текущий отправитель доступен через проекцию")
	require.NotNil(t, projected.Data, "проекция сохраняет переход к доступной заявке")
	var projectedData map[string]any
	require.NoError(t, json.Unmarshal([]byte(*projected.Data), &projectedData))
	assert.EqualValues(t, app.ID, projectedData["application_id"])
	assert.Contains(t, projectedData, "sender_name")
	assert.Nil(t, projected.ProjectionRef)
	publicJSON, err := json.Marshal(projected)
	require.NoError(t, err)
	assert.NotContains(t, string(publicJSON), "projection_ref")
	assert.NotContains(t, string(publicJSON), "Test Organization")
	assert.NotContains(t, string(publicJSON), "Привоз мебели")
	assert.NotContains(t, string(publicJSON), "<p>")
	// The same stored event loses its object details as soon as the current
	// effective viewer loses access to the Center page.
	require.NoError(t, db.Model(&centerAccess).Update("value", "deny").Error)
	permissions, err = resolver.ResolveFresh(ctx, approver.ID)
	require.NoError(t, err)
	require.False(t, permissions.Has(services.KeyPageCenter), "fresh explicit deny must remove %s: %+v", services.KeyPageCenter, permissions)
	deniedFeed, err := services.NewNotificationService(db).GetByUserID(ctx, approver.ID)
	require.NoError(t, err)
	var denied *models.Notification
	for i := range deniedFeed {
		if deniedFeed[i].ID == forApprover[0].ID {
			denied = &deniedFeed[i]
			break
		}
	}
	require.NotNil(t, denied, "revocation preserves the notification row")
	assert.Equal(t, forApprover[0].Count, denied.Count)
	assert.Equal(t, "Есть обновление", *denied.Title)
	assert.Equal(t, "Есть обновление", *denied.Message)
	assert.Nil(t, denied.Data)
	assert.Nil(t, denied.GroupKey)
	assert.Nil(t, denied.ProjectionRef)

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
	rec := testutil.POST(t, e, "/applications/submit-complete-application", withApplicationConfirmation(t, db, body), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var count int64
	require.NoError(t, db.Model(&models.Notification{}).Where("user_id = ? AND type = ?",
		user.ID, services.NotificationTypeApplicationPendingAcceptance).Count(&count).Error)
	assert.Zero(t, count, "автор заявки не должен звать сам себя принять её")
}
