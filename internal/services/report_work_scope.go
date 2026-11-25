package services

import (
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

const workScopeNow = "now"
const workScopeToday = "today"

// A shift may start on the previous Moscow date. Windows are half-open:
// at the end of a shift it no longer contributes to the current register.
func applyWorkScope(tx *gorm.DB, scope string, at time.Time) *gorm.DB {
	local := at.In(MoscowLocation())
	today := local.Format("2006-01-02")
	yesterday := local.AddDate(0, 0, -1).Format("2006-01-02")
	clock := local.Format("15:04:05")
	active, args := activeApplicationCond("app")
	tx = tx.Where(active, args...).
		Where("att.status = 1 AND app.confirmation = ? AND app.status IN ?", models.ConfirmationApproved, []string{models.StatusInWork, models.StatusCompleted})

	const schedule = `SELECT NULLIF(TRIM(att.entry_date_from), '') AS first_day,
		NULLIF(TRIM(att.entry_date_to), '') AS last_day,
		COALESCE(NULLIF(TRIM(att.entry_time_from), '')::time, TIME '00:00') AS starts,
		COALESCE(NULLIF(TRIM(att.entry_time_to), '')::time, TIME '24:00') AS ends`
	const dayInRange = `(first_day IS NOT NULL OR last_day IS NOT NULL)
		AND (first_day IS NULL OR shift_day >= first_day)
		AND (last_day IS NULL OR shift_day <= last_day)`
	const fromShiftDay = `EXISTS (SELECT 1 FROM (` + schedule + `) AS hours
		CROSS JOIN (VALUES (?::text, TRUE), (?::text, FALSE)) AS days(shift_day, is_today)
		WHERE ` + dayInRange + ` AND `
	if scope == workScopeToday {
		return tx.Where(fromShiftDay+`((is_today AND starts <> ends) OR
			(NOT is_today AND ends < starts AND ends > TIME '00:00')))`, today, yesterday)
	}
	return tx.Where(fromShiftDay+`((is_today AND ?::time >= starts AND (ends < starts OR ?::time < ends)) OR
		(NOT is_today AND ends < starts AND ?::time < ends)))`, today, yesterday, clock, clock, clock)
}
