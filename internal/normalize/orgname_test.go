package normalize

import (
	"strings"
	"testing"
)

// TestOrgNameCollapsesWritings - написания одного юрлица дают один ключ. Это ядро
// дедупликации: пара «ООО "Петрушка"» / «ооо петрушка» и есть тот случай, ради
// которого ключ вводится (#1437).
func TestOrgNameCollapsesWritings(t *testing.T) {
	t.Parallel()

	groups := []struct {
		name     string
		writings []string
	}{
		{
			name: "кавычки и регистр",
			writings: []string{
				`ООО "Петрушка"`,
				`ООО «Петрушка»`,
				`ООО Петрушка`,
				`ооо петрушка`,
				`  ООО   "Петрушка"  `,
			},
		},
		{
			name: "полная форма и аббревиатура",
			writings: []string{
				`Общество с ограниченной ответственностью "Петрушка"`,
				`О.О.О. Петрушка`,
				`ООО "Петрушка"`,
			},
		},
		{
			name: "закрытое акционерное общество не распадается на ао",
			writings: []string{
				`Закрытое акционерное общество "Ромашка"`,
				`ЗАО "Ромашка"`,
				`зао ромашка`,
			},
		},
		{
			name: "дефис с пробелами и без",
			writings: []string{
				`ООО "Ромашка-Строй"`,
				`ООО «Ромашка - Строй»`,
				`ООО Ромашка — Строй`,
			},
		},
		{
			name: "ё и латинские омоглифы",
			writings: []string{
				`ООО "Артём"`,
				`ООО "Артем"`,
				`ООО "Аpтем"`, // p латинская
			},
		},
	}

	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			t.Parallel()
			want := OrgName(g.writings[0])
			if want == "" {
				t.Fatalf("нормализация %q дала пустой ключ", g.writings[0])
			}
			for _, w := range g.writings[1:] {
				if got := OrgName(w); got != want {
					t.Errorf("OrgName(%q) = %q, ожидался тот же ключ, что у %q: %q", w, got, g.writings[0], want)
				}
			}
		})
	}
}

// TestOrgNameKeepsDistinct - разные юрлица остаются разными. Пересхлопывание опаснее
// недосхлопывания: две несвязанные организации под одним ключом сольются в одну запись,
// и заявки уедут не туда.
func TestOrgNameKeepsDistinct(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{
		{`ООО "Ромашка"`, `ООО "Ромашка-Строй"`},
		{`ИП Петров`, `Петров`},
		{`ООО "Ромашка"`, `ЗАО "Ромашка"`},
		{`ООО "Петрушка"`, `ООО "Петрушкин"`},
		{`ООО "Строй"`, `ООО "Строй 2"`},
	}

	for _, p := range pairs {
		if OrgName(p[0]) == OrgName(p[1]) {
			t.Errorf("OrgName(%q) и OrgName(%q) совпали (%q), а это разные организации", p[0], p[1], OrgName(p[0]))
		}
	}
}

// TestOrgNameExact фиксирует конкретный вид ключа: он уезжает в базу и в SQL-запрос
// поиска, поэтому молчаливая смена формы сломает совпадение с уже сохранёнными строками.
func TestOrgNameExact(t *testing.T) {
	t.Parallel()

	cases := []struct{ in, want string }{
		{`ООО "Петрушка"`, "ооо петрушка"},
		{`Общество с ограниченной ответственностью «Ромашка-Строй»`, "ооо ромашка-строй"},
		{`  ИП   Петров  `, "ип петров"},
		{`ПАО "Сбербанк"`, "пао сбербанк"},
		{``, ""},
		{`   `, ""},
	}

	for _, c := range cases {
		if got := OrgName(c.in); got != c.want {
			t.Errorf("OrgName(%q) = %q, ожидалось %q", c.in, got, c.want)
		}
	}
}

// TestOrgNameCore - ядро для подсказок: ОПФ снята, смысловая часть на месте. Именно по
// ядру ищется «максима», когда в справочнике лежит «ООО "Максима Групп"» (#1437).
func TestOrgNameCore(t *testing.T) {
	t.Parallel()

	cases := []struct{ in, want string }{
		{`ООО "Максима Групп"`, "максима групп"},
		{`Общество с ограниченной ответственностью «Победа»`, "победа"},
		{`ЧОП "АРЕС"`, "арес"},
		{`ИП Иванов И.И.`, "иванов ии"},
		{`АО "Регионы-Энтертейнмент"`, "регионы-энтертейнмент"},
		// ОПФ в середине наименования тоже служебная.
		{`Торговый дом ООО Ромашка`, "торговый дом ромашка"},
		// Наименование без ОПФ ядром себе и является.
		{`Технический департамент`, "технический департамент"},
		// «м-н» и «р-н» - не ОПФ, вырезать их нельзя: без них останется одно слово.
		{`м-н Летуаль`, "м-н летуаль"},
		// ОПФ, приклеенная дефисом, - часть наименования: отдельным словом её не писали.
		{`ИП-Сервис`, "ип-сервис"},
		{``, ""},
	}

	for _, c := range cases {
		if got := OrgNameCore(c.in); got != c.want {
			t.Errorf("OrgNameCore(%q) = %q, ожидалось %q", c.in, got, c.want)
		}
	}
}

// TestOrgNameCoreKeepsBareLegalForm - у наименования из одной ОПФ ядром остаётся сам ключ.
// Пустое ядро выкинуло бы такую запись из подсказок и из сравнения целиком.
func TestOrgNameCoreKeepsBareLegalForm(t *testing.T) {
	t.Parallel()

	for _, in := range []string{`ООО`, `ЗАО`, `Общество с ограниченной ответственностью`} {
		if got, want := OrgNameCore(in), OrgName(in); got != want {
			t.Errorf("OrgNameCore(%q) = %q, ожидался ключ %q", in, got, want)
		}
	}
}

// TestOrgLegalFormPatternCoversTokens - в SQL-паттерн попадают все токены ОПФ. Ядро
// записи снимает Postgres этим паттерном, ядро запроса - OrgNameCore: пропущенный токен
// развёл бы их. Эквивалентность на реальном движке (границы слова \m и \M в Go-regexp
// не воспроизводятся - там \b только для ASCII) проверяет TestDirectorySuggest.
func TestOrgLegalFormPatternCoversTokens(t *testing.T) {
	t.Parallel()

	pattern := OrgLegalFormPattern()
	for token := range orgLegalFormTokens {
		if !strings.Contains(pattern, token+"|") && !strings.Contains(pattern, token+")") {
			t.Errorf("токен ОПФ %q не попал в SQL-паттерн %q", token, pattern)
		}
	}
}

// TestOrgNameIdempotent - повторная нормализация ничего не меняет. Свойство нужно
// бэкфиллу: он прогоняется при каждом старте, и второй проход не должен смещать ключ.
func TestOrgNameIdempotent(t *testing.T) {
	t.Parallel()

	inputs := []string{
		`ООО "Петрушка"`,
		`Общество с ограниченной ответственностью "Ромашка - Строй"`,
		`ЗАО «Артём»`,
		`ип петров`,
	}

	for _, in := range inputs {
		once := OrgName(in)
		if twice := OrgName(once); twice != once {
			t.Errorf("OrgName не идемпотентна: %q -> %q -> %q", in, once, twice)
		}
	}
}

// Наименование без букв и цифр записи справочника не даёт: из него не выйдет ни ключа
// дедупликации, ни осмысленного имени. Проверка живёт отдельно от OrgName, потому что тот
// выбрасывает кавычки и точки, но оставляет дефисы - «---» приходит с непустым ключом.
func TestOrgNameMeaningless(t *testing.T) {
	meaningless := []string{
		"", "   ", `"`, `""`, "--", "---", "- - -", "...", "!!!", `«»`, "?!", " ", "—", "–",
		`" "`, ".,;:", "***",
	}
	for _, in := range meaningless {
		if !OrgNameMeaningless(in) {
			t.Errorf("OrgNameMeaningless(%q) = false, а содержания в нём нет", in)
		}
	}

	meaningful := []string{
		"ООО Ромашка", `ооо "братишк`, "585", "ООО 585 Золото", "м-н Летуаль", "ОРППиПдБиРОП",
		"Acme Ltd", "北京公司", "ИП Иванов И.И.", "А", "1", "ООО \"ЭФКО-ЦР\"", "-1-",
	}
	for _, in := range meaningful {
		if OrgNameMeaningless(in) {
			t.Errorf("OrgNameMeaningless(%q) = true, хотя буква или цифра в нём есть", in)
		}
	}
}
