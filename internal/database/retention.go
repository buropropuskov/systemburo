package database

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// Уборка накопленных данных (#1614). Партиционированные журналы (request_logs,
// pd_audit_logs) чистятся дропом партиций в MaintainLogPartitions - здесь всё
// остальное, где удалять приходится строками.
//
// Две группы с разной политикой. Токены сессий, прочитанные и непрочитанные
// уведомления (#1748, S9 - свой, более мягкий срок) обесцениваются сами по себе,
// их сметает суточный воркер (SweepRoutine).
// История сущностей, слепки таблиц и дневные агрегаты журнала запросов удаляются
// только руками оператора через подкоманду cleanup: у них есть ценность, и решение
// «этого больше не нужно» человеческое, а не автоматическое.
type RetentionTarget string

const (
	// TargetTokens - refresh-токены, которые уже нельзя предъявить: истёкшие и
	// отозванные. Самая объёмная группа мусора: при 15-минутном access-токене
	// ротация даёт ~32 строки на человека за смену, и ни одна не удалялась.
	TargetTokens RetentionTarget = "tokens"
	// TargetNotifications - прочитанные уведомления.
	TargetNotifications RetentionTarget = "notifications"
	// TargetUnreadNotifications - непрочитанные уведомления (#1748, S9). Отдельная
	// цель с заметно более мягким сроком: непрочитанное не бросали, его ещё не видели.
	// Без второго порога такие уведомления копились у человека вечно, а лента их
	// все грузила.
	TargetUnreadNotifications RetentionTarget = "unread-notifications"
	// TargetAudit - история сущностей. Чистится с оговорками, см. auditRetentionWhere.
	TargetAudit RetentionTarget = "audit"
	// TargetSnapshots - суточные слепки таблиц постов. Ручные снимки не трогаются:
	// их делают осознанно и под конкретную задачу.
	TargetSnapshots RetentionTarget = "snapshots"
	// TargetRequestAggregates - дневные агрегаты журнала запросов. Сами детальные
	// записи дропаются партициями за REQUEST_LOG_DETAIL_DAYS, а свёртка копилась вечно.
	TargetRequestAggregates RetentionTarget = "request-aggregates"
	// TargetPushSubscriptions - подписки Web Push (#974), у которых давно не было ни
	// одной успешной доставки: явно мёртвый endpoint без 404/410 (сеть, таймаут, 5xx -
	// счётчик неудач в push_service.go ещё не дошёл до своего порога) либо просто
	// забытое устройство, которое отвечает 2xx, но человек им давно не пользуется.
	// Обесценивается сама по себе - в автоматической уборке рядом с уведомлениями.
	TargetPushSubscriptions RetentionTarget = "push-subscriptions"
)

// AllRetentionTargets - порядок вывода в отчёте: сначала мусор, потом то, что
// удаляется осознанно.
var AllRetentionTargets = []RetentionTarget{
	TargetTokens, TargetNotifications, TargetUnreadNotifications, TargetPushSubscriptions, TargetAudit, TargetSnapshots, TargetRequestAggregates,
}

// auditRetentionWhere - условие удаления истории сущностей. Два исключения не про
// срок, а про то, что audit_log держит текущее состояние системы, а не только летопись:
//
//   - action='delete' - признак того, что элемент лежит в корзине таблицы поста
//     (trash_service ищет именно эту запись). Удалив её, вычистишь корзину.
//   - последние entry и exit каждой сущности - из них считается «последний выезд»
//     в карточке машины и человека. У редко приезжающей машины такая запись может
//     быть старше любого разумного срока хранения.
const auditRetentionWhere = `
	created_at < ?
	AND action <> 'delete'
	AND id NOT IN (
		SELECT DISTINCT ON (entity_type, entity_id, action) id
		FROM audit_log
		WHERE action IN ('entry', 'exit')
		ORDER BY entity_type, entity_id, action, created_at DESC, id DESC
	)`

// retentionRule описывает, что и по какому условию удаляется. cutoffArgs задаёт,
// сколько раз граничная дата подставляется в условие.
type retentionRule struct {
	table      string
	where      string
	cutoffArgs int
	// timeColumn - по какому столбцу отсчитывается возраст записи. Нужен фильтру
	// нижней границы: без него «удалить только за 2023 год» не выразить.
	timeColumn string
	// entityFilter и tableFilter - применим ли к группе фильтр по типу сущности и по
	// таблице поста. Фильтр, заданный для группы, которая его не понимает, отклоняется
	// с объяснением: молча проигнорировать его при удалении опаснее, чем упасть.
	entityFilter bool
	tableFilter  bool
	// defaultAge - срок хранения по умолчанию, если оператор не задал свой.
	defaultAge func(now time.Time) time.Time
	// description - строка для отчёта команды, человеку в терминал.
	description string
}

var retentionRules = map[RetentionTarget]retentionRule{
	TargetTokens: {
		table:       "refresh_tokens",
		where:       "expires_at < ? OR (is_revoked AND COALESCE(revoked_at, created_at) < ?)",
		cutoffArgs:  2,
		timeColumn:  "created_at",
		defaultAge:  func(now time.Time) time.Time { return now.AddDate(0, 0, -30) },
		description: "недействительные токены сессий",
	},
	TargetNotifications: {
		table:       "notifications",
		where:       "is_read AND created_at < ?",
		cutoffArgs:  1,
		timeColumn:  "created_at",
		defaultAge:  func(now time.Time) time.Time { return now.AddDate(0, 0, -30) },
		description: "прочитанные уведомления",
	},
	TargetUnreadNotifications: {
		table: "notifications",
		// NOT is_read - обязательное исключение, а не украшение условия: без него
		// строка попадала бы под ОБА условия сразу (это и TargetNotifications делят
		// одну таблицу), и счётчик удалённого в логе задваивался бы. Прочитанные
		// уходят по своему, более короткому сроку (TargetNotifications выше по файлу);
		// здесь - только то, что человек ещё не видел.
		where:       "NOT is_read AND created_at < ?",
		cutoffArgs:  1,
		timeColumn:  "created_at",
		defaultAge:  func(now time.Time) time.Time { return now.AddDate(0, 0, -90) },
		description: "непрочитанные уведомления",
	},
	TargetPushSubscriptions: {
		table: "push_subscriptions",
		// COALESCE на last_success_at, как и у ленты уведомлений (COALESCE(last_event_at,
		// created_at)) выше: подписка, которая ещё ни разу не доставилась, отсчитывается
		// от момента подписки, а не улетает в "никогда не обесценится".
		where:       "COALESCE(last_success_at, created_at) < ?",
		cutoffArgs:  1,
		timeColumn:  "created_at",
		defaultAge:  func(now time.Time) time.Time { return now.AddDate(0, 0, -180) },
		description: "подписки Web Push без успешной доставки за срок",
	},
	TargetAudit: {
		table:        "audit_log",
		where:        auditRetentionWhere,
		cutoffArgs:   1,
		timeColumn:   "created_at",
		entityFilter: true,
		defaultAge:  func(now time.Time) time.Time { return now.AddDate(-3, 0, 0) },
		description: "история сущностей, кроме корзины и последних отметок прохода",
	},
	TargetSnapshots: {
		table:       "table_snapshots",
		where:       "reason = 'scheduled' AND taken_at < ?",
		cutoffArgs:  1,
		timeColumn:  "taken_at",
		tableFilter: true,
		defaultAge:  func(now time.Time) time.Time { return now.AddDate(-1, 0, 0) },
		description: "суточные слепки таблиц постов, кроме ручных",
	},
	TargetRequestAggregates: {
		table:       "request_logs_daily",
		where:       "day < (?)::date",
		cutoffArgs:  1,
		timeColumn:  "day",
		defaultAge:  func(now time.Time) time.Time { return now.AddDate(-2, 0, 0) },
		description: "дневные агрегаты журнала запросов",
	},
}

// RetentionResult - итог по одной группе. Matched считается всегда, Deleted остаётся
// нулём в режиме предварительного показа.
//
// TotalRows и TableBytes отвечают на вопрос «а что вообще занимает место»: без них
// оператор видит число записей под удаление, но не знает, стоит ли овчинка выделки.
// FreedBytes - ОЦЕНКА по доле строк, а не точный замер: строки одной таблицы разного
// размера (в audit_log details у одних пустой, у других килобайт), да и место
// удалённых строк база отдаёт не операционной системе, а себе под новые записи.
type RetentionResult struct {
	Target      RetentionTarget
	Description string
	Cutoff      time.Time
	From        *time.Time
	Matched     int64
	Deleted     int64
	TotalRows   int64
	TableBytes  int64
	FreedBytes  int64
}

// ParseRetentionTarget разбирает имя группы из аргументов команды.
func ParseRetentionTarget(s string) (RetentionTarget, error) {
	t := RetentionTarget(strings.TrimSpace(s))
	if _, ok := retentionRules[t]; !ok {
		names := make([]string, 0, len(AllRetentionTargets))
		for _, n := range AllRetentionTargets {
			names = append(names, string(n))
		}
		return "", fmt.Errorf("неизвестная группа %q (доступны: %s)", s, strings.Join(names, ", "))
	}
	return t, nil
}

// SelectRetentionTargets раскрывает аргументы команды в перечень групп: targets -
// список через запятую либо all, except - что из него вычесть. Порядок вывода общий
// для всех вызовов (AllRetentionTargets), повторы схлопываются.
//
// Исключение существует потому, что оператор рассуждает «почисти всё, только историю
// не трогай», а не перечисляет четыре имени из пяти.
func SelectRetentionTargets(targets, except string) ([]RetentionTarget, error) {
	chosen, err := parseTargetList(targets)
	if err != nil {
		return nil, err
	}
	if len(chosen) == 0 {
		return nil, fmt.Errorf("не указана ни одна группа")
	}
	excluded, err := parseTargetList(except)
	if err != nil {
		return nil, err
	}
	out := make([]RetentionTarget, 0, len(chosen))
	for _, t := range AllRetentionTargets {
		if chosen[t] && !excluded[t] {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("исключены все выбранные группы, удалять нечего")
	}
	return out, nil
}

// parseTargetList разбирает перечень групп через запятую; "all" разворачивается в
// полный набор, пустая строка даёт пустое множество.
func parseTargetList(s string) (map[RetentionTarget]bool, error) {
	set := make(map[RetentionTarget]bool, len(AllRetentionTargets))
	if strings.TrimSpace(s) == "all" {
		for _, t := range AllRetentionTargets {
			set[t] = true
		}
		return set, nil
	}
	for _, part := range strings.Split(s, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		t, err := ParseRetentionTarget(part)
		if err != nil {
			return nil, err
		}
		set[t] = true
	}
	return set, nil
}

// DefaultRetentionCutoff возвращает границу по умолчанию для группы.
func DefaultRetentionCutoff(target RetentionTarget, now time.Time) time.Time {
	return retentionRules[target].defaultAge(now)
}

// ParseRetentionAge разбирает срок хранения в виде "30d" (суток) или "12m" (месяцев)
// и возвращает границу относительно now. time.ParseDuration тут не годится: она не
// знает ни суток, ни месяцев, а оператор мыслит именно ими.
func ParseRetentionAge(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("срок %q не разобран (ожидается вид 30d или 12m)", s)
	}
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n < 0 {
		return time.Time{}, fmt.Errorf("срок %q не разобран (ожидается вид 30d или 12m)", s)
	}
	switch unit {
	case 'd':
		return now.AddDate(0, 0, -n), nil
	case 'm':
		return now.AddDate(0, -n, 0), nil
	default:
		return time.Time{}, fmt.Errorf("срок %q не разобран: единица %q неизвестна (d - сутки, m - месяцы)", s, string(unit))
	}
}

// SweepRetention удаляет данные одной группы старше cutoff. При apply=false ничего
// не удаляет и только считает, сколько попало бы под условие - это режим по умолчанию
// для команды cleanup: оператор сначала смотрит на числа.
func SweepRetention(ctx context.Context, db *gorm.DB, target RetentionTarget, opts SweepOptions) (RetentionResult, error) {
	rule, ok := retentionRules[target]
	if !ok {
		return RetentionResult{}, fmt.Errorf("неизвестная группа %q", target)
	}
	if err := opts.validateFor(target, rule); err != nil {
		return RetentionResult{}, err
	}
	res := RetentionResult{Target: target, Description: rule.description, Cutoff: opts.Cutoff, From: opts.From}

	// Условие группы берётся в скобки: у токенов оно содержит OR, и без скобок
	// добавленный фильтр прилип бы только ко второй его половине.
	where := "(" + rule.where + ")"
	args := make([]interface{}, 0, rule.cutoffArgs+3)
	for i := 0; i < rule.cutoffArgs; i++ {
		args = append(args, opts.Cutoff)
	}
	if opts.From != nil {
		if rule.timeColumn == "day" {
			where += " AND day >= (?)::date" // столбец типа date, приведение обязательно
		} else {
			where += fmt.Sprintf(" AND %s >= ?", rule.timeColumn)
		}
		args = append(args, *opts.From)
	}
	if opts.EntityType != "" {
		where += " AND entity_type = ?"
		args = append(args, opts.EntityType)
	}
	if opts.TableID != nil {
		where += " AND table_id = ?"
		args = append(args, *opts.TableID)
	}

	countSQL := fmt.Sprintf("SELECT count(*) FROM %s WHERE %s", rule.table, where)
	if err := db.WithContext(ctx).Raw(countSQL, args...).Scan(&res.Matched).Error; err != nil {
		return res, fmt.Errorf("подсчёт %s: %w", rule.table, err)
	}
	if err := measureRetentionTable(ctx, db, rule.table, &res); err != nil {
		return res, err
	}
	if !opts.Apply || res.Matched == 0 {
		return res, nil
	}

	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE %s", rule.table, where)
	tx := db.WithContext(ctx).Exec(deleteSQL, args...)
	if tx.Error != nil {
		return res, fmt.Errorf("удаление из %s: %w", rule.table, tx.Error)
	}
	res.Deleted = tx.RowsAffected
	return res, nil
}

// SweepOptions - что и с какими сужениями удалять. Cutoff обязателен, остальное
// сужает выборку внутри группы: период снизу, тип сущности в журнале, таблица поста
// у слепков. Появились по запросу эксплуатации (#1632): «удалить историю только по
// машинам» или «снять слепки одного поста» без сужений не выразить.
type SweepOptions struct {
	Cutoff     time.Time
	From       *time.Time
	EntityType string
	TableID    *int
	Apply      bool
}

// validateFor отклоняет фильтр, которого группа не понимает. Тихо игнорировать его
// нельзя: оператор просил сузить удаление, а получил бы удаление всей группы.
func (o SweepOptions) validateFor(target RetentionTarget, rule retentionRule) error {
	if o.EntityType != "" && !rule.entityFilter {
		return fmt.Errorf("фильтр по типу сущности применим только к группе %s, а выбрана %s",
			TargetAudit, target)
	}
	if o.TableID != nil && !rule.tableFilter {
		return fmt.Errorf("фильтр по таблице поста применим только к группе %s, а выбрана %s",
			TargetSnapshots, target)
	}
	if o.From != nil && !o.From.Before(o.Cutoff) {
		return fmt.Errorf("начало периода (%s) должно быть раньше его конца (%s)",
			o.From.Format(time.DateOnly), o.Cutoff.Format(time.DateOnly))
	}
	return nil
}

// ValidateEntityType проверяет тип сущности по перечню известных: опечатка иначе
// молча даст пустую выборку, и оператор решит, что чистить нечего.
func ValidateEntityType(s string) error {
	for _, known := range models.AllAuditEntities {
		if s == known {
			return nil
		}
	}
	return fmt.Errorf("неизвестный тип сущности %q (доступны: %s)",
		s, strings.Join(models.AllAuditEntities, ", "))
}

// StorageReport - обзор занятого места: вся база, крупнейшие таблицы и остаток по
// прочим. Строится для команды storage, которая только читает.
type StorageReport struct {
	DatabaseBytes int64
	Tables        []TableSize
	OthersBytes   int64
}

// TableSize - размер одной таблицы вместе с её индексами и вынесенными значениями.
// Rows берётся из статистики планировщика (reltuples), а не точным подсчётом: обзор
// не должен читать таблицу целиком, а порядок величины для решения «что чистить»
// статистика даёт. После массового удаления число уточняется автоочисткой.
type TableSize struct {
	Name  string
	Rows  int64
	Bytes int64
}

// StorageOverview собирает обзор: размер базы, top крупнейших таблиц и суммарный
// размер остальных.
func StorageOverview(ctx context.Context, db *gorm.DB, top int) (StorageReport, error) {
	var rep StorageReport
	if err := db.WithContext(ctx).Raw("SELECT pg_database_size(current_database())").Scan(&rep.DatabaseBytes).Error; err != nil {
		return rep, fmt.Errorf("размер базы: %w", err)
	}

	rows := []struct {
		Name  string
		Rows  int64
		Bytes int64
	}{}
	// Разделы журнальных таблиц (request_logs_2026_07_25 и подобные) отсекаются
	// условием relispartition и досчитываются к родителю обходом дерева: иначе перечень
	// забивают три десятка суточных разделов, а итог задваивается. Ветвление по relkind
	// обязательно: pg_partition_tree на обычной таблице возвращает пустое множество, а
	// не саму таблицу, и без CASE все непартиционированные таблицы показали бы ноль.
	if err := db.WithContext(ctx).Raw(`
		SELECT c.relname AS name,
		       CASE WHEN c.relkind = 'p' THEN
		           (SELECT COALESCE(sum(GREATEST(p.reltuples, 0)), 0)::bigint
		              FROM pg_partition_tree(c.oid) t
		              JOIN pg_class p ON p.oid = t.relid)
		       ELSE GREATEST(c.reltuples, 0)::bigint END AS rows,
		       CASE WHEN c.relkind = 'p' THEN
		           (SELECT COALESCE(sum(pg_total_relation_size(t.relid)), 0)
		              FROM pg_partition_tree(c.oid) t)
		       ELSE pg_total_relation_size(c.oid) END AS bytes
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p') AND NOT c.relispartition
		ORDER BY bytes DESC`).Scan(&rows).Error; err != nil {
		return rep, fmt.Errorf("размеры таблиц: %w", err)
	}

	for i, r := range rows {
		if i < top {
			rep.Tables = append(rep.Tables, TableSize{Name: r.Name, Rows: r.Rows, Bytes: r.Bytes})
			continue
		}
		rep.OthersBytes += r.Bytes
	}

	// Статистика планировщика пуста, пока автоочистка не дошла до таблицы: у молодой
	// базы это половина перечня, и обзор показывал бы нули там, где данные есть.
	// Пересчитываем только показываемые таблицы - их немного.
	for i := range rep.Tables {
		if rep.Tables[i].Rows > 0 {
			continue
		}
		var exact int64
		if err := db.WithContext(ctx).Raw(fmt.Sprintf("SELECT count(*) FROM %q", rep.Tables[i].Name)).Scan(&exact).Error; err != nil {
			return rep, fmt.Errorf("подсчёт строк %s: %w", rep.Tables[i].Name, err)
		}
		rep.Tables[i].Rows = exact
	}
	return rep, nil
}

// measureRetentionTable заполняет размер группы: сколько в ней всего записей, сколько
// места занимает таблица со своими индексами и сколько освободит удаление.
//
// Размер берётся по таблице целиком (pg_total_relation_size), а освобождаемое
// оценивается долей строк - точного ответа тут нет и быть не может: строки разной
// длины, а место всё равно достаётся не диску, а самой базе под новые записи.
func measureRetentionTable(ctx context.Context, db *gorm.DB, table string, res *RetentionResult) error {
	if err := db.WithContext(ctx).Raw(fmt.Sprintf("SELECT count(*) FROM %s", table)).Scan(&res.TotalRows).Error; err != nil {
		return fmt.Errorf("подсчёт строк %s: %w", table, err)
	}
	if err := db.WithContext(ctx).Raw("SELECT pg_total_relation_size(?::regclass)", table).Scan(&res.TableBytes).Error; err != nil {
		return fmt.Errorf("размер %s: %w", table, err)
	}
	if res.TotalRows > 0 {
		res.FreedBytes = res.TableBytes * res.Matched / res.TotalRows
	}
	return nil
}

// SweepRoutine - суточная уборка технического мусора: недействительные токены,
// прочитанные уведомления, непрочитанные уведомления (по своему, более мягкому сроку -
// unreadNotificationDays) и подписки Web Push без единой успешной доставки (#974). Ошибка
// одной группы не отменяет остальные: это обслуживание, а не транзакция.
func SweepRoutine(ctx context.Context, db *gorm.DB, tokenDays, notificationDays, unreadNotificationDays, pushSubscriptionDays int) {
	now := time.Now().UTC()
	plan := []struct {
		target RetentionTarget
		cutoff time.Time
	}{
		{TargetTokens, now.AddDate(0, 0, -tokenDays)},
		{TargetNotifications, now.AddDate(0, 0, -notificationDays)},
		{TargetUnreadNotifications, now.AddDate(0, 0, -unreadNotificationDays)},
		{TargetPushSubscriptions, now.AddDate(0, 0, -pushSubscriptionDays)},
	}
	for _, p := range plan {
		res, err := SweepRetention(ctx, db, p.target, SweepOptions{Cutoff: p.cutoff, Apply: true})
		if err != nil {
			slog.Error("уборка не выполнена", "target", p.target, "error", err)
			continue
		}
		if res.Deleted > 0 {
			slog.Info("уборка выполнена", "target", p.target, "deleted", res.Deleted, "older_than", p.cutoff.Format(time.DateOnly))
		}
	}
}
