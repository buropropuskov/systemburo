package services

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"systemburo/internal/normalize"

	"gorm.io/gorm"
)

// applyRegistryScope сужает выборку реестра (unique_cars/unique_employees) до строк,
// которые пользователь вправе видеть. Зеркало ветки "all" из buildCarsQuery
// (unique_car_service.go) и buildEmployeesQuery (unique_employee_service.go): свои
// записи плюс записи своей организации и своей компании; полный срез системы -- только
// тем, кому его открывает searchCanSeeAllSystem.
//
// Скоуп применяется внутри провайдера, а не в хендлере: гейт эндпоинта и отсев по
// PermissionKey отвечают на "показывать ли раздел", но не на "какие строки раздела".
// Разведение этих двух вопросов и есть защита от повторения #1524/#1528.
//
// Ветки organization/company навешиваются только при непустом значении у пользователя.
// В реестрах на их месте подставляется 0, что безопасно (колонки nullable, id с
// единицы), но лишнее условие в OR ничего не находит и только удлиняет запрос.
func applyRegistryScope(q *gorm.DB, alias string, req searchRequest) *gorm.DB {
	if req.CanSeeAllSystem {
		return q
	}

	cond := fmt.Sprintf("%s.user_id = ?", alias)
	args := []interface{}{req.UserID}
	if req.OrgID != nil && *req.OrgID != 0 {
		cond += fmt.Sprintf(" OR %s.organization_id = ?", alias)
		args = append(args, *req.OrgID)
	}
	if req.CompanyID != nil && *req.CompanyID != 0 {
		cond += fmt.Sprintf(" OR %s.company_id = ?", alias)
		args = append(args, *req.CompanyID)
	}
	return q.Where(cond, args...)
}

// searchCanSeeAllSystem -- вправе ли пользователь видеть системный срез реестров.
//
// Намеренно обёртка вокруг userIsSystemAdmin, а не собственный предикат: поиск не
// должен быть шире листинга ни для кого. В системе есть рассинхрон -- реестры гейтят
// системный срез флагами is_super_admin/is_admin напрямую из users, минуя резолвер,
// тогда как грант section.registry.all_system существует в каталоге и раздаётся ролям,
// из-за чего его носитель видит вкладку и получает 403.
//
// Чинить рассинхрон здесь нельзя: переход на резолвер расширил бы доступ носителям
// гранта, то есть поменял матрицу доступа. Хуже того, починка только в поиске родила бы
// второй перекос -- запись нашлась бы в поиске и не открылась в реестре. Когда
// рассинхрон будут устранять, менять придётся ровно эту функцию.
func searchCanSeeAllSystem(ctx context.Context, db *gorm.DB, userID int) bool {
	return userIsSystemAdmin(ctx, db, userID)
}

// matchRankExpr возвращает выражение ступени совпадения для сортировки: точное
// совпадение колонки с запросом, затем совпадение с начала, затем всё остальное.
// Ожидает два аргумента-плейсхолдера с сырым запросом.
//
// Выражение идёт в SELECT под именем match_rank, а не прямо в ORDER BY, и это
// существенно: gorm не подставляет аргументы в Order, из-за чего условие тихо теряется
// вместе со всей сортировкой -- выдача приезжала в порядке физического чтения таблицы,
// то есть от самых старых записей, и обрезка по лимиту выбрасывала как раз свежие.
func matchRankExpr(col string) string {
	return matchRankExprAny(col)
}

// matchRankExprAny -- та же ступень, но по нескольким колонкам сразу: берём лучшую из
// них. Нужно там, где запись узнают не единственным способом: учётную запись ищут и по
// фамилии, и по логину, и набравший логин целиком ждёт эту запись первой, а не под
// однофамильцами.
//
// Каждая колонка требует двух аргументов (точное равенство и префикс) в порядке
// перечисления - вызывающий передаёт запрос столько раз, сколько колонок, дважды.
func matchRankExprAny(cols ...string) string {
	parts := make([]string, 0, len(cols))
	for _, col := range cols {
		parts = append(parts, `CASE
		WHEN LOWER(TRIM(COALESCE(`+col+`, ''))) = LOWER(TRIM(?)) THEN 0
		WHEN LOWER(COALESCE(`+col+`, '')) LIKE LOWER(?) || '%' THEN 1
		ELSE 2 END`)
	}
	if len(parts) == 1 {
		return parts[0] + " AS match_rank"
	}
	return "LEAST(" + strings.Join(parts, ", ") + ") AS match_rank"
}

// searchMaxWords -- сколько слов запроса учитывать. Каждое слово добавляет свой блок
// условий, а осмысленный запрос длиннее шести слов в этой системе не встречается.
const searchMaxWords = 6

// multiWordCondition строит условие поиска по набору колонок с разбором запроса на слова.
//
// Человек вводит «В 543 НЕ 654 Мерседес» или «Иванов Иван», а номер с маркой и фамилия с
// именем лежат в разных колонках. Поиск строки целиком не находит ничего: такой
// последовательности символов нет ни в одном поле. Поэтому каждое слово ищется отдельно
// по всем колонкам, а между словами стоит И: запись подходит, если каждое слово нашлось
// хоть где-то.
//
// Требование «все слова» существенно. С ИЛИ запрос из двух слов возвращал бы объединение
// двух поисков -- по «Мерседес Запорожец» приехал бы весь Мерседес, хотя такой записи нет.
func multiWordCondition(cols []string, raw string) (string, []interface{}) {
	words := strings.Fields(raw)
	if len(words) > searchMaxWords {
		words = words[:searchMaxWords]
	}
	// Одно слово -- обычное условие, без лишней вложенности скобок.
	if len(words) <= 1 {
		return ilikePatternsArgs(cols, buildSearchVariantsFor(raw))
	}

	parts := make([]string, 0, len(words))
	args := make([]interface{}, 0, len(words)*len(cols)*2)
	for _, w := range words {
		cond, wordArgs := ilikePatternsArgs(cols, buildSearchVariantsFor(w))
		if cond == "" {
			continue
		}
		parts = append(parts, "("+cond+")")
		args = append(args, wordArgs...)
	}
	if len(parts) == 0 {
		return ilikePatternsArgs(cols, buildSearchVariantsFor(raw))
	}
	return strings.Join(parts, " AND "), args
}

// buildSearchVariantsFor -- варианты запроса для сквозного поиска.
//
// Отличается от buildSearchVariants (application_helpers.go) тем, что не добавляет
// заведомо бесполезные варианты: каждый лишний вариант умножается на число колонок
// раздела и превращается в отдельное ILIKE-условие.
//
//   - раскладка добавляется всегда: она ловит реальную ошибку ввода в обе стороны
//     (набрал "Hjujktd" вместо "Роголев" и наоборот для номеров и марок);
//   - normalize.Plate добавляется только для запросов с цифрами. Он приводит строку к
//     верхнему регистру и удаляет пробелы -- для ФИО это даёт либо дубль (ILIKE
//     регистронезависим), либо склейку "ИВАНПЕТРОВ", которая не встречается в данных.
//     Смысл он имеет для госномеров, а те всегда с цифрами;
//   - дедупликация идёт по нижнему регистру, тогда как buildSearchVariants сравнивает
//     точные строки и оставляет пару "Роголев"/"РОГОЛЕВ" целиком.
//
// buildSearchVariants не трогаем: у него другие потребители (Центр заявок, реестры,
// доступные вложения), и смена их семантики в объём сквозного поиска не входит.
func buildSearchVariantsFor(raw string) []string {
	variants := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		variants = append(variants, v)
	}

	add(raw)
	add(normalize.SwitchLayout(raw))
	if strings.ContainsAny(raw, "0123456789") {
		add(normalize.Plate(raw))
	}
	return variants
}

// Порог нечёткого сравнения. Тот же, что у поиска сотрудников в Центре заявок:
// strict_word_similarity > 0.3. Ниже начинают проходить общие триграммы («арбуз» к
// «Карбышев»), выше — перестают ловиться настоящие опечатки в короткой фамилии.
const searchTrigramThreshold = "0.3"

// searchFuzzyMinWordLen -- короче этого слова нечётко не сравниваем. На трёх символах
// почти любая пара слов оказывается «похожей», и выдача превращается в шум; точное
// вхождение для таких фрагментов и так работает.
const searchFuzzyMinWordLen = 4

// withTrigramThreshold выполняет fn в транзакции с выставленным порогом нечёткого
// сравнения.
//
// Порог задаётся через SET LOCAL, а не глобальной настройкой базы: настройка на уровне
// базы не подействует на уже открытые соединения пула, поэтому сразу после развёртывания
// поиск вёл бы себя по-разному в зависимости от того, какое соединение достанется
// запросу. Транзакция на чтение стоит доли миллисекунды и даёт предсказуемость.
func withTrigramThreshold(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL pg_trgm.strict_word_similarity_threshold = " + searchTrigramThreshold).Error; err != nil {
			return fmt.Errorf("выставить порог нечёткого поиска: %w", err)
		}
		return fn(tx)
	})
}

// fuzzyWordCondition добавляет к точному поиску нечёткое сравнение по тем же колонкам.
//
// Оператор %>> (а не функция strict_word_similarity) взят намеренно: только он
// опирается на GIN-индекс. Функция от concat_ws индексом не покрывается ни при каких
// индексах и заставляет просматривать таблицу целиком — с запросом на каждый введённый
// символ это недопустимо.
//
// Возвращает пустую строку, если нечётко сравнивать нечего: все слова короткие.
func fuzzyWordCondition(cols []string, raw string) (string, []interface{}) {
	words := strings.Fields(raw)
	if len(words) > searchMaxWords {
		words = words[:searchMaxWords]
	}

	parts := make([]string, 0, len(words))
	args := make([]interface{}, 0, len(words)*len(cols))
	for _, w := range words {
		if utf8.RuneCountInString(w) < searchFuzzyMinWordLen {
			continue
		}
		colParts := make([]string, 0, len(cols))
		for _, c := range cols {
			colParts = append(colParts, c+" %>> ?")
			args = append(args, w)
		}
		parts = append(parts, "("+strings.Join(colParts, " OR ")+")")
	}
	if len(parts) == 0 {
		return "", nil
	}
	// Между словами И, как и у точного поиска: запись подходит, когда похоже каждое слово.
	return strings.Join(parts, " AND "), args
}

// searchCondition -- полное условие поиска по набору колонок: точное вхождение или
// нечёткое совпадение. Опечатку ловит вторая ветка, точный фрагмент -- первая.
func searchCondition(cols []string, raw string) (string, []interface{}) {
	return searchConditionFuzzyIn(cols, cols, raw)
}

// searchConditionFuzzyIn -- то же, но нечётко сравнивается только часть колонок.
//
// Нужно длинным текстовым полям: тело письма к заявке, полный текст новости, текст
// обращения. Оператор %>> просматривает значение целиком, и на письме в 70 килобайт
// одно такое сравнение стоит дороже всего остального запроса вместе взятого -- на
// стенде поиск по заявкам из-за него не укладывался в свой бюджет 800 мс (1123 мс) и
// стабильно попадал в degraded с "Не удалось опросить: Заявки".
//
// Потери смысла нет: в длинном тексте ищут точный фрагмент, а не приблизительный. У
// коротких полей -- номера, фамилии, названия -- нечёткое сравнение остаётся.
func searchConditionFuzzyIn(cols, fuzzyCols []string, raw string) (string, []interface{}) {
	cond, args := multiWordCondition(cols, raw)
	fuzzyCond, fuzzyArgs := fuzzyWordCondition(fuzzyCols, raw)
	if fuzzyCond == "" {
		return cond, args
	}
	return "(" + cond + ") OR (" + fuzzyCond + ")", append(args, fuzzyArgs...)
}
