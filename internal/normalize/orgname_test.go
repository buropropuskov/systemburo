package normalize

import "testing"

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
