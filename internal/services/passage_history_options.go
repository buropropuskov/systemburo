package services

// PassageFilterUser - учётная запись в выпадающем списке «Пользователь» журнала
// проходов: только те, кто действительно отмечал проход.
type PassageFilterUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// CarsHistoryFilterOptions - значения выпадающих списков журнала машин. Отдаются
// отдельным методом, а не вместе со страницей: список, собранный из 50 загруженных
// строк, предлагал бы выбрать не тех, кто отмечал, а тех, кто попал в эту страницу.
//
// Машин здесь нет: их список модалка получает от таблицы проходной, в которой открыта,
// и дублировать его выборкой по журналу незачем.
type CarsHistoryFilterOptions struct {
	Users []PassageFilterUser `json:"users"`
}

// PassageFilterEmployee - человек в выпадающем списке журнала людей. ФИО приходит из
// employees и может быть неполным у ручных записей, поэтому указатели.
type PassageFilterEmployee struct {
	ID         int     `json:"id"`
	LastName   *string `json:"last_name"`
	FirstName  *string `json:"first_name"`
	MiddleName *string `json:"middle_name"`
}

// EmployeesHistoryFilterOptions - значения выпадающих списков журнала людей: кто
// отмечал проходы и кого отмечали.
//
// Людей, в отличие от машин, отдаёт сервер: у модалки машин список берётся из таблицы
// проходной, в которой она открыта, а сотрудников таблица целиком не знает - до #2469
// список собирался из загруженной истории.
type EmployeesHistoryFilterOptions struct {
	Users     []PassageFilterUser     `json:"users"`
	Employees []PassageFilterEmployee `json:"employees"`
}
