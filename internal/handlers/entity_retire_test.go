package handlers_test

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Срез 4 entity-archive (offboarding): server entity retire/restore. DB-тест лежит в
// internal/handlers (не в entityarchive) по правилу проекта - второй тестовый бинарь,
// открывающий соединение с базой, гоняет AutoMigrate параллельно с этим пакетом и даёт
// гонку миграций и ложные красные.

// seedRetireUser создаёт пользователя организации напрямую, минуя HTTP-регистрацию:
// консольная команда работает с базой мимо хендлеров, а тесту нужен только точный
// is_active на старте, который регистрация не даёт задать.
//
// Погашение - отдельным UPDATE, а не полем в Create: IsActive у модели помечен
// gorm:"default:true", и gorm ради самого этого тега пропускает в INSERT колонку,
// если Go-значение поля равно zero value типа (у bool это false) - IsActive:false в
// структуре Create молча заменяется дефолтом базы (true). Тот же путь, каким
// деактивирует пользователя сама система (UPDATE, не Create), и единственный способ
// в тесте гарантированно завести уже неактивную строку.
func seedRetireUser(t *testing.T, db *gorm.DB, username string, orgID int, active bool) int {
	t.Helper()
	u := models.User{Username: username, OrganizationID: &orgID}
	require.NoError(t, db.Create(&u).Error)
	if !active {
		require.NoError(t, db.Exec("UPDATE users SET is_active = false WHERE id = ?", u.ID).Error)
	}
	return u.ID
}

// seedRetireSuperAdmin создаёт супер-админа организации напрямую. EnforceSingleSuperAdmin
// правит лишних супер-админов только внутри AutoMigrate (однократно на весь тестовый
// бинарь, testutil.dbOnce), поэтому запись, созданная здесь через Create после старта
// тестов, никем не тронется - тот же приём, что и в super_admin_invariant_test.go.
func seedRetireSuperAdmin(t *testing.T, db *gorm.DB, username string, orgID int) int {
	t.Helper()
	u := models.User{Username: username, OrganizationID: &orgID, IsSuperAdmin: true}
	require.NoError(t, db.Create(&u).Error)
	return u.ID
}

func orgIsActive(t *testing.T, db *gorm.DB, id int) bool {
	t.Helper()
	var active bool
	require.NoError(t, db.Raw("SELECT is_active FROM organizations WHERE id = ?", id).Scan(&active).Error)
	return active
}

func userIsActive(t *testing.T, db *gorm.DB, id int) bool {
	t.Helper()
	var active bool
	require.NoError(t, db.Raw("SELECT is_active FROM users WHERE id = ?", id).Scan(&active).Error)
	return active
}

// TestEntityRetire_DeactivatesOrgAndUsers: dry-run не трогает базу и показывает то же,
// что применит -apply; apply гасит организацию и её активного пользователя одним
// действием и пишет запись retired в audit_log.
func TestEntityRetire_DeactivatesOrgAndUsers(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	userID := seedRetireUser(t, db, "retire_active_user", td.OrgID, true)
	rec := services.NewAuditRecorder(db)
	ctx := context.Background()

	dry, err := entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, false)
	require.NoError(t, err)
	assert.Equal(t, []int{td.OrgID}, dry.Organizations, "dry-run видит активную организацию")
	assert.Equal(t, []int{userID}, dry.Users, "dry-run видит активного пользователя")
	assert.True(t, orgIsActive(t, db, td.OrgID), "dry-run не меняет базу")
	assert.True(t, userIsActive(t, db, userID), "dry-run не меняет базу")

	res, err := entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)
	assert.Equal(t, []int{td.OrgID}, res.Organizations)
	assert.Equal(t, []int{userID}, res.Users)
	assert.False(t, orgIsActive(t, db, td.OrgID), "retire погасил организацию")
	assert.False(t, userIsActive(t, db, userID), "retire погасил пользователя организации")

	var entry models.AuditLog
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ? AND action = ?",
		models.AuditEntityOrganization, td.OrgID, models.OrganizationActionRetired).First(&entry).Error)
}

// TestEntityRetire_PreservesAlreadyInactiveUser - ключевой инвариант обратимости:
// пользователь, погашенный ДО retire по другой причине (уволен, архив), не должен
// "ожить" после restore. Это держится на фильтре WHERE is_active = true у UPDATE -
// без него пользователь попал бы в список погашенных retire и был бы включён обратно
// вместе с остальными.
func TestEntityRetire_PreservesAlreadyInactiveUser(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	activeID := seedRetireUser(t, db, "retire_stays_active_then_off", td.OrgID, true)
	alreadyOffID := seedRetireUser(t, db, "retire_already_inactive", td.OrgID, false)
	rec := services.NewAuditRecorder(db)
	ctx := context.Background()

	res, err := entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int{activeID}, res.Users,
		"retire захватывает только реально активных пользователей, не уже погашенного")
	assert.False(t, userIsActive(t, db, alreadyOffID), "уже неактивный пользователь остаётся неактивным")

	restored, err := entityarchive.Restore(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int{activeID}, restored.Users)
	assert.True(t, userIsActive(t, db, activeID), "restore вернул к жизни того, кого погасил retire")
	assert.False(t, userIsActive(t, db, alreadyOffID),
		"restore не оживил пользователя, погашенного до retire - ключевой инвариант обратимости")
}

// TestEntityRestore_RequiresPriorRetire: без записи retire restore обязан отказать, а не
// молча включить всё, что сейчас неактивно.
func TestEntityRestore_RequiresPriorRetire(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	rec := services.NewAuditRecorder(db)

	_, err := entityarchive.Restore(context.Background(), db, rec, entityarchive.TypeOrganization, td.OrgID, nil, false)
	require.Error(t, err, "restore без предшествующего retire должен отказать")
}

// TestEntityRestore_RejectsAfterAlreadyRestored: повторный restore после того, как
// организацию уже вернули, тоже обязан отказать - последним действием стал restore, а
// не retire, значит откатывать нечего.
func TestEntityRestore_RejectsAfterAlreadyRestored(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	rec := services.NewAuditRecorder(db)
	ctx := context.Background()

	_, err := entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)
	_, err = entityarchive.Restore(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)

	_, err = entityarchive.Restore(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, false)
	require.Error(t, err, "повторный restore после уже выполненного должен отказать")
}

// TestEntityRetire_DryRunChangesNothing: без -apply retire ничего не пишет ни в
// организацию/пользователей, ни в audit_log.
func TestEntityRetire_DryRunChangesNothing(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	userID := seedRetireUser(t, db, "retire_dry_run_user", td.OrgID, true)
	rec := services.NewAuditRecorder(db)

	_, err := entityarchive.Retire(context.Background(), db, rec, entityarchive.TypeOrganization, td.OrgID, nil, false)
	require.NoError(t, err)

	assert.True(t, orgIsActive(t, db, td.OrgID))
	assert.True(t, userIsActive(t, db, userID))
	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ?", models.AuditEntityOrganization, td.OrgID).
		Count(&count).Error)
	assert.Zero(t, count, "dry-run не пишет в audit_log")
}

// TestEntityRetire_RejectsMissingOrganization - дефект, найденный ревью: Retire нигде не
// проверял существование организации, и опечатка в -id получала то же "уже погашена, новая
// запись не создана", что и настоящий повторный retire. Оператор пошёл бы искать в
// audit_log запись retired, не нашёл бы её и потерял время на ровном месте. dry-run и apply
// обязаны отвечать одинаково - иначе правда вскроется только на боевом запуске.
func TestEntityRetire_RejectsMissingOrganization(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	rec := services.NewAuditRecorder(db)
	ctx := context.Background()
	const missingID = 999999

	_, err := entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, missingID, nil, false)
	require.Error(t, err, "dry-run по несуществующей организации обязан отказать")
	assert.Contains(t, err.Error(), "не найдена", "сообщение отличается от «уже погашена»")

	_, err = entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, missingID, nil, true)
	require.Error(t, err, "apply по несуществующей организации обязан отказать так же")
	assert.Contains(t, err.Error(), "не найдена")

	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ?", models.AuditEntityOrganization, missingID).
		Count(&count).Error)
	assert.Zero(t, count, "по несуществующей организации ничего не пишется в audit_log")
}

// TestEntityRetire_DistinguishesMissingFromAlreadyRetired: существующая, но уже погашенная
// организация, и организация, которой никогда не было, обязаны получать РАЗНЫЕ сообщения -
// иначе они неразличимы для оператора.
func TestEntityRetire_DistinguishesMissingFromAlreadyRetired(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	rec := services.NewAuditRecorder(db)
	ctx := context.Background()

	_, err := entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)

	_, alreadyErr := entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.Error(t, alreadyErr)
	_, missingErr := entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID+999999, nil, true)
	require.Error(t, missingErr)

	assert.NotEqual(t, alreadyErr.Error(), missingErr.Error(), "«уже погашена» и «не найдена» не должны совпадать")
	assert.Contains(t, alreadyErr.Error(), "уже погашена")
	assert.Contains(t, missingErr.Error(), "не найдена")
}

// TestEntityRetire_RepeatDoesNotOverwriteHistory - дефект, найденный ревью: повторный
// retire, которому уже нечего гасить, не должен писать пустую запись в audit_log - иначе
// она станет для restore "последней retire" и заслонит список id настоящего первого
// retire. Организация осталась бы погашенной без штатного способа вернуться.
func TestEntityRetire_RepeatDoesNotOverwriteHistory(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	userID := seedRetireUser(t, db, "retire_repeat_user", td.OrgID, true)
	rec := services.NewAuditRecorder(db)
	ctx := context.Background()

	first, err := entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)
	assert.Equal(t, []int{td.OrgID}, first.Organizations)
	assert.Equal(t, []int{userID}, first.Users)

	_, err = entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.Error(t, err, "повторный retire, которому нечего гасить, обязан отказать")

	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityOrganization, td.OrgID, models.OrganizationActionRetired).
		Count(&count).Error)
	assert.EqualValues(t, 1, count, "повторный retire не добавил вторую (пустую) запись")

	restored, err := entityarchive.Restore(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)
	assert.Equal(t, []int{td.OrgID}, restored.Organizations, "restore видит организацию из ПЕРВОГО retire")
	assert.Equal(t, []int{userID}, restored.Users, "restore видит пользователя из ПЕРВОГО retire")
	assert.True(t, orgIsActive(t, db, td.OrgID))
	assert.True(t, userIsActive(t, db, userID))
}

// TestEntityRetire_SkipsSuperAdmin - дефект, найденный ревью: retire не должен гасить
// супер-администратора организации (тот же запрет, что в user_service.setActive -
// "иначе админ может вырубить владельца"). Пропуск обязан быть виден в результате и в
// dry-run, и после apply - молчание превратило бы неполный офбординг в мнимо полный.
func TestEntityRetire_SkipsSuperAdmin(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	regularID := seedRetireUser(t, db, "retire_regular_user", td.OrgID, true)
	superID := seedRetireSuperAdmin(t, db, "retire_super_admin", td.OrgID)
	rec := services.NewAuditRecorder(db)
	ctx := context.Background()

	dry, err := entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, false)
	require.NoError(t, err)
	assert.Equal(t, []int{superID}, dry.SkippedSuperAdmins, "dry-run уже показывает пропуск супер-админа")
	assert.NotContains(t, dry.Users, superID)

	res, err := entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)
	assert.Equal(t, []int{regularID}, res.Users, "погашен только обычный пользователь")
	assert.Equal(t, []int{superID}, res.SkippedSuperAdmins)
	assert.False(t, orgIsActive(t, db, td.OrgID))
	assert.False(t, userIsActive(t, db, regularID), "обычный пользователь погашен")
	assert.True(t, userIsActive(t, db, superID), "супер-админ организации остаётся активным")
}

// TestEntityRetire_TreatsNullSuperAdminAsRegularUser - дефект, найденный ревью:
// users.is_super_admin в схеме DEFAULT false, но БЕЗ NOT NULL. "AND is_super_admin" и
// "AND NOT is_super_admin" на строке с NULL дают NULL (SQL three-valued logic) - такая
// строка не попадала бы ни в гашение, ни в список пропущенных, оставаясь активной, пока
// команда рапортует полный офбординг. gorm не умеет вставить NULL в bool-поле структурой,
// поэтому NULL заводится сырым SQL, как и советовало ревью.
func TestEntityRetire_TreatsNullSuperAdminAsRegularUser(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	nullSuperID := seedRetireUser(t, db, "retire_null_super_admin", td.OrgID, true)
	require.NoError(t, db.Exec("UPDATE users SET is_super_admin = NULL WHERE id = ?", nullSuperID).Error)
	rec := services.NewAuditRecorder(db)
	ctx := context.Background()

	dry, err := entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, false)
	require.NoError(t, err)
	assert.Contains(t, dry.Users, nullSuperID, "NULL в is_super_admin - обычный пользователь, попадает под гашение")
	assert.NotContains(t, dry.SkippedSuperAdmins, nullSuperID, "NULL не должен попасть и в список пропущенных")

	res, err := entityarchive.Retire(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)
	assert.Contains(t, res.Users, nullSuperID)
	assert.NotContains(t, res.SkippedSuperAdmins, nullSuperID)
	assert.False(t, userIsActive(t, db, nullSuperID), "пользователь с NULL в is_super_admin погашен как обычный")
}

// TestEntityRetire_RevokesActiveRefreshTokens: retire отзывает активные refresh-токены
// погашенных пользователей - зеркалит user_service.setActive (обычная архивация делает то
// же самое рядом с is_active=false).
func TestEntityRetire_RevokesActiveRefreshTokens(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	userID := seedRetireUser(t, db, "retire_token_user", td.OrgID, true)
	token := models.RefreshToken{UserID: userID, TokenHash: "retire-test-hash", ExpiresAt: time.Now().Add(24 * time.Hour)}
	require.NoError(t, db.Create(&token).Error)
	rec := services.NewAuditRecorder(db)

	_, err := entityarchive.Retire(context.Background(), db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)

	var revoked bool
	require.NoError(t, db.Raw("SELECT is_revoked FROM refresh_tokens WHERE id = ?", token.ID).Scan(&revoked).Error)
	assert.True(t, revoked, "retire отзывает активные refresh-токены погашенных пользователей")
}

// TestEntityRetire_DoesNotTouchOtherOrganization: retire одной организации не задевает
// пользователей другой.
func TestEntityRetire_DoesNotTouchOtherOrganization(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	otherOrg := models.Organization{Name: "Соседняя организация"}
	require.NoError(t, db.Create(&otherOrg).Error)
	otherUserID := seedRetireUser(t, db, "retire_other_org_user", otherOrg.ID, true)
	rec := services.NewAuditRecorder(db)

	_, err := entityarchive.Retire(context.Background(), db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)

	assert.True(t, orgIsActive(t, db, otherOrg.ID), "чужая организация не задета")
	assert.True(t, userIsActive(t, db, otherUserID), "пользователь чужой организации не задет")
}
