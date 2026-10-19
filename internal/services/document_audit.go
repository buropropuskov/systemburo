package services

import (
	"encoding/json"

	"systemburo/internal/models"
)

func isEmployeeDocumentField(field string) bool {
	switch field {
	case "passport_series_number", "patent_number", "other_permission":
		return true
	default:
		return false
	}
}

// documentAuditDetails removes values only from the known flat person-field
// schema. It is used for both new events and response copies of legacy events.
// Other entities, free text, metadata and the caller's original bytes remain intact.
func documentAuditDetails(entityType string, raw json.RawMessage) json.RawMessage {
	if entityType != models.AuditEntityEmployee && entityType != models.AuditEntityUniqueEmployee {
		return raw
	}
	var fields map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &fields) != nil || fields == nil {
		return raw
	}
	var field string
	if json.Unmarshal(fields["field_name"], &field) != nil || !isEmployeeDocumentField(field) {
		return raw
	}
	delete(fields, "old_value")
	delete(fields, "new_value")
	safe, err := json.Marshal(fields)
	if err != nil {
		return nil
	}
	return safe
}

// These expressions are used only by person-history sources with audit alias a.
// Redaction happens before filters, pagination and downstream export readers.
const employeeDocumentFieldSQL = `a.details->>'field_name' IN ('passport_series_number', 'patent_number', 'other_permission')`
const employeeAuditOldValueSQL = `CASE WHEN ` + employeeDocumentFieldSQL + ` THEN NULL ELSE a.details->>'old_value' END`
const employeeAuditNewValueSQL = `CASE WHEN ` + employeeDocumentFieldSQL + ` THEN NULL ELSE a.details->>'new_value' END`
