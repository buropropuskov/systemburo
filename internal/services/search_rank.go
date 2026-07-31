package services

import (
	"strings"

	"systemburo/internal/normalize"
)

// Ступени релевантности внутри группы. Считаются в Go по уже отобранным строкам:
// сортировать в SQL по similarity значило бы вычислять её на каждой строке матча,
// тогда как строк здесь не больше limit+1 и пересортировка бесплатна.
const (
	scoreExact     = 1.0
	scorePrefix    = 0.9
	scoreWordStart = 0.8
	scoreContains  = 0.6
	scoreNoMatch   = 0.3
)

// scoreMatch оценивает, насколько точно строка отвечает запросу. Сравнение идёт по
// normalize.Name: он приводит регистр, схлопывает пробелы и чинит омоглифы, поэтому
// "РОГОЛЕВ" и "Роголев" дают одну ступень, а латинская "o" в кириллической фамилии не
// роняет точное совпадение до подстроки.
func scoreMatch(value, raw string) float64 {
	v := normalize.Name(value)
	q := normalize.Name(raw)
	if v == "" || q == "" {
		return scoreNoMatch
	}

	switch {
	case v == q:
		return scoreExact
	case strings.HasPrefix(v, q):
		return scorePrefix
	case strings.Contains(v, " "+q):
		return scoreWordStart
	case strings.Contains(v, q):
		return scoreContains
	default:
		// Совпадение пришло из другого варианта запроса (раскладка, госномер) или из
		// колонки, которой нет в Title -- строка найдена по делу, просто заголовок
		// сам по себе запросу не отвечает.
		return scoreNoMatch
	}
}

// rankItems проставляет score, помечает подсвечиваемую строку и сортирует группу по
// убыванию релевантности. Сортировка устойчивая: при равном score сохраняется порядок,
// заданный SQL (свежие записи первыми), иначе выдача дёргалась бы между одинаковыми
// совпадениями.
func rankItems(items []SearchItem, raw string) {
	for i := range items {
		items[i].Score = scoreMatch(items[i].Title, raw)
		items[i].MatchedField = matchedFieldTitle
		// Совпало не в заголовке -- значит строка пришла из подзаголовка (организация,
		// марка, должность). Фронту это нужно, чтобы подсветить ту строку, где реально
		// есть вхождение, а не мигать заголовком без совпадения.
		if items[i].Score <= scoreNoMatch && items[i].Subtitle != "" {
			items[i].MatchedField = matchedFieldSubtitle
		}
	}
	// Вставками: элементов не больше limit+1, а сортировка стабильна без аллокаций.
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].Score > items[j-1].Score; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// hasExactMatch сообщает, есть ли в группе точное совпадение с запросом. Такая группа
// поднимается на первое место: человек, набравший госномер целиком, ждёт машину сверху,
// а не на четвёртой позиции. Это единственное отступление от фиксированного порядка
// разделов -- одно правило, объяснимое и проверяемое тестом.
func hasExactMatch(g SearchGroup) bool {
	for _, it := range g.Items {
		if it.Score >= scoreExact {
			return true
		}
	}
	return false
}
