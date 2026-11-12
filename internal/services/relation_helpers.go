package services

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

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
	ordered := slices.Clone(ids)
	slices.Sort(ordered)
	if len(slices.Compact(ordered)) != len(ids) {
		return apperr.New(http.StatusUnprocessableEntity, "Связанные записи не найдены или повторяются")
	}
	return validateReferenceIDs(tx, target, ids)
}

// validateReferenceIDs locks a deterministic, deduplicated set until commit.
// Archival state is deliberately not filtered: existence is not an access rule.
func validateReferenceIDs(tx *gorm.DB, target any, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	ids = slices.Clone(ids)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	var found []int
	if err := tx.Model(target).Clauses(clause.Locking{Strength: "KEY SHARE"}).Where("id IN ?", ids).Order("id").Pluck("id", &found).Error; err != nil {
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
