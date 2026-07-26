package services

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"systemburo/internal/models"
	"systemburo/internal/normalize"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Подсказки по справочникам организаций и компаний для ручного ввода в заявке (#1437).
//
// Заявитель с правом application.organization.override печатает наименование сам, а
// незнакомое наименование заводит запись «на проверке» (см. application_org_resolve.go).
// Подсказка нужна, чтобы он выбрал существующую запись и не плодил черновик из-за
// сокращения или опечатки. Это поле выбора под формой, а не поиск по справочнику: отдаём
// пять близких совпадений, и только проверенных - черновики чужих заявок не показываем.
// Гейт по праву тот же, что разблокирует ручной ввод: без него организация заявки берётся
// из профиля, и подсказывать нечего.
//
// Сравниваем СМЫСЛОВЫЕ ЯДРА (наименование без токена ОПФ, normalize.OrgNameCore): человек
// печатает «максима», в справочнике лежит «ООО "Максима Групп"», и общий для всех записей
// префикс «ооо» только размывает близость. Ядро записи считает SQL по тому же списку ОПФ -
// паттерн приходит параметром из Go, поэтому второго источника правды нет.

const (
	// directorySuggestMinQuery - минимум символов ЯДРА запроса. На одной-двух буквах
	// подсказка вырождается в выдачу справочника, а ради неё он и закрыт.
	directorySuggestMinQuery = 3

	// directorySuggestMaxQuery - потолок длины ядра запроса. Наименование в справочнике
	// ограничено сотней символов (CreateOrganizationRequest), поэтому более длинный ввод
	// смысла не несёт, а similarity по нему считается на каждой строке справочника.
	directorySuggestMaxQuery = 100

	// directorySuggestLimit - сколько подсказок показываем. Список выбора, не поиск.
	directorySuggestLimit = 5

	// directorySuggestThreshold - порог близости ядер для опечаток. Откалиброван на
	// справочнике staging (23 наименования): опечатка в одну букву даёт 0.57 («максма» ->
	// «Максима Групп», «побега» -> «Победа»), неродственные пары не поднимаются выше 0.25
	// («ромашка» -> «РПС»). Порог 0.6 уже терял опечатки, ниже 0.5 растёт шум. Значение
	// мягче проектных 0.7 у чёрного списка (vehicle_blacklist_service): там метрика решает,
	// предупредить ли охрану, здесь - показать ли строку в списке выбора.
	directorySuggestThreshold = 0.5
)

// DirectorySuggestion - одна подсказка справочника. Отдаём id и наименование как оно
// хранится: форму записи задаёт справочник, а не то, что напечатал пользователь.
type DirectorySuggestion struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// DirectorySuggestAnswer - ответ подсказок целиком.
//
// Кроме близких записей форма получает два поля, которые иначе пришлось бы вычислять на
// фронте второй копией правил:
//
//	Canonical  - каноничное оформление введённого наименования (normalize.OrgNameDisplay).
//	             Поле подставляет его вместо набранного текста, поэтому человек видит
//	             заранее, что именно уйдёт в справочник.
//	Matched    - есть ли в справочнике активная запись с тем же ключом дедупликации. False
//	             значит, что подача заведёт новую запись «на проверке», и форма об этом
//	             предупреждает. Статус разбора существующей записи не важен: заявка ляжет
//	             и на чужой черновик, новой записи не появится.
//	Degenerate - у наименования нет ключа дедупликации (одни кавычки или точки). Подача
//	             такое отклоняет, запись из него не заводится, поэтому форма обязана
//	             сказать «укажите наименование», а не обещать проверку.
type DirectorySuggestAnswer struct {
	Items      []DirectorySuggestion `json:"items"`
	Canonical  string                `json:"canonical"`
	Matched    bool                  `json:"matched"`
	Degenerate bool                  `json:"degenerate"`
}

// DirectoryCoreSQL возвращает SQL-выражение смыслового ядра над source (колонкой ключа
// или плейсхолдером). Паттерн ОПФ приходит именованным параметром @opf, список форм
// остаётся в normalize. Наименование из одной ОПФ ядра не имеет - для него выражение
// возвращает сам ключ, как и OrgNameCore.
//
// Экспортировано ради теста: совпадение этого выражения с normalize.OrgNameCore
// доказывается только на живом Postgres - границы POSIX в Go-regexp не воспроизводятся,
// а разъехавшиеся ядра молча перестают находить совпадения.
func DirectoryCoreSQL(source string) string {
	return `COALESCE(NULLIF(btrim(regexp_replace(
		regexp_replace(` + source + `, @opf, ' ', 'g'), '\s+', ' ', 'g')), ''), ` + source + `)`
}

// escapeLikePattern экранирует спецсимволы LIKE в пользовательском вводе: без него
// наименование с «%» или «_» превращается в маску и вытаскивает весь справочник.
func escapeLikePattern(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
}

// suggestDirectory ищет близкие записи справочника table по наименованию. Возвращает
// пустой список (не ошибку), если запрос короче directorySuggestMinQuery: для поля ввода
// это нормальное состояние, а не сбой.
//
// В выдачу идут только активные и уже проверенные записи: черновик «на проверке» -
// частный ввод одного заявителя, предлагать его остальным нельзя, пока принимающий его
// не разобрал.
func suggestDirectory(ctx context.Context, db *gorm.DB, table, rawQuery string) (DirectorySuggestAnswer, error) {
	answer := DirectorySuggestAnswer{
		Items:     []DirectorySuggestion{},
		Canonical: normalize.OrgNameDisplay(rawQuery),
	}
	// Канон и признаки считаем от любого непустого ввода: поле подставляет оформление и
	// предупреждает ещё до того, как наберётся достаточно символов для самих подсказок.
	// Пустой ключ - вырожденное наименование (одни кавычки или точки): записи из него не
	// будет, поэтому «есть ли совпадение» тут не вопрос, отвечаем отдельным признаком.
	if key := normalize.OrgName(rawQuery); key != "" {
		matched, err := activeDirectoryKeyExists(ctx, db, table, key)
		if err != nil {
			return DirectorySuggestAnswer{}, err
		}
		answer.Matched = matched
	} else if strings.TrimSpace(rawQuery) != "" {
		answer.Degenerate = true
	}

	runes := []rune(normalize.OrgNameCore(rawQuery))
	if len(runes) < directorySuggestMinQuery {
		return answer, nil
	}
	if len(runes) > directorySuggestMaxQuery {
		runes = runes[:directorySuggestMaxQuery]
	}
	core := string(runes)

	suggestions := make([]DirectorySuggestion, 0, directorySuggestLimit)
	// Пустой ключ (наименование из одних кавычек или дефисов) отсекаем: ядра у такой
	// записи нет, и similarity к ней бессмысленна.
	query := `
		SELECT id, name FROM (
			SELECT id, name, core,
			       GREATEST(
			         similarity(core, @q),
			         word_similarity(@q, core),
			         word_similarity(core, @q)
			       ) AS sim
			FROM (
				SELECT id, name, ` + DirectoryCoreSQL("name_normalized") + ` AS core
				FROM ` + table + `
				WHERE is_active = true AND moderation_status = @approved AND name_normalized <> ''
			) c
		) t
		WHERE sim >= @threshold OR core LIKE '%' || @like || '%'
		ORDER BY (core LIKE @like || '%') DESC, sim DESC, name
		LIMIT @limit`

	err := db.WithContext(ctx).Raw(query, map[string]any{
		"q":         core,
		"like":      escapeLikePattern(core),
		"opf":       normalize.OrgLegalFormPattern(),
		"approved":  models.ModerationApproved,
		"threshold": directorySuggestThreshold,
		"limit":     directorySuggestLimit,
	}).Scan(&suggestions).Error
	if err != nil {
		slog.Error("не удалось подобрать подсказки справочника", "table", table, "error", err)
		return DirectorySuggestAnswer{}, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка поиска по справочнику")
	}
	answer.Items = suggestions
	return answer, nil
}

// activeDirectoryKeyExists отвечает, есть ли активная запись с таким ключом дедупликации.
// Условие повторяет резолв подачи (findActiveDirectoryEntry): статус разбора не смотрим -
// заявка ложится и на черновик, новой записи от этого не появляется.
func activeDirectoryKeyExists(ctx context.Context, db *gorm.DB, table, key string) (bool, error) {
	var exists bool
	q := "SELECT EXISTS (SELECT 1 FROM " + table + " WHERE is_active = true AND name_normalized = ?)"
	if err := db.WithContext(ctx).Raw(q, key).Scan(&exists).Error; err != nil {
		slog.Error("не удалось проверить наличие наименования в справочнике", "table", table, "error", err)
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка поиска по справочнику")
	}
	return exists, nil
}
