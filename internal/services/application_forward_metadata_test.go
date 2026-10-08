package services

import (
	"context"
	"encoding/json"
	"testing"
)

func TestForwardMetadata_RecipientPurposeIsRecordedOnce(t *testing.T) {
	first := forwardRecipientDetail{UserID: 1, Purpose: "approval", RequiredApproval: true, AccessGranted: true}
	details := appendForwardRecipient(nil, first)
	details = appendForwardRecipient(details, forwardRecipientDetail{UserID: 1, Purpose: "view"})
	details = appendForwardRecipient(details, forwardRecipientDetail{UserID: 2, Purpose: "view", AccessGranted: false})
	if len(details) != 2 || details[0] != first || details[1].AccessGranted {
		t.Fatalf("unexpected operation details: %#v", details)
	}
}

func TestForwardMetadata_AcceptedApprovalWinsInEitherOrder(t *testing.T) {
	approval := forwardRecipientDetail{UserID: 1, Purpose: "approval", RequiredApproval: true, AccessGranted: true}
	view := forwardRecipientDetail{UserID: 1, Purpose: "view", AccessGranted: true}
	for _, order := range [][]forwardRecipientDetail{{view, approval}, {approval, view}} {
		var result []forwardRecipientDetail
		for _, detail := range order {
			result = appendForwardRecipient(result, detail)
		}
		if len(result) != 1 || result[0] != approval {
			t.Fatalf("accepted approval lost: %#v", result)
		}
	}
	result := appendForwardRecipient([]forwardRecipientDetail{view}, forwardRecipientDetail{UserID: 1, Purpose: "view", AccessGranted: false})
	if !result[0].AccessGranted {
		t.Fatal("repeat delivery lost the grant from this action")
	}
}

func TestForwardMetadata_MalformedStructuredRecipientsAreUnavailable(t *testing.T) {
	for _, raw := range []string{
		`{"recipient_details":[{"user_id":1,"purpose":"view","required_approval":false,"access_granted":true}]}`,
		`{"forward_schema_version":3,"recipient_details":[]}`,
		`{"forward_schema_version":2,"recipient_details":[{"user_id":1}]}`,
		`{"forward_schema_version":2,"recipient_details":[{"user_id":1,"purpose":"unknown","required_approval":false,"access_granted":true}]}`,
		`{"forward_schema_version":2,"recipient_details":[{"user_id":1,"purpose":"view","access_granted":true}]}`,
		`{"forward_schema_version":2,"recipient_details":[{"user_id":1,"purpose":"view","required_approval":false}]}`,
		`{"forward_schema_version":2,"recipient_details":[{"user_id":1,"purpose":"view","required_approval":true,"access_granted":true}]}`,
		`{"forward_schema_version":2,"recipient_details":[{"user_id":1,"purpose":"approval","required_approval":false,"access_granted":false}]}`,
		`{"forward_schema_version":2,"recipient_details":[{"user_id":1,"purpose":"view","required_approval":false,"access_granted":true},{"user_id":1,"purpose":"approval","required_approval":true,"access_granted":true}]}`,
	} {
		original := json.RawMessage(raw)
		result, err := sanitizeForwardRecipientMetadata(context.Background(), nil, []json.RawMessage{original})
		if err != nil {
			t.Fatal(err)
		}
		var meta map[string]json.RawMessage
		if err := json.Unmarshal(result[0], &meta); err != nil {
			t.Fatal(err)
		}
		if string(meta["recipients"]) != "[]" || string(meta["recipient_details"]) != "[]" || string(meta["recipient_details_available"]) != "false" {
			t.Fatalf("invented audience for %s: %s", raw, result[0])
		}
		if string(original) != raw {
			t.Fatal("source modified")
		}
	}
}

func TestForwardMetadata_ValidStructuredPurposesArePreserved(t *testing.T) {
	var meta map[string]json.RawMessage
	if err := json.Unmarshal([]byte(`{"forward_schema_version":2,"recipient_details":[{"user_id":1,"purpose":"approval","required_approval":true,"access_granted":true},{"user_id":2,"purpose":"approval","required_approval":false,"access_granted":true},{"user_id":3,"purpose":"view","required_approval":false,"access_granted":false}]}`), &meta); err != nil {
		t.Fatal(err)
	}
	result := decodeForwardRecipientDetails(meta)
	if len(result) != 3 || !result[0].RequiredApproval || result[1].RequiredApproval || result[2].Purpose != "view" || result[2].AccessGranted {
		t.Fatalf("lost complete historical facts: %#v", result)
	}
}

func TestForwardMetadata_SelectedScopeDoesNotBecomeAllForInvalidIDs(t *testing.T) {
	if forwardAttachmentScope(nil) != "all" || forwardAttachmentScope([]int{999}) != "selected" {
		t.Fatal("scope must describe the request without guessing from resolved attachment names")
	}
}

func TestForwardMetadata_LegacyNamesAreNotReturned(t *testing.T) {
	input := json.RawMessage(`{"recipients":["Legacy recipient"],"whole":true,"attachments":[]}`)
	before := string(input)
	results, err := sanitizeForwardRecipientMetadata(context.Background(), nil, []json.RawMessage{input})
	if err != nil {
		t.Fatal(err)
	}
	var actual struct {
		Recipients []string                  `json:"recipients"`
		Details    []forwardRecipientDisplay `json:"recipient_details"`
		Available  bool                      `json:"recipient_details_available"`
		Whole      bool                      `json:"whole"`
	}
	if err := json.Unmarshal(results[0], &actual); err != nil {
		t.Fatal(err)
	}
	if len(actual.Recipients) != 0 || len(actual.Details) != 0 || actual.Available || !actual.Whole {
		t.Fatalf("unsafe or lossy legacy response: %s", results[0])
	}
	if string(input) != before {
		t.Fatal("sanitizing a response changed source metadata")
	}
}

func TestForwardMetadata_MalformedLegacyDegradesSafely(t *testing.T) {
	for _, input := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`{"recipient_details":"invalid","recipients":["Legacy"]}`)} {
		results, err := sanitizeForwardRecipientMetadata(context.Background(), nil, []json.RawMessage{input})
		if err != nil || len(results) != 1 || !json.Valid(results[0]) {
			t.Fatalf("unsafe fallback: %v", err)
		}
		var actual map[string]json.RawMessage
		if err := json.Unmarshal(results[0], &actual); err != nil {
			t.Fatal(err)
		}
		if string(actual["recipients"]) != "[]" || string(actual["recipient_details_available"]) != "false" {
			t.Fatalf("unexpected fallback: %s", results[0])
		}
	}
}
