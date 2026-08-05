package handlers_test

// Проверка среза наливки реестров сотрудников и машин (#1682, том 3): после
// прогона fakedata.Run реестры личного кабинета реально наполнены, ссылаются на
// реальные записи организаций/гражданств/марок (не выдуманные id), всё созданное
// зарегистрировано в партии, повторный прогон не падает на проверках уникальности
// паспорта/госномера. testutil.SetupTestApp поднимает базу -- по правилу проекта
// такие тесты живут только в internal/handlers. Профиль "small" выбран нарочно
// маленьким: пакет handlers и так на грани CI-таймаута под -race.

import (
	"context"
	"testing"

	"systemburo/internal/fakedata"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestFakeRegistries_RunFillsEmployeesAndCars(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	// CleanDB здесь принципиален (в отличие от TestFakeDictionaries): без него в
	// unique_employees/unique_cars могли остаться строки других тестов пакета, и
	// проверка "все ссылки реальны" читала бы чужие данные вместо только что
	// созданных этим прогоном.
	testutil.CleanDB(t, db)

	admin := models.User{
		Username:     uniq("fake_reg_admin"),
		Password:     "x",
		TypeID:       1,
		IsSuperAdmin: true,
		IsActive:     true,
	}
	require.NoError(t, db.Create(&admin).Error)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)

	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-reg"), 4242, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 4242}))

	// --- реестры реально наполнены и ссылаются на реальные справочники ---

	orgSvc := services.NewOrganizationService(db)
	orgs, err := orgSvc.GetAll(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(orgs), profile.Organizations)
	orgIDs := make(map[int]bool, len(orgs))
	for _, o := range orgs {
		orgIDs[o.ID] = true
	}

	citizenshipSvc := services.NewCitizenshipService(db)
	citizenships, err := citizenshipSvc.GetAll(ctx, false)
	require.NoError(t, err)
	citizenshipIDs := make(map[int]bool, len(citizenships))
	for _, c := range citizenships {
		citizenshipIDs[c.ID] = true
	}

	markSvc := services.NewMarkService(db)
	marks, err := markSvc.GetAll(ctx, false)
	require.NoError(t, err)
	markNames := make(map[string]bool, len(marks))
	for _, m := range marks {
		markNames[m.Name] = true
	}

	employeeSvc := services.NewUniqueEmployeeService(db)
	employees, err := employeeSvc.GetAll(ctx, admin.Username, "all_system")
	require.NoError(t, err)
	require.Len(t, employees, profile.Employees, "реестр должен получить ровно столько сотрудников, сколько просит профиль")

	for _, e := range employees {
		require.NotNil(t, e.OrganizationID, "у сотрудника реестра должна быть проставлена организация")
		require.True(t, orgIDs[*e.OrganizationID], "организация сотрудника должна быть реальной записью справочника")

		require.NotNil(t, e.CitizenshipID, "у сотрудника реестра должно быть проставлено гражданство")
		require.True(t, citizenshipIDs[*e.CitizenshipID], "гражданство сотрудника должно быть реальной записью справочника")
	}

	carSvc := services.NewUniqueCarService(db)
	cars, err := carSvc.GetAll(ctx, admin.Username, "all_system")
	require.NoError(t, err)
	require.Len(t, cars, profile.Cars, "реестр должен получить ровно столько машин, сколько просит профиль")

	for _, c := range cars {
		require.NotNil(t, c.Mark)
		require.True(t, markNames[*c.Mark], "марка машины должна быть реальной записью справочника")
		require.NotNil(t, c.OrganizationID, "у машины реестра должна быть проставлена организация")
		require.True(t, orgIDs[*c.OrganizationID], "организация машины должна быть реальной записью справочника")
		require.NotNil(t, c.FormatID, "у машины реестра должен быть проставлен формат номера")
	}

	// --- всё созданное зарегистрировано в партии ---

	var itemCount int64
	require.NoError(t, db.Model(&models.FakeBatchItem{}).
		Where("batch_id = ?", batch.ID()).Count(&itemCount).Error)
	require.Equal(t, int64(batch.Total()), itemCount, "число строк в fake_batch_items должно совпасть со сводкой партии")
	require.Equal(t, profile.Employees, batch.Counts()[models.AuditEntityUniqueEmployee])
	require.Equal(t, profile.Cars, batch.Counts()[models.AuditEntityUniqueCar])

	// --- повторный прогон не падает на проверках уникальности паспорта/номера ---

	batch2, err := fakedata.OpenBatch(ctx, db, uniq("fake-reg-2"), 9999, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch2, Profile: profile, Seed: 9999}))
	require.Equal(t, profile.Employees, batch2.Counts()[models.AuditEntityUniqueEmployee])
	require.Equal(t, profile.Cars, batch2.Counts()[models.AuditEntityUniqueCar])
}
