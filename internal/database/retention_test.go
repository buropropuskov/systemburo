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

	res, err := database.SweepRetention(context.Background(), db, database.TargetTokens, now.AddDate(0, 0, -30), true)
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

	_, err := database.SweepRetention(context.Background(), db, database.TargetNotifications, now.AddDate(0, 0, -30), true)
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

	res, err := database.SweepRetention(context.Background(), db, database.TargetAudit, now.AddDate(-3, 0, 0), true)
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

	res, err := database.SweepRetention(context.Background(), db, database.TargetAudit, now.AddDate(-3, 0, 0), false)
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

	_, err := database.SweepRetention(context.Background(), db, database.TargetSnapshots, now.AddDate(-1, 0, 0), true)
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
