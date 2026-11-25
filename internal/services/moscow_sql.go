package services

import (
	"fmt"
	"time"

	"systemburo/internal/apperr"

	"gorm.io/gorm"
)

// Сравнение сроков заявки с «сейчас» идёт в московской зоне (#2327).
//
// entry_date_from, entry_date_to и entry_time_to хранятся строками и заполняются
// по московским часам бюро - зоны в них нет. Соединение с базой открыто с
// TimeZone=UTC (issue #184, чтобы timestamptz читались одинаково независимо от
// зоны контейнера), поэтому CURRENT_DATE в запросе - это дата по UTC, и до 03:00
// МСК она показывает вчерашний день: заявка, истёкшая вчера, ещё считалась
// действующей всю ночь.
//
// Держим выражения здесь, а не литералом в каждом из двух десятков запросов:
// иначе следующее место снова напишут через CURRENT_DATE, и разойдётся уже не
// зона, а сама модель срока.
const (
	// moscowTodaySQL - сегодняшняя дата по Москве. Замена CURRENT_DATE в сравнениях
	// с entry_date_from / entry_date_to.
	moscowTodaySQL = `(now() AT TIME ZONE 'Europe/Moscow')::date`

	// moscowNowSQL - текущий момент по Москве без зоны, для сравнения с собранным
	// из строк заявки моментом «последний день плюс крайнее время пребывания».
	moscowNowSQL = `(now() AT TIME ZONE 'Europe/Moscow')`
)

// passValidNowSQL - условие «пропуск по вложению ещё действует на этот момент».
//
// Срок кончается не в полночь, а в крайнее время пребывания последнего дня: у
// заявки с 09:00-18:00 по 4 сентября в 18:01 четвёртого пропуск уже недействителен.
// Вложения без времени доживают день до конца - COALESCE подставляет 23:59:59.
//
// Форму зеркалит крон завершения просроченных вложений
// (application_workflow_service.go), только там условие обратное - «уже истёк».
//
// alias - псевдоним таблицы, где лежат entry_date_to и entry_time_to.
func passValidNowSQL(alias string) string {
	return `(NULLIF(TRIM(` + alias + `.entry_date_to), '') IS NULL
		OR (NULLIF(TRIM(` + alias + `.entry_date_to), '')::date
		    + COALESCE(NULLIF(TRIM(` + alias + `.entry_time_to), '')::time, TIME '23:59:59'))
		   > ` + moscowNowSQL + `)`
}

// nextApplicationNumber резервирует порядковый номер в транзакции подачи.
// Ключ счётчика и дата на бланке берутся из одного момента отправки.
func nextApplicationNumber(tx *gorm.DB, sentAt time.Time) (string, error) {
	day := sentAt.In(moscowWorkModeLoc).Format("20060102")
	var sequence string
	err := tx.Raw(`
		INSERT INTO application_number_counters (day_key, last_number) VALUES (?, 1)
		ON CONFLICT (day_key) DO UPDATE SET last_number = application_number_counters.last_number + 1
		RETURNING last_number::text
	`, day).Row().Scan(&sequence)
	if err != nil {
		return "", apperr.Internal("Не удалось выделить номер заявки", err)
	}
	return fmt.Sprintf("№ %s/%03s", day, sequence), nil
}
