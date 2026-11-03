package services

import (
	"fmt"

	"systemburo/internal/apperr"
)

// MaxApplicationRowsPerType — потолок каждого вида строк на весь запрос подачи или дополнения.
// Виды считаются независимо; существующее содержимое заявки в бюджет не входит.
const MaxApplicationRowsPerType = 2000

type applicationCompositionCounts struct {
	employees int
	vehicles  int
	items     int
}

// ValidateSubmitComposition проверяет суммарный состав всех вложений запроса.
func ValidateSubmitComposition(req CompleteApplicationRequest) error {
	var counts applicationCompositionCounts
	for _, att := range req.Attachments {
		if att.Data.Employees != nil {
			counts.employees += len(*att.Data.Employees)
		}
		if att.Data.Vehicles != nil {
			counts.vehicles += len(*att.Data.Vehicles)
		}
		if att.Data.Items != nil {
			counts.items += len(*att.Data.Items)
		}
	}
	return validateApplicationComposition(counts)
}

// ValidateSupplementComposition проверяет все добавления без учёта прежнего состава заявки.
func ValidateSupplementComposition(req CreateSupplementRequest) error {
	var counts applicationCompositionCounts
	for _, addition := range req.Additions {
		counts.employees += len(addition.Employees)
		counts.vehicles += len(addition.Vehicles)
		counts.items += len(addition.Items)
	}
	return validateApplicationComposition(counts)
}

func validateApplicationComposition(counts applicationCompositionCounts) error {
	for _, list := range []struct {
		count int
		label string
	}{
		{counts.employees, "сотрудников"},
		{counts.vehicles, "машин"},
		{counts.items, "ТМЦ"},
	} {
		if list.count > MaxApplicationRowsPerType {
			return apperr.Validation(fmt.Sprintf("Слишком много %s в заявке: %d, максимум %d. Разбейте заявку на несколько частей.", list.label, list.count, MaxApplicationRowsPerType))
		}
	}
	return nil
}
