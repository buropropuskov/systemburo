package services

import (
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"
)

func normalizedQuery(t *testing.T, q models.PassageHistoryQuery) models.PassageHistoryQuery {
	t.Helper()
	q.Normalize()
	return q
}

// Порядок журнала обязан включать id вторым ключом. Отметки, попавшие в одну
// секунду, при сортировке только по времени возвращаются в произвольном порядке, и
// подгрузка страницами то повторяет строку, то теряет её. На живой базе это не
// ловится: один и тот же план обычно отдаёт стабильный порядок и без ключа, поэтому
// условие стережётся здесь, на самой строке запроса.
func TestPassageHistoryOrderSQL_HasIDTiebreaker(t *testing.T) {
	desc := passageHistoryOrderSQL(normalizedQuery(t, models.PassageHistoryQuery{}), "h")
	if !strings.Contains(desc, "h.created_at DESC") || !strings.Contains(desc, "h.id DESC") {
		t.Errorf("порядок по умолчанию должен быть «свежие сверху» с id вторым ключом, получили %q", desc)
	}

	asc := passageHistoryOrderSQL(normalizedQuery(t, models.PassageHistoryQuery{Order: "ASC"}), "eh")
	if !strings.Contains(asc, "eh.created_at ASC") || !strings.Contains(asc, "eh.id ASC") {
		t.Errorf("обратный порядок должен разворачивать оба ключа, получили %q", asc)
	}
}

// Предел страницы держится в Normalize: перебор per_page срезается, отрицательная
// страница читается как первая. Без этого один запрос снова выгружал бы весь журнал.
func TestPassageHistoryQuery_NormalizeBounds(t *testing.T) {
	q := normalizedQuery(t, models.PassageHistoryQuery{Page: -5, PerPage: 100000, Order: "как-нибудь", Search: "  Volvo  "})
	if q.Page != 1 {
		t.Errorf("страница: got %d, want 1", q.Page)
	}
	if q.PerPage != models.PassageHistoryMaxPerPage {
		t.Errorf("размер страницы: got %d, want %d", q.PerPage, models.PassageHistoryMaxPerPage)
	}
	if q.Order != "desc" {
		t.Errorf("неизвестный порядок читается как desc, got %q", q.Order)
	}
	if q.Search != "Volvo" {
		t.Errorf("поиск должен приходить без окружающих пробелов, got %q", q.Search)
	}

	empty := normalizedQuery(t, models.PassageHistoryQuery{})
	if empty.PerPage != models.PassageHistoryDefaultPerPage {
		t.Errorf("размер страницы по умолчанию: got %d, want %d", empty.PerPage, models.PassageHistoryDefaultPerPage)
	}
	if got := normalizedQuery(t, models.PassageHistoryQuery{Page: 3, PerPage: 20}).Offset(); got != 40 {
		t.Errorf("смещение третьей страницы по 20: got %d, want 40", got)
	}
}

// Условия и аргументы должны идти строго парами: собранный запрос подставляет `?` по
// порядку, и лишний или потерянный аргумент сдвинул бы весь фильтр.
func TestPassageHistoryConditions_ArgsMatchPlaceholders(t *testing.T) {
	carID := 17
	userID := 4
	q := normalizedQuery(t, models.PassageHistoryQuery{
		UserID:   &userID,
		DateFrom: "2026-03-12",
		DateTo:   "2026-03-12",
		Search:   "Volvo",
	})

	cond, args, err := passageHistoryConditions(q, passageFilterSpec{
		alias:        "h",
		entityColumn: "h.car_id",
		entityID:     &carID,
		searchExprs:  []string{"c.car_number", "c.car_brand"},
	})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if got, want := strings.Count(cond, "?"), len(args); got != want {
		t.Errorf("мест подстановки %d, аргументов %d: %q", got, want, cond)
	}
	if !strings.HasPrefix(cond, " AND ") {
		t.Errorf("условие дописывается к готовому WHERE, поэтому обязано начинаться с AND: %q", cond)
	}
	if strings.Count(cond, "ILIKE ?") != 2 {
		t.Errorf("поиск идёт по каждому выражению: %q", cond)
	}

	// Верхняя граница периода - конец московских суток, то есть начало следующих.
	from, ok := args[2].(time.Time)
	if !ok {
		t.Fatalf("границей периода ожидалось время, получили %T", args[2])
	}
	to, ok := args[3].(time.Time)
	if !ok {
		t.Fatalf("границей периода ожидалось время, получили %T", args[3])
	}
	if to.Sub(from) != 24*time.Hour {
		t.Errorf("день периода должен покрывать сутки целиком: %s .. %s", from, to)
	}
	if from.Location().String() != MoscowLocation().String() {
		t.Errorf("границы считаются в московской зоне, получили %s", from.Location())
	}
}

// Пустой фильтр не должен дописывать к запросу ничего: иначе выборка «весь журнал»
// получила бы висячий AND.
func TestPassageHistoryConditions_EmptyStaysEmpty(t *testing.T) {
	cond, args, err := passageHistoryConditions(normalizedQuery(t, models.PassageHistoryQuery{}), passageFilterSpec{alias: "h", entityColumn: "h.car_id"})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if cond != "" || len(args) != 0 {
		t.Errorf("пустой фильтр: got (%q, %v)", cond, args)
	}
}

// Битая дата - ошибка запроса, а не «фильтр не применился»: иначе опечатка в поле
// периода отдавала бы весь журнал, и человек решил бы, что записей стало больше.
func TestPassageHistoryConditions_BadDateIsError(t *testing.T) {
	for _, q := range []models.PassageHistoryQuery{{DateFrom: "12.03.2026"}, {DateTo: "2026-13-40"}} {
		if _, _, err := passageHistoryConditions(normalizedQuery(t, q), passageFilterSpec{alias: "h", entityColumn: "h.car_id"}); err == nil {
			t.Errorf("ожидали ошибку на %+v", q)
		}
	}
}
