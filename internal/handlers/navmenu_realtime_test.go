package handlers_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// TestCreateFeedback_PublishesFeedbackNew: новое обращение обратной связи шлёт
// feedback.new аудитории бейджа - активным супер-админам (FE гейтит по isSuperAdmin),
// обычный юзер в аудиторию не попадает (#840).
func TestCreateFeedback_PublishesFeedbackNew(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID) // testadmin: super
	testutil.RegisterAndLogin(t, e, "fbuser", "pass123", 1, td.OrgID, td.CompanyID)
	superID := getUserID(t, db, "testadmin")
	normalID := getUserID(t, db, "fbuser")

	fake := &fakePublisher{}
	svc := services.NewFeedbackService(db, services.WithFeedbackRealtimePublisher(fake))

	_, err := svc.Create(context.Background(), "fbuser",
		models.CreateFeedbackRequest{Message: "Тестовое обращение обратной связи по багу"})
	require.NoError(t, err)

	require.NotEmpty(t, fake.audiences, "Create должен опубликовать feedback.new")
	last := fake.audiences[len(fake.audiences)-1]
	assert.Contains(t, last, superID, "супер-админ - в аудитории feedback.new")
	assert.NotContains(t, last, normalID, "обычный юзер не в аудитории (бейдж super-only)")

	ev := fake.events[len(fake.events)-1]
	assert.Equal(t, "feedback.new", ev.Type)
	assert.Equal(t, "feedback", ev.Scope)
}

// TestCreateSystemTable_PublishesTablesRefresh: создание системной таблицы шлёт
// system-tables.refresh всем активным юзерам (список таблиц в нав-меню виден всем)
// (#840).
func TestCreateSystemTable_PublishesTablesRefresh(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "stuser", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "stuser")

	fake := &fakePublisher{}
	svc := services.NewSystemTableService(db, "/tmp", 1024, services.NewPermissionService(db),
		services.WithSystemTableRealtimePublisher(fake))

	_, err := svc.Create(context.Background(), models.CreateSystemTableRequest{
		Name:        "kpp_rt_navmenu",
		DisplayName: "КПП RT",
		TableType:   "cars",
	})
	require.NoError(t, err)

	require.NotEmpty(t, fake.audiences, "Create таблицы должен опубликовать system-tables.refresh")
	last := fake.audiences[len(fake.audiences)-1]
	assert.Contains(t, last, userID, "активный юзер - в broadcast-аудитории")

	ev := fake.events[len(fake.events)-1]
	assert.Equal(t, "system-tables.refresh", ev.Type)
	assert.Equal(t, "system-tables", ev.Scope)
}
