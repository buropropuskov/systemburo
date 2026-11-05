package services

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"time"
)

// notificationReference is server-only provenance, never accepted from an HTTP
// notification payload. It contains identifiers and bounded event facts, not a
// personal snapshot. A nil reference deliberately makes a row unprojectable.
type notificationReference struct {
	Version       int                    `json:"v"`
	Type          string                 `json:"type"`
	Variant       string                 `json:"variant,omitempty"`
	EventKey      string                 `json:"event_key,omitempty"`
	ActorID       int                    `json:"actor_id,omitempty"`
	ApplicationID int                    `json:"application_id,omitempty"`
	SupplementID  int                    `json:"supplement_id,omitempty"`
	QuestionID    int                    `json:"question_id,omitempty"`
	AnswerID      int                    `json:"answer_id,omitempty"`
	DirectoryKind string                 `json:"directory_kind,omitempty"`
	DirectoryID   int                    `json:"directory_id,omitempty"`
	EntityKind    string                 `json:"entity_kind,omitempty"`
	EntityID      int                    `json:"entity_id,omitempty"`
	NewsID        int                    `json:"news_id,omitempty"`
	DocumentID    int                    `json:"document_id,omitempty"`
	FeedbackID    int                    `json:"feedback_id,omitempty"`
	Facts         notificationEventFacts `json:"facts"`
}

type notificationEventFacts struct {
	Outcome        string    `json:"outcome,omitempty"`
	At             time.Time `json:"at,omitempty"`
	Until          time.Time `json:"until,omitempty"`
	WaitingDays    int       `json:"waiting_days,omitempty"`
	DaysLeft       int       `json:"days_left,omitempty"`
	Attempts       int       `json:"attempts,omitempty"`
	WindowMinutes  int       `json:"window_minutes,omitempty"`
	Errors         int64     `json:"errors,omitempty"`
	Requests       int64     `json:"requests,omitempty"`
	UsedPercent    float64   `json:"used_percent,omitempty"`
	WarnPercent    int       `json:"warn_percent,omitempty"`
	FreeBytes      int64     `json:"free_bytes,omitempty"`
	Marked         int       `json:"marked,omitempty"`
	Failed         int       `json:"failed,omitempty"`
	Changed        int       `json:"changed,omitempty"`
	SkippedNoMail  int       `json:"skipped_no_mail,omitempty"`
	OldPeriod      string    `json:"old_period,omitempty"`
	NewPeriod      string    `json:"new_period,omitempty"`
	ApprovalsReset bool      `json:"approvals_reset,omitempty"`
	SelfChange     bool      `json:"self_change,omitempty"`
	InWork         bool      `json:"in_work,omitempty"`
}

type notificationReferenceKey struct{}

func withNotificationReference(ctx context.Context, ref notificationReference) context.Context {
	ref.Version = 1
	return context.WithValue(ctx, notificationReferenceKey{}, ref)
}

func notificationReferenceFromContext(ctx context.Context, kind string) *notificationReference {
	ref, ok := ctx.Value(notificationReferenceKey{}).(notificationReference)
	if !ok || ref.Type != kind {
		return nil
	}
	return &ref
}

func encodeNotificationReference(ref *notificationReference) *string {
	if ref == nil || ref.Version != 1 || !validNotificationReference(*ref) {
		return nil
	}
	encoded, err := json.Marshal(ref)
	if err != nil {
		return nil
	}
	value := string(encoded)
	return &value
}

func notificationStoredType(kind string) *string {
	if _, known := NotificationTypeMeta(kind); !known {
		return nil
	}
	return notificationString(kind)
}

// The only duplicated notification data is technical identity/scalar metadata.
// In particular expiry deduplication still uses application_id/days_left without
// loading any legacy snapshot. No caller-supplied Data enters this column.
func notificationStoredData(rawRef *string) *string {
	if rawRef == nil {
		return nil
	}
	var ref notificationReference
	if json.Unmarshal([]byte(*rawRef), &ref) != nil {
		return nil
	}
	data := map[string]any{}
	for key, id := range map[string]int{"application_id": ref.ApplicationID, "supplement_id": ref.SupplementID,
		"question_id": ref.QuestionID, "entity_id": ref.EntityID, "news_id": ref.NewsID,
		"document_id": ref.DocumentID, "feedback_id": ref.FeedbackID} {
		if id > 0 {
			data[key] = id
		}
	}
	if ref.Type == NotificationTypeApplicationExpiring {
		data["days_left"] = ref.Facts.DaysLeft
		if !ref.Facts.Until.IsZero() {
			data["entry_date_to"] = ref.Facts.Until.Format("2006-01-02")
		}
	}
	if len(data) == 0 {
		return nil
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	return notificationString(string(encoded))
}

func decodeNotificationReference(raw *string, kind *string) (notificationReference, bool) {
	var ref notificationReference
	if raw == nil || kind == nil || len(*raw) > 4096 {
		return ref, false
	}
	decoder := json.NewDecoder(bytes.NewBufferString(*raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&ref) != nil || decoder.Decode(new(any)) != io.EOF {
		return ref, false
	}
	if ref.Version != 1 || ref.Type != *kind {
		return ref, false
	}
	if _, ok := NotificationTypeMeta(ref.Type); !ok {
		return ref, false
	}
	for _, id := range []int{ref.ActorID, ref.ApplicationID, ref.SupplementID, ref.QuestionID, ref.AnswerID, ref.DirectoryID, ref.EntityID, ref.NewsID, ref.DocumentID, ref.FeedbackID} {
		if id < 0 {
			return ref, false
		}
	}
	if !validNotificationReference(ref) {
		return ref, false
	}
	return ref, true
}

func safeNotificationPeriod(value string) bool {
	if len(value) > 250 {
		return false
	}
	for _, r := range value {
		if !stringsContainsPeriodRune(r) {
			return false
		}
	}
	return true
}

func stringsContainsPeriodRune(r rune) bool {
	return r >= '0' && r <= '9' || r == ' ' || r == '.' || r == ':' || r == '-' || r == ';' || r == '/' || r == '—'
}

func validNotificationReference(ref notificationReference) bool {
	f := ref.Facts
	if f.WaitingDays < 0 || f.DaysLeft < 0 || f.Attempts < 0 || f.Errors < 0 || f.Requests < 0 || f.FreeBytes < 0 || f.Marked < 0 || f.Failed < 0 || f.Changed < 0 || f.SkippedNoMail < 0 || math.IsNaN(f.UsedPercent) || math.IsInf(f.UsedPercent, 0) || !safeNotificationPeriod(f.OldPeriod) || !safeNotificationPeriod(f.NewPeriod) {
		return false
	}
	switch ref.Type {
	case NotificationTypeApplicationCreated, NotificationTypeApplicationPendingAcceptance, NotificationTypeApplicationForwarded, NotificationTypeApplicationWithdrawn:
		return ref.ApplicationID > 0 && ref.ActorID > 0 && ref.Variant == ""
	case NotificationTypeApplicationApprovalRequired:
		return ref.ApplicationID > 0 && ref.ActorID > 0 && (ref.Variant == "" || ref.Variant == "forward" || ref.Variant == "supplement" && ref.SupplementID > 0)
	case NotificationTypeApplicationQuestion:
		return ref.ApplicationID > 0 && ref.ActorID > 0 && ref.QuestionID > 0
	case NotificationTypeApplicationAnswer:
		return ref.ApplicationID > 0 && ref.ActorID > 0 && ref.QuestionID > 0 && ref.AnswerID > 0
	case NotificationTypeApplicationStatusChanged:
		return ref.ApplicationID > 0 && (f.Outcome == statusOutcomeAccepted || f.Outcome == statusOutcomeRejected || f.Outcome == statusOutcomeApproved || f.Outcome == statusOutcomeNotApproved || f.Outcome == statusOutcomeCompleted)
	case NotificationTypeApplicationSupplementReady, NotificationTypeApplicationSupplementDecided:
		return ref.ApplicationID > 0 && ref.SupplementID > 0
	case NotificationTypeApplicationDatesChanged:
		return ref.ApplicationID > 0 && ref.ActorID > 0 && (ref.Variant == "" || ref.Variant == "attachments" || ref.Variant == "entity" && ref.EntityID > 0 && (ref.EntityKind == "cars" || ref.EntityKind == "employees"))
	case NotificationTypeApprovalReminder, NotificationTypeApplicationExpiring:
		return ref.ApplicationID > 0
	case NotificationTypeDirectoryPending:
		return ref.ApplicationID > 0 && ref.DirectoryID > 0 && (ref.DirectoryKind == "organizations" || ref.DirectoryKind == "companies")
	case NotificationTypeDirectoryResolved:
		return ref.DirectoryID > 0 && (ref.DirectoryKind == "organizations" || ref.DirectoryKind == "companies")
	case NotificationTypeNewsPublished:
		return ref.NewsID > 0
	case NotificationTypeDocumentPublished:
		return ref.DocumentID > 0
	case NotificationTypeFeedbackCreated, NotificationTypeFeedbackAnswered:
		return ref.FeedbackID > 0
	case NotificationTypeTrashRestored:
		return ref.EntityID > 0 && (ref.EntityKind == "cars" || ref.EntityKind == "employees")
	case NotificationTypePasswordRotationReport:
		return ref.Variant == "marked" || ref.Variant == "rotated" || ref.Variant == "no_mail"
	case NotificationTypePasswordChanged, NotificationTypeUserBanned, NotificationTypeUserUnbanned, NotificationTypeLoginBlocked, NotificationTypePasswordRotated, NotificationTypePasswordExpiring, NotificationTypeMaintenanceScheduled, NotificationTypeArchiveQuotaWarning, NotificationTypeErrorSpike:
		return ref.Variant == ""
	default:
		return false
	}
}
