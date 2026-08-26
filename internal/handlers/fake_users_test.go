package handlers_test

// Проверка среза наливки пользователей и согласующих (#1682, том 5): после прогона
// fakedata.Run пользователи реально созданы через сервисный слой и читаются обратно,
// под созданным (не заблокированным/архивным) пользователем реально проходит вход, у
// организаций есть согласующие с required_approval, есть и заблокированные, и
// архивные, всё зарегистрировано в партии, повторный прогон не падает.
// testutil.SetupTestApp поднимает базу -- по правилу проекта такие тесты живут только в
// internal/handlers. Профиль "small" выбран нарочно маленьким: пакет handlers и так на
// грани CI-таймаута под -race.

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

func TestFakeUsers_RunCreatesUsersAndAssignsRoles(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)

	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-users"), 9090, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 9090}))

	// --- пользователи реально созданы через сервисный слой и зарегистрированы в партии ---

	require.Equal(t, profile.Users, batch.Counts()[models.AuditEntityUser])

	userSvc := services.NewUserService(db, nil)
	allUsers, err := userSvc.GetAll(ctx, true)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(allUsers), profile.Users, "среди всех пользователей должны быть хотя бы созданные партией (плюс сидовый админ)")

	var userItems []models.FakeBatchItem
	require.NoError(t, db.Where("batch_id = ? AND entity = ?", batch.ID(), models.AuditEntityUser).
		Order("id").Find(&userItems).Error)
	require.Len(t, userItems, profile.Users)

	// --- типы пользователей разнообразны, а не сплошной "user" ---

	var typeIDs []int
	require.NoError(t, db.Table("users").
		Joins("JOIN fake_batch_items fbi ON fbi.entity_id = users.id AND fbi.entity = ?", models.AuditEntityUser).
		Where("fbi.batch_id = ?", batch.ID()).
		Pluck("users.type_id", &typeIDs).Error)
	distinctTypes := map[int]bool{}
	for _, id := range typeIDs {
		distinctTypes[id] = true
	}
	require.Greater(t, len(distinctTypes), 1, "тип пользователя не должен быть одним и тем же для всей партии")

	// --- есть и заблокированные, и архивные -- на стенде должно быть на чём проверять оба состояния ---

	var bannedCount, archivedCount int64
	require.NoError(t, db.Table("users").
		Joins("JOIN fake_batch_items fbi ON fbi.entity_id = users.id AND fbi.entity = ?", models.AuditEntityUser).
		Where("fbi.batch_id = ? AND users.is_banned = true", batch.ID()).
		Count(&bannedCount).Error)
	require.Positive(t, bannedCount, "среди пользователей партии должен быть хотя бы один заблокированный")

	require.NoError(t, db.Table("users").
		Joins("JOIN fake_batch_items fbi ON fbi.entity_id = users.id AND fbi.entity = ?", models.AuditEntityUser).
		Where("fbi.batch_id = ? AND users.is_active = false", batch.ID()).
		Count(&archivedCount).Error)
	require.Positive(t, archivedCount, "среди пользователей партии должен быть хотя бы один архивный")

	// --- часть получает права администратора (реальный механизм привилегий после
	// отвязки от user_type, см. project_auth_decouple_usertype) ---

	var adminCount int64
	require.NoError(t, db.Table("users").
		Joins("JOIN fake_batch_items fbi ON fbi.entity_id = users.id AND fbi.entity = ?", models.AuditEntityUser).
		Where("fbi.batch_id = ? AND users.is_admin = true", batch.ID()).
		Count(&adminCount).Error)
	require.Positive(t, adminCount, "часть партии должна получить права администратора")

	// --- принимающие: количество совпадает с тем, что честно показал Plan ---

	items := fakedata.Plan(profile)
	var wantApprovers int
	for _, item := range items {
		if item.Entity == models.AuditEntityApprover {
			wantApprovers = item.Count
		}
	}
	require.Positive(t, wantApprovers)

	approverSvc := services.NewApproverService(db)
	approvers, err := approverSvc.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, approvers, wantApprovers)
	require.Equal(t, wantApprovers, batch.Counts()[models.AuditEntityApprover])

	// --- у каждой организации есть согласующий с required_approval -- иначе будущим
	// заявкам некому назначить согласование (ключевое требование #1682, том 5) ---

	orgSvc := services.NewOrganizationService(db)
	orgs, err := orgSvc.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, orgs, profile.Organizations)
	for _, org := range orgs {
		responsibles, err := orgSvc.GetOrganizationUsers(ctx, org.ID)
		require.NoError(t, err)
		found := false
		for _, u := range responsibles {
			if u.RequiredApproval != nil && *u.RequiredApproval {
				found = true
				break
			}
		}
		require.True(t, found, "организация %q (id=%d) должна иметь согласующего с required_approval", org.Name, org.ID)
	}

	// --- под реально созданным (активным) пользователем проходит вход ---

	var activeUsername string
	require.NoError(t, db.Table("users").
		Joins("JOIN fake_batch_items fbi ON fbi.entity_id = users.id AND fbi.entity = ?", models.AuditEntityUser).
		Where("fbi.batch_id = ? AND users.is_banned = false AND users.is_active = true", batch.ID()).
		Limit(1).Pluck("users.username", &activeUsername).Error)
	require.NotEmpty(t, activeUsername, "среди созданных пользователей должен быть хотя бы один активный для входа")

	access, refresh := testutil.LoginUser(t, e, activeUsername, fakedata.DefaultUserPassword)
	require.NotEmpty(t, access, "под созданным наливкой пользователем должен реально проходить вход")
	require.NotEmpty(t, refresh)

	// --- повторный прогон не падает и добирает согласующих новым организациям ---

	batch2, err := fakedata.OpenBatch(ctx, db, uniq("fake-users-2"), 9191, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch2, Profile: profile, Seed: 9191}))
	require.Equal(t, profile.Users, batch2.Counts()[models.AuditEntityUser])

	orgsAfter, err := orgSvc.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, orgsAfter, 2*profile.Organizations, "второй прогон заводит свою порцию новых организаций")
	for _, org := range orgsAfter {
		responsibles, err := orgSvc.GetOrganizationUsers(ctx, org.ID)
		require.NoError(t, err)
		found := false
		for _, u := range responsibles {
			if u.RequiredApproval != nil && *u.RequiredApproval {
				found = true
				break
			}
		}
		require.True(t, found, "после второго прогона у организации %q (id=%d) тоже должен быть согласующий", org.Name, org.ID)
	}
}

// Пользователей нечем привязывать без организаций/компаний -- шаг обязан честно упасть,
// а не молча создать пользователей без принадлежности (userService.Create такого и не
// позволит, но диагностика должна указать на первопричину, а не на низкоуровневую ошибку).
func TestFakeUsers_FailsWhenNoOrganizationsOrCompanies(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile := fakedata.Profile{
		Name: "no-orgs", Organizations: 0, Companies: 0, Users: 10,
		Employees: 0, Cars: 0, Applications: 0, Blacklists: 0, DaysBack: 30,
	}

	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-users-empty"), 1234, profile.Name)
	require.NoError(t, err)

	err = fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 1234})

	require.Error(t, err, "наливка пользователей без организаций и компаний обязана сообщить об отказе")
	require.Contains(t, err.Error(), "проверенной организации")
}

// Наливка не имеет права терять строку ответственного, чей пользователь заархивирован.
//
// Состав ответственных заменяется целиком, а сервис чтения отдаёт только активных: если
// собрать список из него, архивный ответственный исчезнет из organization_users при
// первом же повторном прогоне, вместе со сведениями о том, кто был согласующим.
func TestFakeUsers_KeepsArchivedApproverRow(t *testing.T) {
	e, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	admin := seedFakeAdmin(t, db)
	_ = e

	orgSvc := services.NewOrganizationService(db)
	orgType := models.OrgTypeValues[0]
	org, err := orgSvc.Create(ctx, admin.ID, services.CreateOrganizationRequest{
		Name: uniq("Архивная орг"), Type: &orgType,
	})
	require.NoError(t, err)

	userSvc := services.NewUserService(db, services.NewNotificationService(db))
	archivedName := uniq("fake_archived_appr")
	require.NoError(t, userSvc.Create(ctx, admin.ID, models.RegisterRequest{
		Username: archivedName, Password: fakedata.DefaultUserPassword, OrganizationID: org.ID,
	}))
	yes := true
	require.NoError(t, orgSvc.UpdateOrganizationUsers(ctx, admin.ID, org.ID, services.UpdateOrganizationUsersRequest{
		Users: []services.OrganizationUserRequest{{Username: archivedName, IsPrimary: &yes, RequiredApproval: &yes}},
	}))
	require.NoError(t, userSvc.Delete(ctx, admin.ID, archivedName))

	rowsBefore := countOrgUserRows(t, db, org.ID)
	require.Equal(t, int64(1), rowsBefore, "строка ответственного должна пережить архивацию пользователя")

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-archived"), 424, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 424}))

	require.Equal(t, rowsBefore+1, countOrgUserRows(t, db, org.ID),
		"наливка должна была добавить согласующего, не удалив архивного")
}

func countOrgUserRows(t *testing.T, db *gorm.DB, orgID int) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table("organization_users").Where("organization_id = ?", orgID).Count(&n).Error)
	return n
}

// Наливка заводит пользователей одним общим паролем. Существующих учётных записей это
// касаться не должно: совпадение логина обязано быть отказом на создание, а не тихой
// сменой пароля живому человеку.
func TestFakeUsers_DoesNotTouchExistingPassword(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	admin := seedFakeAdmin(t, db)

	var before string
	require.NoError(t, db.Raw(`SELECT password FROM users WHERE id = ?`, admin.ID).Scan(&before).Error)
	require.NotEmpty(t, before)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-pass"), 909, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 909}))

	var after string
	require.NoError(t, db.Raw(`SELECT password FROM users WHERE id = ?`, admin.ID).Scan(&after).Error)
	require.Equal(t, before, after, "пароль существующего пользователя наливка менять не должна")
}
