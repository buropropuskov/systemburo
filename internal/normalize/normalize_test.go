package normalize

import "testing"

func TestName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{"базовое ФИО", []string{"Иванов", "Иван", "Иванович"}, "иванов иван иванович"},
		{"регистр и пробелы", []string{"  ИВАНОВ ", "иван", ""}, "иванов иван"},
		{"ё в е", []string{"Алёшин", "Пётр", ""}, "алешин петр"},
		{"латинские омоглифы в кириллицу", []string{"Ивaнов", "Иван", ""}, "иванов иван"},
		{"смешанная латиница c o p", []string{"Сoрoкин", "Пётр", ""}, "сорокин петр"},
		{"без отчества короче", []string{"Иванов", "Иван", ""}, "иванов иван"},
		{"схлопывание двойных пробелов", []string{"Иванов  Иван"}, "иванов иван"},
		{"пустой ввод", []string{"", "", ""}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Name(tt.parts...); got != tt.want {
				t.Errorf("Name(%q) = %q, want %q", tt.parts, got, tt.want)
			}
		})
	}
}

func TestNameHomoglyphMatchesCyrillic(t *testing.T) {
	t.Parallel()
	// Обход латиницей даёт ту же каноническую форму, что и оригинал кириллицей.
	cyr := Name("Иванов", "Иван", "Иванович")
	lat := Name("Ivanov", "Иван", "Иванович") // I, v не омоглифы - проверяем частичный случай
	if cyr == lat {
		t.Fatalf("ожидалось расхождение на неомоглифных буквах I/v, но %q == %q", cyr, lat)
	}
	// А вот точечная подмена о/о (лат o) должна схлопнуться.
	if a, b := Name("Сорокин"), Name("Сoрoкин"); a != b {
		t.Errorf("подмена лат o не схлопнулась: %q != %q", a, b)
	}
}

func TestFixLatinInName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		in        string
		wantFixed string
		wantLatin bool
	}{
		{"чистая кириллица без изменений", "Иванов", "Иванов", false},
		{"омоглиф в середине сохраняет регистр", "Ивaнов", "Иванов", true},
		{"омоглиф в начале слова заглавный", "Aнна", "Анна", true},
		{"несколько омоглифов", "Пeтpов", "Петров", true},
		{"лишние пробелы схлопываются без латиницы", "Иванов   Иван", "Иванов Иван", false},
		{"неомоглифная латиница не трогается", "Fizli", "Fizli", false},
		{"пустая строка", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fixed, latin := FixLatinInName(tt.in)
			if fixed != tt.wantFixed || latin != tt.wantLatin {
				t.Errorf("FixLatinInName(%q) = (%q, %v), want (%q, %v)", tt.in, fixed, latin, tt.wantFixed, tt.wantLatin)
			}
		})
	}
}

func TestFixLatinInNameIdempotent(t *testing.T) {
	t.Parallel()
	// Повторное применение к уже исправленной строке не меняет её и не находит латиницу.
	for _, in := range []string{"Ивaнов", "Пeтpов", "Сoрoкин", "Иванов"} {
		once, _ := FixLatinInName(in)
		twice, latinAgain := FixLatinInName(once)
		if once != twice {
			t.Errorf("не идемпотентно: %q -> %q -> %q", in, once, twice)
		}
		if latinAgain {
			t.Errorf("повторный проход снова нашёл латиницу в уже исправленной строке %q", once)
		}
	}
}

func TestPlate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		number string
		want   string
	}{
		{"кириллический номер", "О123ОО", "О123ОО"},
		{"нижний регистр", "о123оо", "О123ОО"},
		{"латинские омоглифы", "O123OO", "О123ОО"},
		{"ноль как буква О", "0123ОО", "О123ОО"},
		{"пробелы и дефисы", "О 123 ОО - 77", "О123ОО77"},
		{"полный латинский ABEKMHOPCTYX", "A123BE", "А123ВЕ"},
		{"регион сохраняется", "А123ВЕ199", "А123ВЕ199"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Plate(tt.number); got != tt.want {
				t.Errorf("Plate(%q) = %q, want %q", tt.number, got, tt.want)
			}
		})
	}
}

func TestPlateHomoglyphEvasionCollapses(t *testing.T) {
	t.Parallel()
	// Все варианты обхода одного номера дают одну каноническую форму.
	canon := Plate("О123ОО77")
	for _, evasion := range []string{"O123OO77", "0123ОО77", "о123оо77", "О 123 ОО 77"} {
		if got := Plate(evasion); got != canon {
			t.Errorf("вариант обхода %q -> %q, ожидалось %q", evasion, got, canon)
		}
	}
}
