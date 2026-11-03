package services

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// reserveSupplementEntityIDs использует последовательность, принадлежащую id таблицы,
// а не предполагает её имя. Ordinal задаёт соответствие входной строки и ID независимо
// от порядка возвращённых строк. Последовательность не откатывается вместе с tx:
// после ошибки допустимы пропуски, как и при обычном INSERT с DEFAULT id.
func reserveSupplementEntityIDs(tx *gorm.DB, table string, count int) ([]int, error) {
	switch table {
	case "cars", "items":
	default:
		return nil, fmt.Errorf("unsupported supplement entity table")
	}
	if count == 0 {
		return nil, nil
	}
	var sequence struct{ Name *string }
	if err := tx.Raw("SELECT pg_get_serial_sequence(?, 'id') AS name", table).Scan(&sequence).Error; err != nil {
		return nil, fmt.Errorf("load %s id sequence: %w", table, err)
	}
	if sequence.Name == nil || *sequence.Name == "" {
		return nil, fmt.Errorf("missing %s id sequence", table)
	}
	var rows []struct {
		Ordinal int
		ID      int
	}
	if err := tx.Raw(`
		SELECT input.ordinal, nextval(?::regclass) AS id
		FROM generate_series(1, ?) AS input(ordinal)
		ORDER BY input.ordinal
	`, *sequence.Name, count).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("reserve %s ids: %w", table, err)
	}
	if len(rows) != count {
		return nil, fmt.Errorf("reserve %s ids: got %d rows, expected %d", table, len(rows), count)
	}
	ids := make([]int, count)
	for _, row := range rows {
		if row.Ordinal < 1 || row.Ordinal > count || row.ID <= 0 || ids[row.Ordinal-1] != 0 {
			return nil, fmt.Errorf("invalid %s id reservation", table)
		}
		ids[row.Ordinal-1] = row.ID
	}
	// Читатели карточки и бланка сортируют по id: сохраняем порядок входного списка.
	// Нестандартная убывающая/циклическая последовательность не должна менять порядок.
	for i, id := range ids {
		if id <= 0 || (i > 0 && id <= ids[i-1]) {
			return nil, fmt.Errorf("unordered %s id reservation", table)
		}
	}
	return ids, nil
}

// insertSupplementCarsBatch сохраняет raw SQL и наследование дат из attachments.
// Явные id связывают машины с pending-флагами/аудитом/постами без RETURNING-порядка.
func insertSupplementCarsBatch(tx *gorm.DB, attachmentID, supplementID int, vehicles []VehicleInput, actorID int, now time.Time) ([]int, error) {
	ids, err := reserveSupplementEntityIDs(tx, "cars", len(vehicles))
	if err != nil {
		return nil, err
	}
	for start := 0; start < len(vehicles); start += carInsertBatchSize {
		end := min(start+carInsertBatchSize, len(vehicles))
		values := make([]string, 0, end-start)
		args := make([]interface{}, 0, (end-start)*7+2)
		for i := start; i < end; i++ {
			v := vehicles[i]
			values = append(values, "(?::integer, ?::bigint, ?::text, ?::text, ?::text, ?::timestamptz, ?::bigint)")
			args = append(args, i, ids[i], v.CarNumber, v.CarBrand, v.UnloadPlace,
				consentAt(v.PDConsent, now), consentBy(v.PDConsent, actorID))
		}
		query := `INSERT INTO cars (id, attachment_id, supplement_id, car_number, car_brand, unload_place, entry_date_from, entry_time_from, entry_date_to, entry_time_to, status, pd_consent_at, pd_consent_by_user_id)
			SELECT input.id, a.id, ?, input.car_number, input.car_brand, input.unload_place,
			       a.entry_date_from, a.entry_time_from, a.entry_date_to, a.entry_time_to, 0,
			       input.pd_consent_at, input.pd_consent_by_user_id
			FROM (VALUES ` + strings.Join(values, ", ") + `) AS input(ordinal, id, car_number, car_brand, unload_place, pd_consent_at, pd_consent_by_user_id)
			JOIN attachments a ON a.id = ?
			ORDER BY input.ordinal`
		// Первый placeholder SELECT находится перед VALUES, последний относится к JOIN.
		args = append([]interface{}{supplementID}, args...)
		args = append(args, attachmentID)
		result := tx.Exec(query, args...)
		if result.Error != nil {
			return nil, fmt.Errorf("insert supplement cars: %w", result.Error)
		}
		if result.RowsAffected != int64(end-start) {
			return nil, fmt.Errorf("insert supplement cars: got %d rows, expected %d", result.RowsAffected, end-start)
		}
	}
	return ids, nil
}

// insertSupplementItemsBatch сохраняет только прежние колонки и DB defaults остальных.
// Дата, как и в построчной вставке, передаётся строкой YYYY-MM-DD без смены часового пояса.
func insertSupplementItemsBatch(tx *gorm.DB, attachmentID, supplementID int, items []ItemInput, now time.Time) error {
	ids, err := reserveSupplementEntityIDs(tx, "items", len(items))
	if err != nil {
		return err
	}
	for start := 0; start < len(items); start += carInsertBatchSize {
		end := min(start+carInsertBatchSize, len(items))
		values := make([]string, 0, end-start)
		args := make([]interface{}, 0, (end-start)*6)
		for i := start; i < end; i++ {
			values = append(values, "(?, ?, ?, ?, ?, ?)")
			args = append(args, ids[i], attachmentID, supplementID, items[i].Name, items[i].Count, now.Format("2006-01-02"))
		}
		query := "INSERT INTO items (id, attachment_id, supplement_id, name, count, date_created) VALUES " + strings.Join(values, ", ")
		result := tx.Exec(query, args...)
		if result.Error != nil {
			return fmt.Errorf("insert supplement items: %w", result.Error)
		}
		if result.RowsAffected != int64(end-start) {
			return fmt.Errorf("insert supplement items: got %d rows, expected %d", result.RowsAffected, end-start)
		}
	}
	return nil
}
