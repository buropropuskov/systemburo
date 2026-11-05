package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"systemburo/internal/models"
)

func TestNotificationReferenceRequiresClosedVersionedProvenance(t *testing.T) {
	kind := NotificationTypeApplicationForwarded
	ref := notificationReference{Version: 1, Type: kind, ActorID: 17, ApplicationID: 23}
	raw := encodeNotificationReference(&ref)
	decoded, valid := decodeNotificationReference(raw, &kind)
	require.True(t, valid)
	require.Equal(t, 17, decoded.ActorID)
	for _, input := range []string{
		`{"v":2,"type":"application_forwarded","actor_id":17,"application_id":23}`,
		`{"v":1,"type":"application_forwarded","actor_id":0,"application_id":23}`,
		`{"v":1,"type":"application_forwarded","actor_id":17,"application_id":-1}`,
		`{"v":1,"type":"application_forwarded","actor_id":17,"application_id":23.5}`,
		`{"v":1,"type":"application_forwarded","actor_id":17,"application_id":23,"actor_name":"private"}`,
		`{"v":1,"type":"application_forwarded","actor_id":17,"application_id":23} {}`,
		`{"v":1,"type":"application_forwarded","actor_id":17,"application_id":23,"variant":"unknown"}`,
	} {
		_, valid := decodeNotificationReference(&input, &kind)
		require.False(t, valid, input)
	}
	_, valid = decodeNotificationReference(nil, &kind)
	require.False(t, valid)
}

func TestNotificationLegacyProjectionNeverUsesSnapshotOrNeedsPolicyDB(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	private := "private snapshot marker"
	kind := NotificationTypeApplicationStatusChanged
	group := "app:23"
	row := models.Notification{ID: 7, UserID: 9, Type: &kind, Title: &private,
		Message: &private, Data: &private, GroupKey: &group, Count: 4, IsRead: true,
		CreatedAt: now, LastEventAt: &now}
	// A nil DB makes any accidental policy/snapshot read fail immediately.
	svc := &notificationService{}
	items, err := svc.projectNotifications(context.Background(), 9, []models.Notification{row})
	require.NoError(t, err)
	require.Len(t, items, 1)
	projected := items[0]
	require.Equal(t, 7, projected.ID)
	require.Equal(t, 4, projected.Count)
	require.True(t, projected.IsRead)
	require.Equal(t, now, projected.CreatedAt)
	require.Equal(t, &now, projected.LastEventAt)
	require.Nil(t, projected.GroupKey, "legacy object IDs must not escape through grouping metadata")
	require.Nil(t, projected.Data)
	require.Equal(t, genericNotificationText, *projected.Title)
	body, err := json.Marshal(projected)
	require.NoError(t, err)
	require.NotContains(t, string(body), private)
	require.NotContains(t, string(body), "projection_ref")
	// The original persistence object must not be rewritten by response masking.
	require.Equal(t, private, *row.Message)
	row.Type, row.GroupKey = &private, &group
	neutral := neutralNotification(row)
	require.Nil(t, neutral.Type)
	require.Nil(t, neutral.GroupKey)
	row.Type = &kind
	invalidRef := `{"v":99,"type":"application_status_changed","application_id":23}`
	row.ProjectionRef = &invalidRef
	items, err = svc.projectNotifications(context.Background(), 9, []models.Notification{row})
	require.NoError(t, err)
	require.Nil(t, items[0].GroupKey, "invalid references cannot disclose an object ID through group_key")
}

func TestNotificationReferenceCannotComeFromPublicDataOrUnrelatedContext(t *testing.T) {
	ctx := withNotificationReference(context.Background(), notificationReference{Type: NotificationTypeApplicationForwarded, ActorID: 17, ApplicationID: 23})
	require.NotNil(t, notificationReferenceFromContext(ctx, NotificationTypeApplicationForwarded))
	require.Nil(t, notificationReferenceFromContext(ctx, NotificationTypePasswordChanged))
	require.Nil(t, notificationReferenceFromContext(context.Background(), NotificationTypeApplicationForwarded))
	var request models.CreateNotificationRequest
	require.NoError(t, json.Unmarshal([]byte(`{"user_id":9,"title":"private","projection_ref":{"v":1,"actor_id":17},"data":"{\"projection_ref\":{\"v\":1}}"}`), &request))
	require.Nil(t, notificationReferenceFromContext(context.Background(), NotificationTypeApplicationForwarded))
}

func TestNotificationIntentReferenceRetainsOriginalActorAndSupplement(t *testing.T) {
	supplementID := 31
	intent := models.NotificationIntent{EventKey: "supplement-created:31", ApplicationID: 23, ActorUserID: 17, SupplementID: &supplementID, InWork: true}
	draft := notificationDraft(intent, 9, NotificationTypeApplicationApprovalRequired, "title", "snapshot", nil)
	require.Equal(t, 17, draft.reference.ActorID)
	require.Equal(t, 31, draft.reference.SupplementID)
	require.Equal(t, "supplement-created:31", draft.reference.EventKey)
	require.Equal(t, "supplement", draft.reference.Variant)
	require.True(t, draft.reference.Facts.InWork)
	intent.ActorUserID = 99
	require.Equal(t, 17, draft.reference.ActorID)
}

func TestNotificationPushPreviewHasOnlyNeutralContentAndCabinetURL(t *testing.T) {
	payload := genericPushPayload(37)
	require.Equal(t, 37, payload.NotificationID)
	require.Equal(t, "/personal-cabinet", payload.URL)
	require.Nil(t, payload.ApplicationID)
	require.Empty(t, payload.Type)
	require.Equal(t, genericNotificationText, payload.Message)
	body, valid := preparePushMessage(payload)
	require.True(t, valid)
	require.NotContains(t, string(body), "application_id")
	for _, field := range strings.Split(notificationMetadataColumns, ",") {
		require.NotContains(t, []string{"title", "message", "data"}, field)
	}
}

func TestNotificationPeriodFactsDoNotAdmitFreeText(t *testing.T) {
	require.True(t, safeNotificationPeriod("01.10.2026 09:00 - 03.10.2026 18:00"))
	require.False(t, safeNotificationPeriod("Private Person"))
	require.False(t, safeNotificationPeriod(strings.Repeat("1", 251)))
}

func TestNotificationStoredDataKeepsOnlyTechnicalDeduplicationFacts(t *testing.T) {
	until := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	ref := notificationReference{Version: 1, Type: NotificationTypeApplicationExpiring, ApplicationID: 23,
		Facts: notificationEventFacts{Until: until, DaysLeft: 3}}
	raw := encodeNotificationReference(&ref)
	require.NotNil(t, raw)
	data := notificationStoredData(raw)
	require.NotNil(t, data)
	require.JSONEq(t, `{"application_id":23,"days_left":3,"entry_date_to":"2026-01-05"}`, *data)
	ref.Facts.OldPeriod = "PrivateSnapshotName"
	require.Nil(t, encodeNotificationReference(&ref), "unverifiable facts must not become stored references")
	require.Nil(t, notificationStoredData(nil))
	require.Nil(t, notificationStoredType("PrivateTypeContainingName"))
}

func TestNotificationProjectedGroupKeyMustMatchReferenceAndCatalogue(t *testing.T) {
	appKey, otherAppKey, questionKey := "app:23", "app:24", "question:31"
	forward := notificationReference{Type: NotificationTypeApplicationForwarded, ApplicationID: 23}
	require.Equal(t, &appKey, projectedNotificationGroupKey(forward, &appKey))
	require.Nil(t, projectedNotificationGroupKey(forward, &otherAppKey))
	require.Nil(t, projectedNotificationGroupKey(forward, &questionKey))
	require.Nil(t, projectedNotificationGroupKey(forward, nil))
	answer := notificationReference{Type: NotificationTypeApplicationAnswer, ApplicationID: 23, QuestionID: 31}
	require.Equal(t, &questionKey, projectedNotificationGroupKey(answer, &questionKey))
	require.Nil(t, projectedNotificationGroupKey(answer, &appKey))
	nonAggregatable := notificationReference{Type: NotificationTypeApplicationStatusChanged, ApplicationID: 23}
	require.Nil(t, projectedNotificationGroupKey(nonAggregatable, &appKey))
	unknown := notificationReference{Type: "PrivateUnknownType", ApplicationID: 23}
	require.Nil(t, projectedNotificationGroupKey(unknown, &appKey))
}

func TestGenericPushPreservesCallerPayloadAdmission(t *testing.T) {
	// This original payload exceeds the encrypted push record limit even though
	// its neutral replacement would fit. No DB or delivery is needed to reject it.
	largeID := int64(9223372036854775807)
	id := int(largeID)
	payload := PushPayload{Title: strings.Repeat("\x00", 96) + "xxé😀",
		Message: strings.Repeat("\x00", 500), Type: strings.Repeat("\x00", 50),
		NotificationID: id, ApplicationID: &id}
	_, valid := preparePushMessage(payload)
	require.False(t, valid)
	service := NewPushService(nil, "public", "private", "", WithPushSyncSend()).(*pushService)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service.Send(ctx, 1, payload)
	require.EqualValues(t, 1, service.delivery.counters.invalid.Load())
	require.Nil(t, service.delivery.jobs)
	service.Shutdown(context.Background())
}
