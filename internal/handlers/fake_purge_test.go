package handlers_test

// Проверка удаления партии (#1682): наливка обязана уметь убирать за собой ровно то,
// что налила, не задевая данные, заведённые на стенде руками.

import (
	"context"
	"testing"

	"systemburo/internal/fakedata"
	"systemburo/internal/models"
	"systemburo/internal/services"
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

// Партия обязана унести и посты со своими столбцами, и привязки машин/сотрудников к
// постам. Раньше пост партии удалить не удавалось (его держали его же столбцы, у связи
// нет каскада), отчёт писал «оставлено: на них ссылаются данные вне партии», а привязки
// оставались висеть на удалённых машинах -- у car_target_tables внешних ключей нет вовсе,
// и мусор копился с каждой наливкой.
func TestFakePurge_RemovesPostsWithColumnsAndTargetLinks(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	tables := []string{"system_tables", "table_fields", "table_field_facts",
		"car_target_tables", "employee_target_tables"}
	before := make(map[string]int64, len(tables))
	for _, tbl := range tables {
		before[tbl] = countTableRows(t, db, tbl)
	}

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	label := uniq("fake-purge-posts")
	batch, err := fakedata.OpenBatch(ctx, db, label, 6161, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 6161}))

	require.Greater(t, countTableRows(t, db, "car_target_tables"), before["car_target_tables"],
		"наливка обязана была привязать машины к постам -- иначе проверять нечего")

	res, err := fakedata.PurgeBatch(ctx, db, label, true)
	require.NoError(t, err)
	require.Zero(t, res.TotalKept(), "оставлять нечего: все посты партии её собственные")

	for _, tbl := range tables {
		require.Equal(t, before[tbl], countTableRows(t, db, tbl),
			"таблица %s должна вернуться к тому, что было до партии", tbl)
	}

	var orphans int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM car_target_tables ctt
		LEFT JOIN cars c ON c.id = ctt.car_id WHERE c.id IS NULL`).Scan(&orphans).Error)
	require.Zero(t, orphans, "привязки к постам не должны переживать свои машины")
}

// createPostByHand заводит пост тем же путём, что и администратор стенда: сервис сам
// создаёт права table.<слаг>.<глагол>, без которых пост нельзя выдать ни одной роли.
func createPostByHand(t *testing.T, db *gorm.DB, name string) int {
	t.Helper()
	svc := services.NewSystemTableService(db, "", 0, services.NewPermissionService(db))
	id, err := svc.Create(context.Background(), models.CreateSystemTableRequest{
		Name: name, DisplayName: "Пост " + name, TableType: models.TableTypeCars,
	})
	require.NoError(t, err)
	return id
}

// postPermissionKeys -- ключи прав постов, id которых переданы.
func postPermissionKeys(t *testing.T, db *gorm.DB, tableIDs []int) []string {
	t.Helper()
	var keys []string
	require.NoError(t, db.Raw(
		`SELECT key FROM permissions WHERE category = 'table' AND entity_id IN (?)`, tableIDs,
	).Scan(&keys).Error)
	return keys
}

// batchPostIDs -- посты, записанные в перечень партии.
func batchPostIDs(t *testing.T, db *gorm.DB, batchID int) []int {
	t.Helper()
	var ids []int
	require.NoError(t, db.Raw(
		`SELECT entity_id FROM fake_batch_items WHERE batch_id = ? AND entity = ?`,
		batchID, models.AuditEntitySystemTable,
	).Scan(&ids).Error)
	return ids
}

// permHolders -- носители выдач: роль, группа и пользователь. Один набор на весь тест,
// чтобы выдачи на права живого поста и на права поста партии лежали бок о бок и
// проверка отличала «снесли лишнее» от «сняли ровно нужное».
type permHolders struct {
	roleID  int
	groupID int
	userID  int
}

func newPermHolders(t *testing.T, db *gorm.DB, userID int) permHolders {
	t.Helper()
	role := models.Role{Code: uniq("role"), Name: "Роль стенда"}
	require.NoError(t, db.Create(&role).Error)
	group := models.PermissionGroup{Name: uniq("группа")}
	require.NoError(t, db.Create(&group).Error)
	return permHolders{roleID: role.ID, groupID: group.ID, userID: userID}
}

// grantKeysTo выдаёт ключи всеми четырьмя способами, какими на право можно сослаться:
// роли, группе, личным переопределением и legacy-таблицей личных прав. Внешних ключей
// у этих ссылок нет -- ключ хранится строкой, поэтому удаление права само по себе их
// не уносит.
func grantKeysTo(t *testing.T, db *gorm.DB, h permHolders, keys []string) {
	t.Helper()
	for _, key := range keys {
		require.NoError(t, db.Create(&models.RolePermissionGrant{
			RoleID: h.roleID, PermissionKey: key, Value: "allow",
		}).Error)
		require.NoError(t, db.Create(&models.PermissionGroupGrant{
			GroupID: h.groupID, PermissionKey: key, Value: "allow",
		}).Error)
		require.NoError(t, db.Create(&models.UserPermissionOverride{
			UserID: h.userID, PermissionKey: key, Value: "deny",
		}).Error)
		require.NoError(t, db.Create(&models.UserPermission{
			UserID: h.userID, PermissionKey: key, Value: "allow",
		}).Error)
	}
}

// countKeyReferences -- сколько строк ссылается на эти ключи во всех четырёх таблицах.
func countKeyReferences(t *testing.T, db *gorm.DB, keys []string) int64 {
	t.Helper()
	var total int64
	require.NoError(t, db.Raw(`
		SELECT (SELECT COUNT(*) FROM role_permission_grants WHERE permission_key IN (?))
		     + (SELECT COUNT(*) FROM permission_group_grants WHERE permission_key IN (?))
		     + (SELECT COUNT(*) FROM user_permission_overrides WHERE permission_key IN (?))
		     + (SELECT COUNT(*) FROM user_permissions WHERE permission_key IN (?))`,
		keys, keys, keys, keys).Scan(&total).Error)
	return total
}

// Права поста заводятся вместе с ним (иначе пост нельзя выдать ни одной роли), но
// ссылок на permissions в базе нет ни одной: выдачи и переопределения держат ключ
// строкой. Поэтому удаление партии уносило пост и оставляло его права висеть -- по
// десять на пост с каждой наливкой, и разобрать их через интерфейс уже нельзя: таблицы,
// которой они принадлежат, не существует.
func TestFakePurge_RemovesPostPermissionsWithGrants(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	admin := seedFakeAdmin(t, db)

	// Контрольный пост: заведён руками, к партии отношения не имеет и обязан пережить
	// её удаление вместе со всеми выдачами на свои права.
	liveID := createPostByHand(t, db, uniq("live-post"))
	liveKeys := postPermissionKeys(t, db, []int{liveID})
	require.NotEmpty(t, liveKeys, "пост заводится вместе со своими правами")

	holders := newPermHolders(t, db, admin.ID)
	grantKeysTo(t, db, holders, liveKeys)
	liveRefsBefore := countKeyReferences(t, db, liveKeys)
	require.Positive(t, liveRefsBefore)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	label := uniq("fake-purge-perms")
	batch, err := fakedata.OpenBatch(ctx, db, label, 1950, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 1950}))

	postIDs := batchPostIDs(t, db, batch.ID())
	require.NotEmpty(t, postIDs, "наливка обязана была завести посты -- иначе проверять нечего")
	fakeKeys := postPermissionKeys(t, db, postIDs)
	require.NotEmpty(t, fakeKeys, "посты партии заводятся вместе со своими правами")

	// Сама наливка выдач не создаёт: их на стенде расставляет человек, раздавая пост
	// ролям и людям. Заводим их руками -- иначе проверять, что вместе с правом уходит
	// ссылающееся на него, было бы не на чем.
	grantKeysTo(t, db, holders, fakeKeys)
	require.Positive(t, countKeyReferences(t, db, fakeKeys))

	_, err = fakedata.PurgeBatch(ctx, db, label, true)
	require.NoError(t, err)

	var fakePermsLeft int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM permissions WHERE key IN (?)`, fakeKeys,
	).Scan(&fakePermsLeft).Error)
	require.Zero(t, fakePermsLeft, "права постов партии обязаны уйти вместе с постами")
	require.Zero(t, countKeyReferences(t, db, fakeKeys),
		"выдачи на права удалённых постов -- такой же мусор, как сами права")

	var liveTableLeft, livePermsLeft int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM system_tables WHERE id = ?`, liveID).Scan(&liveTableLeft).Error)
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM permissions WHERE key IN (?)`, liveKeys).Scan(&livePermsLeft).Error)
	require.Equal(t, int64(1), liveTableLeft, "пост, заведённый руками, удалению не подлежит")
	require.Equal(t, int64(len(liveKeys)), livePermsLeft, "права живого поста удалению не подлежат")
	require.Equal(t, liveRefsBefore, countKeyReferences(t, db, liveKeys),
		"выдачи на права живого поста удалению не подлежат")

	var orphanPerms int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM permissions p
		WHERE p.category = 'table' AND p.entity_id IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM system_tables t WHERE t.id = p.entity_id)`).Scan(&orphanPerms).Error)
	require.Zero(t, orphanPerms, "права без своего поста разобрать через интерфейс уже нельзя")
}

// Показ без -apply гоняет тот же путь и откатывает его, поэтому новые удаления обязаны
// в нём считаться, а не выполняться. Отчёт считает сами записи, а не принадлежащие им
// строки, поэтому проверяем и то, что посты партии посчитаны удаляемыми (а не
// оставленными), и то, что их права после показа на месте.
func TestFakePurge_PreviewKeepsPostPermissions(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	admin := seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	label := uniq("fake-preview-perms")
	batch, err := fakedata.OpenBatch(ctx, db, label, 1951, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 1951}))

	postIDs := batchPostIDs(t, db, batch.ID())
	require.NotEmpty(t, postIDs)
	fakeKeys := postPermissionKeys(t, db, postIDs)
	require.NotEmpty(t, fakeKeys)

	holders := newPermHolders(t, db, admin.ID)
	grantKeysTo(t, db, holders, fakeKeys)
	refsBefore := countKeyReferences(t, db, fakeKeys)
	require.Positive(t, refsBefore)

	res, err := fakedata.PurgeBatch(ctx, db, label, false)
	require.NoError(t, err)

	postLine := fakedata.EntityTitle(models.AuditEntitySystemTable)
	var deleted, kept int
	for _, line := range res.Lines {
		if line.Title == postLine {
			deleted, kept = line.Deleted, line.Kept
		}
	}
	require.Equal(t, len(postIDs), deleted, "показ обязан сказать, что посты партии удалились бы все")
	require.Zero(t, kept, "оставлять посты партии не из-за чего: их права уходят вместе с ними")

	var permsLeft int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM permissions WHERE key IN (?)`, fakeKeys).Scan(&permsLeft).Error)
	require.Equal(t, int64(len(fakeKeys)), permsLeft, "показ не имеет права удалять права постов")
	require.Equal(t, refsBefore, countKeyReferences(t, db, fakeKeys), "показ не имеет права снимать выдачи")

	var postsLeft int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM system_tables WHERE id IN (?)`, postIDs).Scan(&postsLeft).Error)
	require.Equal(t, int64(len(postIDs)), postsLeft, "показ не имеет права удалять сами посты")
}

// Пометки о возможном обходе чёрного списка и решения по ним живут без внешних ключей
// намеренно -- они снимок момента подачи и переживают правку самого элемента. Но читают
// их только по заявке, поэтому вместе с ней они обязаны уходить: иначе каждая наливка
// оставляла бы их висеть на несуществующих заявках.
func TestFakePurge_RemovesBlacklistFlagsOfDeletedApplications(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	label := uniq("fake-purge-flags")
	batch, err := fakedata.OpenBatch(ctx, db, label, 1952, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 1952}))

	require.Positive(t, countTableRows(t, db, "application_blacklist_flags"),
		"наливка обязана была пометить похожие на чёрный список элементы -- иначе проверять нечего")

	_, err = fakedata.PurgeBatch(ctx, db, label, true)
	require.NoError(t, err)

	var flagOrphans, overrideOrphans int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM application_blacklist_flags f
		WHERE NOT EXISTS (SELECT 1 FROM applications a WHERE a.id = f.application_id)`).Scan(&flagOrphans).Error)
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM application_blacklist_overrides o
		WHERE NOT EXISTS (SELECT 1 FROM applications a WHERE a.id = o.application_id)`).Scan(&overrideOrphans).Error)
	require.Zero(t, flagOrphans, "пометки не должны переживать свою заявку")
	require.Zero(t, overrideOrphans, "решения по пометкам не должны переживать свою заявку")
}

// Посты заводит первая партия, а пользуются ими и следующие. Удаление первой уносит
// посты -- значит обязано унести и привязки машин/сотрудников второй партии к ним:
// внешних ключей у этих привязок нет, и раньше они оставались смотреть в пустоту.
func TestFakePurge_RemovesTargetLinksOfOtherBatchToDeletedPosts(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)

	firstLabel := uniq("fake-posts-owner")
	first, err := fakedata.OpenBatch(ctx, db, firstLabel, 1953, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: first, Profile: profile, Seed: 1953}))

	secondLabel := uniq("fake-posts-guest")
	second, err := fakedata.OpenBatch(ctx, db, secondLabel, 1954, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: second, Profile: profile, Seed: 1954}))

	require.Empty(t, batchPostIDs(t, db, second.ID()),
		"посты завела первая партия, вторая должна была найти их готовыми")
	require.Positive(t, countTableRows(t, db, "car_target_tables"))

	_, err = fakedata.PurgeBatch(ctx, db, firstLabel, true)
	require.NoError(t, err)

	var carOrphans, employeeOrphans, permOrphans int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM car_target_tables c
		WHERE NOT EXISTS (SELECT 1 FROM system_tables t WHERE t.id = c.table_id)`).Scan(&carOrphans).Error)
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM employee_target_tables e
		WHERE NOT EXISTS (SELECT 1 FROM system_tables t WHERE t.id = e.table_id)`).Scan(&employeeOrphans).Error)
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM permissions p
		WHERE p.category = 'table' AND p.entity_id IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM system_tables t WHERE t.id = p.entity_id)`).Scan(&permOrphans).Error)
	require.Zero(t, carOrphans, "привязки машин не должны переживать пост")
	require.Zero(t, employeeOrphans, "привязки сотрудников не должны переживать пост")
	require.Zero(t, permOrphans, "права не должны переживать свой пост")
}
