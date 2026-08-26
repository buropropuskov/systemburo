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

// TestApplicationUpdated_OnWithdraw защищает продюсер детали заявки (#840 V4):
// мутация заявки (здесь - отзыв) шлёт участникам сигнал application.updated со
// scope application:<id>, чтобы открытая деталь перезапросила статус без F5.
// Аудитория - участники заявки (applicationParticipants = centerAudience),
// минимально включает автора. Хук единый для всех мутаций (approve/forward/
// question/answer/withdraw/...), поэтому один сквозной путь доказывает механику.
func TestApplicationUpdated_OnWithdraw(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "aupdsender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "aupdsender")
	uaID := seedUniqueAttachment(t, db, "cars", "cars_upd", "Cars Upd")

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
				Vehicles: &[]services.VehicleInput{{CarNumber: "A004AA777", CarBrand: "Toyota"}},
			},
		}},
	}
	created, err := svc.SubmitCompleteApplication(context.Background(), "aupdsender", req, true)
	require.NoError(t, err)

	fake.reset() // отбрасываем applications.refresh от submit

	require.NoError(t, svc.WithdrawApplication(context.Background(), "aupdsender", created.ApplicationID))

	scope := fmt.Sprintf("application:%d", created.ApplicationID)
	var audience []int
	found := false
	for i, ev := range fake.events {
		if ev.Type == "application.updated" && ev.Scope == scope {
			found = true
			audience = fake.audiences[i]
		}
	}
	require.True(t, found, "отзыв заявки должен послать application.updated со scope application:<id>")
	assert.Contains(t, audience, senderID, "аудитория детали должна включать автора заявки")
}
