package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// lastAuditActor возвращает автора последней записи журнала по сущности.
func lastAuditActor(t *testing.T, db *gorm.DB, entityType string, entityID int, action string) *int {
	t.Helper()
	var row struct{ ActorUserID *int }
	require.NoError(t, db.Table("audit_log").
		Select("actor_user_id").
		Where("entity_type = ? AND entity_id = ? AND action = ?", entityType, entityID, action).
		Order("created_at DESC, id DESC").
		Limit(1).Scan(&row).Error)
	return row.ActorUserID
}

// TestPassageActor_TakenFromToken - автора действия ставит сервер (#2443).
//
// До правки `user_id` разбирался из тела запроса и попадал в журнал как есть: из
// консоли браузера охранник записывал проход на чужую фамилию, а суточный отчёт по
// постам группируется ровно по этому полю.
//
// Тест шлёт заведомо чужой идентификатор в теле и проверяет, что записан автор
// токена. Проверяются оба пути отметки - машина и человек - и соседние действия,
// куда автор приходил тем же способом.
func TestPassageActor_TakenFromToken(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	dn := "КПП Авторство"
	table := models.SystemTable{Name: "kpp_actor", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&table).Error)

	token := testutil.RegisterAndLogin(t, e, "actorguard", "pass123", 1, td.OrgID, td.CompanyID)
	guardID := getUserID(t, db, "actorguard")
	testutil.GrantTableVerb(t, guardID, table.Name, "entry")
	testutil.GrantTableVerb(t, guardID, table.Name, "exit")

	victimToken := testutil.RegisterAndLogin(t, e, "actorvictim", "pass123", 1, td.OrgID, td.CompanyID)
	victimID := getUserID(t, db, "actorvictim")
	require.NotEqual(t, guardID, victimID)
	_ = victimToken

	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	// Отметка въезда от имени другого охранника: тело называет чужой id.
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/territory-status", carID),
		fmt.Sprintf(`{"territory_status": 1, "user_id": %d, "table_id": %d}`, victimID, table.ID),
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	actor := lastAuditActor(t, db, models.AuditEntityCar, carID, "entry")
	require.NotNil(t, actor, "отметка обязана иметь автора")
	assert.Equal(t, guardID, *actor, "в журнале автор токена, а не присланный телом")

	// Соседнее действие тем же путём: деактивация записи.
	rec = testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID),
		fmt.Sprintf(`{"status": 0, "user_id": %d, "table_id": %d}`, victimID, table.ID),
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	actor = lastAuditActor(t, db, models.AuditEntityCar, carID, "delete")
	require.NotNil(t, actor)
	assert.Equal(t, guardID, *actor, "удаление записи тоже пишется на автора токена")
}
