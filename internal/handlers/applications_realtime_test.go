package handlers_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"systemburo/internal/realtime"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// fakePublisher записывает вызовы PublishMany, чтобы проверить аудиторию
// real-time сигнала обновления Центра (#840) без поднятия реального хаба.
type fakePublisher struct {
	mu        sync.Mutex
	audiences [][]int
	events    []realtime.Event
}

func (f *fakePublisher) Publish(userID int, ev realtime.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.audiences = append(f.audiences, []int{userID})
	f.events = append(f.events, ev)
}

func (f *fakePublisher) PublishMany(userIDs []int, ev realtime.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.audiences = append(f.audiences, userIDs)
	f.events = append(f.events, ev)
}

// TestSubmitCompleteApplication_CenterAudienceIncludesReader защищает аудиторию
// real-time сигнала при создании заявки (#840): она обязана зеркалить фильтр
// видимости списка Центра целиком, включая читателей-получателей (#884,
// application_viewers), а не только отправителя/ответственных/принимающих.
// Юнит на mergeUniqueIDs покрывает лишь слияние; класс "забыл ветку фильтра"
// ловится только сквозным вызовом submit -> centerAudience -> PublishMany.
func TestSubmitCompleteApplication_CenterAudienceIncludesReader(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "rtsender", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "rtreader", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "rtsender")
	readerID := getUserID(t, db, "rtreader")
	uaID := seedUniqueAttachment(t, db, "cars", "cars_rt", "Cars RT")

	fake := &fakePublisher{}
	recorder := services.NewAuditRecorder(db)
	svc := services.NewApplicationService(
		db,
		services.NewPermissionService(db),
		services.NewNotificationService(db),
		services.NewVehicleBlacklistService(db, recorder),
		services.NewPersonBlacklistService(db, recorder),
		recorder,
		services.WithRealtimePublisher(fake),
	)

	from := "2026-04-01"
	to := "2099-12-31"
	readers := []int{readerID}
	req := services.CompleteApplicationRequest{
		Organization:      "Test Organization",
		ResponsiblePerson: "Test Person",
		ContactPhone:      "+79001234567",
		DataApproval:      true,
		Readers:           &readers,
		Attachments: []services.AttachmentData{{
			AttachmentType:        "cars",
			AttachmentName:        "cars_template",
			AttachmentDisplayName: "Cars Template",
			UniqueAttachmentID:    uaID,
			EntryDateFrom:         &from,
			EntryDateTo:           &to,
			Data: services.AttachmentContentData{
				Vehicles: &[]services.VehicleInput{{CarNumber: "A002AA777", CarBrand: "Toyota"}},
			},
		}},
	}

	_, err := svc.SubmitCompleteApplication(context.Background(), "rtsender", req, true)
	require.NoError(t, err)

	require.NotEmpty(t, fake.audiences, "PublishMany должен быть вызван при создании заявки")
	last := fake.audiences[len(fake.audiences)-1]
	assert.Contains(t, last, senderID, "аудитория должна включать отправителя")
	assert.Contains(t, last, readerID, "аудитория должна включать читателя-получателя (#884)")

	lastEv := fake.events[len(fake.events)-1]
	assert.Equal(t, "applications.refresh", lastEv.Type)
	assert.Equal(t, "applications-center", lastEv.Scope)
}
