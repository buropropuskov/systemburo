package database

import (
	"net/url"
	"strings"
)

// EnsureUTCTimezone добавляет TimeZone=UTC к Postgres DSN, если параметр не задан.
//
// Поддерживает оба формата:
//   - URL: "postgres://user:pass@host/db?sslmode=disable" -> добавляет ?TimeZone=UTC
//   - keyword: "host=... user=... dbname=..." -> добавляет " TimeZone=UTC"
//
// Цель: заставить gorm/pgx отдавать timestamptz в UTC независимо от TZ
// контейнера и сервера БД. Без этого "удаление в 21:22" в Алматы (+5/+6)
// записывается как 15:22 после неявной конверсии в локальную зону. Issue #184.
func EnsureUTCTimezone(dsn string) string {
	if dsn == "" {
		return dsn
	}

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return dsn
		}
		q := u.Query()
		if hasTimeZoneKey(q) {
			return dsn
		}
		q.Set("TimeZone", "UTC")
		u.RawQuery = q.Encode()
		return u.String()
	}

	// keyword/value формат: пропускаем если уже задан timezone.
	low := strings.ToLower(dsn)
	if strings.Contains(low, "timezone=") || strings.Contains(low, "time_zone=") {
		return dsn
	}
	if strings.HasSuffix(dsn, " ") {
		return dsn + "TimeZone=UTC"
	}
	return dsn + " TimeZone=UTC"
}

func hasTimeZoneKey(q url.Values) bool {
	for k := range q {
		switch strings.ToLower(k) {
		case "timezone", "time_zone":
			return true
		}
	}
	return false
}
