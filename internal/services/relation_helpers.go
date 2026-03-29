package services

import (
	"fmt"

	"gorm.io/gorm"
)

// replaceRelations заменяет связи в junction-таблице: удаляет старые и вставляет новые.
func replaceRelations(tx *gorm.DB, table string, fkColumn string, entityID int, newIDs []int, targetColumn string) error {
	if err := tx.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE %s = ?", table, fkColumn),
		entityID,
	).Error; err != nil {
		return err
	}

	for _, id := range newIDs {
		if err := tx.Exec(
			fmt.Sprintf("INSERT INTO %s (%s, %s) VALUES (?, ?)", table, fkColumn, targetColumn),
			entityID, id,
		).Error; err != nil {
			return err
		}
	}

	return nil
}
