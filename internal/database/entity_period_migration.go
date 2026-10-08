package database

import (
	"fmt"
	"gorm.io/gorm"
	"systemburo/internal/models"
)

// BackfillEntityPeriodModes preserves existing car windows. No model default is
// applied before classification: that would mark every legacy car as inherited.
// Explicit modes are never recalculated, including equal individual windows.
func BackfillEntityPeriodModes(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext('systemburo.entity-period-modes'))`).Error; err != nil {
			return fmt.Errorf("lock entity period migration: %w", err)
		}
		// Classify from one stable parent/row snapshot during a rolling deployment.
		// Otherwise an old writer can change a car or attachment between comparison
		// and final constraints, silently assigning the wrong inheritance intent.
		if err := tx.Exec(`LOCK TABLE attachments, employees, cars IN SHARE ROW EXCLUSIVE MODE`).Error; err != nil {
			return fmt.Errorf("lock entity period sources: %w", err)
		}
		// Additive defaults preserve the existing attachment-only behavior. Never
		// backfill from parents: only an explicit selected-car transfer creates own flags.
		if err := tx.Exec(`ALTER TABLE cars ADD COLUMN IF NOT EXISTS individual_roof_access boolean NOT NULL DEFAULT false, ADD COLUMN IF NOT EXISTS individual_free_parking boolean NOT NULL DEFAULT false`).Error; err != nil {
			return fmt.Errorf("add individual car access flags: %w", err)
		}
		if err := tx.Exec(`UPDATE employees SET period_mode = 'inherit' WHERE period_mode IS NULL OR period_mode = ''`).Error; err != nil {
			return fmt.Errorf("classify employee periods: %w", err)
		}
		// Normalize only for comparison; source values are never rewritten.
		normalize := func(alias, column string, clock bool) string {
			expr := fmt.Sprintf("NULLIF(BTRIM(%s.%s), '')", alias, column)
			if clock {
				return fmt.Sprintf("CASE WHEN %s ~ '^[0-9]{2}:[0-9]{2}$' THEN %s || ':00' ELSE %s END", expr, expr, expr)
			}
			return expr
		}
		different := ""
		for _, field := range []struct {
			name  string
			clock bool
		}{{"entry_date_from", false}, {"entry_date_to", false}, {"entry_time_from", true}, {"entry_time_to", true}} {
			if different != "" {
				different += " OR "
			}
			different += normalize("c", field.name, field.clock) + " IS DISTINCT FROM " + normalize("a", field.name, field.clock)
		}
		query := `UPDATE cars c SET period_mode = CASE WHEN (` + different + `) THEN 'individual' ELSE 'inherit' END FROM attachments a WHERE c.attachment_id = a.id AND (c.period_mode IS NULL OR c.period_mode = '')`
		if err := tx.Exec(query).Error; err != nil {
			return fmt.Errorf("classify car periods: %w", err)
		}
		for _, table := range []string{"employees", "cars"} {
			var invalid int64
			if err := tx.Table(table).Where("period_mode IS NULL OR period_mode NOT IN ?", []string{"inherit", "individual"}).Count(&invalid).Error; err != nil {
				return fmt.Errorf("check %s period modes: %w", table, err)
			}
			if invalid != 0 {
				return fmt.Errorf("%s has unclassified or invalid period modes; inventory required", table)
			}
			// Incomplete legacy individual dates must not turn into an unlimited pass.
			lastID := 0
			for {
				var rows []struct {
					ID            int
					EntryDateFrom *string
					EntryDateTo   *string
					EntryTimeFrom *string
					EntryTimeTo   *string
				}
				if err := tx.Table(table).Select("id, entry_date_from, entry_date_to, entry_time_from, entry_time_to").Where("period_mode = 'individual' AND id > ?", lastID).Order("id").Limit(1000).Scan(&rows).Error; err != nil {
					return fmt.Errorf("validate %s individual periods: %w", table, err)
				}
				if len(rows) == 0 {
					break
				}
				for _, row := range rows {
					if err := models.ValidateStoredIndividualPeriod(models.EntryPeriod{EntryDateFrom: row.EntryDateFrom, EntryDateTo: row.EntryDateTo, EntryTimeFrom: row.EntryTimeFrom, EntryTimeTo: row.EntryTimeTo}); err != nil {
						return fmt.Errorf("%s has invalid individual periods; inventory required", table)
					}
					lastID = row.ID
				}
			}
			if err := tx.Exec("ALTER TABLE " + table + " ALTER COLUMN period_mode SET DEFAULT 'inherit', ALTER COLUMN period_mode SET NOT NULL").Error; err != nil {
				return fmt.Errorf("finalize %s period mode: %w", table, err)
			}
			constraint := "chk_" + table + "_period_mode"
			var exists int64
			if err := tx.Raw(`SELECT COUNT(*) FROM pg_constraint WHERE conrelid = ?::regclass AND conname = ?`, table, constraint).Scan(&exists).Error; err != nil {
				return fmt.Errorf("check %s mode constraint: %w", table, err)
			}
			if exists == 0 {
				if err := tx.Exec("ALTER TABLE " + table + " ADD CONSTRAINT " + constraint + " CHECK (period_mode IN ('inherit', 'individual'))").Error; err != nil {
					return fmt.Errorf("constrain %s period mode: %w", table, err)
				}
			}
		}
		return nil
	})
}
