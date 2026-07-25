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

// submitCarAppForCenter создаёт простую заявку с машиной и возвращает её id.
// Вынесено, чтобы оба теста Центра ниже не дублировали тело CompleteApplicationRequest.
func submitCarAppForCenter(t *testing.T, svc services.ApplicationService, username string, uaID int) int {
	t.Helper()
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
	created, err := svc.SubmitCompleteApplication(context.Background(), username, req, true)
	require.NoError(t, err)
	return created.ApplicationID
}

// TestApplicationsRefresh_OnMutation защищает продюсер списка Центра (#840 B+C):
// мутация заявки (здесь - отзыв) должна, помимо application.updated для открытой
// детали, послать applications.refresh (scope applications-center), чтобы у всех,
// кто видит заявку в Центре, тихо перерисовались столбцы подтверждения/статуса и
// производные теги. Хук notifyApplicationUpdated единый для всех 12 мутаций
// (approve/reject/forward/take/revoke/restore/withdraw/accept/question/answer/
// blacklist-override), поэтому один сквозной путь доказывает механику. Аудитория
// applications.refresh - та же applicationParticipants, что и у application.updated.
func TestApplicationsRefresh_OnMutation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "actrsender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "actrsender")
	uaID := seedUniqueAttachment(t, db, "cars", "cars_actr", "Cars ACtr")

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

	appID := submitCarAppForCenter(t, svc, "actrsender", uaID)

	fake.reset() // отбрасываем applications.refresh от самого submit

	require.NoError(t, svc.WithdrawApplication(context.Background(), "actrsender", appID))

	// Мутация обязана послать applications.refresh в scope applications-center.
	var refreshAudience []int
	found := false
	for i, ev := range fake.events {
		if ev.Type == "applications.refresh" && ev.Scope == "applications-center" {
			found = true
			refreshAudience = fake.audiences[i]
		}
	}
	require.True(t, found, "мутация заявки должна послать applications.refresh со scope applications-center")
	assert.Contains(t, refreshAudience, senderID, "аудитория обновления Центра включает автора заявки")
}

// TestUnreadCount_MutationPreservesReads защищает решение переиспользовать
// applications.refresh для смены статуса (#840 B+C): сигнал дёргает и счётчик
// непрочитанных NavMenu (со звуком), поэтому мутация НЕ должна воскрешать
// прочитанную заявку в непрочитанные - иначе ложный звук "новая заявка".
// Класс двух прошлых инцидентов ложного звука (#1021, #632): фиксируем, что
// смена статуса через общий хук notifyApplicationUpdated не удаляет отметку
// application_reads (проверяем строку напрямую - минуя archive-фильтр GetUnreadCount).
func TestUnreadCount_MutationPreservesReads(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "urmsender", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "cars_urm", "Cars URM")

	// approver видит все заявки, в т.ч. в счётчике непрочитанных.
	approver := models.User{Username: "urmapprover", Password: "x", TypeID: 1, OrganizationID: &td.OrgID, CompanyID: &td.CompanyID}
	require.NoError(t, db.Create(&approver).Error)
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: approver.ID}).Error)

	recorder := services.NewAuditRecorder(db)
	svc := services.NewApplicationService(
		db,
		services.NewPermissionService(db),
		services.NewNotificationService(db),
		services.NewVehicleBlacklistService(db, recorder),
		services.NewPersonBlacklistService(db, recorder),
		recorder,
	)

	appID := submitCarAppForCenter(t, svc, "urmsender", uaID)

	// approver прочитал заявку -> непрочитанных 0.
	require.NoError(t, svc.MarkAsRead(context.Background(), appID, "urmapprover"))
	before, err := svc.GetUnreadCount(context.Background(), "urmapprover")
	require.NoError(t, err)
	require.Equal(t, 0, before.Count, "после прочтения счётчик непрочитанных = 0")

	// Смена статуса (отзыв) идёт через тот же notifyApplicationUpdated, что шлёт applications.refresh.
	require.NoError(t, svc.WithdrawApplication(context.Background(), "urmsender", appID))

	// Отметка прочтения approver'а НЕ удалена мутацией -> счётчик не воскреснет -> нет ложного звука.
	var readRows int64
	require.NoError(t, db.Table("application_reads").
		Where("application_id = ? AND user_id = ?", appID, approver.ID).
		Count(&readRows).Error)
	assert.Equal(t, int64(1), readRows, "мутация статуса не удаляет отметку о прочтении (иначе ложный рост счётчика/звука)")
}
