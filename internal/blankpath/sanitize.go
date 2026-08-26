package blankpath

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	// MaxComponentBytes - предел на один уровень пути. Меряем в БАЙТАХ: ext4
	// считает именно их, и кириллическое имя упирается в предел вдвое раньше
	// латинского. NTFS считает единицы UTF-16, то есть мягче, - ограничение ext4
	// строже и покрывает оба.
	MaxComponentBytes = 255

	// FallbackName подставляется вместо уровня, от которого после очистки ничего
	// не осталось. Выбрасывать уровень нельзя: заявка поднялась бы на уровень выше
	// и перемешалась с чужими.
	FallbackName = "без-названия"
)

// replacer приводит символы, запрещённые в именах файлов Windows, к читаемому виду.
// Одним подчёркиванием на всё заменять нельзя: "ООО _Ромашка_ 15_08_2026" оператор
// прочитать не сможет, а разделители пути и двоеточие имеют естественные замены.
var replacer = strings.NewReplacer(
	"/", "-",
	"\\", "-",
	":", ".",
	`"`, "",
	"«", "",
	"»", "",
	"*", "_",
	"?", "_",
	"<", "_",
	">", "_",
	"|", "_",
)

// dosReserved - имена устройств DOS. Windows отказывается создавать файл с таким
// именем до первой точки, поэтому к ним приписывается подчёркивание. Надстрочные
// варианты COM¹ и LPT¹ резервируются наравне с обычными.
var dosReserved = func() map[string]struct{} {
	m := map[string]struct{}{
		"CON": {}, "PRN": {}, "AUX": {}, "NUL": {}, "CLOCK$": {},
	}
	for _, suffix := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³"} {
		m["COM"+suffix] = struct{}{}
		m["LPT"+suffix] = struct{}{}
	}
	return m
}()

// Component приводит ОДИН уровень пути к безопасному виду. Возвращает пустую
// строку, если после очистки ничего не осталось: подстановку FallbackName делает
// вызывающий, чтобы функция оставалась чистой и идемпотентной.
//
// Порядок правил важен и проверяется тестами; в частности, обрезка хвостовых точек
// идёт до и после усечения по длине: Windows молча отбрасывает точку в конце имени,
// и без обрезки путь в базе навсегда разошёлся бы с тем, что лежит на диске.
//
// Инвариант: Component(Component(x)) == Component(x). Без него сверка фактического
// пути с желаемым зациклила бы переименования.
func Component(s string) string {
	s = norm.NFC.String(s)
	s = stripControl(s)
	s = replacer.Replace(s)
	s = collapseSpaces(s)

	if s == "." || s == ".." {
		return "_"
	}

	s = trimTrailingDotsSpaces(s)
	s = escapeReserved(s)
	s = truncateBytes(s, MaxComponentBytes)
	return trimTrailingDotsSpaces(s)
}

// ComponentOr - Component с подстановкой запасного имени вместо пустого результата.
func ComponentOr(s, fallback string) string {
	if c := Component(s); c != "" {
		return c
	}
	return fallback
}

// FileName собирает имя файла из очищенной базы и расширения (ext с точкой).
// База усекается так, чтобы имя целиком уложилось в MaxComponentBytes: расширение
// неприкосновенно, иначе файл перестанет открываться по двойному щелчку.
func FileName(base, ext string) string {
	base = Component(base)

	limit := MaxComponentBytes - len(ext)
	if limit < 1 {
		// Расширение само по себе длиннее предела - вырожденный случай, отдаём
		// его как есть: усекать расширение хуже, чем превысить лимит.
		return ext
	}

	base = truncateBytes(base, limit)
	base = trimTrailingDotsSpaces(base)
	if base == "" {
		base = truncateBytes(FallbackName, limit)
	}
	return base + ext
}

// JoinUnder собирает абсолютный путь из корня и уровней и проверяет, что результат
// не вышел за корень. Второй рубеж после Component: даже если уровень каким-то
// образом принесёт "..", запись мимо архива не состоится.
func JoinUnder(root string, parts ...string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("failed to resolve archive root %q: %w", root, err)
	}
	rootAbs = filepath.Clean(rootAbs)

	full := filepath.Clean(filepath.Join(append([]string{rootAbs}, parts...)...))
	if full != rootAbs && !strings.HasPrefix(full, rootAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes archive root %q", full, rootAbs)
	}
	return full, nil
}

// stripControl убирает управляющие символы и переопределение направления письма.
// Bidi-override - классический приём маскировки расширения: символ U+202E
// разворачивает хвост имени, и "файл<U+202E>gpj.exe" показывается пользователю
// как "файлexe.jpg". Сам символ в исходник не вставляем: он и здесь остался бы
// невидимым и вводил бы в заблуждение при чтении кода.
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r < 0x20, r == 0x7F:
			return -1
		case r >= 0x202A && r <= 0x202E:
			return -1
		case r >= 0x2066 && r <= 0x2069:
			return -1
		}
		return r
	}, s)
}

// collapseSpaces сводит любые пробельные последовательности (включая неразрывный
// пробел) к одному обычному пробелу и обрезает края.
func collapseSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// trimTrailingDotsSpaces срезает точки и пробелы в конце имени: Windows отбрасывает
// их при создании файла, и без обрезки фактическое имя разойдётся с ожидаемым.
func trimTrailingDotsSpaces(s string) string {
	return strings.TrimRight(s, ". ")
}

// escapeReserved приписывает подчёркивание к именам устройств DOS.
func escapeReserved(s string) string {
	base := s
	if i := strings.IndexByte(s, '.'); i >= 0 {
		base = s[:i]
	}
	if _, reserved := dosReserved[strings.ToUpper(base)]; reserved {
		return "_" + s
	}
	return s
}

// truncateBytes режет строку до max байт по границе руны.
func truncateBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	b := s[:max]
	for len(b) > 0 && !utf8.ValidString(b) {
		b = b[:len(b)-1]
	}
	return b
}
