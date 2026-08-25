package blankpath

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleValues() Values {
	return Values{
		Date:           time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC),
		Number:         "№ 20260731/001",
		ApplicationID:  4821,
		Sender:         "Иванов И.И.",
		Initiator:      "Петров П.П.",
		Status:         "Завершено",
		Confirmation:   "Согласовано",
		Organization:   "Мегобари",
		Company:        "ООО СтройГрупп",
		AttachmentType: "Заявка на работы",
		Period:         "01.08.2026 - 05.08.2026",
		AttachmentID:   9134,
	}
}

func TestRenderPath_DefaultTemplate(t *testing.T) {
	t.Parallel()

	got := RenderPath(DefaultDirTemplate, sampleValues())

	assert.Equal(t, []string{
		"2026",
		"7 ИЮЛЬ 2026",
		"31.07.2026",
		"31.07.2026 №20260731-001 Мегобари",
	}, got)
}

func TestRenderName_DefaultTemplate(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Заявка на работы - Мегобари.xlsx",
		RenderName(DefaultFileTemplate, sampleValues(), ".xlsx"))
}

// TestRenderPath_SlashInValueStaysInsideLevel - слэш разделяет уровни только в
// шаблоне. Номер заявки в системе выглядит как "№ 20260731/001", и если бы значение
// участвовало в разбиении, каждая заявка порождала бы лишний уровень каталога.
func TestRenderPath_SlashInValueStaysInsideLevel(t *testing.T) {
	t.Parallel()

	got := RenderPath("{номер}", sampleValues())

	require.Len(t, got, 1)
	assert.Equal(t, "20260731-001", got[0])
}

// TestRenderPath_EmptyTokensCollapse - главный риск шаблонизатора: у заявки без
// компании или без номера имя папки не должно превращаться в "Мегобари  Иванов"
// с двойным пробелом или в "31.07.2026 № Мегобари" с висячим знаком номера.
func TestRenderPath_EmptyTokensCollapse(t *testing.T) {
	t.Parallel()

	const tmpl = "{дата} №{номер} {организация} {компания} {заявитель}"

	cases := []struct {
		name   string
		mutate func(*Values)
		want   string
	}{
		{
			name:   "все значения на месте",
			mutate: func(*Values) {},
			want:   "31.07.2026 №20260731-001 Мегобари ООО СтройГрупп Иванов И.И",
		},
		{
			name:   "нет компании",
			mutate: func(v *Values) { v.Company = "" },
			want:   "31.07.2026 №20260731-001 Мегобари Иванов И.И",
		},
		{
			name:   "нет организации",
			mutate: func(v *Values) { v.Organization = "" },
			want:   "31.07.2026 №20260731-001 ООО СтройГрупп Иванов И.И",
		},
		{
			name:   "нет номера - знак номера уходит вместе с ним",
			mutate: func(v *Values) { v.Number = "" },
			want:   "31.07.2026 Мегобари ООО СтройГрупп Иванов И.И",
		},
		{
			name:   "нет заявителя - хвостовой разделитель не остаётся",
			mutate: func(v *Values) { v.Sender = "" },
			want:   "31.07.2026 №20260731-001 Мегобари ООО СтройГрупп",
		},
		{
			name: "нет ни организации, ни компании",
			mutate: func(v *Values) {
				v.Organization = ""
				v.Company = ""
			},
			want: "31.07.2026 №20260731-001 Иванов И.И",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v := sampleValues()
			tc.mutate(&v)

			got := RenderPath(tmpl, v)
			require.Len(t, got, 1)
			assert.Equal(t, tc.want, got[0])
			assert.NotContains(t, got[0], "  ", "остался двойной пробел")
			assert.NotContains(t, got[0], "№ ", "знак номера повис без значения")
		})
	}
}

// TestRenderPath_NumberSignNotDoubled - номер в базе хранится вместе со знаком
// ("№ 20260731/001"), а в шаблоне знак обычно пишут руками. Без нормализации
// получалось бы "№№ 20260731-001".
func TestRenderPath_NumberSignNotDoubled(t *testing.T) {
	t.Parallel()

	got := RenderPath("№{номер}", sampleValues())

	require.Len(t, got, 1)
	assert.Equal(t, "№20260731-001", got[0])
}

// TestRenderPath_AllTokensEmptyKeepsLevel - уровень, от которого ничего не осталось,
// заменяется запасным именем. Выбросить его нельзя: заявка поднялась бы на уровень
// выше и перемешалась с чужими.
func TestRenderPath_AllTokensEmptyKeepsLevel(t *testing.T) {
	t.Parallel()

	got := RenderPath("{год}/№{номер} {организация} {компания}", Values{
		Date: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
	})

	assert.Equal(t, []string{"2026", FallbackName}, got)
}

func TestRenderPath_UnknownTokenCollapses(t *testing.T) {
	t.Parallel()

	// До рендера шаблон уже прошёл Validate, но опечатка в настройке не должна
	// ронять запись бланка: неизвестный ключ ведёт себя как пустое значение.
	got := RenderPath("{дата} {фирма} {организация}", sampleValues())

	require.Len(t, got, 1)
	assert.Equal(t, "31.07.2026 Мегобари", got[0])
}

func TestRenderName_EmptyTemplateFallsBack(t *testing.T) {
	t.Parallel()

	assert.Equal(t, FallbackName+".xlsx", RenderName("{компания}", Values{}, ".xlsx"))
}

func TestValidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		tmpl    string
		scope   Scope
		wantErr string
	}{
		{"шаблон каталогов по умолчанию", DefaultDirTemplate, ScopeDir, ""},
		{"шаблон имени по умолчанию", DefaultFileTemplate, ScopeFile, ""},
		{"текст без плейсхолдеров", "Заявки бюро", ScopeDir, ""},
		{"пустой шаблон", "   ", ScopeDir, "не может быть пустым"},
		{"шаблон каталогов без уровней", "//", ScopeDir, "ни одного уровня"},
		{"неизвестный плейсхолдер", "{дата} {фирма}", ScopeDir, "{фирма} - неизвестный плейсхолдер"},
		{"тип вложения в имени папки", "{дата} {тип}", ScopeDir, "{тип} - недопустим в имени папки"},
		{"тип вложения в имени файла", "{тип}", ScopeFile, ""},
		{"период вложения в имени папки", "{период}", ScopeDir, "{период} -"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := Validate(tc.tmpl, tc.scope)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestCheck_ReportsEachTokenOnce(t *testing.T) {
	t.Parallel()

	problems := Check("{фирма} {фирма} {тип}", ScopeDir)

	require.Len(t, problems, 2)
	assert.Equal(t, "фирма", problems[0].Token)
	assert.Equal(t, "тип", problems[1].Token)
}

// TestRenderPath_ValueTailIsNotTrimmed - обрезка разделителей относится только к
// литералам шаблона. Организация "Ромашка-Строй-" обязана дойти до имени папки со
// своим дефисом: молча срезанный хвост значения потом не отличить от опечатки.
func TestRenderPath_ValueTailIsNotTrimmed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		tmpl string
		v    Values
		want string
	}{
		{"дефис в конце названия", "{организация}", Values{Organization: "Ромашка-Строй-"}, "Ромашка-Строй-"},
		{"точка с запятой в конце", "{статус}", Values{Status: "Завершено;"}, "Завершено;"},
		{"дефис внутри значения", "{организация}", Values{Organization: "Ромашка - Строй"}, "Ромашка - Строй"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RenderPath(tc.tmpl, tc.v)
			require.Len(t, got, 1)
			assert.Equal(t, tc.want, got[0])
		})
	}
}

// TestRenderPath_MixedLiteralCollapses - литерал, где разделитель слеплен с текстом
// (", ООО"), должен терять у пустого соседа только разделительную часть. Иначе
// оператор увидит в проводнике папку, начинающуюся с запятой.
func TestRenderPath_MixedLiteralCollapses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		tmpl string
		want string
	}{
		{"запятая перед текстом", "{компания}, ООО", "ООО"},
		{"знак номера и двоеточие", "№{номер}: файл", "файл"},
		{"текст перед пустым значением", "Заявка {номер}", "Заявка"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RenderPath(tc.tmpl, Values{Date: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)})
			require.Len(t, got, 1)
			assert.Equal(t, tc.want, got[0])
		})
	}
}

// TestRenderPath_EdgeSeparatorsTrimmedBothSides - висячий разделитель убирается с
// обоих краёв уровня. Знак номера исключение: "№{номер}" начинают с него осмысленно.
func TestRenderPath_EdgeSeparatorsTrimmedBothSides(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		tmpl string
		want string
	}{
		{"разделитель слева", " - {организация}", "Мегобари"},
		{"разделитель справа", "{организация} -", "Мегобари"},
		{"разделитель с обеих сторон", "_ {организация} _", "Мегобари"},
		{"знак номера сохраняется", "№{номер}", "№20260731-001"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RenderPath(tc.tmpl, sampleValues())
			require.Len(t, got, 1)
			assert.Equal(t, tc.want, got[0])
		})
	}
}

func TestParse_MalformedBracesStayLiteral(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		tmpl string
		want string
	}{
		{"незакрытая скобка", "{дата} {незакрытый", "31.07.2026 {незакрытый"},
		{"вложенные скобки", "{{номер}}", "{20260731-001}"},
		{"пустой плейсхолдер", "заявка {}", "заявка {}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RenderPath(tc.tmpl, sampleValues())
			require.Len(t, got, 1)
			assert.Equal(t, tc.want, got[0])
		})
	}
}

func TestFitRelPath(t *testing.T) {
	t.Parallel()

	base := []string{"2026", "7 ИЮЛЬ 2026", "31.07.2026"}
	deep := strings.Repeat("a", 100)
	const fileName = "b.xlsx"

	t.Run("укладывается - не трогаем", func(t *testing.T) {
		t.Parallel()
		levels := append(append([]string{}, base...), deep)
		assert.Equal(t, levels, FitRelPath(levels, fileName, 1000))
	})

	t.Run("режется только самый глубокий уровень", func(t *testing.T) {
		t.Parallel()
		levels := append(append([]string{}, base...), deep)

		got := FitRelPath(levels, fileName, 100)

		assert.Equal(t, base, got[:3], "верхние уровни общие для многих заявок и резаться не должны")
		assert.Equal(t, 100, relLen(got, fileName))
	})

	t.Run("ниже минимальной длины не режем", func(t *testing.T) {
		t.Parallel()
		levels := append(append([]string{}, base...), deep)

		got := FitRelPath(levels, fileName, 30)

		assert.Len(t, got[3], minDeepestLevelBytes)
		assert.Greater(t, relLen(got, fileName), 30, "осознанный компромисс в пользу читаемости")
	})

	t.Run("кириллица режется по границе руны", func(t *testing.T) {
		t.Parallel()
		levels := append(append([]string{}, base...), strings.Repeat("я", 100))

		got := FitRelPath(levels, fileName, 90)

		assert.True(t, utf8.ValidString(got[3]))
	})

	t.Run("пустых уровней не создаёт", func(t *testing.T) {
		t.Parallel()
		got := FitRelPath([]string{"2026", "ab"}, fileName, 1)
		assert.NotEmpty(t, got[1])
	})
}

func TestTokens_ScopeFlags(t *testing.T) {
	t.Parallel()

	byKey := make(map[string]Token)
	for _, tk := range Tokens() {
		byKey[tk.Key] = tk
	}

	require.Contains(t, byKey, "дата")
	assert.True(t, byKey["дата"].DirAllowed)
	assert.True(t, byKey["дата"].FileAllowed)

	require.Contains(t, byKey, "тип")
	assert.False(t, byKey["тип"].DirAllowed, "тип принадлежит вложению, а папка - заявке")
	assert.True(t, byKey["тип"].FileAllowed)

	_, ok := TokenByKey("несуществующий")
	assert.False(t, ok)
}
