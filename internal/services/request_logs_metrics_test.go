package services

import (
	"strings"
	"testing"
)

// Длительности показываются в миллисекундах с одним знаком: ответ за 300 микросекунд
// не должен выглядеть нулевым, иначе перцентили журнала снова выродятся в ноль (#2125).
func TestUsToMs(t *testing.T) {
	cases := []struct {
		name string
		us   float64
		want float64
	}{
		{"быстрее миллисекунды", 300, 0.3},
		{"ровно миллисекунда", 1000, 1},
		{"обычный ответ", 147_400, 147.4},
		{"округление вверх", 149_960, 150},
		{"нет данных", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := usToMs(tc.us); got != tc.want {
				t.Fatalf("usToMs(%v): ожидали %v, получили %v", tc.us, tc.want, got)
			}
		})
	}
}

// Долгоживущие соединения не участвуют в расчёте длительностей: у них в журнале
// записано время жизни подписки, а не время ответа.
func TestNotStreamingSQL(t *testing.T) {
	got := notStreamingSQL()
	for _, path := range streamingLogPaths {
		if !strings.Contains(got, "'"+path+"'") {
			t.Fatalf("условие не отсекает %s: %s", path, got)
		}
	}
	// Отсечение идёт по пути без query: подписка приходит с одноразовым билетом
	// в адресе, и сравнение с целым URL никогда бы не сработало.
	if !strings.Contains(got, "split_part") {
		t.Fatalf("сравнение должно идти по пути без query: %s", got)
	}
	// Разделитель задан через chr(63): знак вопроса в кавычках gorm принимает за
	// место подстановки и разъезжается на аргументах запроса.
	if !strings.Contains(got, "chr(63)") {
		t.Fatalf("разделитель должен задаваться через chr(63): %s", got)
	}
}
