package handlers_test

// Проверка справочного среза наливки вымышленных данных (#1682, том 2):
// организации, компании, места разгрузки, марки, гражданства, форматы номеров,
// таблицы постов. testutil.SetupTestApp поднимает базу -- по правилу проекта
// такие тесты живут только в internal/handlers, иначе второй бинарь с базой
// делит тест-БД с этим пакетом и роняет чужие тесты. Профиль "small" (3
// организации, 3 компании) выбран нарочно маленьким: пакет handlers и так на
// грани CI-таймаута под -race (см. соседние тесты), раздувать наливку тут незачем.

import (
	"context"
	"testing"

	"systemburo/internal/fakedata"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

// TestFakeDictionaries_RunFillsRealDirectories прогоняет fakedata.Run и
// проверяет результат ЧЕРЕЗ СЕРВИСЫ (не тем же SQL, которым шаги писали):
// справочники реально наполнены, всё созданное зарегистрировано в партии,
// повторный прогон не падает на уникальных индексах.
func TestFakeDictionaries_RunFillsRealDirectories(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()

	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)

	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-dict"), 555, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 555}))

	// --- справочники реально наполнены ---

	orgSvc := services.NewOrganizationService(db)
	orgs, err := orgSvc.GetAll(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(orgs), profile.Organizations)

	compSvc := services.NewCompanyService(db)
	companies, err := compSvc.GetAll(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(companies), profile.Companies)

	placeSvc := services.NewUnloadPlaceService(db)
	places, err := placeSvc.GetAll(ctx, true)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(places), 10)
	require.True(t, hasName(places, func(p services.UnloadPlaceWithDetails) string { return p.Name }, "Склад №1"))

	markSvc := services.NewMarkService(db)
	marks, err := markSvc.GetAll(ctx, true)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(marks), 20)
	require.True(t, hasName(marks, func(m models.Mark) string { return m.Name }, "КамАЗ"))

	citizenshipSvc := services.NewCitizenshipService(db)
	citizenships, err := citizenshipSvc.GetAll(ctx, true)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(citizenships), 10)
	require.True(t, hasName(citizenships, func(c models.Citizenship) string { return c.Name }, "Россия"))

	lpfSvc := services.NewLicensePlateFormatService(db)
	formats, err := lpfSvc.GetAll(ctx, true)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(formats), 3)
	ruFormat, ok := findFormat(formats, "Россия")
	require.True(t, ok, "формат номеров Россия должен быть создан")
	require.NotEmpty(t, ruFormat.Cells, "формат номеров без ячеек нельзя использовать для проверки номера")

	tableSvc := services.NewSystemTableService(db, "", 0, services.NewPermissionService(db))
	tables, err := tableSvc.GetAll(ctx, false)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(tables), 4)
	post, ok := findTable(tables, "kpp-central")
	require.True(t, ok, "таблица поста kpp-central должна быть создана")
	require.Equal(t, models.TableTypeCars, post.Table.TableType)
	require.NotEmpty(t, post.Fields, "таблица поста должна получить поля по умолчанию")

	// --- всё созданное зарегистрировано в партии ---

	var itemCount int64
	require.NoError(t, db.Model(&models.FakeBatchItem{}).
		Where("batch_id = ?", batch.ID()).Count(&itemCount).Error)
	require.Equal(t, int64(batch.Total()), itemCount, "число строк в fake_batch_items должно совпасть со сводкой партии")

	stored, err := fakedata.FindBatch(ctx, db, batch.Label())
	require.NoError(t, err)
	require.Equal(t, batch.Counts(), fakedata.SummaryCounts(*stored), "сохранённая сводка партии должна совпасть со счётчиками в памяти")

	// --- повторный прогон не падает на уникальных индексах ---

	batch2, err := fakedata.OpenBatch(ctx, db, uniq("fake-dict-2"), 777, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch2, Profile: profile, Seed: 777}))
}

func hasName[T any](items []T, name func(T) string, want string) bool {
	for _, item := range items {
		if name(item) == want {
			return true
		}
	}
	return false
}

func findFormat(items []models.LicensePlateFormatWithCells, name string) (models.LicensePlateFormatWithCells, bool) {
	for _, item := range items {
		if item.Format.Name == name {
			return item, true
		}
	}
	return models.LicensePlateFormatWithCells{}, false
}

func findTable(items []models.SystemTableWithDetails, name string) (models.SystemTableWithDetails, bool) {
	for _, item := range items {
		if item.Table.Name == name {
			return item, true
		}
	}
	return models.SystemTableWithDetails{}, false
}

// История наливки должна принадлежать администратору стенда, а не актору «0».
// Колонка actor_user_id живёт без внешнего ключа, поэтому несуществующий актор
// записался бы молча, и в истории справочника появился бы автор, которого нет.
func TestFakeDictionaries_HistoryBelongsToAdmin(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)

	admin := models.User{
		Username:     uniq("fake_admin"),
		Password:     "x",
		TypeID:       1,
		IsSuperAdmin: true,
		IsActive:     true,
	}
	require.NoError(t, db.Create(&admin).Error)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-actor"), 777, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 777}))

	var orphaned int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND (actor_user_id IS NULL OR actor_user_id NOT IN (SELECT id FROM users))",
			models.AuditEntityOrganization).
		Count(&orphaned).Error)
	require.Zero(t, orphaned, "записи истории не должны ссылаться на несуществующего пользователя")

	var byAdmin int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND actor_user_id = ?", models.AuditEntityOrganization, admin.ID).
		Count(&byAdmin).Error)
	require.Positive(t, byAdmin, "организации должен был завести администратор стенда")
}
