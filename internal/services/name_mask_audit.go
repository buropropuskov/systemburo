package services

import "encoding/json"

// maskUserAuditDetails filters a response copy of known user-profile changes.
// Hiding only the actor would still expose a masked account through its own
// last-name/contact change event. Event, time and non-private changes remain.
func maskUserAuditDetails(masks map[int]string, userID int, raw json.RawMessage) json.RawMessage {
	if !isMasked(masks, userID) || len(raw) == 0 {
		return raw
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil
	}
	for _, key := range []string{"last_name", "first_name", "middle_name", "email", "phone"} {
		delete(fields, key)
	}
	safe, err := json.Marshal(fields)
	if err != nil {
		return nil
	}
	return safe
}
