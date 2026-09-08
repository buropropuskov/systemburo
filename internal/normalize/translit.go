package normalize

import "strings"

// Транслитерация поисковых запросов между кириллицей и латиницей (#2414).
//
// Отличается и от SwitchLayout, и от FixLatinInName, хотя все трое ходят рядом:
//   - SwitchLayout - забытый переключатель раскладки: «траттория» по клавишам даёт
//     «nhfnnjhbz», то есть тот же набор клавиш в другом языке;
//   - FixLatinInName - омоглифы: латинская «a» вместо русской «а», буквы визуально
//     одинаковые;
//   - здесь - ЗВУЧАНИЕ: «траттория» и «trattoria» пишутся по-разному, а читаются
//     одинаково. Без этого человек, знающий заведение как «Траттория», не находил
//     запись «La Trattoria» вовсе.
//
// Написание неоднозначно, поэтому вариантов несколько. Схемы фиксированы и их мало:
// декартово произведение по каждой спорной букве дало бы сотни строк на слово, а
// каждая строка - это ещё одно условие в запросе, помноженное на число колонок.

// cyrToLatMain - основная схема: ГОСТ-подобная, самая частая в документах.
var cyrToLatMain = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh",
	'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o",
	'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "kh", 'ц': "ts",
	'ч': "ch", 'ш': "sh", 'щ': "shch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e",
	'ю': "yu", 'я': "ya",
}

// cyrToLatAlt - вторая схема: как пишут в вывесках и на визитках. Отличия ровно в
// спорных буквах, ради которых всё и заводилось: «Траттория» на вывеске - Trattoria,
// а не Trattoriya.
var cyrToLatAlt = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "j",
	'з': "z", 'и': "i", 'й': "i", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o",
	'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "c",
	'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e",
	'ю': "u", 'я': "a",
}

// latToCyrPhonetic - обратный разбор. Многобуквенные сочетания идут первыми: иначе
// «sh» разберётся как «с»+«х», и «Sergey» не встретит «Сергей».
var latToCyrDigraphs = []struct {
	lat string
	cyr string
}{
	{"shch", "щ"}, {"sch", "щ"}, {"zh", "ж"}, {"kh", "х"}, {"ch", "ч"}, {"sh", "ш"},
	{"ts", "ц"}, {"yu", "ю"}, {"ya", "я"}, {"ia", "я"}, {"ye", "е"}, {"yo", "ё"},
	{"ey", "ей"}, {"ay", "ай"}, {"oy", "ой"},
}

var latToCyrSingle = map[rune]string{
	'a': "а", 'b': "б", 'c': "к", 'd': "д", 'e': "е", 'f': "ф", 'g': "г", 'h': "х",
	'i': "и", 'j': "й", 'k': "к", 'l': "л", 'm': "м", 'n': "н", 'o': "о", 'p': "п",
	'q': "к", 'r': "р", 's': "с", 't': "т", 'u': "у", 'v': "в", 'w': "в", 'x': "кс",
	'y': "й", 'z': "з",
}

// translitMinLen - короче этого транслитерировать вредно: трёхбуквенный фрагмент после
// перевода в другой алфавит попадает в случайные подстроки. Живой пример из теста поиска:
// «рга» превращается в «rga», а это кусок слова «Organization» в подписи любой записи
// тестовой организации. Тот же порог, что у нечёткого сравнения (searchFuzzyMinWordLen):
// короткие фрагменты ищутся точным вхождением, и этого достаточно.
const translitMinLen = 4

// Translit возвращает варианты записи строки на другом алфавите - без оригинала и без
// пустых значений. Кириллица разбирается по двум схемам, латиница по одной; к каждому
// варианту добавляется версия без удвоенных согласных («tratoria» к «trattoria»),
// потому что удвоение теряют чаще всего.
//
// Порядок устойчив: сначала основная схема, потом альтернативная, потом версии без
// удвоений. Короткий ввод вариантов не даёт вовсе - см. translitMinLen.
func Translit(s string) []string {
	lowered := strings.ToLower(strings.TrimSpace(s))
	if len([]rune(lowered)) < translitMinLen {
		return nil
	}

	out := make([]string, 0, 4)
	seen := map[string]bool{lowered: true}
	add := func(v string) {
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		out = append(out, v)
	}

	if hasCyrillic(lowered) {
		add(mapRunes(lowered, cyrToLatMain))
		add(mapRunes(lowered, cyrToLatAlt))
	}
	if hasLatin(lowered) {
		add(latToCyr(lowered))
	}

	// Версии без удвоений - для каждого уже собранного варианта и для самого запроса.
	for _, v := range append([]string{lowered}, out...) {
		add(collapseDoubles(v))
	}
	return out
}

func hasCyrillic(s string) bool {
	for _, r := range s {
		if r >= 'а' && r <= 'я' || r == 'ё' {
			return true
		}
	}
	return false
}

func hasLatin(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return true
		}
	}
	return false
}

func mapRunes(s string, table map[rune]string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if v, ok := table[r]; ok {
			b.WriteString(v)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func latToCyr(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		matched := false
		for _, d := range latToCyrDigraphs {
			if strings.HasPrefix(s[i:], d.lat) {
				b.WriteString(d.cyr)
				i += len(d.lat)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		r := rune(s[i])
		if v, ok := latToCyrSingle[r]; ok {
			b.WriteString(v)
		} else {
			b.WriteByte(s[i])
		}
		i++
	}
	return b.String()
}

// collapseDoubles схлопывает подряд идущие одинаковые буквы: «trattoria» -> «tratoria».
// Удвоение теряют и в кириллице («Аллея» -> «Алея»), поэтому применяется к обоим алфавитам.
func collapseDoubles(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	var prev rune
	for i, r := range s {
		if i > 0 && r == prev {
			continue
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}
