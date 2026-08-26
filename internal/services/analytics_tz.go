package services

import (
	"fmt"
	"time"
)

// analyticsTimeZone — бизнес-таймзона аналитики (МСК). Бакетинг по времени
// (date_trunc/EXTRACT) и границы периода считаются в ней, иначе сутки режутся по
// UTC-полуночи (= 03:00 МСК) и точки графика «съезжают» на 3 часа. Колонки времени
// хранятся как timestamptz, поэтому `ts AT TIME ZONE 'Europe/Moscow'` переводит
// UTC-инстант в московское настенное время для группировки.
const analyticsTimeZone = "Europe/Moscow"

// analyticsLocation — та же зона для разбора входных дат периода (YYYY-MM-DD ->
// инстант начала/конца московских суток). При сбое загрузки падаем на UTC.
var analyticsLocation = func() *time.Location {
	loc, err := time.LoadLocation(analyticsTimeZone)
	if err != nil {
		return time.UTC
	}
	return loc
}()

// AnalyticsLocation возвращает бизнес-таймзону аналитики для разбора дат периода
// на уровне хендлеров (границы периода считаются в МСК, как и бакетинг).
func AnalyticsLocation() *time.Location {
	return analyticsLocation
}

// tzColumn оборачивает timestamptz-колонку переводом в московское настенное время.
// Подставляется в date_trunc/EXTRACT, чтобы бакеты резались по локальным суткам.
// Имя колонки — только из whitelist (не пользовательский ввод), литерал зоны
// константный, поэтому подстановка в SQL безопасна.
func tzColumn(col string) string {
	return fmt.Sprintf("(%s AT TIME ZONE '%s')", col, analyticsTimeZone)
}
