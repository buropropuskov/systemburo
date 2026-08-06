package handlers_test

// Проверка удаления партии (#1682): наливка обязана уметь убирать за собой ровно то,
// что налила, не задевая данные, заведённые на стенде руками.

import (
	"context"
	"testing"

	"systemburo/internal/fakedata"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func countTableRows(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(table).Count(&n).Error)
	return n
}

// Партия удаляется целиком, а счётчики таблиц возвращаются к тому, что было до неё.
func TestFakePurge_RemovesEverythingBatchCreated(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	tables := []string{"applications", "attachments", "cars", "employees", "unique_cars",
		"unique_employees", "users", "vehicle_blacklists", "person_blacklists", "organizations"}
	before := make(map[string]int64, len(tables))
	for _, tbl := range tables {
		before[tbl] = countTableRows(t, db, tbl)
	}

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	label := uniq("fake-purge")
	batch, err := fakedata.OpenBatch(ctx, db, label, 4242, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 4242}))

	require.Greater(t, countTableRows(t, db, "applications"), before["applications"],
		"наливка должна была создать заявки, иначе удалять нечего")

	res, err := fakedata.PurgeBatch(ctx, db, label, true)
	require.NoError(t, err)
	require.Positive(t, res.TotalDeleted())

	for _, tbl := range tables {
		require.Equal(t, before[tbl], countTableRows(t, db, tbl),
			"таблица %s не вернулась к состоянию до наливки", tbl)
	}

	var batches int64
	require.NoError(t, db.Model(&models.FakeBatch{}).Where("label = ?", label).Count(&batches).Error)
	require.Zero(t, batches, "сама партия должна исчезнуть вместе с перечнем")

	var orphanItems int64
	require.NoError(t, db.Model(&models.FakeBatchItem{}).Where("batch_id = ?", batch.ID()).Count(&orphanItems).Error)
	require.Zero(t, orphanItems)
}

// Показ без -apply обязан быть безвредным: он гоняет тот же путь удаления и откатывает
// его, поэтому важно убедиться, что после него данные на месте.
func TestFakePurge_PreviewKeepsData(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	label := uniq("fake-preview")
	batch, err := fakedata.OpenBatch(ctx, db, label, 77, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 77}))

	appsBefore := countTableRows(t, db, "applications")

	res, err := fakedata.PurgeBatch(ctx, db, label, false)
	require.NoError(t, err)
	require.Positive(t, res.TotalDeleted(), "показ обязан сказать, сколько удалилось бы")

	require.Equal(t, appsBefore, countTableRows(t, db, "applications"), "показ не имеет права удалять")
	stored, err := fakedata.FindBatch(ctx, db, label)
	require.NoError(t, err)
	require.Equal(t, label, stored.Label, "партия должна остаться на месте")
}

// Удаление партии не трогает данные, заведённые на стенде руками, даже если они того
// же вида: перечень партии -- единственный источник того, что удалять.
func TestFakePurge_KeepsManualData(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	admin := seedFakeAdmin(t, db)

	manual := models.User{
		Username: uniq("manual_user"), Password: "x", TypeID: 1, IsActive: true,
	}
	require.NoError(t, db.Create(&manual).Error)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	label := uniq("fake-manual")
	batch, err := fakedata.OpenBatch(ctx, db, label, 5, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 5}))

	_, err = fakedata.PurgeBatch(ctx, db, label, true)
	require.NoError(t, err)

	var manualLeft, adminLeft int64
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", manual.ID).Count(&manualLeft).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", admin.ID).Count(&adminLeft).Error)
	require.Equal(t, int64(1), manualLeft, "заведённый руками пользователь удалению не подлежит")
	require.Equal(t, int64(1), adminLeft, "администратор стенда удалению не подлежит")
}

// Вторая партия пользуется общими записями первой (справочники, посты, шаблоны
// вложений), поэтому удаление первой обязано их оставить, а не увести за собой данные
// второй. Ровно этот случай ловит счётчик «оставлено».
func TestFakePurge_KeepsSharedRecordsUsedByOtherBatch(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)

	firstLabel := uniq("fake-shared-1")
	first, err := fakedata.OpenBatch(ctx, db, firstLabel, 1, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: first, Profile: profile, Seed: 1}))

	secondLabel := uniq("fake-shared-2")
	second, err := fakedata.OpenBatch(ctx, db, secondLabel, 2, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: second, Profile: profile, Seed: 2}))

	secondApps := countTableRows(t, db, "applications")

	_, err = fakedata.PurgeBatch(ctx, db, firstLabel, true)
	require.NoError(t, err)

	require.Less(t, countTableRows(t, db, "applications"), secondApps,
		"заявки первой партии должны исчезнуть")

	var secondAppsLeft int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM applications a
		JOIN fake_batch_items i ON i.entity = 'application' AND i.entity_id = a.id AND i.batch_id = ?`,
		second.ID()).Scan(&secondAppsLeft).Error)
	require.Positive(t, secondAppsLeft, "заявки второй партии обязаны пережить удаление первой")

	var templatesLeft int64
	require.NoError(t, db.Table("unique_attachments").Count(&templatesLeft).Error)
	require.Positive(t, templatesLeft,
		"шаблоны вложений нужны заявкам второй партии и удалению вместе с первой не подлежат")
}
