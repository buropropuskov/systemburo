package services

import (
	"fmt"
	"strings"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
)

// passageDateLayout - формат дат фильтра журнала проходов: календарный день без
// времени, как его присылает поле ввода даты.
const passageDateLayout = "2006-01-02"

// passageFilterSpec описывает, куда прикладывать общие фильтры журнала: alias
// подзапроса истории, колонка сущности и выражения, по которым идёт поиск строкой.
type passageFilterSpec struct {
	alias        string
	entityColumn string
	entityID     *int
	searchExprs  []string
}

// passageHistoryConditions собирает хвост WHERE для журнала проходов и аргументы к
// нему. Результат начинается с " AND " (или пуст) и дописывается к готовому условию
// выборки.
//
// Даты читаются как московские сутки включительно. Соединение с базой открыто в UTC
// (#184), поэтому «с 12 сентября» без зоны началось бы в 03:00 МСК и утренняя смена
// выпала бы из выборки.
func passageHistoryConditions(q models.PassageHistoryQuery, spec passageFilterSpec) (string, []any, error) {
	var conds []string
	var args []any

	if spec.entityID != nil {
		conds = append(conds, spec.entityColumn+" = ?")
		args = append(args, *spec.entityID)
	}
	if q.UserID != nil {
		conds = append(conds, spec.alias+".user_id = ?")
		args = append(args, *q.UserID)
	}
	if q.DateFrom != "" {
		from, err := time.ParseInLocation(passageDateLayout, q.DateFrom, MoscowLocation())
		if err != nil {
			return "", nil, apperr.Validation("Некорректная дата начала периода")
		}
		conds = append(conds, spec.alias+".created_at >= ?")
		args = append(args, from)
	}
	if q.DateTo != "" {
		to, err := time.ParseInLocation(passageDateLayout, q.DateTo, MoscowLocation())
		if err != nil {
			return "", nil, apperr.Validation("Некорректная дата окончания периода")
		}
		conds = append(conds, spec.alias+".created_at < ?")
		args = append(args, to.AddDate(0, 0, 1))
	}
	if q.Search != "" && len(spec.searchExprs) > 0 {
		pattern := "%" + escapeLikePattern(q.Search) + "%"
		parts := make([]string, 0, len(spec.searchExprs))
		for _, expr := range spec.searchExprs {
			parts = append(parts, expr+" ILIKE ?")
			args = append(args, pattern)
		}
		conds = append(conds, "("+strings.Join(parts, " OR ")+")")
	}

	if len(conds) == 0 {
		return "", nil, nil
	}
	return " AND " + strings.Join(conds, " AND "), args, nil
}

// passageHistoryOrderSQL - порядок журнала по времени отметки. id вторым ключом
// обязателен: отметки, попавшие в одну секунду, без него перемешиваются между
// запросами, и подгрузка страницами то повторяет строку, то теряет её.
func passageHistoryOrderSQL(q models.PassageHistoryQuery, alias string) string {
	dir := "DESC"
	if q.Ascending() {
		dir = "ASC"
	}
	return fmt.Sprintf(" ORDER BY %s.created_at %s, %s.id %s", alias, dir, alias, dir)
}

// passageHistoryLimitSQL - хвост страницы. Normalize у запроса обязателен до вызова:
// он и держит предел, за которым журнал снова стал бы выгрузкой всей истории.
func passageHistoryLimitSQL(q models.PassageHistoryQuery) (string, []any) {
	return " LIMIT ? OFFSET ?", []any{q.PerPage, q.Offset()}
}
