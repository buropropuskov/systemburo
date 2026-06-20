package normalize

import "testing"

func TestSwitchLayout(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"латиница в кириллицу (набрал не переключив раскладку)", "ghbdtn", "привет"},
		{"кириллица в латиницу", "привет", "ghbdtn"},
		{"фамилия латиницей в кириллицу", "bdfyjd", "иванов"},
		{"цифры и неизвестные символы без изменений", "123-45", "123-45"},
		{"пустая строка", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SwitchLayout(tt.in); got != tt.want {
				t.Errorf("SwitchLayout(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSwitchLayoutRoundTrip(t *testing.T) {
	for _, s := range []string{"иванов", "петров", "сидоров"} {
		if got := SwitchLayout(SwitchLayout(s)); got != s {
			t.Errorf("round-trip SwitchLayout(SwitchLayout(%q)) = %q, want %q", s, got, s)
		}
	}
}
