package handlers_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// TestCreateForUser_PublishesNotificationNew защищает real-time доставку
// уведомлений (#840 V1): CreateForUser обязан публиковать сигнал
// "notification.new" адресно юзеру после успешной вставки записи, чтобы
// фронт мгновенно перезапросил колокольчик вместо ожидания 30с-поллинга.
// fakePublisher переиспользован из applications_realtime_test.go.
func TestCreateForUser_PublishesNotificationNew(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "rtnotifuser", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "rtnotifuser")

	fake := &fakePublisher{}
	svc := services.NewNotificationService(db, services.WithNotificationRealtimePublisher(fake))

	err := svc.CreateForUser(context.Background(), userID, "submit", "T", "M", nil)
	require.NoError(t, err)

	require.NotEmpty(t, fake.audiences, "Publish должен быть вызван при создании уведомления")
	last := fake.audiences[len(fake.audiences)-1]
	assert.Equal(t, []int{userID}, last)

	lastEv := fake.events[len(fake.events)-1]
	assert.Equal(t, "notification.new", lastEv.Type)
	assert.Equal(t, "notifications", lastEv.Scope)
}
