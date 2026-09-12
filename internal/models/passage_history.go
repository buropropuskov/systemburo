package models

import "strings"

// Границы страницы журнала проходов. 50 строк - порция подгрузки в модалке, 200 -
// предел, за которым запрос снова превращается в выгрузку всей истории (ради этого
// предела #2469 и заводился: до него журнал отдавался целиком).
const (
	PassageHistoryDefaultPerPage = 50
	PassageHistoryMaxPerPage     = 200
)

// PassageHistoryQuery - фильтры, порядок и страница журнала проходов: въездов и
// выездов машин, входов и выходов людей.
//
// Набор полей повторяет то, что предлагает интерфейс журнала. Иначе смысла в
// серверных границах нет: фильтр, которого на сервере не существует, фронт снова
// начнёт применять к загруженной странице, и человек получит поиск по 50 строкам
// вместо поиска по журналу.
//
// CarID и EmployeeID разведены, потому что методы у сущностей свои, а модель
// запроса общая: у машин в журнале строка привязана к car_id, у людей к employee_id.
type PassageHistoryQuery struct {
	// UserID - кто отметил проход.
	UserID     *int   `query:"user_id"`
	CarID      *int   `query:"car_id"`
	EmployeeID *int   `query:"employee_id"`
	DateFrom   string `query:"date_from"`
	DateTo     string `query:"date_to"`
	Search     string `query:"search"`
	// Order - порядок по времени отметки: desc (свежие сверху) или asc.
	Order   string `query:"order"`
	Page    int    `query:"page"`
	PerPage int    `query:"per_page"`
}

// Normalize приводит страницу, размер и порядок к допустимым значениям. Мусор в
// параметре не должен отдавать всю историю: перебор per_page срезается до предела,
// неизвестный порядок читается как «свежие сверху».
func (q *PassageHistoryQuery) Normalize() {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PerPage < 1 {
		q.PerPage = PassageHistoryDefaultPerPage
	}
	if q.PerPage > PassageHistoryMaxPerPage {
		q.PerPage = PassageHistoryMaxPerPage
	}
	if strings.EqualFold(q.Order, "asc") {
		q.Order = "asc"
	} else {
		q.Order = "desc"
	}
	q.Search = strings.TrimSpace(q.Search)
}

// Ascending сообщает, что журнал запрошен от старых отметок к свежим.
func (q PassageHistoryQuery) Ascending() bool { return q.Order == "asc" }

// Offset - смещение страницы. Normalize обязателен до вызова.
func (q PassageHistoryQuery) Offset() int { return (q.Page - 1) * q.PerPage }
