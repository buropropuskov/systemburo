package services

import (
	"fmt"

	"gorm.io/gorm"
)

// Use the local GORM command clock, as passage commands do. A trusted
// generator session can replay historical events without bypassing admission.
// Lock in the same order as expiry and period changes. Inactive rows are
// intentional here: before acceptance the application's admissions are inactive.
func applicationAdmissionsExpired(tx *gorm.DB, applicationID int) (bool, error) {
	var attachments []struct{ ID int }
	if err := tx.Raw("SELECT id FROM attachments WHERE application_id = ? ORDER BY id FOR UPDATE", applicationID).Scan(&attachments).Error; err != nil {
		return false, err
	}
	if len(attachments) == 0 {
		return false, nil // Preserve the existing policy for applications without attachments.
	}
	ids := make([]int, len(attachments))
	for i, attachment := range attachments {
		ids[i] = attachment.ID
	}
	for _, table := range []string{"employees", "cars"} {
		var entities []struct{ ID int }
		if err := tx.Raw("SELECT id FROM "+table+" WHERE attachment_id IN ? ORDER BY id FOR UPDATE", ids).Scan(&entities).Error; err != nil {
			return false, err
		}
	}
	period, err := EntityEffectivePeriodSQL("e", "a")
	if err != nil {
		return false, err
	}
	child := func(table, kind, removed string) string {
		return fmt.Sprintf(`SELECT %s AS date_to, %s AS time_to, %s AS valid_mode
			FROM attachments a JOIN %s e ON e.attachment_id = a.id
			WHERE a.application_id = ? AND a.attachment_type = '%s'
			AND NOT e.is_purged AND e.%s IS NULL`, period.DateTo, period.TimeTo, period.ValidMode, table, kind, removed)
	}
	// Empty and non-person/vehicle attachments use their own admission window.
	fallback := `SELECT a.entry_date_to AS date_to, a.entry_time_to AS time_to, TRUE AS valid_mode
		FROM attachments a WHERE a.application_id = ? AND (
		COALESCE(a.attachment_type, '') NOT IN ('people', 'cars')
		OR (a.attachment_type = 'people' AND NOT EXISTS (SELECT 1 FROM employees e WHERE e.attachment_id=a.id AND NOT e.is_purged AND e.date_deleted IS NULL))
		OR (a.attachment_type = 'cars' AND NOT EXISTS (SELECT 1 FROM cars e WHERE e.attachment_id=a.id AND NOT e.is_purged AND e.date_removed IS NULL)))`
	query := `SELECT NOT EXISTS (SELECT 1 FROM (` + child("employees", "people", "date_deleted") + ` UNION ALL ` + child("cars", "cars", "date_removed") + ` UNION ALL ` + fallback + `) admission
		WHERE admission.valid_mode AND (NULLIF(BTRIM(admission.date_to), '') IS NULL
		OR (NULLIF(BTRIM(admission.date_to), '')::date + COALESCE(NULLIF(BTRIM(admission.time_to), '')::time, TIME '23:59:59')) > (?::timestamptz AT TIME ZONE 'Europe/Moscow'))) AS expired`
	var result struct{ Expired bool }
	if err := tx.Raw(query, applicationID, applicationID, applicationID, tx.NowFunc().UTC()).Scan(&result).Error; err != nil {
		return false, err
	}
	return result.Expired, nil
}
