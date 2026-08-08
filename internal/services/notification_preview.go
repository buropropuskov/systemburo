package services

import (
	"fmt"
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

// notificationPreviewLimit -- сколько знаков сообщения заявки показывать. Больше не
// имеет смысла: в шторке телефона видно две-три строки, остальное человек прочтёт,
// открыв заявку.
const notificationPreviewLimit = 160

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

// fileCountLabel -- «Файлов: 3» с правильным словом. Ноль файлов строкой не показывается
// вовсе: «Файлов: 0» ничего не сообщает, только занимает строку в шторке.
func fileCountLabel(n int) string {
	if n <= 0 {
		return ""
	}
	word := "файлов"
	switch {
	case n%100 >= 11 && n%100 <= 14:
	case n%10 == 1:
		word = "файл"
	case n%10 >= 2 && n%10 <= 4:
		word = "файла"
	}
	return fmt.Sprintf("Вложено %d %s", n, word)
}
