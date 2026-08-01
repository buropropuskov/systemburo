package services

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// uniqueEmployeeSearchProvider ищет по реестру сотрудников (unique_employees).
//
// Зеркалит buildEmployeesQuery (unique_employee_service.go): те же незашифрованные
// колонки (ФИО, должность, организация, компания, гражданство) и тот же фильтр
// видимости. Паспорт и патент не ищутся и не выдаются: в БД лежит шифротекст, ILIKE по
// нему бессмыслен, а поиск по HMAC превратил бы эндпоинт в оракул "есть ли в системе
// человек с таким паспортом".
//
// Одно расхождение с листингом сознательное: здесь нет ветки
// strict_word_similarity(?, concat_ws(...)) > 0.3. Функция от concat_ws не покрывается
// GIN-индексом (выражение не IMMUTABLE, индекс по нему не создать), поэтому такая ветка
// заставляет сканировать таблицу целиком на каждое нажатие клавиши. Листингу это
// прощается -- он открывается по кнопке, а подсказки поиска летят на каждый ввод.
// Опечатки, если понадобятся, добавляются оператором %>> по отдельным колонкам: он, в
// отличие от функции, идёт по индексу.
type uniqueEmployeeSearchProvider struct{}

func (uniqueEmployeeSearchProvider) Type() SearchEntityType { return SearchTypeEmployees }
func (uniqueEmployeeSearchProvider) Title() string          { return "Сотрудники" }
func (uniqueEmployeeSearchProvider) PermissionKey() string  { return KeyEntityEmployeesRead }

func (uniqueEmployeeSearchProvider) Search(ctx context.Context, db *gorm.DB, req searchRequest) ([]SearchItem, error) {
	// position -- зарезервированное слово, отсюда кавычки (как в buildEmployeesQuery).
	cols := []string{"ue.last_name", "ue.first_name", "ue.middle_name", `ue."position"`, "o.name", "c.name", "cit.name"}
	cond, args := searchCondition(cols, req.Raw)

	rows := make([]searchRow, 0, req.Limit+1)
	err := withTrigramThreshold(ctx, db, func(tx *gorm.DB) error {
		q := tx.
			Table("unique_employees ue").
			Joins("LEFT JOIN organizations o ON ue.organization_id = o.id").
			Joins("LEFT JOIN companies c ON ue.company_id = c.id").
			Joins("LEFT JOIN citizenships cit ON ue.citizenship_id = cit.id").
			Select(`ue.id AS id,
				TRIM(CONCAT_WS(' ', ue.last_name, ue.first_name, ue.middle_name)) AS title,
				CONCAT_WS(' · ', NULLIF(ue."position", ''), COALESCE(o.name, c.name)) AS subtitle,
				`+matchRankExpr("ue.last_name"), req.Raw, req.Raw).
			Where(cond, args...)

		q = applyRegistryScope(q, "ue", req)

		return q.
			Order("match_rank, ue.id DESC").
			Limit(req.Limit + 1).
			Scan(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("поиск по реестру сотрудников: %w", err)
	}

	return rowsToItems(SearchTypeEmployees, "unique_employee", rows), nil
}
