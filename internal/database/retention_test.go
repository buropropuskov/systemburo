package database_test

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Тесты делят базу с остальным пакетом, поэтому проверяют судьбу СВОИХ строк по
// идентификаторам, а не общие счётчики таблицы.

// createRetentionUser заводит владельца токенов и уведомлений.
func createRetentionUser(t *testing.T, db *gorm.DB, username string) models.User {
	t.Helper()
	user := models.User{Username: username, Password: "x"}
	require.NoError(t, db.Create(&user).Error)
	return user
}

// insertAudit кладёт запись истории напрямую и возвращает её id.
func insertAudit(t *testing.T, db *gorm.DB, entityType string, entityID int, action string, createdAt time.Time) int {
	t.Helper()
	var id int
	require.NoError(t, db.Raw(
		`INSERT INTO audit_log (entity_type, entity_id, action, created_at) VALUES (?,?,?,?) RETURNING id`,
		entityType, entityID, action, createdAt,
	).Scan(&id).Error)
	return id
}

func auditExists(t *testing.T, db *gorm.DB, id int) bool {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM audit_log WHERE id = ?`, id).Scan(&n).Error)
	return n > 0
}

// TestSweepRetention_TokensKeepsValid проверяет, что уборка токенов сносит истёкшие
// и отозванные, но не трогает действующий.
func TestSweepRetention_TokensKeepsValid(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := createRetentionUser(t, db, "retention-tokens")
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -90)

	newToken := func(hash string, expires time.Time, revoked bool, revokedAt *time.Time) int {
		var id int
		require.NoError(t, db.Raw(`
			INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at, created_at, is_revoked, revoked_at)
			VALUES (?,?,?,?,?,?,?) RETURNING id`,
			user.ID, "fam-"+hash, hash, expires, old, revoked, revokedAt,
		).Scan(&id).Error)
		return id
	}
	revokedAt := old
	expired := newToken("retention-expired", old, false, nil)
	revoked := newToken("retention-revoked", now.AddDate(0, 0, 7), true, &revokedAt)
	alive := newToken("retention-alive", now.AddDate(0, 0, 7), false, nil)

	res, err := database.SweepRetention(context.Background(), db, database.TargetTokens, database.SweepOptions{Cutoff: now.AddDate(0, 0, -30), Apply: true})
	require.NoError(t, err)
	require.GreaterOrEqual(t, res.Deleted, int64(2))

	tokenExists := func(id int) bool {
		var n int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM refresh_tokens WHERE id = ?`, id).Scan(&n).Error)
		return n > 0
	}
	require.False(t, tokenExists(expired), "истёкший токен должен быть удалён")
	require.False(t, tokenExists(revoked), "отозванный токен должен быть удалён")
	require.True(t, tokenExists(alive), "действующий токен трогать нельзя")
}

// TestSweepRetention_NotificationsOnlyRead проверяет, что уборка уведомлений сносит
// только прочитанные и только старые.
func TestSweepRetention_NotificationsOnlyRead(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := createRetentionUser(t, db, "retention-notif")
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -90)

	newNotification := func(title string, isRead bool, createdAt time.Time) int {
		var id int
		require.NoError(t, db.Raw(
			`INSERT INTO notifications (user_id, title, is_read, created_at) VALUES (?,?,?,?) RETURNING id`,
			user.ID, title, isRead, createdAt,
		).Scan(&id).Error)
		return id
	}
	readOld := newNotification("прочитанное старое", true, old)
	unreadOld := newNotification("непрочитанное старое", false, old)
	readFresh := newNotification("прочитанное свежее", true, now)

	_, err := database.SweepRetention(context.Background(), db, database.TargetNotifications, database.SweepOptions{Cutoff: now.AddDate(0, 0, -30), Apply: true})
	require.NoError(t, err)

	exists := func(id int) bool {
		var n int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM notifications WHERE id = ?`, id).Scan(&n).Error)
		return n > 0
	}
	require.False(t, exists(readOld), "прочитанное старое уведомление должно быть удалено")
	require.True(t, exists(unreadOld), "непрочитанное удалять нельзя, сколько бы ему ни было")
	require.True(t, exists(readFresh), "свежее уведомление удалять рано")
}

// TestSweepRetention_UnreadNotificationsOwnThreshold проверяет вторую цель уборки
// уведомлений (#1748, S9): непрочитанные удаляются по своему, более мягкому сроку,
// а не по короткому сроку прочитанных - и наоборот, прочитанные не задерживаются
// до мягкого срока непрочитанных.
func TestSweepRetention_UnreadNotificationsOwnThreshold(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := createRetentionUser(t, db, "retention-notif-unread")
	now := time.Now().UTC()
	// old старше срока прочитанных (30d по умолчанию), но моложе срока непрочитанных
	// (90d по умолчанию) - именно на этом окне разница между целями видна.
	old := now.AddDate(0, 0, -60)
	veryOld := now.AddDate(0, 0, -120)

	newNotification := func(title string, isRead bool, createdAt time.Time) int {
		var id int
		require.NoError(t, db.Raw(
			`INSERT INTO notifications (user_id, title, is_read, created_at) VALUES (?,?,?,?) RETURNING id`,
			user.ID, title, isRead, createdAt,
		).Scan(&id).Error)
		return id
	}
	unreadOld := newNotification("непрочитанное 60 дней", false, old)
	unreadVeryOld := newNotification("непрочитанное 120 дней", false, veryOld)
	unreadFresh := newNotification("непрочитанное свежее", false, now)
	readOld := newNotification("прочитанное 60 дней", true, old)

	exists := func(id int) bool {
		var n int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM notifications WHERE id = ?`, id).Scan(&n).Error)
		return n > 0
	}

	// Срок прочитанных (30d): "старое" непрочитанное (60d) под него формально
	// попадает по возрасту, но условие "is_read" его исключает - должно остаться.
	_, err := database.SweepRetention(context.Background(), db, database.TargetNotifications,
		database.SweepOptions{Cutoff: now.AddDate(0, 0, -30), Apply: true})
	require.NoError(t, err)
	require.True(t, exists(unreadOld), "непрочитанное не должно уйти по сроку прочитанных")
	require.False(t, exists(readOld), "прочитанное 60-дневной давности должно уйти по своему сроку")

	// Срок непрочитанных (90d): 60-дневное остаётся, 120-дневное и свежее ведут себя
	// как и положено - старое уходит, свежее остаётся.
	_, err = database.SweepRetention(context.Background(), db, database.TargetUnreadNotifications,
		database.SweepOptions{Cutoff: now.AddDate(0, 0, -90), Apply: true})
	require.NoError(t, err)
	require.True(t, exists(unreadOld), "непрочитанное младше своего срока трогать нельзя")
	require.False(t, exists(unreadVeryOld), "непрочитанное старше своего срока должно быть удалено")
	require.True(t, exists(unreadFresh), "свежее непрочитанное удалять рано")
}

// TestSweepRetention_PushSubscriptionsFreshNotSwept защищает push-подписки (#974) от
// уборки раньше срока: свежая подписка без единой успешной доставки (last_success_at
// NULL) отсчитывается от created_at, а не считается устаревшей сразу же. Дешёвый тест
// против опечатки в знаке сравнения COALESCE(last_success_at, created_at) < cutoff,
// которую в диффе глазами не видно (разбор team-lead, #974, пункт 5).
func TestSweepRetention_PushSubscriptionsFreshNotSwept(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := createRetentionUser(t, db, "retention-push")
	now := time.Now().UTC()

	newSub := func(endpoint string, createdAt time.Time, lastSuccessAt *time.Time) int {
		var id int
		require.NoError(t, db.Raw(
			`INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, created_at, last_success_at)
			 VALUES (?,?,?,?,?,?) RETURNING id`,
			user.ID, endpoint, "p256dh", "auth", createdAt, lastSuccessAt,
		).Scan(&id).Error)
		return id
	}

	fresh := newSub("https://push.example.com/retention-fresh", now.Add(-time.Minute), nil)
	staleNeverSucceeded := newSub("https://push.example.com/retention-stale", now.AddDate(0, 0, -200), nil)
	recentSuccess := now.Add(-time.Hour)
	oldButRecentSuccess := newSub("https://push.example.com/retention-recent-success", now.AddDate(0, 0, -200), &recentSuccess)

	cutoff := database.DefaultRetentionCutoff(database.TargetPushSubscriptions, now)
	_, err := database.SweepRetention(context.Background(), db, database.TargetPushSubscriptions,
		database.SweepOptions{Cutoff: cutoff, Apply: true})
	require.NoError(t, err)

	exists := func(id int) bool {
		var n int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM push_subscriptions WHERE id = ?`, id).Scan(&n).Error)
		return n > 0
	}
	require.True(t, exists(fresh), "подписка минуту назад не должна попадать под уборку")
	require.False(t, exists(staleNeverSucceeded), "подписка без единой доставки за 200 дней должна быть удалена")
	require.True(t, exists(oldButRecentSuccess), "недавняя успешная доставка защищает даже старую подписку")
}

// TestSweepRetention_AuditKeepsTrashAndLastPassage - главная защита уборки истории:
// запись об удалении держит корзину таблицы поста, а последние entry/exit дают
// «последний выезд» в карточке. Обе переживают любой срок хранения.
func TestSweepRetention_AuditKeepsTrashAndLastPassage(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	now := time.Now().UTC()
	old := now.AddDate(-4, 0, 0)
	older := now.AddDate(-5, 0, 0)
	const entity = "zzz-retention-car"

	created := insertAudit(t, db, entity, 1, "create", older)
	firstExit := insertAudit(t, db, entity, 1, "exit", older)
	lastExit := insertAudit(t, db, entity, 1, "exit", old)
	lastEntry := insertAudit(t, db, entity, 1, "entry", old)
	deleted := insertAudit(t, db, entity, 1, "delete", older)
	otherEntityExit := insertAudit(t, db, entity, 2, "exit", older)
	fresh := insertAudit(t, db, entity, 1, "update", now)

	res, err := database.SweepRetention(context.Background(), db, database.TargetAudit, database.SweepOptions{Cutoff: now.AddDate(-3, 0, 0), Apply: true})
	require.NoError(t, err)
	require.GreaterOrEqual(t, res.Deleted, int64(2))

	require.False(t, auditExists(t, db, created), "рядовая старая запись должна быть удалена")
	require.False(t, auditExists(t, db, firstExit), "не последний выезд удаляется как рядовая запись")
	require.True(t, auditExists(t, db, lastExit), "последний выезд держит колонку «последний выезд»")
	require.True(t, auditExists(t, db, lastEntry), "последний въезд сохраняется наравне с выездом")
	require.True(t, auditExists(t, db, deleted), "запись об удалении держит корзину таблицы поста")
	require.True(t, auditExists(t, db, otherEntityExit), "последний выезд считается для каждой сущности отдельно")
	require.True(t, auditExists(t, db, fresh), "свежая запись под срок не попадает")
}

// TestSweepRetention_DryRunDeletesNothing проверяет режим предварительного показа:
// команда считает записи, но база остаётся нетронутой.
func TestSweepRetention_DryRunDeletesNothing(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	now := time.Now().UTC()
	const entity = "zzz-retention-dryrun"
	id := insertAudit(t, db, entity, 1, "update", now.AddDate(-5, 0, 0))

	res, err := database.SweepRetention(context.Background(), db, database.TargetAudit, database.SweepOptions{Cutoff: now.AddDate(-3, 0, 0), Apply: false})
	require.NoError(t, err)
	require.GreaterOrEqual(t, res.Matched, int64(1), "показ должен посчитать подходящие записи")
	require.Zero(t, res.Deleted, "в режиме показа удалять нечего")
	require.True(t, auditExists(t, db, id), "запись должна остаться на месте")
}

// TestSweepRetention_SnapshotsKeepManual проверяет, что ручные слепки таблиц
// переживают уборку суточных.
func TestSweepRetention_SnapshotsKeepManual(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	table := models.SystemTable{Name: "Пост для уборки слепков"}
	require.NoError(t, db.Create(&table).Error)
	now := time.Now().UTC()
	old := now.AddDate(-2, 0, 0)

	newSnapshot := func(reason string) int {
		var id int
		require.NoError(t, db.Raw(`
			INSERT INTO table_snapshots (table_id, taken_at, reason, payload, counts, created_at)
			VALUES (?,?,?,'{}'::jsonb,'{}'::jsonb,?) RETURNING id`,
			table.ID, old, reason, old,
		).Scan(&id).Error)
		return id
	}
	scheduled := newSnapshot("scheduled")
	manual := newSnapshot("manual")

	_, err := database.SweepRetention(context.Background(), db, database.TargetSnapshots, database.SweepOptions{Cutoff: now.AddDate(-1, 0, 0), Apply: true})
	require.NoError(t, err)

	exists := func(id int) bool {
		var n int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM table_snapshots WHERE id = ?`, id).Scan(&n).Error)
		return n > 0
	}
	require.False(t, exists(scheduled), "суточный слепок старше срока должен быть удалён")
	require.True(t, exists(manual), "снятый вручную слепок удалять нельзя")
}

// TestParseRetentionAge проверяет разбор срока в сутках и месяцах.
func TestParseRetentionAge(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)

	days, err := database.ParseRetentionAge("30d", now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC), days)

	months, err := database.ParseRetentionAge("12m", now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2025, 7, 31, 12, 0, 0, 0, time.UTC), months)

	for _, bad := range []string{"", "d", "30", "30y", "-5d", "abcd"} {
		_, err := database.ParseRetentionAge(bad, now)
		require.Error(t, err, "срок %q не должен разбираться", bad)
	}
}

// TestParseRetentionTarget проверяет, что имена групп берутся из белого списка:
// имя таблицы подставляется в SQL, и произвольная строка туда попасть не должна.
func TestParseRetentionTarget(t *testing.T) {
	target, err := database.ParseRetentionTarget(" audit ")
	require.NoError(t, err)
	require.Equal(t, database.TargetAudit, target)

	_, err = database.ParseRetentionTarget("users; DROP TABLE users")
	require.Error(t, err)
}

// TestSelectRetentionTargets проверяет разбор групп: перечень, all, исключение и
// понятные ошибки на мусоре.
func TestSelectRetentionTargets(t *testing.T) {
	all, err := database.SelectRetentionTargets("all", "")
	require.NoError(t, err)
	require.Equal(t, database.AllRetentionTargets, all)

	// Исключение - главный сценарий: «почисти всё, историю не трогай».
	except, err := database.SelectRetentionTargets("all", "audit")
	require.NoError(t, err)
	require.NotContains(t, except, database.TargetAudit)
	require.Len(t, except, len(database.AllRetentionTargets)-1)

	// Порядок вывода общий, повторы схлопываются.
	list, err := database.SelectRetentionTargets("snapshots,tokens,tokens", "")
	require.NoError(t, err)
	require.Equal(t, []database.RetentionTarget{database.TargetTokens, database.TargetSnapshots}, list)

	_, err = database.SelectRetentionTargets("all", "all")
	require.Error(t, err, "исключение всех выбранных групп - ошибка, а не пустая работа")

	_, err = database.SelectRetentionTargets("", "")
	require.Error(t, err, "пустой перечень групп - ошибка")

	_, err = database.SelectRetentionTargets("all", "users; DROP TABLE users")
	require.Error(t, err, "имя группы берётся из белого списка и в исключениях тоже")
}

// TestSweepRetention_ReportsSize проверяет, что показ отдаёт размер группы: без этого
// оператор не поймёт, стоит ли чистить.
func TestSweepRetention_ReportsSize(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	now := time.Now().UTC()
	insertAudit(t, db, "zzz-retention-size", 1, "update", now.AddDate(-5, 0, 0))

	res, err := database.SweepRetention(context.Background(), db, database.TargetAudit, database.SweepOptions{Cutoff: now.AddDate(-3, 0, 0), Apply: false})
	require.NoError(t, err)
	require.Positive(t, res.TotalRows, "всего записей в группе должно считаться")
	require.Positive(t, res.TableBytes, "размер таблицы должен быть известен")
	require.GreaterOrEqual(t, res.TotalRows, res.Matched, "под удаление не может попасть больше, чем есть")
	require.LessOrEqual(t, res.FreedBytes, res.TableBytes, "освободить больше размера таблицы нельзя")
}

// TestStorageOverview проверяет обзор занятого места: база не пуста, таблицы
// перечислены с размерами, разделы журнальных таблиц не попадают отдельными строками.
func TestStorageOverview(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	rep, err := database.StorageOverview(context.Background(), db, 10)
	require.NoError(t, err)
	require.Positive(t, rep.DatabaseBytes, "размер базы должен быть известен")
	require.NotEmpty(t, rep.Tables, "перечень таблиц не должен быть пустым")
	require.LessOrEqual(t, len(rep.Tables), 10, "показывается не больше запрошенного числа таблиц")

	for _, tbl := range rep.Tables {
		require.Positive(t, tbl.Bytes, "у таблицы %s нулевой размер", tbl.Name)
		require.NotRegexp(t, `_\d{4}_\d{2}(_\d{2})?$`, tbl.Name,
			"раздел %s должен считаться внутри родительской таблицы, а не отдельной строкой", tbl.Name)
	}
}

// TestSweepRetention_EntityFilter проверяет сужение истории по типу сущности:
// «почистить только по машинам» не должно задеть заявки.
func TestSweepRetention_EntityFilter(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	now := time.Now().UTC()
	old := now.AddDate(-5, 0, 0)
	car := insertAudit(t, db, models.AuditEntityCar, 9001, "update", old)
	app := insertAudit(t, db, models.AuditEntityApplication, 9002, "update", old)

	res, err := database.SweepRetention(context.Background(), db, database.TargetAudit,
		database.SweepOptions{Cutoff: now.AddDate(-3, 0, 0), EntityType: models.AuditEntityCar, Apply: true})
	require.NoError(t, err)
	require.Positive(t, res.Deleted)

	require.False(t, auditExists(t, db, car), "история по машинам должна быть удалена")
	require.True(t, auditExists(t, db, app), "история заявок под фильтр не попадает")
}

// TestSweepRetention_FromLimitsPeriod проверяет нижнюю границу периода: записи
// старше начала интервала остаются нетронутыми.
func TestSweepRetention_FromLimitsPeriod(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	now := time.Now().UTC()
	const entity = "zzz-retention-period"
	older := insertAudit(t, db, entity, 1, "update", now.AddDate(-8, 0, 0))
	inside := insertAudit(t, db, entity, 2, "update", now.AddDate(-5, 0, 0))

	from := now.AddDate(-6, 0, 0)
	_, err := database.SweepRetention(context.Background(), db, database.TargetAudit,
		database.SweepOptions{Cutoff: now.AddDate(-3, 0, 0), From: &from, EntityType: "", Apply: true})
	require.NoError(t, err)

	require.True(t, auditExists(t, db, older), "запись старше начала периода трогать нельзя")
	require.False(t, auditExists(t, db, inside), "запись внутри периода должна быть удалена")
}

// TestSweepRetention_TableFilter проверяет сужение слепков по таблице поста.
func TestSweepRetention_TableFilter(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	first := models.SystemTable{Name: "Пост фильтра А"}
	second := models.SystemTable{Name: "Пост фильтра Б"}
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)

	now := time.Now().UTC()
	old := now.AddDate(-2, 0, 0)
	newSnapshot := func(tableID int) int {
		var id int
		require.NoError(t, db.Raw(`
			INSERT INTO table_snapshots (table_id, taken_at, reason, payload, counts, created_at)
			VALUES (?,?,'scheduled','{}'::jsonb,'{}'::jsonb,?) RETURNING id`, tableID, old, old).Scan(&id).Error)
		return id
	}
	kept := newSnapshot(first.ID)
	removed := newSnapshot(second.ID)

	_, err := database.SweepRetention(context.Background(), db, database.TargetSnapshots,
		database.SweepOptions{Cutoff: now.AddDate(-1, 0, 0), TableID: &second.ID, Apply: true})
	require.NoError(t, err)

	exists := func(id int) bool {
		var n int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM table_snapshots WHERE id = ?`, id).Scan(&n).Error)
		return n > 0
	}
	require.True(t, exists(kept), "слепки другого поста трогать нельзя")
	require.False(t, exists(removed), "слепки выбранного поста должны быть удалены")
}

// TestSweepRetention_RejectsInapplicableFilter проверяет, что неприменимый фильтр
// отклоняется. Молча его проигнорировать нельзя: оператор просил сузить удаление,
// а получил бы удаление всей группы.
func TestSweepRetention_RejectsInapplicableFilter(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	now := time.Now().UTC()
	tableID := 1

	_, err := database.SweepRetention(context.Background(), db, database.TargetTokens,
		database.SweepOptions{Cutoff: now, EntityType: models.AuditEntityCar, Apply: true})
	require.Error(t, err, "фильтр по сущности к токенам неприменим")

	_, err = database.SweepRetention(context.Background(), db, database.TargetAudit,
		database.SweepOptions{Cutoff: now, TableID: &tableID, Apply: true})
	require.Error(t, err, "фильтр по таблице поста к истории неприменим")

	after := now.AddDate(1, 0, 0)
	_, err = database.SweepRetention(context.Background(), db, database.TargetAudit,
		database.SweepOptions{Cutoff: now, From: &after, Apply: true})
	require.Error(t, err, "начало периода позже его конца - ошибка")
}

// TestValidateEntityType ловит опечатку в типе сущности: без проверки она дала бы
// пустую выборку, и оператор решил бы, что чистить нечего.
func TestValidateEntityType(t *testing.T) {
	require.NoError(t, database.ValidateEntityType(models.AuditEntityEmployee))
	require.Error(t, database.ValidateEntityType("cars"))
	require.Error(t, database.ValidateEntityType(""))
}
