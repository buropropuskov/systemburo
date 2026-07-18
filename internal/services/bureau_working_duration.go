package services

// bureauWorkingDuration -- рабочие секунды Бюро между двумя моментами по его
// расписанию (аналитика обработки заявок #1251). Зеркалит форму durationBetween
// (from, to -- SQL-выражения колонок, порядок «позже минус раньше»), но вместо
// календарной разницы EXTRACT(EPOCH ...) считает только время, когда Бюро реально
// работает: ночь и выходные из длительности выпадают. Логика инкапсулирована в
// SQL-функции bureau_working_seconds (installSQLFunctions в migrate.go).
//
// S1 добавляет только хелпер и функцию; переключение этапов согласования/принятия/
// обработки с durationBetween на bureauWorkingDuration -- отдельный срез S2.
func bureauWorkingDuration(from, to string) string {
	return "bureau_working_seconds(" + from + ", " + to + ")"
}
