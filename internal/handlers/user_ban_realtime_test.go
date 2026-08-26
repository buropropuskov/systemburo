package handlers_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// TestUserBan_PublishesBanEventToTarget: бан/разбан шлёт адресный real-time
// сигнал (scope user:<id>) ТОЛЬКО заблокированному - его App.vue перезапросит
// права (баннер ЧС + блокировка UI) без ожидания 30с-опроса (#840). Актор
// сигнал не получает: аудитория адресная, а не broadcast.
func TestUserBan_PublishesBanEventToTarget(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "banactor", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "bantarget", "pass123", 1, td.OrgID, td.CompanyID)
	actorID := getUserID(t, db, "banactor")
	targetID := getUserID(t, db, "bantarget")

	fake := &fakePublisher{}
	svc := services.NewUserBanService(db, services.NewPermissionResolver(db), nil,
		services.NewAuditRecorder(db), services.WithBanRealtimePublisher(fake))

	// Бан -> user.banned адресно цели.
	require.NoError(t, svc.Ban(context.Background(), targetID, actorID, "Нарушение режима"))

	require.NotEmpty(t, fake.audiences, "бан должен опубликовать user.banned")
	banAudience := fake.audiences[len(fake.audiences)-1]
	assert.Equal(t, []int{targetID}, banAudience, "сигнал бана адресован только заблокированному")
	assert.NotContains(t, banAudience, actorID, "актор (кто банил) сигнал бана не получает")

	banEv := fake.events[len(fake.events)-1]
	assert.Equal(t, "user.banned", banEv.Type)
	assert.Equal(t, fmt.Sprintf("user:%d", targetID), banEv.Scope)

	// Разбан -> user.unbanned адресно цели.
	require.NoError(t, svc.Unban(context.Background(), targetID, actorID))

	unbanAudience := fake.audiences[len(fake.audiences)-1]
	assert.Equal(t, []int{targetID}, unbanAudience, "сигнал разбана адресован только разблокированному")

	unbanEv := fake.events[len(fake.events)-1]
	assert.Equal(t, "user.unbanned", unbanEv.Type)
	assert.Equal(t, fmt.Sprintf("user:%d", targetID), unbanEv.Scope)
}
