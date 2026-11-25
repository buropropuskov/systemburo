package services

import (
	"os"
	"strings"
	"testing"
)

// Сроки заявок считаются по московским часам (#2327).
//
// Предикаты срока используют текущее время БД, поэтому проверяем выражение
// независимо от часа запуска. Нумерацию проверяет TestApplicationNumberCountedInMoscow.

// sourceOf читает файл пакета целиком.
func sourceOf(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("не прочитать %s: %v", name, err)
	}
	return string(data)
}

// goFilesOfPackage - все .go пакета services, кроме тестов.
func goFilesOfPackage(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("не прочитать каталог пакета: %v", err)
	}
	var files []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			files = append(files, n)
		}
	}
	return files
}

func TestNoCurrentDateInPassPredicates(t *testing.T) {
	// CURRENT_DATE в запросе - это дата по зоне сессии, а она открыта с
	// TimeZone=UTC (#184): до 03:00 МСК он показывает вчерашний день, и заявка,
	// истёкшая вчера, всю ночь считается действующей.
	for _, name := range goFilesOfPackage(t) {
		if name == "moscow_sql.go" {
			continue
		}
		src := sourceOf(t, name)
		for i, line := range strings.Split(src, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "--") {
				continue
			}
			if strings.Contains(line, "CURRENT_DATE") {
				t.Errorf("%s:%d: CURRENT_DATE считается по UTC-зоне сессии; "+
					"для сроков заявки нужен moscowTodaySQL или passValidNowSQL", name, i+1)
			}
		}
	}
}

func TestPassValidNowSQLShape(t *testing.T) {
	cond := passValidNowSQL("a")

	// Момент собирается из даты и крайнего времени пребывания: срок кончается не в
	// полночь, а в entry_time_to последнего дня.
	if !strings.Contains(cond, "a.entry_date_to") || !strings.Contains(cond, "a.entry_time_to") {
		t.Errorf("условие обязано смотреть и на дату, и на время окончания: %s", cond)
	}
	// Зона задана явно: без неё сравнение уедет в зону сессии, то есть в UTC.
	if !strings.Contains(cond, "Europe/Moscow") {
		t.Errorf("в условии нет московской зоны: %s", cond)
	}
	// Вложение без времени доживает день до конца, а не пропадает в полночь.
	if !strings.Contains(cond, "23:59:59") {
		t.Errorf("нет запасного времени для вложений без entry_time_to: %s", cond)
	}
	// Псевдоним подставляется, а не зашит.
	if strings.Contains(passValidNowSQL("att"), "a.entry_date_to") {
		t.Error("псевдоним таблицы игнорируется - условие всегда смотрит на a")
	}
}

// Доступ только из внешнего тестового пакета с учебным PostgreSQL.
var NextApplicationNumberForTest = nextApplicationNumber
