package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"systemburo/internal/models"
	"systemburo/internal/services"
)

func projectionJSON(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	require.NoError(t, err)
	return string(b)
}

func TestNotificationProjectionCurrentViewerMasksAndRevocation(t *testing.T) {
	f := setupIntentFixture(t)
	app := f.createApplication(t, f.db)
	actorName, display := "OriginalActorName", "BureauDisplayMask"
	require.NoError(t, f.db.Model(&models.User{}).Where("id = ?", f.author.ID).Updates(map[string]any{"last_name": actorName, "is_super_admin": true}).Error)
	require.NoError(t, f.db.Create(&models.ApplicationApprover{UserID: f.author.ID, DisplayName: &display}).Error)
	kind, snapshot := services.NotificationTypeApplicationForwarded, "PrivateSnapshot OtherPrivatePerson private-file.pdf"
	ref := fmt.Sprintf(`{"v":1,"type":%q,"actor_id":%d,"application_id":%d,"facts":{}}`, kind, f.author.ID, app.ID)
	group := fmt.Sprintf("app:%d", app.ID)
	n := models.Notification{UserID: f.approver.ID, Type: &kind, Title: &snapshot, Message: &snapshot, Data: &snapshot, ProjectionRef: &ref, GroupKey: &group, Count: 3}
	// Data is jsonb; use a valid opaque snapshot that must never leave the DB.
	data := fmt.Sprintf(`{"forwarded_by":%q,"decision_comment":%q}`, snapshot, snapshot)
	n.Data = &data
	require.NoError(t, f.db.Create(&n).Error)
	ctx := services.WithSessionIdentity(context.Background(), models.SessionIdentity{OwnerUserID: f.author.ID, EffectiveUserID: f.approver.ID})
	for _, tc := range []struct {
		name         string
		admin, super bool
		want         string
	}{
		{"effective_regular_despite_owner", false, false, display},
		{"admin", true, false, actorName},
		{"super", false, true, actorName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, f.db.Model(&models.User{}).Where("id = ?", f.approver.ID).Updates(map[string]any{"is_admin": tc.admin, "is_super_admin": tc.super}).Error)
			items, err := f.notifications.GetByUserID(ctx, f.approver.ID)
			require.NoError(t, err)
			require.Len(t, items, 1)
			body := projectionJSON(t, items)
			require.Contains(t, body, tc.want)
			require.NotContains(t, body, "PrivateSnapshot")
			require.NotContains(t, body, "OtherPrivatePerson")
			require.NotContains(t, body, "projection_ref")
			if !tc.admin && !tc.super {
				require.NotContains(t, body, actorName)
			}
		})
	}
	// Revocation applies even if the actor no longer belongs to gatedUsersWhere,
	// and even when the viewer is an administrator or super administrator.
	revokedAt := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	require.NoError(t, f.db.Create(&models.PDConsent{UserID: f.author.ID, ConsentType: services.ConsentTypePDProcessing, Granted: false, GrantedAt: revokedAt.Add(-time.Hour), RevokedAt: &revokedAt}).Error)
	require.NoError(t, f.db.Model(&models.User{}).Where("id = ?", f.author.ID).Updates(map[string]any{"is_banned": true, "is_active": false}).Error)
	for _, role := range []map[string]any{
		{"is_admin": false, "is_super_admin": false},
		{"is_admin": true, "is_super_admin": false},
		{"is_admin": false, "is_super_admin": true},
	} {
		require.NoError(t, f.db.Model(&models.User{}).Where("id = ?", f.approver.ID).Updates(role).Error)
		all, err := f.notifications.GetByUserID(ctx, f.approver.ID)
		require.NoError(t, err)
		page, total, unread, err := f.notifications.GetByUserIDPaginated(ctx, f.approver.ID, 10, 0, "unread")
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.EqualValues(t, 1, unread)
		read, err := f.notifications.MarkRead(ctx, f.approver.ID, n.ID, models.MarkNotificationReadRequest{IsRead: true})
		require.NoError(t, err)
		for _, response := range []any{all, page, read} {
			body := projectionJSON(t, response)
			require.NotContains(t, body, actorName)
			require.NotContains(t, body, display)
			require.NotContains(t, body, "PrivateSnapshot")
			require.Contains(t, body, "@"+f.author.Username)
		}
		require.Equal(t, 3, read.Count)
		require.Equal(t, &group, read.GroupKey)
		_, err = f.notifications.MarkRead(ctx, f.approver.ID, n.ID, models.MarkNotificationReadRequest{IsRead: false})
		require.NoError(t, err)
	}
}

func TestNotificationProjectionLegacyManualAndSQLStayMetadataOnly(t *testing.T) {
	f := setupIntentFixture(t)
	ctx := context.Background()
	snapshot := "UntrustedPrivateSnapshot"
	kind := "PrivateTypeOutsideCatalog"
	data := `{"projection_ref":{"v":1,"type":"application_forwarded","application_id":1,"actor_id":1},"application_id":1,"actor_name":"UntrustedPrivateSnapshot"}`
	created, err := f.notifications.Create(ctx, models.CreateNotificationRequest{UserID: f.approver.ID, Type: &kind, Title: &snapshot, Message: &snapshot, Data: &data})
	require.NoError(t, err)
	require.NotContains(t, projectionJSON(t, created), snapshot)
	require.NotContains(t, projectionJSON(t, created), kind)
	require.Nil(t, created.Data)
	var persisted models.Notification
	require.NoError(t, f.db.First(&persisted, created.ID).Error)
	require.Nil(t, persisted.ProjectionRef)
	require.Equal(t, "Есть обновление", *persisted.Title)
	require.Equal(t, "Есть обновление", *persisted.Message)
	require.Nil(t, persisted.Data)
	require.Nil(t, persisted.Type, "an arbitrary manual type is not stored as metadata")
	legacy := models.Notification{UserID: f.approver.ID, Type: &kind, Title: &snapshot, Message: &snapshot, Data: &data}
	require.NoError(t, f.db.Create(&legacy).Error)
	var originalLegacy models.Notification
	require.NoError(t, f.db.Select("id,message,data").First(&originalLegacy, legacy.ID).Error)
	var queries []string
	capturing := true
	callback := "notification_projection_sql_guard"
	require.NoError(t, f.db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if capturing && tx.Statement.Table == "notifications" {
			queries = append(queries, tx.Statement.SQL.String())
		}
	}))
	t.Cleanup(func() { f.db.Callback().Query().Remove(callback) })
	all, err := f.notifications.GetByUserID(ctx, f.approver.ID)
	require.NoError(t, err)
	page, total, unread, err := f.notifications.GetByUserIDPaginated(ctx, f.approver.ID, 1, 0, "all")
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.EqualValues(t, 2, unread)
	read, err := f.notifications.MarkRead(ctx, f.approver.ID, created.ID, models.MarkNotificationReadRequest{IsRead: true})
	require.NoError(t, err)
	for _, response := range []any{all, page, read} {
		body := projectionJSON(t, response)
		require.NotContains(t, body, snapshot)
		require.NotContains(t, body, "application_id")
		require.NotContains(t, body, kind)
	}
	require.NoError(t, f.notifications.Delete(ctx, f.approver.ID, created.ID))
	capturing = false
	var unchangedLegacy models.Notification
	require.NoError(t, f.db.Select("id,message,data").First(&unchangedLegacy, legacy.ID).Error)
	require.Equal(t, originalLegacy.Message, unchangedLegacy.Message, "reading does not rewrite historical snapshots")
	require.Equal(t, originalLegacy.Data, unchangedLegacy.Data)
	require.NotEmpty(t, queries)
	metadataReads := 0
	for _, sql := range queries {
		selectPart := strings.SplitN(strings.ToLower(sql), " from ", 2)[0]
		if selectPart == "select count(*)" {
			continue // Counts never select a stored snapshot.
		}
		metadataReads++
		require.NotContains(t, selectPart, "*")
		for _, forbidden := range []string{"title", "message", "\"data\""} {
			require.NotContains(t, selectPart, forbidden)
		}
	}
	require.Positive(t, metadataReads)
}

func TestNotificationNewWritesNeverPersistCallerSnapshots(t *testing.T) {
	f := setupIntentFixture(t)
	app := f.createApplication(t, f.db)
	ctx := context.Background()
	private := "PrivateSnapshotName private-file.pdf PrivateFreeComment"
	callerData := fmt.Sprintf(`{"application_id":%d,"sender_name":%q,"decision_comment":%q}`, app.ID, private, private)
	for i := 0; i < 2; i++ {
		require.NoError(t, f.notifications.CreateForUser(ctx, f.approver.ID, services.NotificationTypeApplicationForwarded, private, private, &callerData))
	}
	var grouped models.Notification
	require.NoError(t, f.db.Where("user_id = ? AND type = ?", f.approver.ID, services.NotificationTypeApplicationForwarded).First(&grouped).Error)
	require.Equal(t, 2, grouped.Count, "grouping remains independent of storing a preview")
	require.Equal(t, "Есть обновление", *grouped.Title)
	require.Equal(t, "Есть обновление", *grouped.Message)
	require.Nil(t, grouped.Data)
	require.Nil(t, grouped.ProjectionRef)
	// Force the intent builder to encounter a personal source name and free
	// application text. Neither is a prerequisite for safe notification storage.
	require.NoError(t, f.db.Model(&models.User{}).Where("id = ?", f.author.ID).Update("last_name", private).Error)
	require.NoError(t, f.db.Model(&models.Application{}).Where("id = ?", app.ID).Update("message", private).Error)
	require.NoError(t, f.db.Create(&models.ApplicationApprover{UserID: f.approver.ID}).Error)
	require.NoError(t, services.EnqueueApplicationSubmittedNotificationTx(ctx, f.db, app.ID, f.author.ID))
	processed, failed := f.worker.Process(ctx)
	require.Equal(t, 1, processed)
	require.Zero(t, failed)
	supplement := models.ApplicationSupplement{ApplicationID: app.ID, Number: 1, Status: models.SupplementPending, CreatedByUserID: f.author.ID, Comment: &private}
	require.NoError(t, f.db.Create(&supplement).Error)
	require.NoError(t, services.EnqueueApplicationSupplementNotificationTx(ctx, f.db, app.ID, supplement.ID, f.author.ID, false))
	processed, failed = f.worker.Process(ctx)
	require.Equal(t, 1, processed)
	require.Zero(t, failed)
	var rows []models.Notification
	require.NoError(t, f.db.Find(&rows).Error)
	for _, row := range rows {
		require.Equal(t, "Есть обновление", *row.Title)
		require.Equal(t, "Есть обновление", *row.Message)
		for _, marker := range []string{"PrivateSnapshotName", "private-file.pdf", "PrivateFreeComment"} {
			if row.Data != nil {
				require.NotContains(t, *row.Data, marker)
			}
			if row.ProjectionRef != nil {
				require.NotContains(t, *row.ProjectionRef, marker)
			}
		}
	}
	var collapsed models.Notification
	require.NoError(t, f.db.Where("user_id = ? AND type = ?", f.approver.ID, services.NotificationTypeApplicationApprovalRequired).First(&collapsed).Error)
	require.Equal(t, 2, collapsed.Count)
	require.NotNil(t, collapsed.ProjectionRef)
	var ref struct {
		ActorID      int `json:"actor_id"`
		SupplementID int `json:"supplement_id"`
	}
	require.NoError(t, json.Unmarshal([]byte(*collapsed.ProjectionRef), &ref))
	require.Equal(t, f.author.ID, ref.ActorID)
	require.Equal(t, supplement.ID, ref.SupplementID)
}

func TestNotificationProjectionLostAccessDeletedActorAndPolicyFailureClose(t *testing.T) {
	f := setupIntentFixture(t)
	app := f.createApplication(t, f.db)
	kind, snapshot := services.NotificationTypeApplicationForwarded, "PrivateFallbackMustNotEscape"
	ref := fmt.Sprintf(`{"v":1,"type":%q,"actor_id":%d,"application_id":%d,"facts":{}}`, kind, f.author.ID, app.ID)
	group := fmt.Sprintf("app:%d", app.ID)
	n := models.Notification{UserID: f.approver.ID, Type: &kind, Title: &snapshot, Message: &snapshot, ProjectionRef: &ref, GroupKey: &group}
	require.NoError(t, f.db.Create(&n).Error)
	ctx := services.WithSessionIdentity(context.Background(), models.SessionIdentity{OwnerUserID: f.approver.ID, EffectiveUserID: f.approver.ID})
	callback := "notification_projection_policy_failure"
	require.NoError(t, f.db.Callback().Row().Before("gorm:row").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "system_settings" {
			tx.AddError(errors.New("synthetic policy source failure"))
		}
	}))
	items, err := f.notifications.GetByUserID(ctx, f.approver.ID)
	require.Error(t, err)
	require.Nil(t, items)
	_, err = f.notifications.MarkRead(ctx, f.approver.ID, n.ID, models.MarkNotificationReadRequest{IsRead: true})
	require.Error(t, err)
	require.NoError(t, f.db.Callback().Row().Remove(callback))
	var unread bool
	require.NoError(t, f.db.Model(&models.Notification{}).Where("id = ?", n.ID).Select("is_read").Scan(&unread).Error)
	require.False(t, unread, "policy failures must not apply the read mutation")
	require.NoError(t, f.db.Where("application_id = ? AND user_id = ?", app.ID, f.approver.ID).Delete(&models.ApplicationResponsibleUser{}).Error)
	items, err = f.notifications.GetByUserID(ctx, f.approver.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Nil(t, items[0].Data)
	require.Nil(t, items[0].GroupKey)
	require.NotContains(t, projectionJSON(t, items), snapshot)
	// Restore membership, then leave an unresolvable actor reference.
	require.NoError(t, f.db.Create(&models.ApplicationResponsibleUser{ApplicationID: app.ID, UserID: f.approver.ID}).Error)
	badRef := fmt.Sprintf(`{"v":1,"type":%q,"actor_id":2147483647,"application_id":%d,"facts":{}}`, kind, app.ID)
	require.NoError(t, f.db.Model(&models.Notification{}).Where("id = ?", n.ID).Update("projection_ref", badRef).Error)
	items, err = f.notifications.GetByUserID(ctx, f.approver.ID)
	require.NoError(t, err)
	require.Nil(t, items[0].Data)
	require.Nil(t, items[0].GroupKey)
	require.NotContains(t, projectionJSON(t, items), snapshot)
}
