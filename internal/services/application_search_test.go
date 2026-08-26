package services

import (
	"strings"
	"testing"
)

func TestBuildSearchVariants(t *testing.T) {
	t.Run("включает оригинал и переключённую раскладку", func(t *testing.T) {
		variants := buildSearchVariants("ghbdtn")
		if !sliceContains(variants, "ghbdtn") {
			t.Errorf("нет оригинала в %v", variants)
		}
		if !sliceContains(variants, "привет") {
			t.Errorf("нет варианта раскладки 'привет' в %v", variants)
		}
	})

	t.Run("дедуп: одинаковые варианты схлопываются", func(t *testing.T) {
		// для "123" оригинал == раскладка == номер, должен остаться один вариант
		variants := buildSearchVariants("123")
		if len(variants) != 1 {
			t.Errorf("ожидали 1 вариант для '123', получили %v", variants)
		}
	})

	t.Run("пустые варианты отбрасываются", func(t *testing.T) {
		for _, v := range buildSearchVariants("иванов") {
			if strings.TrimSpace(v) == "" {
				t.Errorf("пустой вариант в наборе")
			}
		}
	})
}

func TestIlikePatternsArgs(t *testing.T) {
	cond, args := ilikePatternsArgs([]string{"a.x", "b.y"}, []string{"foo", "bar"})

	if got := strings.Count(cond, "ILIKE ?"); got != 4 {
		t.Errorf("ожидали 4 ILIKE-условия (2 колонки x 2 варианта), получили %d в %q", got, cond)
	}
	if len(args) != 4 {
		t.Fatalf("ожидали 4 аргумента, получили %d: %v", len(args), args)
	}
	if args[0] != "%foo%" || args[1] != "%bar%" {
		t.Errorf("аргументы обёрнуты неверно: %v", args)
	}
	if !strings.Contains(cond, " OR ") {
		t.Errorf("условия не объединены через OR: %q", cond)
	}
}

func sliceContains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}
