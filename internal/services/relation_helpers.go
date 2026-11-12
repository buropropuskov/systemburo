package services

import (
	"errors"
	"fmt"
	"net/http"

	"systemburo/internal/apperr"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// validateDirectoryLinkTargets keeps the parent and referenced rows available
// until the caller commits its replacement transaction.
func validateDirectoryLinkTargets(tx *gorm.DB, parent any, parentID int, target any, ids []int) error {
	err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", parentID).Take(parent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound("Запись справочника не найдена")
	}
	if err != nil || len(ids) == 0 {
		return err
	}
	var found []int
	if err := tx.Model(target).Clauses(clause.Locking{Strength: "KEY SHARE"}).Where("id IN ?", ids).Pluck("id", &found).Error; err != nil {
		return err
	}
	if len(found) != len(ids) {
		return apperr.New(http.StatusUnprocessableEntity, "Связанные записи не найдены или повторяются")
	}
	return nil
}

var allowedRelationTables = map[string]bool{
	"organization_users":     true,
	"companies_users":        true,
	"organization_tables":    true,
	"companies_tables":       true,
	"employee_target_tables": true,
}

// replaceRelations заменяет связи в junction-таблице: удаляет старые и вставляет новые.
func replaceRelations(tx *gorm.DB, table string, fkColumn string, entityID int, newIDs []int, targetColumn string) error {
	if !allowedRelationTables[table] {
		return fmt.Errorf("disallowed table in replaceRelations: %s", table)
	}

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
