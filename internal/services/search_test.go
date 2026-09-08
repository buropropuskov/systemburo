package services

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
)

// Провайдер без права -- это раздел, открытый всем подряд, поэтому реестр проверяется на
// старте приложения, а не при первом запросе. Тест держит саму проверку: если её
// ослабят, он покажет это раньше, чем неправильный провайдер доедет до прода.
func TestValidateSearchProviders(t *testing.T) {
	t.Run("боевой реестр валиден", func(t *testing.T) {
		require.NoError(t, validateSearchProviders(searchProviderOrder()))
	})

	t.Run("пустой ключ отвергается", func(t *testing.T) {
		err := validateSearchProviders([]searchProvider{stubSearchProvider{typ: "stub", key: ""}})
		require.ErrorContains(t, err, "без permission-ключа")
	})

	t.Run("ключ вне каталога отвергается", func(t *testing.T) {
		err := validateSearchProviders([]searchProvider{stubSearchProvider{typ: "stub", key: "page.made.up"}})
		require.ErrorContains(t, err, "неизвестный ключ")
	})

	t.Run("дубль раздела отвергается", func(t *testing.T) {
		p := stubSearchProvider{typ: SearchTypeCars, key: KeyEntityCarsRead}
		err := validateSearchProviders([]searchProvider{p, p})
		require.ErrorContains(t, err, "дубль типа")
	})
}

// Состав реестра выписан явно: новый раздел обязан обновить этот список, а вместе с ним
// -- дописать свой случай в тесты видимости. Механическая защита от "забыл сузить
// выборку", а не надежда на внимательность.
func TestSearchProviderRegistryIsExpected(t *testing.T) {
	want := []SearchEntityType{SearchTypeEmployees, SearchTypeCars, SearchTypeApplications,
		SearchTypeUsers, SearchTypeBlacklist, SearchTypeDirectories,
		SearchTypeContent, SearchTypeFeedback}

	got := make([]SearchEntityType, 0, len(searchProviderOrder()))
	for _, p := range searchProviderOrder() {
		got = append(got, p.Type())
	}
	require.Equal(t, want, got, "изменился состав разделов поиска: обнови и тесты видимости")
}

// Каждый раздел обязан объявить право. Отдельно от validateSearchProviders: тот
// проверяет функцию, этот -- боевой реестр целиком.
func TestSearchProvidersHaveGates(t *testing.T) {
	for _, p := range searchProviderOrder() {
		t.Run(string(p.Type()), func(t *testing.T) {
			require.NotEmpty(t, p.PermissionKey(), "раздел без права виден всем")
			require.True(t, IsValidKey(p.PermissionKey()), "право отсутствует в каталоге")
			require.NotEmpty(t, p.Title())
		})
	}
}

func TestBuildSearchVariantsFor(t *testing.T) {
	t.Run("фамилия: раскладка и транслитерация есть, дубля по регистру нет", func(t *testing.T) {
		got := buildSearchVariantsFor("Роголев")

		require.Contains(t, got, "Роголев")
		require.Contains(t, got, "hjujktd", "забытая раскладка")
		require.Contains(t, got, "rogolev", "латинское написание той же фамилии (#2414)")
		// normalize.Plate дал бы "РОГОЛЕВ" -- для ILIKE это тот же запрос, и лишнее
		// условие только удлиняет SQL.
		require.NotContains(t, got, "РОГОЛЕВ")
		// Вариантов немного: каждый уходит в запрос отдельным условием на каждую колонку.
		require.LessOrEqual(t, len(got), 5, "слишком много вариантов: %v", got)
	})

	t.Run("госномер: вариант без пробелов добавляется", func(t *testing.T) {
		got := buildSearchVariantsFor("а 777 аа")
		require.Contains(t, got, "а 777 аа")
		require.Contains(t, got, "А777АА")
	})

	t.Run("пустой запрос не даёт вариантов", func(t *testing.T) {
		require.Empty(t, buildSearchVariantsFor("   "))
	})
}

func TestScoreMatch(t *testing.T) {
	cases := []struct {
		name, value, query string
		want               float64
	}{
		{"точное совпадение", "Роголев", "роголев", scoreExact},
		{"регистр и пробелы не мешают точному", "  РОГОЛЕВ ", "Роголев", scoreExact},
		{"начало строки", "Роголев Иван Петрович", "роголев", scorePrefix},
		{"начало слова внутри строки", "Иван Роголев", "роголев", scoreWordStart},
		{"вхождение в середине слова", "Пророголев", "роголев", scoreContains},
		{"совпадения в заголовке нет", "Иванов Иван", "роголев", scoreNoMatch},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, scoreMatch(tc.value, tc.query))
		})
	}
}

func TestRankItems(t *testing.T) {
	items := []SearchItem{
		{Title: "Иван Роголев", Subtitle: "водитель"},
		{Title: "Роголев", Subtitle: "водитель"},
		{Title: "Роголев Иван Петрович", Subtitle: "водитель"},
	}

	rankItems(items, "Роголев")

	require.Equal(t, "Роголев", items[0].Title, "точное совпадение должно быть первым")
	require.Equal(t, "Роголев Иван Петрович", items[1].Title)
	require.Equal(t, "Иван Роголев", items[2].Title)
	require.Equal(t, matchedFieldTitle, items[0].MatchedField)
}

// Совпало не в заголовке -- подсвечивать надо подзаголовок, иначе фронт мигает строкой
// без вхождения.
func TestRankItemsMarksSubtitleWhenTitleDoesNotMatch(t *testing.T) {
	items := []SearchItem{{Title: "А777АА", Subtitle: "BMW · ООО Роголев и партнёры"}}

	rankItems(items, "Роголев")

	require.Equal(t, matchedFieldSubtitle, items[0].MatchedField)
}

func TestHasExactMatch(t *testing.T) {
	exact := SearchGroup{Items: []SearchItem{{Score: scoreContains}, {Score: scoreExact}}}
	partial := SearchGroup{Items: []SearchItem{{Score: scorePrefix}}}

	require.True(t, hasExactMatch(exact))
	require.False(t, hasExactMatch(partial))
}

// stubSearchProvider -- заглушка для проверки самой валидации реестра.
type stubSearchProvider struct {
	typ SearchEntityType
	key string
}

func (s stubSearchProvider) Type() SearchEntityType { return s.typ }
func (s stubSearchProvider) Title() string          { return "Заглушка" }
func (s stubSearchProvider) PermissionKey() string  { return s.key }
func (s stubSearchProvider) Search(_ context.Context, _ *gorm.DB, _ searchRequest) ([]SearchItem, error) {
	return nil, nil
}

// Нечёткое сравнение (%>>) читает значение целиком. На письме к заявке в 70 килобайт
// одно такое сравнение стоило дороже всего остального запроса: на стенде поиск по
// заявкам занимал 1123 мс при бюджете 800 и стабильно попадал в degraded - человек
// видел "Не удалось опросить: Заявки". Длинные тексты ищем только точным вхождением.
func TestSearchConditionFuzzyIn(t *testing.T) {
	cols := []string{"a.application_number", "a.message", "o.name"}
	fuzzy := []string{"a.application_number", "o.name"}

	t.Run("длинное поле остаётся в точной части и не попадает в нечёткую", func(t *testing.T) {
		cond, _ := searchConditionFuzzyIn(cols, fuzzy, "Шумилин")

		require.Contains(t, cond, "a.message ILIKE")
		require.NotContains(t, cond, "a.message %>>")
		require.Contains(t, cond, "a.application_number %>>")
		require.Contains(t, cond, "o.name %>>")
	})

	t.Run("searchCondition сравнивает нечётко все колонки - поведение не менялось", func(t *testing.T) {
		cond, _ := searchCondition(cols, "Шумилин")

		require.Contains(t, cond, "a.message %>>")
	})

	t.Run("короткое слово не даёт нечёткой части ни там, ни там", func(t *testing.T) {
		cond, _ := searchConditionFuzzyIn(cols, fuzzy, "ку")

		require.NotContains(t, cond, "%>>")
	})

	t.Run("число аргументов совпадает с числом плейсхолдеров", func(t *testing.T) {
		cond, args := searchConditionFuzzyIn(cols, fuzzy, "Шумилин Кирилл")

		require.Equal(t, strings.Count(cond, "?"), len(args))
	})
}

// Учётную запись узнают двумя способами - по фамилии и по логину. Пока ступень
// считалась только по фамилии, набравший логин целиком получал свою запись под
// однофамильцами: у всех был одинаковый ранг, и порядок решал возраст записи.
func TestMatchRankExprAny(t *testing.T) {
	t.Run("одна колонка - выражение без LEAST, как было", func(t *testing.T) {
		expr := matchRankExprAny("u.last_name")

		require.NotContains(t, expr, "LEAST")
		require.Contains(t, expr, "u.last_name")
		require.True(t, strings.HasSuffix(expr, "AS match_rank"))
	})

	t.Run("несколько колонок - берётся лучшая ступень", func(t *testing.T) {
		expr := matchRankExprAny("u.last_name", "u.username")

		require.Contains(t, expr, "LEAST")
		require.Contains(t, expr, "u.last_name")
		require.Contains(t, expr, "u.username")
	})

	t.Run("на колонку приходится два плейсхолдера", func(t *testing.T) {
		require.Equal(t, 4, strings.Count(matchRankExprAny("a", "b"), "?"))
	})

	t.Run("matchRankExpr остался частным случаем", func(t *testing.T) {
		require.Equal(t, matchRankExprAny("u.last_name"), matchRankExpr("u.last_name"))
	})
}

// Поиск по контактам ищет по свёртке, поэтому условие собирается отдельно от
// запроса - и ошибиться в нём можно ровно двумя способами: не исключить скрытые
// записи или отключить поиск целиком, если скрытая нашлась хоть одна. Второе и
// случилось после #2351: на стенде масок 59 из 109, и по контактам не находился
// никто.

// TestContactMatchCondition_NoMasks - без скрытых записей условие сводится к
// точному совпадению по свёртке.
func TestContactMatchCondition_NoMasks(t *testing.T) {
	cond, args := contactMatchCondition(" ivanov@example.com ", nil)

	if cond != "(u.email_hmac = ? OR u.phone_hmac = ?)" {
		t.Fatalf("неожиданное условие: %s", cond)
	}
	if len(args) != 2 {
		t.Fatalf("ожидались две свёртки, получено %d", len(args))
	}
	if args[0] != crypto.ComputeHMAC(models.NormalizeEmailForHMAC("ivanov@example.com"), crypto.GetGlobalKey()) {
		t.Fatal("свёртка почты обязана считаться от нормализованного значения")
	}
}

// TestContactMatchCondition_MasksExcludedByID - скрытые записи выпадают поимённо, а
// поиск по остальным продолжает работать. Обратное поведение (общий выключатель) и
// было регрессией.
func TestContactMatchCondition_MasksExcludedByID(t *testing.T) {
	cond, args := contactMatchCondition("89161234567", map[int]string{7: "@ivanov"})

	if !strings.Contains(cond, "u.email_hmac = ?") {
		t.Fatalf("поиск по контактам не должен выключаться из-за скрытых записей: %s", cond)
	}
	if !strings.Contains(cond, "u.id NOT IN (?)") {
		t.Fatalf("скрытые записи обязаны исключаться поимённо: %s", cond)
	}
	if len(args) != 3 {
		t.Fatalf("ожидались две свёртки и список скрытых, получено %d", len(args))
	}
	hidden, ok := args[2].([]int)
	if !ok || len(hidden) != 1 || hidden[0] != 7 {
		t.Fatalf("в исключение должен уйти идентификатор скрытой записи, получено %#v", args[2])
	}
}

// TestContactMatchCondition_EmptyQuery - пустой запрос условия не даёт, иначе к
// поиску прицепилось бы совпадение со свёрткой пустой строки.
func TestContactMatchCondition_EmptyQuery(t *testing.T) {
	if cond, args := contactMatchCondition("   ", nil); cond != "" || args != nil {
		t.Fatalf("пустой запрос не должен давать условия: %q %#v", cond, args)
	}
}
