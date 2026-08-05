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
