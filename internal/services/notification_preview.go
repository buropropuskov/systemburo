package services

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Текст уведомления собирается строками, а не одной фразой (#974): в развёрнутом
// системном уведомлении и в окне подробностей переносы видны (там white-space: pre-wrap),
// и человек читает «что случилось / от кого / о чём» по отдельности вместо сплошного
// предложения. Форматирования богаче переносов в системном уведомлении не существует:
// Notification API принимает только заголовок и текст, ни разметки, ни списков.

// notificationPreviewLimit -- сколько знаков сообщения заявки показывать. Это именно
// намёк на содержание, а не пересказ: сообщение целиком человек прочтёт, открыв заявку,
// а в уведомлении длинный текст оттесняет вниз всё остальное.
const notificationPreviewLimit = 40

// notificationFilesLimit -- предел строки с именами вложений. Имена бывают длиннее
// самого уведомления («Накладная на привоз мебели от 08.08.2026 склад 3.pdf»), и без
// предела одна такая строка съедала бы всю видимую часть.
const notificationFilesLimit = 60

var (
	htmlTagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
	htmlBlockRe = regexp.MustCompile(`(?i)</(p|div|li|tr|h[1-6])>|<br\s*/?>`)
	spacesRe    = regexp.MustCompile(`[ \t]+`)
	newlinesRe  = regexp.MustCompile(`\n{2,}`)
)

// plainTextFromRichText превращает сообщение заявки в строку для уведомления. Поле
// хранит размеченный текст из редактора, и вставить его в уведомление как есть нельзя:
// человек увидел бы теги. Границы абзацев и переносы сохраняются пробелом, иначе слова
// соседних абзацев слипаются в одно.
func plainTextFromRichText(s string) string {
	if s == "" {
		return ""
	}
	s = htmlBlockRe.ReplaceAllString(s, "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	s = newlinesRe.ReplaceAllString(s, "\n")
	s = spacesRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// previewText обрезает текст по границе слова и ставит многоточие. Обрезка по рунам, а
// не по байтам: кириллица весит по два байта, и обрезка по индексу байта разрубила бы
// букву пополам.
func previewText(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	cut := string(runes[:limit])
	if i := strings.LastIndex(cut, " "); i > limit/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:-") + "..."
}

// filesLabel перечисляет вложения по именам: человек по названию понимает, накладная
// это или паспорт машины, тогда как «вложено 2 файла» не говорит ничего. Список
// обрезается целиком, а не по каждому имени: обрезанные с двух сторон названия
// читаются хуже, чем честное «и ещё» в конце. Без вложений строки нет вовсе.
func filesLabel(names []string) string {
	var clean []string
	for _, n := range names {
		if n = strings.TrimSpace(n); n != "" {
			clean = append(clean, n)
		}
	}
	if len(clean) == 0 {
		return ""
	}
	word := "Файлы"
	if len(clean) == 1 {
		word = "Файл"
	}
	return word + ": " + previewText(strings.Join(clean, ", "), notificationFilesLimit)
}
