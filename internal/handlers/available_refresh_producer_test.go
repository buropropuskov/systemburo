package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// TestAvailableAudience_MirrorsTabAccess: аудитория available.new зеркалит гейт
// вкладки "Доступные мне" (#840 V3, #976): тип "Охранник" ИЛИ носитель права
// page.available - в аудитории; обычный юзер без права - нет. Сигнал безданных
// (event-then-fetch), поэтому аудитория - безопасный суперсет доступа к вкладке.
func TestAvailableAudience_MirrorsTabAccess(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	secTypeID := secUserTypeIDByCode(t, db, "security")
	testutil.RegisterAndLogin(t, e, "av_sec", "pass123", secTypeID, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "av_granted", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "av_normal", "pass123", 1, td.OrgID, td.CompanyID)

	secID := getUserID(t, db, "av_sec")
	grantedID := getUserID(t, db, "av_granted")
	normalID := getUserID(t, db, "av_normal")

	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        grantedID,
		PermissionKey: services.KeyPageAvailable,
		Value:         "allow",
		GrantedAt:     time.Now(),
	}).Error)

	fake := &fakePublisher{}
	resolver := services.NewPermissionResolver(db)
	producer := services.NewAvailableRefreshPublisher(db, resolver, fake)

	producer.NotifyAvailableChanged(context.Background())

	require.NotEmpty(t, fake.audiences, "NotifyAvailableChanged должен опубликовать available.new")
	aud := fake.audiences[len(fake.audiences)-1]
	assert.Contains(t, aud, secID, "тип Охранник видит вкладку -> в аудитории")
	assert.Contains(t, aud, grantedID, "носитель page.available -> в аудитории")
	assert.NotContains(t, aud, normalID, "обычный юзер без права -> не в аудитории")

	ev := fake.events[len(fake.events)-1]
	assert.Equal(t, "available.new", ev.Type)
	assert.Equal(t, "available", ev.Scope)
}
