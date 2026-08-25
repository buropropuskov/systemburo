// Package normalize приводит ФИО и госномера к канонической форме для нечёткого
// сравнения с чёрным списком (#481). Цель - схлопнуть визуально близкие варианты,
// которыми обходят точный гард ЧС: латиница вместо кириллицы ("Ивaнов"), регистр,
// лишние пробелы/разделители, ё/е, подмена буквы О на ноль в номере.
//
// Нормализованная форма хранится в отдельных колонках blacklist-таблиц и
// заполняется ТОЙ ЖЕ функцией, что нормализует поисковый запрос, - иначе эталон и
// запрос разойдутся и сравнение перестанет работать.
package normalize

import "strings"

// cyrToLatKeyboard - кириллические буквы на русской раскладке к латинским буквам
// на той же физической клавише QWERTY. Нижний регистр: позиция руки одна, буква другая.
// Позволяет найти запрос, набранный кириллицей когда нужна латиница (и наоборот).
var cyrToLatKeyboard = map[rune]rune{
	'й': 'q', 'ц': 'w', 'у': 'e', 'к': 'r', 'е': 't', 'н': 'y', 'г': 'u', 'ш': 'i',
	'щ': 'o', 'з': 'p', 'х': '[', 'ъ': ']',
	'ф': 'a', 'ы': 's', 'в': 'd', 'а': 'f', 'п': 'g', 'р': 'h', 'о': 'j', 'л': 'k',
	'д': 'l', 'ж': ';', 'э': '\'',
	'я': 'z', 'ч': 'x', 'с': 'c', 'м': 'v', 'и': 'b', 'т': 'n', 'ь': 'm', 'б': ',',
	'ю': '.', 'ё': '`',
}

// latToCyrKeyboard - обратная таблица: латинские буквы к русским на той же физической
// клавише QWERTY. Строится из cyrToLatKeyboard при инициализации.
var latToCyrKeyboard map[rune]rune

func init() {
	latToCyrKeyboard = make(map[rune]rune, len(cyrToLatKeyboard))
	for cyr, lat := range cyrToLatKeyboard {
		latToCyrKeyboard[lat] = cyr
	}
}

// SwitchLayout переключает раскладку строки s: кириллица->латиница или латиница->кириллица
// посимвольно по таблице QWERTY. Символы без соответствия передаются без изменений.
// Используется для порождения альтернативного варианта поискового запроса: если пользователь
// набрал слово не переключив раскладку, переключённый вариант совпадёт с хранимыми данными.
func SwitchLayout(s string) string {
	lowered := strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(lowered))
	for _, r := range lowered {
		if lat, ok := cyrToLatKeyboard[r]; ok {
			b.WriteRune(lat)
		} else if cyr, ok := latToCyrKeyboard[r]; ok {
			b.WriteRune(cyr)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// latToCyrLower - латинские буквы, которыми подменяют кириллицу в ФИО, в нижнем
// регистре. Применяется ПОСЛЕ ToLower, поэтому ключи строчные. Набор - это те же 12
// латинских букв, что омоглифят русские буквы А В Е К М Н О Р С Т У Х (см. latToCyrUpper):
// часть идентична и в нижнем регистре (a/а, e/е, o/о, p/р, c/с, x/х, y/у), часть -
// только в верхнем (B/В, H/Н, K/К, M/М, T/Т), но ToLower сворачивает заглавную подмену
// в начале слова к строчной, поэтому ловим и её. Это слой предупреждения, не блокировки:
// здесь лучше пересхлопнуть, чем пропустить обход (ФИО в системе кириллические, ложного
// сближения легитимных имён латиницей не возникает).
var latToCyrLower = map[rune]rune{
	'a': 'а',
	'b': 'в',
	'c': 'с',
	'e': 'е',
	'h': 'н',
	'k': 'к',
	'm': 'м',
	'o': 'о',
	'p': 'р',
	't': 'т',
	'x': 'х',
	'y': 'у',
}

// latToCyrUpper - заглавные латинские омоглифы для госномеров. Это ровно 12 букв,
// допустимых в российских номерах (А В Е К М Н О Р С Т У Х).
var latToCyrUpper = map[rune]rune{
	'A': 'А',
	'B': 'В',
	'E': 'Е',
	'K': 'К',
	'M': 'М',
	'H': 'Н',
	'O': 'О',
	'P': 'Р',
	'C': 'С',
	'T': 'Т',
	'Y': 'У',
	'X': 'Х',
}

// Name приводит ФИО к канонической форме: нижний регистр, схлопнутые пробелы,
// ё->е, латинские омоглифы->кириллица. Порядок частей сохраняется (фамилия имя
// отчество), пустые части отбрасываются - отсутствие отчества даёт более короткую
// строку, что снижает (но не обнуляет) близость к записи с отчеством.
func Name(parts ...string) string {
	joined := strings.Join(parts, " ")
	lowered := strings.ToLower(joined)

	var b strings.Builder
	b.Grow(len(lowered))
	for _, r := range lowered {
		if r == 'ё' {
			r = 'е'
		}
		if cyr, ok := latToCyrLower[r]; ok {
			r = cyr
		}
		b.WriteRune(r)
	}
	// strings.Fields схлопывает любые пробельные последовательности и обрезает края.
	return strings.Join(strings.Fields(b.String()), " ")
}

// FixLatinInName заменяет латинские омоглифы на кириллицу в ОДНОЙ части ФИО, СОХРАНЯЯ
// регистр (в отличие от Name, которая приводит всё к нижнему для ключа сравнения), и
// схлопывает лишние пробелы. Используется там, где латиница внутри кириллического ФИО
// - предупреждение с показом исправленного варианта, а не блокирующая ошибка (blank-import
// C3): опечатка раскладки при заполнении бланка встречается чаще, чем настоящее
// иностранное имя. Второе возвращаемое значение - была ли заменена хотя бы одна буква.
func FixLatinInName(s string) (fixed string, latinFound bool) {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			if cyr, ok := latToCyrLower[r]; ok {
				b.WriteRune(cyr)
				latinFound = true
				continue
			}
		case r >= 'A' && r <= 'Z':
			if cyr, ok := latToCyrUpper[r]; ok {
				b.WriteRune(cyr)
				latinFound = true
				continue
			}
		}
		b.WriteRune(r)
	}
	fixed = strings.Join(strings.Fields(b.String()), " ")
	return fixed, latinFound
}

// Plate приводит госномер к канонической форме: верхний регистр, удаление всех
// разделителей и пробелов, латинские омоглифы->кириллица, ноль->буква О. Схлопывание
// 0->О - осознанный компромисс: ловит классическую подмену О<->0, ценой редких
// ложных сближений (регион "70" -> "7О"). Для предупреждения (не блокировки) приемлемо.
func Plate(number string) string {
	stripped := StripPlateSeparators(strings.ToUpper(number))

	var b strings.Builder
	b.Grow(len(stripped))
	for _, r := range stripped {
		if r == '0' {
			b.WriteRune('О')
			continue
		}
		if cyr, ok := latToCyrUpper[r]; ok {
			b.WriteRune(cyr)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// StripPlateSeparators убирает пробелы, дефисы и подчёркивания - разделители, которыми
// иногда размечают госномер, но которые не входят в саму комбинацию символов. Вынесено
// из Plate отдельной функцией для разбора номера по ячейкам формата при импорте
// (blank-import-ux U2): там строка сначала делится на сегменты по ячейкам, а уже потом
// каждый сегмент нормализуется по своим правилам - раньше делить нельзя, разделители
// мешают разбиению.
func StripPlateSeparators(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == ' ' || r == '-' || r == '_' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// FixPlateLetterCell нормализует СЕГМЕНТ номера, уже отнесённый разбором по формату к
// буквенной (не смешанной и не числовой) кириллической ячейке: те же замены, что Plate
// (латиница-омоглиф в кириллицу, 0 в букву О), но применённые только к этому сегменту -
// целиком по строке 0->О сломал бы соседние числовые ячейки, где 0 легитимная цифра
// (см. Plate). Строка должна быть уже в верхнем регистре и без разделителей - сегмент
// пришёл из уже нормализованной по StripPlateSeparators строки. Второе значение - была
// ли хоть одна замена (используется для предупреждения строки импорта, а не блокировки).
func FixPlateLetterCell(s string) (fixed string, changed bool) {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '0' {
			b.WriteRune('О')
			changed = true
			continue
		}
		if cyr, ok := latToCyrUpper[r]; ok {
			b.WriteRune(cyr)
			changed = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String(), changed
}
