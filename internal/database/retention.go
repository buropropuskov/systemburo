package database

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Уборка накопленных данных (#1614). Партиционированные журналы (request_logs,
// pd_audit_logs) чистятся дропом партиций в MaintainLogPartitions - здесь всё
// остальное, где удалять приходится строками.
//
// Две группы с разной политикой. Токены сессий и прочитанные уведомления
// обесцениваются сами по себе, их сметает суточный воркер (SweepRoutine).
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
	// TargetAudit - история сущностей. Чистится с оговорками, см. auditRetentionWhere.
	TargetAudit RetentionTarget = "audit"
	// TargetSnapshots - суточные слепки таблиц постов. Ручные снимки не трогаются:
	// их делают осознанно и под конкретную задачу.
	TargetSnapshots RetentionTarget = "snapshots"
	// TargetRequestAggregates - дневные агрегаты журнала запросов. Сами детальные
	// записи дропаются партициями за REQUEST_LOG_DETAIL_DAYS, а свёртка копилась вечно.
	TargetRequestAggregates RetentionTarget = "request-aggregates"
)

// AllRetentionTargets - порядок вывода в отчёте: сначала мусор, потом то, что
// удаляется осознанно.
var AllRetentionTargets = []RetentionTarget{
	TargetTokens, TargetNotifications, TargetAudit, TargetSnapshots, TargetRequestAggregates,
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
		defaultAge:  func(now time.Time) time.Time { return now.AddDate(0, 0, -30) },
		description: "недействительные токены сессий",
	},
	TargetNotifications: {
		table:       "notifications",
		where:       "is_read AND created_at < ?",
		cutoffArgs:  1,
		defaultAge:  func(now time.Time) time.Time { return now.AddDate(0, 0, -30) },
		description: "прочитанные уведомления",
	},
	TargetAudit: {
		table:       "audit_log",
		where:       auditRetentionWhere,
		cutoffArgs:  1,
		defaultAge:  func(now time.Time) time.Time { return now.AddDate(-3, 0, 0) },
		description: "история сущностей, кроме корзины и последних отметок прохода",
	},
	TargetSnapshots: {
		table:       "table_snapshots",
		where:       "reason = 'scheduled' AND taken_at < ?",
		cutoffArgs:  1,
		defaultAge:  func(now time.Time) time.Time { return now.AddDate(-1, 0, 0) },
		description: "суточные слепки таблиц постов, кроме ручных",
	},
	TargetRequestAggregates: {
		table:       "request_logs_daily",
		where:       "day < (?)::date",
		cutoffArgs:  1,
		defaultAge:  func(now time.Time) time.Time { return now.AddDate(-2, 0, 0) },
		description: "дневные агрегаты журнала запросов",
	},
}

// RetentionResult - итог по одной группе. Matched считается всегда, Deleted остаётся
// нулём в режиме предварительного показа.
type RetentionResult struct {
	Target      RetentionTarget
	Description string
	Cutoff      time.Time
	Matched     int64
	Deleted     int64
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
func SweepRetention(ctx context.Context, db *gorm.DB, target RetentionTarget, cutoff time.Time, apply bool) (RetentionResult, error) {
	rule, ok := retentionRules[target]
	if !ok {
		return RetentionResult{}, fmt.Errorf("неизвестная группа %q", target)
	}
	res := RetentionResult{Target: target, Description: rule.description, Cutoff: cutoff}

	args := make([]interface{}, rule.cutoffArgs)
	for i := range args {
		args[i] = cutoff
	}

	countSQL := fmt.Sprintf("SELECT count(*) FROM %s WHERE %s", rule.table, rule.where)
	if err := db.WithContext(ctx).Raw(countSQL, args...).Scan(&res.Matched).Error; err != nil {
		return res, fmt.Errorf("подсчёт %s: %w", rule.table, err)
	}
	if !apply || res.Matched == 0 {
		return res, nil
	}

	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE %s", rule.table, rule.where)
	tx := db.WithContext(ctx).Exec(deleteSQL, args...)
	if tx.Error != nil {
		return res, fmt.Errorf("удаление из %s: %w", rule.table, tx.Error)
	}
	res.Deleted = tx.RowsAffected
	return res, nil
}

// SweepRoutine - суточная уборка технического мусора: недействительные токены и
// прочитанные уведомления. Ошибка одной группы не отменяет вторую: это обслуживание,
// а не транзакция.
func SweepRoutine(ctx context.Context, db *gorm.DB, tokenDays, notificationDays int) {
	now := time.Now().UTC()
	plan := []struct {
		target RetentionTarget
		cutoff time.Time
	}{
		{TargetTokens, now.AddDate(0, 0, -tokenDays)},
		{TargetNotifications, now.AddDate(0, 0, -notificationDays)},
	}
	for _, p := range plan {
		res, err := SweepRetention(ctx, db, p.target, p.cutoff, true)
		if err != nil {
			slog.Error("уборка не выполнена", "target", p.target, "error", err)
			continue
		}
		if res.Deleted > 0 {
			slog.Info("уборка выполнена", "target", p.target, "deleted", res.Deleted, "older_than", p.cutoff.Format(time.DateOnly))
		}
	}
}
