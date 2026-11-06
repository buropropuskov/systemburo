package services_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/clause"
)

func TestConsentVisibilityPolicy(t *testing.T) {
	w := setupPassage2667World(t, services.ElementEmployee)
	anchor := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	name, phone, alias := "VisibilitySubject", "+79000000001", "VisibilitySubject Alias"
	require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.actor).
		Updates(map[string]any{"last_name": name, "phone": phone}).Error)
	require.NoError(t, w.db.Create(&models.ApplicationApprover{UserID: w.actor, DisplayName: &alias}).Error)
	viewer := models.User{Username: "visibility_viewer", Password: "x", TypeID: 1, IsActive: true}
	require.NoError(t, w.db.Create(&viewer).Error)
	require.NoError(t, w.db.Create(&models.UserPermissionOverride{UserID: viewer.ID,
		PermissionKey: services.KeyPageAdminUsers, Value: "allow", GrantedAt: anchor}).Error)
	ctx := services.WithSessionIdentity(context.Background(), models.SessionIdentity{EffectiveUserID: viewer.ID})
	var appID int
	require.NoError(t, w.db.Table("attachments").Where("id = ?", w.attachment).Select("application_id").Scan(&appID).Error)
	workName := "Заявка на работы"
	workType := models.UniqueAttachment{AttachmentType: models.TableTypePeople, DisplayName: &workName, IsActive: true}
	require.NoError(t, w.db.Create(&workType).Error)
	require.NoError(t, w.db.Model(&models.Attachment{}).Where("id = ?", w.attachment).Update("unique_attachment_id", workType.ID).Error)
	require.NoError(t, w.db.Model(&models.Application{}).Where("id = ?", appID).
		Updates(map[string]any{"responsible_user_id": w.actor, "sending_datetime": anchor}).Error)
	for _, prompt := range []string{"enabled", "disabled", "empty"} {
		for _, subject := range []string{"active", "archived", "banned", "super"} {
			for _, consent := range []string{"never", "active", "revoked", "reaccepted"} {
				for _, role := range []string{"ordinary", "admin", "super"} {
					t.Run(fmt.Sprintf("%s/%s/%s/%s", prompt, subject, consent, role), func(t *testing.T) {
						for key, value := range map[string]string{"legal.pd_consent_required": fmt.Sprint(prompt != "disabled"), "legal.pd_consent_text": "<p>Consent</p>"} {
							if prompt == "empty" && key == "legal.pd_consent_text" {
								value = ""
							}
							require.NoError(t, w.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}},
								DoUpdates: clause.Assignments(map[string]any{"value": value})}).Create(&models.SystemSetting{Key: key, Value: value, Type: "string"}).Error)
						}
						require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.actor).Updates(map[string]any{
							"is_active": subject != "archived", "is_banned": subject == "banned", "is_super_admin": subject == "super"}).Error)
						require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", viewer.ID).
							Updates(map[string]any{"is_admin": role == "admin", "is_super_admin": role == "super"}).Error)
						require.NoError(t, w.db.Where("user_id = ?", w.actor).Delete(&models.PDConsent{}).Error)
						if consent == "revoked" || consent == "reaccepted" {
							require.NoError(t, w.db.Create(&models.PDConsent{UserID: w.actor, ConsentType: services.ConsentTypePDProcessing,
								Granted: false, GrantedAt: anchor, RevokedAt: &anchor, CreatedAt: anchor, DocumentVersion: 1}).Error)
						}
						if consent == "active" || consent == "reaccepted" {
							require.NoError(t, w.db.Create(&models.PDConsent{UserID: w.actor, ConsentType: services.ConsentTypePDProcessing,
								Granted: true, GrantedAt: anchor, CreatedAt: anchor, DocumentVersion: 1}).Error)
						}
						hidden := consent == "revoked" || (consent == "never" && prompt == "enabled" && subject == "active")
						users, err := services.NewUserService(w.db, nil).GetAll(ctx, true)
						require.NoError(t, err)
						found := false
						for _, user := range users {
							if user.ID == w.actor {
								found = true
								if hidden {
									require.Nil(t, user.LastName)
									require.Equal(t, "@passage2667_actor", *user.DisplayLabel)
								}
								if !hidden && role != "ordinary" {
									require.Equal(t, name, *user.LastName)
								}
							}
						}
						require.True(t, found)
						approvers, err := services.NewApproverService(w.db).GetAll(ctx)
						require.NoError(t, err)
						require.Len(t, approvers, 1)
						if hidden {
							require.Nil(t, approvers[0].LastName)
							require.Equal(t, "@passage2667_actor", *approvers[0].DisplayName)
						} else {
							require.Equal(t, alias, *approvers[0].DisplayName)
						}
						list, err := services.NewStatisticsService(w.db, 0).RunReportList(ctx, models.ReportRequest{
							Mode: "list", Entity: "work_applications", Columns: []string{"responsible"}, Limit: 1,
							Filters: []models.ReportFilterValue{{Key: "status", Values: []string{models.StatusInWork}}}})
						require.NoError(t, err)
						require.Len(t, list.Rows, 1)
						require.Equal(t, 1, list.Total)
						if hidden {
							require.Equal(t, "@passage2667_actor", list.Rows[0]["responsible"])
						} else if role == "ordinary" {
							require.Equal(t, alias, list.Rows[0]["responsible"])
						} else {
							require.Contains(t, list.Rows[0]["responsible"], name)
							require.Contains(t, list.Rows[0]["responsible"], phone)
						}
						if consent == "revoked" && subject == "active" && role == "admin" {
							search, err := services.NewSearchService(w.db, services.NewPermissionResolver(w.db))
							require.NoError(t, err)
							result, err := search.Search(ctx, viewer.ID, name, []services.SearchEntityType{services.SearchTypeUsers}, 20)
							require.NoError(t, err)
							require.Empty(t, result.Degraded)
							for _, group := range result.Groups {
								for _, item := range group.Items {
									require.NotEqual(t, w.actor, item.ID, "скрытое имя не должно находить этого субъекта; другие нечёткие совпадения допустимы")
								}
							}
						}
					})
				}
			}
		}
	}
}

func TestConsentVisibilityPolicy_ReadError(t *testing.T) {
	w := setupPassage2667World(t, services.ElementEmployee)
	// Even with the prompt disabled, an unreadable consent source is not permission.
	require.NoError(t, w.db.Exec("ALTER TABLE pd_consents RENAME TO visibility_unavailable_consents").Error)
	t.Cleanup(func() {
		require.NoError(t, w.db.Exec("ALTER TABLE visibility_unavailable_consents RENAME TO pd_consents").Error)
	})
	_, err := services.NewUserService(w.db, nil).GetAll(context.Background(), true)
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusServiceUnavailable, httpErr.Code)
}
