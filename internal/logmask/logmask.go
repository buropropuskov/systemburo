// Package logmask прячет значения параметров адреса, попадающего в журнал
// обращений. Единая точка на два входа: запись журнала (middleware) и выгрузку
// журнала файлом (handlers) - иначе перечень безопасных ключей разъехался бы, и
// выгрузка уносила бы то, чего в записи уже нет.
package logmask

import (
	"net/url"
	"strings"
)

// MaskedValue - чем заменяется значение параметра вне белого списка.
const MaskedValue = "***"

// safeKeys - параметры, значение которых можно писать в журнал открытым текстом.
//
// Список белый, а не чёрный. До #2125 затирались четыре ключа с секретами
// (ticket, token, access_token, key), а любой другой параметр попадал в журнал по
// умолчанию - так там осели поисковые строки с ФИО и номерами заявок
// (/api/users?search=Тимофей, /api/applications?search=20260812/003). Журнал живёт
// месяцами, читается через интерфейс и выгружается файлом, поэтому по умолчанию
// значение затирается, а служебные параметры разбора (страницы, сортировка,
// границы периода, коды ответа) остаются: по ним видно, что именно запрашивали.
//
// Новый параметр в белый список попадает осознанно: он не должен нести ни
// персональных данных, ни секретов - ни сейчас, ни когда его смысл расширят.
var safeKeys = map[string]struct{}{
	"page": {}, "per_page": {}, "limit": {}, "offset": {},
	"interval": {}, "from_date": {}, "to_date": {}, "since": {},
	"sort": {}, "order": {},
	"method": {}, "status": {}, "status_min": {}, "status_max": {},
	"min_duration_ms": {}, "archive": {},
}

// Query отдаёт адрес запроса, в котором остались только безопасные значения
// параметров. Ключи сохраняются все: факт «искали по имени» в журнале нужен,
// само имя - нет.
func Query(u *url.URL) string {
	if u == nil {
		return ""
	}
	if u.RawQuery == "" {
		return u.String()
	}

	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		// Разобрать не вышло - значит и решить, что там безопасно, нельзя.
		// Оставлять строку как есть опаснее, чем потерять её целиком.
		clone := *u
		clone.RawQuery = MaskedValue
		return clone.String()
	}

	unsafe := make([]string, 0, len(q))
	for key := range q {
		if _, ok := safeKeys[key]; !ok {
			unsafe = append(unsafe, key)
		}
	}
	if len(unsafe) == 0 {
		return u.String()
	}
	for _, key := range unsafe {
		q.Set(key, MaskedValue)
	}

	clone := *u
	clone.RawQuery = encodeMasked(q)
	return clone.String()
}

// encodeMasked собирает query обратно, оставляя саму метку читаемой: Encode
// экранирует звёздочки в %2A%2A%2A, и адрес в журнале превращается в шифр,
// который человек глазами не разбирает.
func encodeMasked(q url.Values) string {
	return strings.ReplaceAll(q.Encode(), url.QueryEscape(MaskedValue), MaskedValue)
}

// RawURL прогоняет через маску уже записанный адрес. Нужен на выгрузке: записи,
// сделанные до перехода на белый список, лежат в базе с открытыми значениями, и
// файл унёс бы их наружу.
func RawURL(raw string) string {
	if raw == "" || !strings.Contains(raw, "?") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw[:strings.Index(raw, "?")] + "?" + MaskedValue
	}
	return Query(u)
}
