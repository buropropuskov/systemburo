package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/fakedata"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const fakeDataTestDSN = "postgres://postgres:postgres@db:5432/auto_registry_test?sslmode=disable"

// setupFakeDataDB поднимает тестовое приложение и чистит базу: партии и отметка экземпляра
// живут без внешних ключей, поэтому состояние соседнего теста иначе приезжает в
// следующий.
func setupFakeDataDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	return db
}

// seedFakeAdmin заводит администратора стенда: наливка пишет от него историю, а у
// реестров сотрудников и машин владелец держится внешним ключом, поэтому без учётной
// записи шаг честно падает. На живом стенде администратор есть всегда (make staging-seed).
func seedFakeAdmin(t *testing.T, db *gorm.DB) models.User {
	t.Helper()
	admin := models.User{
		Username:     uniq("fake_admin"),
		Password:     "x",
		TypeID:       1,
		IsSuperAdmin: true,
		IsActive:     true,
	}
	require.NoError(t, db.Create(&admin).Error)
	return admin
}

// Экземпляр без отметки наливку не пускает, и отказ объясняет, что делать. Это
// единственное, что стоит между командой и рабочим сервером.
func TestEnsureStand_RefusesUnmarked(t *testing.T) {
	db := setupFakeDataDB(t)
	ctx := context.Background()

	err := fakedata.EnsureStand(ctx, db, fakeDataTestDSN, fakedata.GuardOptions{})

	require.ErrorIs(t, err, fakedata.ErrNotMarked)
	require.Contains(t, err.Error(), "-mark-stand", "отказ обязан подсказать команду отметки")
	require.Contains(t, err.Error(), "auto_registry_test", "отказ показывает, к какой базе подключение")
}

func TestEnsureStand_PassesAfterMark(t *testing.T) {
	db := setupFakeDataDB(t)
	ctx := context.Background()

	require.NoError(t, fakedata.MarkStand(ctx, db))

	kind, err := fakedata.InstanceKind(ctx, db)
	require.NoError(t, err)
	require.Equal(t, fakedata.InstanceKindStand, kind)
	require.NoError(t, fakedata.EnsureStand(ctx, db, fakeDataTestDSN, fakedata.GuardOptions{}))
}

// Повторная отметка не должна падать на уникальном ключе: команду запускают руками, и
// второй запуск - обычное дело.
func TestMarkStand_Repeatable(t *testing.T) {
	db := setupFakeDataDB(t)
	ctx := context.Background()

	require.NoError(t, fakedata.MarkStand(ctx, db))
	require.NoError(t, fakedata.MarkStand(ctx, db))

	var count int64
	require.NoError(t, db.Model(&models.SystemSetting{}).
		Where("key = ?", fakedata.InstanceKindKey).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

// Отметку правят и руками через панель управления базой. Отказ из-за регистра толкал
// бы к обходу там, где он не нужен.
func TestEnsureStand_MarkIsCaseInsensitive(t *testing.T) {
	db := setupFakeDataDB(t)
	ctx := context.Background()

	require.NoError(t, db.Create(&models.SystemSetting{
		Key: fakedata.InstanceKindKey, Value: "Staging", Type: "string",
	}).Error)

	require.NoError(t, fakedata.EnsureStand(ctx, db, fakeDataTestDSN, fakedata.GuardOptions{}))
}

// Обход отметки требует ввести имя базы. Одного флага мало: его дописывают по
// привычке, а имя базы приходится посмотреть.
func TestEnsureStand_ForceRequiresMatchingDBName(t *testing.T) {
	db := setupFakeDataDB(t)
	ctx := context.Background()

	err := fakedata.EnsureStand(ctx, db, fakeDataTestDSN, fakedata.GuardOptions{ForceUnmarked: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "-confirm-db=auto_registry_test")

	err = fakedata.EnsureStand(ctx, db, fakeDataTestDSN, fakedata.GuardOptions{
		ForceUnmarked: true, ConfirmDB: "auto_registry",
	})
	require.Error(t, err, "чужое имя базы обход не открывает")

	require.NoError(t, fakedata.EnsureStand(ctx, db, fakeDataTestDSN, fakedata.GuardOptions{
		ForceUnmarked: true, ConfirmDB: "auto_registry_test",
	}))
}

func TestBatch_RegistersCreatedRecords(t *testing.T) {
	db := setupFakeDataDB(t)
	ctx := context.Background()

	batch, err := fakedata.OpenBatch(ctx, db, "test-batch", 42, "small")
	require.NoError(t, err)
	require.NoError(t, batch.Add(ctx, models.AuditEntityOrganization, 11, 12))
	require.NoError(t, batch.Add(ctx, models.AuditEntityUser, 21))
	require.NoError(t, batch.Close(ctx))

	require.Equal(t, 3, batch.Total())

	var items []models.FakeBatchItem
	require.NoError(t, db.Where("batch_id = ?", batch.ID()).Order("id").Find(&items).Error)
	require.Len(t, items, 3)
	require.Equal(t, models.AuditEntityOrganization, items[0].Entity)
	require.Equal(t, 11, items[0].EntityID)

	stored, err := fakedata.FindBatch(ctx, db, "test-batch")
	require.NoError(t, err)
	require.Equal(t, int64(42), stored.Seed)
	require.Equal(t, map[string]int{
		models.AuditEntityOrganization: 2,
		models.AuditEntityUser:         1,
	}, fakedata.SummaryCounts(*stored))
}

// Ноль в реестре превратил бы удаление партии в удаление всего, что попало под
// entity_id = 0, поэтому такой идентификатор до базы не доходит.
func TestBatch_RejectsNonPositiveID(t *testing.T) {
	db := setupFakeDataDB(t)
	ctx := context.Background()

	batch, err := fakedata.OpenBatch(ctx, db, "test-batch-zero", 1, "small")
	require.NoError(t, err)

	require.Error(t, batch.Add(ctx, models.AuditEntityCar, 0))
	require.Error(t, batch.Add(ctx, models.AuditEntityCar, 5, -1))

	var count int64
	require.NoError(t, db.Model(&models.FakeBatchItem{}).Where("batch_id = ?", batch.ID()).Count(&count).Error)
	require.Zero(t, count, "при отказе в партию не должно попасть ничего")
}

func TestListBatches_FreshFirst(t *testing.T) {
	db := setupFakeDataDB(t)
	ctx := context.Background()

	_, err := fakedata.OpenBatch(ctx, db, "batch-one", 1, "small")
	require.NoError(t, err)
	_, err = fakedata.OpenBatch(ctx, db, "batch-two", 2, "small")
	require.NoError(t, err)

	batches, err := fakedata.ListBatches(ctx, db)
	require.NoError(t, err)
	require.Len(t, batches, 2)
	require.Equal(t, "batch-two", batches[0].Label)
}

// Run прогоняет реальные шаги наполнения (#1682, том 2: организации, компании,
// справочники, таблицы постов) на чистой базе и закрывает партию сводкой по
// количеству созданного. Числа 10/10/3/4 -- размеры кандидатских списков
// lookupsStep/postsStep (internal/fakedata/lookups.go, posts.go): на чистой базе
// шаг добавляет их все, ни одно имя ещё не занято.
//
// Марки проверяются наравне с остальными: раньше таблица стояла в CleanupExempt и
// копила строки между прогонами, поэтому «создано сейчас» переставало быть двадцатью
// уже со второго запуска. Исключение снято тем же срезом, чистка марки теперь уносит.
func TestRun_CreatesDictionariesAndClosesBatch(t *testing.T) {
	db := setupFakeDataDB(t)
	ctx := context.Background()

	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	batch, err := fakedata.OpenBatch(ctx, db, "test-dictionaries-run", 7, profile.Name)
	require.NoError(t, err)

	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{
		DB: db, Batch: batch, Profile: profile, Seed: 7,
	}))

	stored, err := fakedata.FindBatch(ctx, db, "test-dictionaries-run")
	require.NoError(t, err)
	counts := fakedata.SummaryCounts(*stored)
	require.Equal(t, profile.Organizations, counts[models.AuditEntityOrganization])
	require.Equal(t, profile.Companies, counts[models.AuditEntityCompany])
	require.Equal(t, 10, counts[models.AuditEntityUnloadPlace])
	require.Equal(t, 20, counts[models.AuditEntityMark])
	require.Equal(t, 10, counts[models.AuditEntityCitizenship])
	require.Equal(t, 3, counts[models.AuditEntityLicensePlateFormat])
	require.Equal(t, 4, counts[models.AuditEntitySystemTable])
}

// Повторный прогон на той же (уже наполненной) базе не должен падать на
// уникальных индексах организаций/компаний/марок и не должен дублировать
// справочники с фиксированными именами (места разгрузки, гражданства, форматы
// номеров, таблицы постов) -- второй раз шаг видит их уже существующими и
// пропускает. Организации/компании new -- имена случайные, поэтому второй
// прогон добавляет ещё profile.Organizations/Companies штук, а не 0.
func TestRun_RepeatedRunDoesNotFailOnUniqueIndexes(t *testing.T) {
	db := setupFakeDataDB(t)
	ctx := context.Background()

	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)

	firstBatch, err := fakedata.OpenBatch(ctx, db, "test-repeat-run-1", 101, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: firstBatch, Profile: profile, Seed: 101}))

	secondBatch, err := fakedata.OpenBatch(ctx, db, "test-repeat-run-2", 202, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: secondBatch, Profile: profile, Seed: 202}))

	stored, err := fakedata.FindBatch(ctx, db, "test-repeat-run-2")
	require.NoError(t, err)
	counts := fakedata.SummaryCounts(*stored)
	// Второй прогон снова заводит свою порцию организаций/компаний (новые
	// случайные имена), но справочники с фиксированными кандидатами уже заняты
	// первым прогоном -- второй раз добавлять нечего.
	require.Equal(t, profile.Organizations, counts[models.AuditEntityOrganization])
	require.Equal(t, profile.Companies, counts[models.AuditEntityCompany])
	require.Zero(t, counts[models.AuditEntityUnloadPlace])
	require.Zero(t, counts[models.AuditEntityMark])
	require.Zero(t, counts[models.AuditEntityCitizenship])
	require.Zero(t, counts[models.AuditEntityLicensePlateFormat])
	require.Zero(t, counts[models.AuditEntitySystemTable])
}
