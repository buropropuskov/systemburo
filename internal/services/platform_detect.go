package services

import (
	"strings"

	"systemburo/internal/models"
)

// Грубая классификация платформы по строке User-Agent (#974), для сводки использования
// Web Push - ios (iPhone и iPad) обязан push-приложение на экран "Домой" на iOS, поэтому
// разрыв между "людей на iOS много" и "подписок с iOS мало" - ключевая цифра для решения,
// нужен ли запасной канал доставки.

const (
	PlatformIOS     = "ios"
	PlatformAndroid = "android"
	PlatformDesktop = "desktop"
	PlatformUnknown = "unknown"
)

// DetectPlatform классифицирует строку User-Agent в одну из четырёх групп. Несколько
// проверок подстрок, без внешних библиотек - распознавание ВСЕГДА приблизительное,
// цифры сводки ориентир, а не точность до устройства.
//
// Известная ловушка: строка iPad почти всегда содержит "like Mac OS X" (реальный пример -
// "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) ... Mobile/15E148 Safari/604.1"), поэтому
// наивная проверка "содержит Mac -> desktop", выполненная раньше проверки на iPad,
// ошибочно уводит iPad в настольные. iPhone/iPad проверяются здесь ПЕРВЫМИ, до generic
// macOS/Windows/Linux - порядок проверок и есть защита от ловушки.
//
// Отдельная, более тяжёлая западня остаётся принципиально нерешаемой по одной строке
// User-Agent: начиная с iPadOS 13 Safari по умолчанию отправляет "режим сайта для
// компьютера" и подделывает себя под настоящий Mac побайтово ("Macintosh; Intel Mac OS X
// 10_15_7) ... Safari/605.1.15", без единого "iPad"/"Mobile" в строке). Различить такой
// iPad и настоящий Mac можно только на клиенте через navigator.maxTouchPoints - этой
// информации в User-Agent нет и здесь она не восстановима. Такие iPad неизбежно попадают
// в desktop; сводка это не пытается скрыть, а комментарий фиксирует границу метода.
func DetectPlatform(userAgent string) string {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return PlatformUnknown
	}
	if strings.Contains(ua, "ipad") || strings.Contains(ua, "iphone") {
		return PlatformIOS
	}
	if strings.Contains(ua, "android") {
		return PlatformAndroid
	}
	if strings.Contains(ua, "macintosh") || strings.Contains(ua, "windows") ||
		strings.Contains(ua, "linux") || strings.Contains(ua, "cros") {
		return PlatformDesktop
	}
	return PlatformUnknown
}

// addPlatformCount увеличивает счётчик группы, вычисленной DetectPlatform, - общая точка
// для подписок (push_service.go) и последних входов (push_summary.go), чтобы правило
// "что считать unknown" не разъезжалось между двумя разрезами сводки.
func addPlatformCount(counts *models.PushPlatformCounts, platform string) {
	switch platform {
	case PlatformIOS:
		counts.IOS++
	case PlatformAndroid:
		counts.Android++
	case PlatformDesktop:
		counts.Desktop++
	default:
		counts.Unknown++
	}
}
