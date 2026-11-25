package database

import (
	"fmt"

	"gorm.io/gorm"
)

// InitializeApplicationNumberCounters сохраняет максимальный исторический номер
// за каждые сутки. Таблица и начальные значения появляются одной транзакцией.
func InitializeApplicationNumberCounters(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext('systemburo.application-number-counters'))").Error; err != nil {
			return fmt.Errorf("lock application number migration: %w", err)
		}
		var initialized bool
		if err := tx.Raw("SELECT to_regclass('application_number_counters') IS NOT NULL").Row().Scan(&initialized); err != nil {
			return fmt.Errorf("check application number counters: %w", err)
		}
		if initialized {
			return nil
		}
		if err := tx.Exec(`
			CREATE TABLE application_number_counters (
				day_key text PRIMARY KEY CHECK (day_key ~ '^[0-9]{8}$'),
				last_number numeric(39, 0) NOT NULL CHECK (last_number >= 0)
			)
		`).Error; err != nil {
			return fmt.Errorf("create application number counters: %w", err)
		}
		// Ранее импортированные номера встречаются также без префикса «№».
		// Максимальный суффикс укладывается в оставшиеся 39 символов номера.
		// По журналу восстанавливаем верхнюю границу и для уничтоженных заявок.
		if err := tx.Exec(`
			INSERT INTO application_number_counters (day_key, last_number)
			SELECT parts[1], MAX(parts[2]::numeric)
			FROM (
				SELECT application_number FROM applications
				UNION ALL
				SELECT application_number FROM destruction_log WHERE entity_type = 'application'
			) historical
			CROSS JOIN LATERAL regexp_matches(btrim(historical.application_number),
				'^(?:№[[:space:]]*)?([0-9]{8})/([0-9]{1,39})$') AS number_parts(parts)
			GROUP BY parts[1]
		`).Error; err != nil {
			return fmt.Errorf("initialize application number counters: %w", err)
		}
		return nil
	})
}
