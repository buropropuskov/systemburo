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

// TestCreateNews_PublishesNewsRefresh защищает real-time доставку ленты
// новостей (#840 news.refresh): CreateNews обязан публиковать сигнал
// "news.refresh" всем активным юзерам после успешной вставки, чтобы
// NewsAndReview обновился без F5. Аудитория - все активные аккаунты (новости
// видны всем авторизованным, без гейта прав), в отличие от table.<name>.view
// гейтированных таблиц проходной. fakePublisher переиспользован из
// applications_realtime_test.go.
func TestCreateNews_PublishesNewsRefresh(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "rtnewsuser", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "rtnewsuser")

	fake := &fakePublisher{}
	svc := services.NewNewsService(db, services.WithNewsRealtimePublisher(fake))

	req := models.CreateNewsRequest{Title: "Тестовая новость"}
	_, err := svc.CreateNews(context.Background(), userID, req)
	require.NoError(t, err)

	require.NotEmpty(t, fake.audiences, "PublishMany должен быть вызван при создании новости")
	lastAudience := fake.audiences[len(fake.audiences)-1]
	assert.Contains(t, lastAudience, userID, "аудитория обязана включать зарегистрированного активного юзера")

	lastEv := fake.events[len(fake.events)-1]
	assert.Equal(t, "news.refresh", lastEv.Type)
	assert.Equal(t, "news", lastEv.Scope)
}
