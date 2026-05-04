package services

import "time"

// FormatUTC сериализует момент времени в RFC3339 (с миллисекундной точностью)
// в UTC-зоне. Если t уже не UTC, значение конвертируется без изменения момента.
//
// Это единственное правильное представление времени в API: клиент через
// new Date(s).toLocaleString() сам приведёт к локальной зоне.
func FormatUTC(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// FormatUTCPtr возвращает указатель на FormatUTC(*t), либо nil для nil-входа.
// Удобно для опциональных полей вроде territory_entry_time, last_exit_time.
func FormatUTCPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := FormatUTC(*t)
	return &s
}

// NowUTC возвращает текущий момент в UTC. Использовать вместо time.Now()
// для всех timestamp-полей (created_at, updated_at, sending_datetime и т.д.).
//
// Локальная зона сервера в Go может отличаться от UTC -- если писать time.Now()
// без UTC, в Postgres timestamptz столбец конвертирует, но любая ручная сериализация
// в JSON или сравнения дадут расхождение по часам.
func NowUTC() time.Time {
	return time.Now().UTC()
}
