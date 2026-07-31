package blankpath

import "strings"

// monthsNominative - названия месяцев в именительном падеже. В стандартной
// библиотеке Go локалей нет, а имена каталогов оператор читает глазами: раскладка,
// к которой он привык в почте, выглядит как "7 ИЮЛЬ 2026".
var monthsNominative = [12]string{
	"январь", "февраль", "март", "апрель", "май", "июнь",
	"июль", "август", "сентябрь", "октябрь", "ноябрь", "декабрь",
}

// MonthName возвращает название месяца (1-12) строчными буквами.
// Вне диапазона возвращает пустую строку.
func MonthName(m int) string {
	if m < 1 || m > 12 {
		return ""
	}
	return monthsNominative[m-1]
}

// MonthNameUpper возвращает название месяца прописными буквами.
func MonthNameUpper(m int) string {
	return strings.ToUpper(MonthName(m))
}
