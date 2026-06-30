package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestUserBan_AuditWritesToUserHistory проверяет, что ban/unban пишут историю
// в audit_log[user] (#936): кто/когда/причина -- рядом с created/updated в той же
// модалке истории пользователя. Раньше UserBanService менял только текущее
// состояние users.is_banned и историю нигде не фиксировал.
func TestUserBan_AuditWritesToUserHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Актор -- супер-админ (testadmin), проходит action.ban.user и page.admin.users.
	actorToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	actorID := getUserID(t, db, "testadmin")
	h := testutil.AuthHeader(actorToken)

	// Цель -- обычный пользователь (не супер-админ, иначе бан запрещён).
	testutil.RegisterAndLogin(t, e, "bantarget", "pass123", 1, td.OrgID, td.CompanyID)
	targetID := getUserID(t, db, "bantarget")

	const reason = "Нарушение режима доступа"

	// 1. Бан с причиной -> строка audit_log[user] action=banned с актором и reason.
	recBan := testutil.POST(t, e, fmt.Sprintf("/users/%d/ban", targetID), fmt.Sprintf(`{"reason":%q}`, reason), h)
	require.Equal(t, http.StatusOK, recBan.Code, recBan.Body.String())

	banned := findUserAudit(t, db, targetID, models.UserActionBanned)
	require.NotNil(t, banned, "бан должен писать audit_log[user] action=banned")
	require.NotNil(t, banned.ActorUserID, "у записи бана должен быть актор")
	assert.Equal(t, actorID, *banned.ActorUserID, "актор бана = тот, кто банил")
	var banDetails struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(banned.Details, &banDetails))
	assert.Equal(t, reason, banDetails.Reason, "причина бана сохранена в details")

	// 2. Событие видно в модалке истории (GET /users/:username/history).
	recHist := testutil.GET(t, e, "/users/bantarget/history", h)
	require.Equal(t, http.StatusOK, recHist.Code, recHist.Body.String())
	items := testutil.ParseSlice(t, recHist)
	bannedItem := findHistoryItem(items, models.UserActionBanned)
	require.NotNil(t, bannedItem, "запись banned отдаётся в истории пользователя")
	details, _ := bannedItem["details"].(map[string]interface{})
	require.NotNil(t, details, "у banned-записи в истории есть details")
	assert.Equal(t, reason, details["reason"], "история показывает причину бана")

	// 3. Разбан -> строка audit_log[user] action=unbanned с актором.
	recUnban := testutil.POST(t, e, fmt.Sprintf("/users/%d/unban", targetID), `{}`, h)
	require.Equal(t, http.StatusOK, recUnban.Code, recUnban.Body.String())

	unbanned := findUserAudit(t, db, targetID, models.UserActionUnbanned)
	require.NotNil(t, unbanned, "разбан должен писать audit_log[user] action=unbanned")
	require.NotNil(t, unbanned.ActorUserID, "у записи разбана должен быть актор")
	assert.Equal(t, actorID, *unbanned.ActorUserID, "актор разбана = тот, кто разбанил")
	// Разбан фиксирует снятую причину и момент начала блокировки -- по ним
	// модалка истории показывает "был в блокировке <срок>, причина: ...".
	var unbanDetails struct {
		Reason   string `json:"reason"`
		BannedAt string `json:"banned_at"`
	}
	require.NoError(t, json.Unmarshal(unbanned.Details, &unbanDetails))
	assert.Equal(t, reason, unbanDetails.Reason, "разбан фиксирует снятую причину блокировки")
	assert.NotEmpty(t, unbanDetails.BannedAt, "разбан фиксирует момент начала блокировки для расчёта срока")

	recHist2 := testutil.GET(t, e, "/users/bantarget/history", h)
	require.Equal(t, http.StatusOK, recHist2.Code, recHist2.Body.String())
	require.NotNil(t, findHistoryItem(testutil.ParseSlice(t, recHist2), models.UserActionUnbanned),
		"запись unbanned отдаётся в истории пользователя")
}

// findUserAudit возвращает самую свежую запись audit_log[user] заданного action.
func findUserAudit(t *testing.T, db *gorm.DB, userID int, action string) *models.AuditLog {
	t.Helper()
	var entry models.AuditLog
	err := db.Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityUser, userID, action).
		Order("id DESC").First(&entry).Error
	if err != nil {
		return nil
	}
	return &entry
}

// findHistoryItem ищет в ответе /history запись с заданным action_type.
func findHistoryItem(items []map[string]interface{}, action string) map[string]interface{} {
	for _, it := range items {
		if it["action_type"] == action {
			return it
		}
	}
	return nil
}
