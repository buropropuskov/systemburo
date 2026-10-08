package services

import "fmt"

// applicationPeriodRowsSQL is the common calendar-period source for application
// aggregates. It does not grant visibility or replace admission/lifecycle guards.
// Existing consumers deliberately keep their own status and precision rules.
// Historical rows remain present; live-only consumers filter both status columns.
func applicationPeriodRowsSQL() string {
	period, err := EntityEffectivePeriodSQL("period_entity", "period_attachment")
	if err != nil {
		// Fixed developer-owned aliases: failure is a programming error.
		panic(err)
	}
	child := func(table, attachmentType string) string {
		return fmt.Sprintf(`SELECT period_attachment.application_id,
			period_attachment.id AS attachment_id,
			period_attachment.status AS attachment_status,
			period_entity.status AS entity_status,
			%s AS date_from_text, %s AS date_to_text, %s AS valid_mode
			FROM attachments period_attachment
			JOIN %s period_entity ON period_entity.attachment_id = period_attachment.id
			WHERE period_attachment.attachment_type = '%s'`,
			period.DateFrom, period.DateTo, period.ValidMode, table, attachmentType)
	}
	fallback := `SELECT period_attachment.application_id,
		period_attachment.id AS attachment_id,
		period_attachment.status AS attachment_status,
		period_attachment.status AS entity_status,
		period_attachment.entry_date_from AS date_from_text,
		period_attachment.entry_date_to AS date_to_text, TRUE AS valid_mode
		FROM attachments period_attachment
		WHERE (COALESCE(period_attachment.attachment_type, '') NOT IN ('people', 'cars'))
		OR (period_attachment.attachment_type = 'people' AND NOT EXISTS (
			SELECT 1 FROM employees member WHERE member.attachment_id = period_attachment.id))
		OR (period_attachment.attachment_type = 'cars' AND NOT EXISTS (
			SELECT 1 FROM cars member WHERE member.attachment_id = period_attachment.id))`
	return `SELECT period_rows.application_id, period_rows.attachment_id,
		period_rows.attachment_status, period_rows.entity_status,
		NULLIF(BTRIM(period_rows.date_from_text), '') AS date_from,
		NULLIF(BTRIM(period_rows.date_to_text), '') AS date_to,
		period_rows.valid_mode,
		(NULLIF(BTRIM(period_rows.date_to_text), '') IS NULL) AS unbounded
		FROM (` + child("employees", "people") + ` UNION ALL ` + child("cars", "cars") + ` UNION ALL ` + fallback + `) period_rows`
}
