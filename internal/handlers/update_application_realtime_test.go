package handlers_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// TestUpdateApplication_ApprovedFiresAvailableNew закрывает admin-путь #840 V3:
// прямое выставление confirmation='Согласовано' через UpdateApplication (минуя
// approve-флоу) обязано слать available.new (доступность вложений охране) и
// application.updated (изменение детали). Раньше этот путь сигналов не слал.
func TestUpdateApplication_ApprovedFiresAvailableNew(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Обновляем как супер-админ (обходит гейты доступа/ЧС на прямом set).
	testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID) // testadmin, super
	uaID := seedUniqueAttachment(t, db, "cars", "cars_upd_rt", "Cars UpdRT")

	fake := &fakePublisher{}
	resolver := services.NewPermissionResolver(db)
	availableProducer := services.NewAvailableRefreshPublisher(db, resolver, fake)
	recorder := services.NewAuditRecorder(db)
	svc := services.NewApplicationService(
		db,
		services.NewPermissionService(db),
		services.NewNotificationService(db),
		services.NewVehicleBlacklistService(db, recorder),
		services.NewPersonBlacklistService(db, recorder),
		recorder,
		services.WithRealtimePublisher(fake),
		services.WithApplicationAvailableProducer(availableProducer),
	)

	from := "2026-04-01"
	to := "2099-12-31"
	req := services.CompleteApplicationRequest{
		Organization:      "Test Organization",
		ResponsiblePerson: "Test Person",
		ContactPhone:      "+79001234567",
		DataApproval:      true,
		Attachments: []services.AttachmentData{{
			AttachmentType:        "cars",
			AttachmentName:        "cars_template",
			AttachmentDisplayName: "Cars Template",
			UniqueAttachmentID:    uaID,
			EntryDateFrom:         &from,
			EntryDateTo:           &to,
			Data: services.AttachmentContentData{
				Vehicles: &[]services.VehicleInput{{CarNumber: "A005AA777", CarBrand: "Toyota"}},
			},
		}},
	}
	created, err := svc.SubmitCompleteApplication(context.Background(), "testadmin", req, true)
	require.NoError(t, err)

	fake.reset() // отбрасываем сигналы submit

	approved := "Согласовано"
	_, err = svc.UpdateApplication(context.Background(), "testadmin", created.ApplicationID,
		services.ApplicationUpdateRequest{Confirmation: &approved})
	require.NoError(t, err)

	types := map[string]bool{}
	fake.mu.Lock()
	for _, ev := range fake.events {
		types[ev.Type] = true
	}
	fake.mu.Unlock()

	assert.True(t, types["available.new"], "прямое Согласовано должно послать available.new")
	assert.True(t, types["application.updated"], "изменение заявки должно послать application.updated")
}
