package services

import (
	"testing"

	"systemburo/internal/models"
)

// Реальные строки User-Agent, а не выдуманные - взяты из типовых логов запросов
// соответствующих браузеров/устройств.
const (
	uaIPhoneSafari  = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	uaIPadClassic   = "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	uaAndroidChrome = "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36"
	uaAndroidTablet = "Mozilla/5.0 (Linux; Android 12; SM-X706B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	uaWindowsChrome = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	uaMacSafari     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15"
	uaLinuxFirefox  = "Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0"
	uaChromeOS      = "Mozilla/5.0 (X11; CrOS x86_64 15633.69.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"
	uaCurlBot       = "curl/7.68.0"
)

func TestDetectPlatform(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ua   string
		want string
	}{
		{"iPhone Safari", uaIPhoneSafari, PlatformIOS},
		// Ловушка: строка содержит "like Mac OS X" - проверка на iPad обязана
		// сработать раньше generic-проверки на Mac, иначе он уедет в desktop.
		{"iPad классический UA (содержит iPad)", uaIPadClassic, PlatformIOS},
		{"Android телефон, Chrome", uaAndroidChrome, PlatformAndroid},
		{"Android планшет, Chrome", uaAndroidTablet, PlatformAndroid},
		{"Windows, Chrome", uaWindowsChrome, PlatformDesktop},
		{"настоящий Mac, Safari", uaMacSafari, PlatformDesktop},
		{"Linux, Firefox", uaLinuxFirefox, PlatformDesktop},
		{"ChromeOS", uaChromeOS, PlatformDesktop},
		{"бот без признаков платформы", uaCurlBot, PlatformUnknown},
		{"пустая строка", "", PlatformUnknown},
		{"только пробелы", "   ", PlatformUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := DetectPlatform(c.ua); got != c.want {
				t.Errorf("DetectPlatform(%q) = %q, ожидалось %q", c.ua, got, c.want)
			}
		})
	}
}

// TestDetectPlatform_IPadNeverDesktop - отдельный явный тест на условие из задачи:
// iPad не должен попадать в настольные ни при каких обстоятельствах, для UA, где он
// явно назван (см. ограничение метода в комментарии DetectPlatform насчёт iPadOS 13+
// в режиме "сайта для компьютера", где строка становится неотличима от Mac).
func TestDetectPlatform_IPadNeverDesktop(t *testing.T) {
	t.Parallel()
	got := DetectPlatform(uaIPadClassic)
	if got == PlatformDesktop {
		t.Fatalf("iPad не должен классифицироваться как desktop, получено %q", got)
	}
	if got != PlatformIOS {
		t.Errorf("ожидался ios, получено %q", got)
	}
}

func TestAddPlatformCount(t *testing.T) {
	t.Parallel()
	var counts models.PushPlatformCounts
	addPlatformCount(&counts, PlatformIOS)
	addPlatformCount(&counts, PlatformIOS)
	addPlatformCount(&counts, PlatformAndroid)
	addPlatformCount(&counts, PlatformDesktop)
	addPlatformCount(&counts, PlatformUnknown)
	addPlatformCount(&counts, "что-то незнакомое")

	if counts.IOS != 2 {
		t.Errorf("ios: ожидалось 2, получено %d", counts.IOS)
	}
	if counts.Android != 1 {
		t.Errorf("android: ожидалось 1, получено %d", counts.Android)
	}
	if counts.Desktop != 1 {
		t.Errorf("desktop: ожидалось 1, получено %d", counts.Desktop)
	}
	if counts.Unknown != 2 {
		t.Errorf("unknown: ожидалось 2 (unknown + незнакомое значение), получено %d", counts.Unknown)
	}
}
