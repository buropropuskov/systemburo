package blankpath

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// componentCases - корпус для Component. Он же прогоняется тестом идемпотентности,
// поэтому новые случаи достаточно дописать сюда.
var componentCases = []struct {
	name string
	in   string
	want string
}{
	{"обычное имя", "Заявка на работы - Мегобари", "Заявка на работы - Мегобари"},

	// Номер заявки в системе выглядит как "№ 20260731/001": слэш внутри значения
	// разделителем пути стать не должен.
	{"слэш из номера заявки", "№ 20260731/001", "№ 20260731-001"},
	{"обратный слэш", `ООО\Филиал`, "ООО-Филиал"},
	{"двоеточие", "Заявка: срочно", "Заявка. срочно"},

	{"кавычки", `ООО "Ромашка"`, "ООО Ромашка"},
	{"ёлочки", "ООО «Ромашка»", "ООО Ромашка"},
	{"подстановочные знаки", "отчёт*?<>|", "отчёт_____"},

	// Windows молча отбрасывает точку и пробел в конце имени. Без обрезки путь в
	// базе навсегда разошёлся бы с тем, что реально лежит на диске.
	{"хвостовая точка и пробел", "Заявка. ", "Заявка"},
	{"хвостовые точки", "Иванов И.И.", "Иванов И.И"},

	{"имя устройства", "CON", "_CON"},
	{"имя устройства с расширением", "com1.xlsx", "_com1.xlsx"},
	{"имя устройства строчными", "nul", "_nul"},
	{"надстрочный вариант", "COM¹", "_COM¹"},
	{"похожее, но не устройство", "CONTRACT", "CONTRACT"},

	{"точка", ".", "_"},
	{"две точки", "..", "_"},
	{"попытка выйти вверх", "../../etc", "..-..-etc"},

	{"управляющие символы", "A\x00B\x1fC", "ABC"},
	{"переопределение направления письма", "файл\u202egpj.xlsx", "файлgpj.xlsx"},
	{"неразрывный пробел", "ООО\u00a0Ромашка", "ООО Ромашка"},
	{"повторяющиеся пробелы", "ООО    Ромашка", "ООО Ромашка"},

	{"пусто", "", ""},
	{"только пробелы", "   ", ""},
	{"только кавычки", `"""`, ""},
}

func TestComponent(t *testing.T) {
	t.Parallel()
	for _, tc := range componentCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, Component(tc.in))
		})
	}
}

// TestComponent_Idempotent - гейт против зацикливания переименований: сервис
// каждый прогон сравнивает фактический путь с желаемым, и если очистка уже
// очищенного имени даёт другой результат, папка будет переезжать вечно.
func TestComponent_Idempotent(t *testing.T) {
	t.Parallel()
	for _, tc := range componentCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			once := Component(tc.in)
			assert.Equal(t, once, Component(once), "повторная очистка изменила результат")
		})
	}
}

func TestComponent_LimitIsBytesNotRunes(t *testing.T) {
	t.Parallel()

	// 200 кириллических букв - это 400 байт: предел ext4 наступает вдвое раньше,
	// чем можно решить по числу символов.
	got := Component(strings.Repeat("я", 200))

	assert.LessOrEqual(t, len(got), MaxComponentBytes)
	assert.True(t, utf8.ValidString(got), "усечение разрезало руну пополам")
	assert.Equal(t, 127, utf8.RuneCountInString(got))
}

func TestComponentOr(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Мегобари", ComponentOr("Мегобари", FallbackName))
	assert.Equal(t, FallbackName, ComponentOr("   ", FallbackName))
}

func TestFileName(t *testing.T) {
	t.Parallel()

	t.Run("обычное имя", func(t *testing.T) {
		assert.Equal(t, "Заявка на работы - Мегобари.xlsx",
			FileName("Заявка на работы - Мегобари", ".xlsx"))
	})

	t.Run("расширение неприкосновенно", func(t *testing.T) {
		got := FileName(strings.Repeat("a", 300), ".xlsx")
		assert.Len(t, got, MaxComponentBytes)
		assert.True(t, strings.HasSuffix(got, ".xlsx"))
	})

	t.Run("кириллица режется по границе руны", func(t *testing.T) {
		got := FileName(strings.Repeat("я", 300), ".xlsx")
		assert.LessOrEqual(t, len(got), MaxComponentBytes)
		assert.True(t, utf8.ValidString(got))
		assert.True(t, strings.HasSuffix(got, ".xlsx"))
	})

	t.Run("пустая база заменяется запасным именем", func(t *testing.T) {
		assert.Equal(t, FallbackName+".xlsx", FileName("   ", ".xlsx"))
	})

	t.Run("имя устройства", func(t *testing.T) {
		assert.Equal(t, "_PRN.xlsx", FileName("PRN", ".xlsx"))
	})
}

func TestJoinUnder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	t.Run("собирает путь под корнем", func(t *testing.T) {
		got, err := JoinUnder(root, "2026", "7 ИЮЛЬ 2026", "бланк.xlsx")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(root, "2026", "7 ИЮЛЬ 2026", "бланк.xlsx"), got)
	})

	t.Run("не выпускает за корень", func(t *testing.T) {
		_, err := JoinUnder(root, "..", "..", "etc")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escapes archive root")
	})

	// filepath.Join трактует абсолютный компонент как относительный, поэтому путь
	// не подменяется, а достраивается внутри корня. Ошибки здесь нет и не должно
	// быть - важно, что запись остаётся в архиве.
	t.Run("абсолютный уровень достраивается внутрь корня", func(t *testing.T) {
		got, err := JoinUnder(root, "/etc/passwd")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(root, "etc", "passwd"), got)
	})

	t.Run("сам корень допустим", func(t *testing.T) {
		got, err := JoinUnder(root)
		require.NoError(t, err)
		assert.Equal(t, filepath.Clean(root), got)
	})
}

func TestMonthName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "июль", MonthName(7))
	assert.Equal(t, "ИЮЛЬ", MonthNameUpper(7))
	assert.Equal(t, "", MonthName(0))
	assert.Equal(t, "", MonthName(13))
}
