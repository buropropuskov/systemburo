package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"
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

// Окно графика считается от границы интервала и накрывает ровно запрошенное число
// точек. Без окна запрос группировал журнал целиком и выбрасывал лишнее уже после
// чтения: на стенде это последовательное чтение тридцати восьми партиций каждые
// полминуты ради двух десятков чисел (#2125).
func TestTimelineWindow(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 34, 56, 0, time.UTC)

	cases := []struct {
		name string
		q    models.TimelineQuery
		want time.Time
	}{
		{"минута по секундам", models.TimelineQuery{Interval: 1, Limit: 60},
			time.Date(2026, 8, 18, 12, 33, 57, 0, time.UTC)},
		{"час по минутам", models.TimelineQuery{Interval: 60, Limit: 60},
			time.Date(2026, 8, 18, 11, 35, 0, 0, time.UTC)},
		{"сутки по часам", models.TimelineQuery{Interval: 3600, Limit: 24},
			time.Date(2026, 8, 17, 13, 0, 0, 0, time.UTC)},
		{"конец периода задан вызовом", models.TimelineQuery{Interval: 3600, Limit: 24, To: "2026-08-10T00:00:00Z"},
			time.Date(2026, 8, 9, 1, 0, 0, 0, time.UTC)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := timelineWindow(tc.q, now)
			if !got.Equal(tc.want) {
				t.Fatalf("окно: ожидали %s, получили %s", tc.want, got)
			}
			if tc.q.To == "" && got.After(now) {
				t.Fatalf("окно не накрывает текущий момент: %s > %s", got, now)
			}
		})
	}
}

// Снимок показателей живёт заданный срок и не переживает отказ базы.
func TestStatsCache(t *testing.T) {
	base := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

	newCompute := func(calls *int, err error) func(context.Context) (*models.RequestLogsStats, error) {
		return func(context.Context) (*models.RequestLogsStats, error) {
			*calls++
			if err != nil {
				return nil, err
			}
			return &models.RequestLogsStats{Total: int64(*calls)}, nil
		}
	}

	t.Run("без срока жизни считается каждый раз", func(t *testing.T) {
		calls := 0
		c := &statsCache{ttl: 0}
		for i := 0; i < 3; i++ {
			if _, err := c.get(context.Background(), base, newCompute(&calls, nil)); err != nil {
				t.Fatalf("неожиданная ошибка: %v", err)
			}
		}
		if calls != 3 {
			t.Fatalf("ожидали три расчёта, получили %d", calls)
		}
	})

	t.Run("выключенный кэш не мешает расчёту", func(t *testing.T) {
		calls := 0
		var c *statsCache
		if _, err := c.get(context.Background(), base, newCompute(&calls, nil)); err != nil {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
		if calls != 1 {
			t.Fatalf("ожидали один расчёт, получили %d", calls)
		}
	})

	t.Run("свежий снимок отдаётся без расчёта, протухший пересчитывается", func(t *testing.T) {
		calls := 0
		compute := newCompute(&calls, nil)
		c := &statsCache{ttl: time.Minute}

		first, err := c.get(context.Background(), base, compute)
		if err != nil {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
		second, err := c.get(context.Background(), base.Add(59*time.Second), compute)
		if err != nil {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
		if calls != 1 || second.Total != first.Total {
			t.Fatalf("свежий снимок пересчитали: расчётов %d, значения %v и %v", calls, first.Total, second.Total)
		}

		if _, err = c.get(context.Background(), base.Add(time.Minute), compute); err != nil {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
		if calls != 2 {
			t.Fatalf("протухший снимок не пересчитали: расчётов %d", calls)
		}
	})

	t.Run("отказ базы не запоминается", func(t *testing.T) {
		calls := 0
		c := &statsCache{ttl: time.Minute}

		if _, err := c.get(context.Background(), base, newCompute(&calls, errors.New("база недоступна"))); err == nil {
			t.Fatal("ошибка расчёта обязана дойти до вызова")
		}
		if _, err := c.get(context.Background(), base, newCompute(&calls, nil)); err != nil {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
		if calls != 2 {
			t.Fatalf("после отказа расчёт не повторили: расчётов %d", calls)
		}
	})

	t.Run("снимок отдаётся копией", func(t *testing.T) {
		calls := 0
		compute := newCompute(&calls, nil)
		c := &statsCache{ttl: time.Minute}

		first, err := c.get(context.Background(), base, compute)
		if err != nil {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
		first.Total = 999

		second, err := c.get(context.Background(), base, compute)
		if err != nil {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
		if second.Total == 999 {
			t.Fatal("правка ответа испортила общий снимок")
		}
	})
}
