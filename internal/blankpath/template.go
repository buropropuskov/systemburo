package blankpath

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	// DefaultDirTemplate повторяет раскладку, к которой бюро привыкло в почте:
	// год, месяц словом, день, папка заявки.
	DefaultDirTemplate = "{год}/{месяц_число} {МЕСЯЦ} {год}/{дата}/{дата} №{номер} {организация}"

	// DefaultFileTemplate - имя бланка внутри папки заявки. Тип вложения живёт
	// здесь, а не в имени папки: вложений у заявки несколько и типы у них разные.
	DefaultFileTemplate = "{тип} - {организация}"

	// separatorRunes - символы, которыми в шаблоне склеивают плейсхолдеры. Такой
	// литерал исчезает вместе с опустевшим соседом: иначе "{дата} №{номер}" при
	// пустом номере оставил бы висячее "№".
	separatorRunes = " \t-–—_,;.:·№#"

	// edgeTrimRunes - подмножество separatorRunes, которое убирается ещё и с краёв
	// уровня. Знак номера и решётка сюда не входят намеренно: "№{номер}" начинают
	// с них осмысленно, и краевая обрезка съела бы знак у заполненного номера.
	edgeTrimRunes = " \t-–—_,;.:·"

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

type segKind uint8

const (
	// segValue - подставленное значение плейсхолдера. Не редактируется: организация
	// "Ромашка-Строй-" обязана дойти до имени папки со своим дефисом.
	segValue segKind = iota
	// segSeparator - разделительный литерал шаблона. Единственное, что пакет вправе
	// выбросить.
	segSeparator
	// segText - содержательный литерал шаблона.
	segText
)

type segment struct {
	kind segKind
	text string
	keep bool
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

// renderLevel подставляет значения в один уровень (или в имя файла).
//
// Работает по сегментам, а не по склеенной строке: обрезка разделителей на готовом
// тексте не отличала бы литерал шаблона от хвоста самого значения и молча срезала
// бы дефис у организации "Ромашка-Строй-".
func renderLevel(tmpl string, v Values) string {
	segs := buildSegments(parse(tmpl), v)
	dropSeparatorsAround(segs)
	trimEdgeSeparators(segs)

	var b strings.Builder
	for _, s := range segs {
		if s.keep {
			b.WriteString(s.text)
		}
	}
	return collapseSpaces(b.String())
}

// buildSegments подставляет значения плейсхолдеров и раскладывает литералы на
// разделители и содержательный текст.
func buildSegments(parts []part, v Values) []segment {
	segs := make([]segment, 0, len(parts)*2)
	for _, p := range parts {
		if p.kind == partToken {
			// Неизвестный ключ даёт пустое значение и схлопывается вместе с
			// разделителем: до рендера шаблон уже прошёл Validate, а ронять
			// запись бланка из-за опечатки в настройке нельзя.
			value, _ := v.lookup(p.text)
			value = strings.TrimSpace(value)
			segs = append(segs, segment{kind: segValue, text: value, keep: value != ""})
			continue
		}
		segs = append(segs, splitLiteral(p.text)...)
	}
	return segs
}

// splitLiteral режет литерал на ведущий разделитель, содержательную часть и
// хвостовой разделитель. Литерал целиком из разделителей даёт один сегмент.
func splitLiteral(s string) []segment {
	lead := leadingRun(s, separatorRunes)
	rest := s[len(lead):]
	trail := trailingRun(rest, separatorRunes)
	core := rest[:len(rest)-len(trail)]

	segs := make([]segment, 0, 3)
	if lead != "" {
		segs = append(segs, segment{kind: segSeparator, text: lead, keep: true})
	}
	if core != "" {
		segs = append(segs, segment{kind: segText, text: core, keep: true})
	}
	if trail != "" {
		segs = append(segs, segment{kind: segSeparator, text: trail, keep: true})
	}
	return segs
}

// dropSeparatorsAround убирает у каждого пустого плейсхолдера ОДИН примыкающий
// разделитель - предшествующий, а если его нет или он уже убран, то следующий.
// Убирать оба нельзя: соседние значения склеились бы в "31.07.2026Мегобари".
func dropSeparatorsAround(segs []segment) {
	for i := range segs {
		if segs[i].kind != segValue || segs[i].keep {
			continue
		}
		if j := i - 1; j >= 0 && segs[j].kind == segSeparator && segs[j].keep {
			segs[j].keep = false
			continue
		}
		if j := i + 1; j < len(segs) && segs[j].kind == segSeparator && segs[j].keep {
			segs[j].keep = false
		}
	}
}

// trimEdgeSeparators убирает разделители, оставшиеся по краям уровня: шаблон
// "{организация} -" не должен давать папку "Мегобари -". Знак номера исключён из
// edgeTrimRunes, поэтому "№{номер}" при заполненном номере знак сохраняет.
func trimEdgeSeparators(segs []segment) {
	for i := 0; i < len(segs); i++ {
		if !segs[i].keep {
			continue
		}
		if segs[i].kind != segSeparator || !isEdgeTrimmable(segs[i].text) {
			break
		}
		segs[i].keep = false
	}
	for i := len(segs) - 1; i >= 0; i-- {
		if !segs[i].keep {
			continue
		}
		if segs[i].kind != segSeparator || !isEdgeTrimmable(segs[i].text) {
			break
		}
		segs[i].keep = false
	}
}

// parse разбивает шаблон на литералы и плейсхолдеры. Незакрытая скобка, вложенные
// скобки и пустое "{}" остаются обычным текстом.
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

// isEdgeTrimmable - сегмент состоит только из символов, которые можно убрать с края.
func isEdgeTrimmable(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune(edgeTrimRunes, r) {
			return false
		}
	}
	return true
}

// leadingRun возвращает начальный отрезок строки из символов набора set.
func leadingRun(s, set string) string {
	for i, r := range s {
		if !strings.ContainsRune(set, r) {
			return s[:i]
		}
	}
	return s
}

// trailingRun возвращает конечный отрезок строки из символов набора set.
func trailingRun(s, set string) string {
	for i := len(s); i > 0; {
		r, size := utf8.DecodeLastRuneInString(s[:i])
		if !strings.ContainsRune(set, r) {
			return s[i:]
		}
		i -= size
	}
	return s
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
