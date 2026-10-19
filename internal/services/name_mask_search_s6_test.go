package services_test

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

func TestS6History_SearchUsersAndFeedbackPolicy(t *testing.T) {
	w := setupPassage2667World(t, services.ElementCar)
	last, label := "PrivateActorS6", "PublicOperatorS6"
	require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Update("last_name", last).Error)
	require.NoError(t, w.db.Create(&models.ApplicationApprover{UserID: w.actor, DisplayName: &label}).Error)
	feedback := models.Feedback{UserID: w.actor, Message: "Synthetic unrelated message", CreatedAt: w.entered}
	require.NoError(t, w.db.Create(&feedback).Error)
	viewer := models.User{Username: "s6_search_viewer", Password: "x", TypeID: 1, IsActive: true}
	require.NoError(t, w.db.Create(&viewer).Error)
	for _, key := range []string{services.KeyPageAdminUsers, services.KeyPageAdminFeedback} {
		require.NoError(t, w.db.Create(&models.UserPermissionOverride{UserID: viewer.ID, PermissionKey: key, Value: "allow", GrantedAt: time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)}).Error)
	}
	ctx := services.WithSessionIdentity(context.Background(), models.SessionIdentity{EffectiveUserID: viewer.ID})
	for _, administrator := range []bool{false, true} {
		require.NoError(t, w.db.Model(&models.User{}).Where("id=?", viewer.ID).Update("is_admin", administrator).Error)
		users, err := services.NewUserService(w.db, nil).GetAll(ctx, false)
		require.NoError(t, err)
		foundProfile := false
		for _, user := range users {
			if user.ID != w.actor {
				continue
			}
			foundProfile = true
			if administrator {
				require.Nil(t, user.DisplayLabel)
				require.NotNil(t, user.LastName)
				require.Equal(t, last, *user.LastName)
			} else {
				require.NotNil(t, user.DisplayLabel)
				require.Equal(t, label, *user.DisplayLabel)
				require.Nil(t, user.LastName)
				require.True(t, user.PDHidden)
				require.False(t, user.ConsentRequired)
			}
		}
		require.True(t, foundProfile)
		search, err := services.NewSearchService(w.db, services.NewPermissionResolver(w.db))
		require.NoError(t, err)
		for _, kind := range []services.SearchEntityType{services.SearchTypeUsers, services.SearchTypeFeedback} {
			for _, query := range []string{last, label} {
				response, err := search.Search(ctx, viewer.ID, query, []services.SearchEntityType{kind}, 20)
				require.NoError(t, err)
				require.Empty(t, response.Degraded)
				found := false
				for _, group := range response.Groups {
					for _, item := range group.Items {
						id := w.actor
						if kind == services.SearchTypeFeedback {
							id = feedback.ID
						}
						if item.ID == id {
							found = true
							if !administrator {
								require.NotContains(t, item.Title, last)
								require.NotContains(t, item.Subtitle, last)
							}
						}
					}
				}
				require.Equal(t, administrator == (query == last), found, "role=%v kind=%s query=%s", administrator, kind, query)
			}
		}
	}
}

func TestS6History_ReportAliasesPreserveAggregateTotals(t *testing.T) {
	w := setupPassage2667World(t, services.ElementEmployee)
	var appID int
	require.NoError(t, w.db.Table("attachments").Where("id=?", w.attachment).Select("application_id").Scan(&appID).Error)
	actor2 := models.User{Username: "s6_second_actor", Password: "x", TypeID: 1, IsActive: true}
	require.NoError(t, w.db.Create(&actor2).Error)
	label := "SharedOperatorS6"
	for _, actor := range []int{w.actor, actor2.ID} {
		require.NoError(t, w.db.Create(&models.ApplicationApprover{UserID: actor, DisplayName: &label}).Error)
		require.NoError(t, w.db.Exec("INSERT INTO application_responsible_users (application_id, user_id) VALUES (?, ?)", appID, actor).Error)
	}
	viewer := models.User{Username: "s6_reports_viewer", Password: "x", TypeID: 1, IsActive: true}
	require.NoError(t, w.db.Create(&viewer).Error)
	ctx := services.WithSessionIdentity(context.Background(), models.SessionIdentity{EffectiveUserID: viewer.ID})
	stats := services.NewStatisticsService(w.db, 0)
	result, err := stats.RunReport(ctx, models.ReportRequest{Metric: "approver_votes_count", Dimension: "by_approver"})
	require.NoError(t, err)
	require.Len(t, result.Rows, 1)
	require.Equal(t, label, result.Rows[0].Label)
	require.EqualValues(t, 2, result.Rows[0].Value)
	require.EqualValues(t, 2, result.Total)
	// Switching the viewer role restores per-account labels without changing totals.
	require.NoError(t, w.db.Model(&models.User{}).Where("id=?", viewer.ID).Update("is_admin", true).Error)
	result, err = stats.RunReport(ctx, models.ReportRequest{Metric: "approver_votes_count", Dimension: "by_approver"})
	require.NoError(t, err)
	require.Len(t, result.Rows, 2)
	require.EqualValues(t, 2, result.Total)
	for _, row := range result.Rows {
		require.NotEqual(t, label, row.Label)
	}
	// The list report is another representation of the same receiver identity.
	workName, realName := "Заявка на работы", "ListActorS6"
	workType := models.UniqueAttachment{AttachmentType: models.TableTypePeople, DisplayName: &workName, IsActive: true}
	require.NoError(t, w.db.Create(&workType).Error)
	require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).Update("unique_attachment_id", workType.ID).Error)
	require.NoError(t, w.db.Model(&models.Application{}).Where("id=?", appID).Update("responsible_user_id", w.actor).Error)
	require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Update("last_name", realName).Error)
	for _, administrator := range []bool{false, true} {
		require.NoError(t, w.db.Model(&models.User{}).Where("id=?", viewer.ID).Update("is_admin", administrator).Error)
		list, err := stats.RunReportList(ctx, models.ReportRequest{Mode: "list", Entity: "work_applications", Columns: []string{"responsible"}})
		require.NoError(t, err)
		require.Len(t, list.Rows, 1)
		if administrator {
			require.Contains(t, list.Rows[0]["responsible"], realName)
		} else {
			require.Equal(t, label, list.Rows[0]["responsible"])
		}
	}
}
