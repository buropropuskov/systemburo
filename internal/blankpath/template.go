package blankpath

import (
	"fmt"
	"strings"
)

const (
	// DefaultDirTemplate повторяет раскладку, к которой бюро привыкло в почте:
	// год, месяц словом, день, папка заявки.
	DefaultDirTemplate = "{год}/{месяц_число} {МЕСЯЦ} {год}/{дата}/{дата} №{номер} {организация}"

	// DefaultFileTemplate - имя бланка внутри папки заявки. Тип вложения живёт
	// здесь, а не в имени папки: вложений у заявки несколько и типы у них разные.
	DefaultFileTemplate = "{тип} - {организация}"

	// separatorRunes - символы, которые в шаблоне служат только склейкой между
	// плейсхолдерами. Литерал, целиком состоящий из них, исчезает вместе с пустым
	// соседом: иначе "{дата} №{номер}" при пустом номере оставил бы висячее "№".
	separatorRunes = " \t-–—_,;.·№#"

	// minDeepestLevelBytes - ниже этого предела самый глубокий уровень не режется:
	// путь короче, но нечитаемая папка из четырёх букв хуже длинного пути.
	minDeepestLevelBytes = 24
)

// Problem - претензия к шаблону. Отдаётся интерфейсу, чтобы подсветить конкретный
// плейсхолдер, а не сообщать "шаблон неверен" целиком.
type Problem struct {
	Token  string `json:"token"`
	Reason string `json:"reason"`
}

type partKind uint8

const (
	partLiteral partKind = iota
	partToken
)

// part - кусок разобранного шаблона: либо текст как есть, либо ключ плейсхолдера.
type part struct {
	kind partKind
	text string
}

// Check возвращает список претензий к шаблону: неизвестные плейсхолдеры и те, что
// не имеют смысла в данном контексте. Пустой список - шаблон пригоден.
func Check(tmpl string, scope Scope) []Problem {
	var out []Problem
	seen := make(map[string]struct{})

	for _, p := range parse(tmpl) {
		if p.kind != partToken {
			continue
		}
		if _, dup := seen[p.text]; dup {
			continue
		}
		seen[p.text] = struct{}{}

		t, known := TokenByKey(p.text)
		switch {
		case !known:
			out = append(out, Problem{Token: p.text, Reason: "неизвестный плейсхолдер"})
		case t.Scope&scope == 0:
			out = append(out, Problem{Token: p.text, Reason: outOfScopeReason(scope)})
		}
	}
	return out
}

// Validate проверяет шаблон перед сохранением настройки. Возвращает ошибку с
// перечнем проблемных плейсхолдеров - её текст уходит пользователю как есть.
func Validate(tmpl string, scope Scope) error {
	if strings.TrimSpace(tmpl) == "" {
		return fmt.Errorf("шаблон не может быть пустым")
	}
	if scope&ScopeDir != 0 && countLevels(tmpl) == 0 {
		return fmt.Errorf("шаблон каталогов не содержит ни одного уровня")
	}

	problems := Check(tmpl, scope)
	if len(problems) == 0 {
		return nil
	}

	msgs := make([]string, 0, len(problems))
	for _, p := range problems {
		msgs = append(msgs, fmt.Sprintf("{%s} - %s", p.Token, p.Reason))
	}
	return fmt.Errorf("ошибка в шаблоне: %s", strings.Join(msgs, "; "))
}

// RenderPath раскладывает шаблон каталогов в уровни пути. Уровень, от которого
// после подстановки ничего не осталось, получает запасное имя: выбросить его
// нельзя, иначе заявка поднимется на уровень выше и перемешается с чужими.
func RenderPath(tmpl string, v Values) []string {
	raw := strings.Split(tmpl, "/")
	levels := make([]string, 0, len(raw))
	for _, level := range raw {
		if strings.TrimSpace(level) == "" {
			continue
		}
		levels = append(levels, ComponentOr(renderLevel(level, v), FallbackName))
	}
	return levels
}

// RenderName собирает имя файла бланка; ext передаётся с точкой (".xlsx").
func RenderName(tmpl string, v Values, ext string) string {
	return FileName(renderLevel(tmpl, v), ext)
}

// FitRelPath укорачивает самый глубокий уровень, чтобы относительный путь
// (уровни плюс имя файла) уложился в maxBytes. Верхние уровни не трогает: они
// общие для многих заявок, и обрезка развалила бы группировку.
//
// Если самый глубокий уровень уже упёрся в minDeepestLevelBytes, путь остаётся
// длиннее предела - это осознанный компромисс в пользу читаемости.
func FitRelPath(levels []string, fileName string, maxBytes int) []string {
	if maxBytes <= 0 || len(levels) == 0 {
		return levels
	}

	out := append([]string(nil), levels...)
	last := len(out) - 1

	excess := relLen(out, fileName) - maxBytes
	if excess <= 0 {
		return out
	}

	target := len(out[last]) - excess
	if target < minDeepestLevelBytes {
		target = minDeepestLevelBytes
	}
	if target >= len(out[last]) {
		return out
	}

	out[last] = trimTrailingDotsSpaces(truncateBytes(out[last], target))
	if out[last] == "" {
		out[last] = FallbackName
	}
	return out
}

// renderLevel подставляет значения в один уровень (или в имя файла) и схлопывает
// разделители вокруг пустых плейсхолдеров.
func renderLevel(tmpl string, v Values) string {
	parts := parse(tmpl)

	text := make([]string, len(parts))
	keep := make([]bool, len(parts))
	sepOnly := make([]bool, len(parts))

	for i, p := range parts {
		if p.kind == partToken {
			// Неизвестный ключ даёт пустое значение и схлопывается вместе с
			// разделителем: до рендера шаблон уже прошёл Validate, а ронять
			// запись бланка из-за опечатки в настройке нельзя.
			value, _ := v.lookup(p.text)
			text[i] = strings.TrimSpace(value)
			keep[i] = text[i] != ""
			continue
		}
		text[i] = p.text
		sepOnly[i] = isSeparatorOnly(p.text)
		keep[i] = true
	}

	dropSeparatorsAround(parts, keep, sepOnly)

	var b strings.Builder
	for i := range parts {
		if keep[i] {
			b.WriteString(text[i])
		}
	}
	// Обрезаем разделители только справа. Слева нельзя: "№{номер}" начинается со
	// знака номера намеренно, и общая обрезка съела бы его вместе с висячими
	// хвостами. Пробелы по краям уже снял collapseSpaces.
	return strings.TrimRight(collapseSpaces(b.String()), separatorRunes)
}

// dropSeparatorsAround убирает у каждого пустого плейсхолдера ОДИН примыкающий
// разделитель - предшествующий, а если его нет или он уже убран, то следующий.
// Убирать оба нельзя: соседние значения склеились бы в "31.07.2026Мегобари".
func dropSeparatorsAround(parts []part, keep, sepOnly []bool) {
	for i, p := range parts {
		if p.kind != partToken || keep[i] {
			continue
		}
		if j := i - 1; j >= 0 && sepOnly[j] && keep[j] {
			keep[j] = false
			continue
		}
		if j := i + 1; j < len(parts) && sepOnly[j] && keep[j] {
			keep[j] = false
		}
	}
}

// parse разбивает шаблон на литералы и плейсхолдеры. Незакрытая скобка и пустое
// "{}" остаются обычным текстом.
func parse(s string) []part {
	var out []part
	var lit strings.Builder

	flush := func() {
		if lit.Len() > 0 {
			out = append(out, part{kind: partLiteral, text: lit.String()})
			lit.Reset()
		}
	}

	for i := 0; i < len(s); {
		if s[i] == '{' {
			if j := strings.IndexByte(s[i+1:], '}'); j >= 0 {
				key := s[i+1 : i+1+j]
				if key != "" && !strings.Contains(key, "{") {
					flush()
					out = append(out, part{kind: partToken, text: key})
					i += j + 2
					continue
				}
			}
		}
		lit.WriteByte(s[i])
		i++
	}

	flush()
	return out
}

func isSeparatorOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune(separatorRunes, r) {
			return false
		}
	}
	return true
}

func countLevels(tmpl string) int {
	n := 0
	for _, level := range strings.Split(tmpl, "/") {
		if strings.TrimSpace(level) != "" {
			n++
		}
	}
	return n
}

func relLen(levels []string, fileName string) int {
	n := len(fileName)
	for _, l := range levels {
		n += len(l) + 1
	}
	return n
}

func outOfScopeReason(scope Scope) string {
	if scope&ScopeDir != 0 {
		return "недопустим в имени папки: значение принадлежит вложению, а папка - заявке"
	}
	return "недопустим в имени файла"
}
