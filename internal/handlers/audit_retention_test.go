package handlers_test

// Срок хранения истории сущностей в автоматической уборке (#2355).
//
// Срок у группы audit был описан (три года), но применялся только вручную командой
// cleanup: автоматическая уборка его не трогала, и записи с ФИО в пояснениях к отметкам
// прохода («Сотрудник Иванов Иван прошёл на территорию») копились бессрочно.
//
// Имена в журнале при этом НЕ затираются - он доказывает, кто и когда был на объекте.
// Уходят они вместе с самой записью, по сроку.

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func auditEntry(t *testing.T, db *gorm.DB, action string, at time.Time) int {
	t.Helper()
	var id int
	require.NoError(t, db.Raw(`
		INSERT INTO audit_log (entity_type, entity_id, action, details, created_at)
		VALUES ('employee', 4242, ?, '{"comment": "Сотрудник Журналов Иван прошёл"}'::jsonb, ?)
		RETURNING id`, action, at).Scan(&id).Error)
	return id
}

func auditExists(t *testing.T, db *gorm.DB, id int) bool {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM audit_log WHERE id = ?`, id).Scan(&n).Error)
	return n > 0
}

func TestAuditRetention_SweptOnlyWhenTermSet(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	now := time.Now().UTC()
	old := auditEntry(t, db, "data_changed", now.AddDate(-4, 0, 0))

	// Ноль означает «срок не назначен»: история не трогается вовсе.
	database.SweepRoutine(context.Background(), db, 30, 30, 90, 180, 30, 0)
	assert.True(t, auditExists(t, db, old), "без назначенного срока история не чистится")

	// Срок назначен - запись старше его уходит.
	database.SweepRoutine(context.Background(), db, 30, 30, 90, 180, 30, 36)
	assert.False(t, auditExists(t, db, old), "запись старше назначенного срока обязана уйти")
}

// TestAuditRetention_KeepsLastPassage - последние отметки входа и выхода остаются
// независимо от срока: из них считается «последний выезд» в карточке человека, и у
// редко приезжающего он может быть старше любого разумного срока.
func TestAuditRetention_KeepsLastPassage(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	now := time.Now().UTC()
	lastEntry := auditEntry(t, db, "entry", now.AddDate(-5, 0, 0))
	changed := auditEntry(t, db, "data_changed", now.AddDate(-5, 0, 0))

	database.SweepRoutine(context.Background(), db, 30, 30, 90, 180, 30, 12)

	assert.True(t, auditExists(t, db, lastEntry), "последняя отметка прохода нужна карточке")
	assert.False(t, auditExists(t, db, changed), "обычная запись истории уходит по сроку")
}

// Обезличивание человека и заявки журнал не трогает: решение принято осознанно -
// журнал доказывает факт нахождения на объекте.
func TestAuditRetention_AnonymizeKeepsNames(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	id := auditEntry(t, db, "entry", time.Now().UTC())

	var details string
	require.NoError(t, db.Raw(`SELECT details::text FROM audit_log WHERE id = ?`, id).Scan(&details).Error)
	assert.Contains(t, details, "Журналов Иван",
		"имя в журнале остаётся: подмены на номер быть не должно")
	_ = models.AuditEntityEmployee
}
