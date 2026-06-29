package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newPersonBlacklistService(db *gorm.DB) services.PersonBlacklistService {
	return services.NewPersonBlacklistService(db, services.NewAuditRecorder(db))
}

// TestPersonBlacklist_Lifecycle: create/check/строгое-ФИО/дубль/archive/restore без employees.
func TestPersonBlacklist_Lifecycle(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, userCleanup := setupMWUser(t, db, true, false)
	defer userCleanup()
	svc := newPersonBlacklistService(db)
	ctx := context.Background()

	withMiddle, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Петров", FirstName: "Пётр", MiddleName: "Петрович", Reason: "тест",
	}, userID)
	require.NoError(t, err)
	require.NotZero(t, withMiddle.ID)

	noMiddle, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Сидоров", FirstName: "Сидор", Reason: "тест2",
	}, userID)
	require.NoError(t, err)
	require.Nil(t, noMiddle.MiddleName, "пустое отчество должно храниться как NULL")

	checks := []struct {
		name                string
		last, first, middle string
		wantBlock           bool
	}{
		{"полное совпадение с отчеством", "Петров", "Пётр", "Петрович", true},
		{"регистр/пробелы", " петров ", "ПЁТР", "петрович", true},
		{"есть отчество в ЧС - без отчества не матчит", "Петров", "Пётр", "", false},
		{"есть отчество в ЧС - другое отчество не матчит", "Петров", "Пётр", "Другое", false},
		{"нет отчества в ЧС - без отчества матчит", "Сидоров", "Сидор", "", true},
		{"нет отчества в ЧС - с отчеством не матчит", "Сидоров", "Сидор", "Иванович", false},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			res, err := svc.Check(ctx, tc.last, tc.first, tc.middle)
			require.NoError(t, err)
			assert.Equal(t, tc.wantBlock, res.IsBlacklisted)
		})
	}

	// Дубль (даже в другом регистре/с пробелами) блокируется partial unique index-ом.
	_, err = svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "ПЕТРОВ", FirstName: "пётр", MiddleName: " Петрович ", Reason: "дубль",
	}, userID)
	assertHTTPStatus(t, err, 409)

	// После снятия можно добавить заново.
	require.NoError(t, svc.Archive(ctx, withMiddle.ID, userID))
	res, err := svc.Check(ctx, "Петров", "Пётр", "Петрович")
	require.NoError(t, err)
	assert.False(t, res.IsBlacklisted)

	again, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Петров", FirstName: "Пётр", MiddleName: "Петрович", Reason: "снова",
	}, userID)
	require.NoError(t, err)
	assert.NotEqual(t, withMiddle.ID, again.ID)

	// Restore архивной при активном дубле -> 409.
	assertHTTPStatus(t, svc.Restore(ctx, withMiddle.ID, userID), 409)
}

// TestPersonBlacklist_CascadeDeactivatesActiveEmployee: добавление в ЧС гасит активного сотрудника.
func TestPersonBlacklist_CascadeDeactivatesActiveEmployee(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "pblcasc1", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "pblcasc1")
	appID, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	var before models.Employee
	require.NoError(t, db.First(&before, empID).Error)
	require.NotNil(t, before.Status)
	require.Equal(t, 1, *before.Status, "сотрудник должен быть активен до ЧС")

	svc := newPersonBlacklistService(db)
	entry, err := svc.Create(context.Background(), models.CreatePersonBlacklistRequest{
		LastName: "Ivanov", FirstName: "Ivan", MiddleName: "Ivanovich", Reason: "запрет",
	}, userID)
	require.NoError(t, err)

	var after models.Employee
	require.NoError(t, db.First(&after, empID).Error)
	require.NotNil(t, after.Status)
	assert.Equal(t, 0, *after.Status, "сотрудник должен деактивироваться")
	assert.NotNil(t, after.DateDeleted, "date_deleted должен проставиться")

	var empHistCount int64
	db.Model(&models.AuditLog{}).Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, empID, "blacklisted").Count(&empHistCount)
	assert.Equal(t, int64(1), empHistCount, "должна быть запись audit_log blacklisted (#870, срез 1.13b)")

	var blHistCount int64
	db.Table("audit_log").Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityPersonBlacklist, entry.ID, models.BlacklistActionCreated).Count(&blHistCount)
	assert.Equal(t, int64(1), blHistCount)
}

// TestPersonBlacklist_UnblacklistRestoresActiveApplicationEmployee: снятие возвращает status=1.
func TestPersonBlacklist_UnblacklistRestoresActiveApplicationEmployee(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "pblcasc2", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "pblcasc2")
	appID, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	svc := newPersonBlacklistService(db)
	ctx := context.Background()

	entry, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Ivanov", FirstName: "Ivan", MiddleName: "Ivanovich", Reason: "проверка",
	}, userID)
	require.NoError(t, err)

	var blacklisted models.Employee
	require.NoError(t, db.First(&blacklisted, empID).Error)
	require.Equal(t, 0, *blacklisted.Status)

	require.NoError(t, svc.Archive(ctx, entry.ID, userID))

	var restored models.Employee
	require.NoError(t, db.First(&restored, empID).Error)
	require.NotNil(t, restored.Status)
	assert.Equal(t, 1, *restored.Status, "сотрудник с активной заявкой должен вернуться в status=1")
	assert.Nil(t, restored.DateDeleted, "date_deleted должен очиститься")

	var empHistCount int64
	db.Model(&models.AuditLog{}).Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, empID, "unblacklisted").Count(&empHistCount)
	assert.Equal(t, int64(1), empHistCount)
}

// TestPersonBlacklist_UnblacklistSkipsExpiredPass: просроченный пропуск не возрождается
// (дата берётся из attachments.entry_date_to - у employees своего поля нет).
func TestPersonBlacklist_UnblacklistSkipsExpiredPass(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "pblexp1", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "pblexp1")
	appID, attID, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	svc := newPersonBlacklistService(db)
	ctx := context.Background()

	entry, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Ivanov", FirstName: "Ivan", MiddleName: "Ivanovich", Reason: "проверка",
	}, userID)
	require.NoError(t, err)

	require.NoError(t, db.Exec("UPDATE attachments SET entry_date_to = ? WHERE id = ?", "2000-01-01", attID).Error)

	require.NoError(t, svc.Archive(ctx, entry.ID, userID))

	var after models.Employee
	require.NoError(t, db.First(&after, empID).Error)
	require.NotNil(t, after.Status)
	assert.Equal(t, 0, *after.Status, "сотрудник с истёкшим пропуском не должен возрождаться")
}

// TestPersonBlacklist_Purge: удаление архивной записи навсегда стирает запись, но
// сохраняет историю и пишет событие purged; активную удалять нельзя.
func TestPersonBlacklist_Purge(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, userCleanup := setupMWUser(t, db, true, false)
	defer userCleanup()
	svc := newPersonBlacklistService(db)
	ctx := context.Background()

	entry, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Удалённый", FirstName: "Иван", MiddleName: "Иванович", Reason: "на удаление",
	}, userID)
	require.NoError(t, err)

	assertHTTPStatus(t, svc.Purge(ctx, entry.ID, userID), 400)

	require.NoError(t, svc.Archive(ctx, entry.ID, userID))
	require.NoError(t, svc.Purge(ctx, entry.ID, userID))

	_, err = svc.GetByID(ctx, entry.ID)
	assertHTTPStatus(t, err, 404)

	hist, err := svc.GetHistory(ctx, entry.ID)
	require.NoError(t, err)
	actions := make([]string, 0, len(hist))
	for _, h := range hist {
		actions = append(actions, h.ActionType)
	}
	assert.Contains(t, actions, models.BlacklistActionCreated)
	assert.Contains(t, actions, models.BlacklistActionArchived)
	assert.Contains(t, actions, models.BlacklistActionPurged)
}

// TestPersonBlacklist_GetAllHistory: общий журнал отдаёт события всех записей (включая
// удалённую) с именем пользователя, новые сверху.
func TestPersonBlacklist_GetAllHistory(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, userCleanup := setupMWUser(t, db, true, false)
	defer userCleanup()
	svc := newPersonBlacklistService(db)
	ctx := context.Background()

	e1, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Первый", FirstName: "Один", Reason: "первая",
	}, userID)
	require.NoError(t, err)
	e2, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Второй", FirstName: "Два", Reason: "вторая",
	}, userID)
	require.NoError(t, err)
	require.NoError(t, svc.Archive(ctx, e2.ID, userID))
	require.NoError(t, svc.Purge(ctx, e2.ID, userID))

	all, err := svc.GetAllHistory(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(all), 4)

	var sawE1Created, sawE2Purged bool
	for _, h := range all {
		assert.NotEmpty(t, h.UserName, "должно подтягиваться имя пользователя")
		if h.EntityID == e1.ID && h.ActionType == models.BlacklistActionCreated {
			sawE1Created = true
		}
		if h.EntityID == e2.ID && h.ActionType == models.BlacklistActionPurged {
			sawE2Purged = true
		}
	}
	assert.True(t, sawE1Created, "журнал должен содержать created первой записи")
	assert.True(t, sawE2Purged, "журнал должен содержать purged удалённой записи")

	for i := 1; i < len(all); i++ {
		assert.False(t, all[i].CreatedAt.After(all[i-1].CreatedAt), "события должны идти от новых к старым")
	}
}

// TestPersonBlacklist_FindSimilar: нечёткий поиск (#481) ловит латиничный гомоглиф, опечатку
// и отсутствие отчества; не ловит постороннее ФИО; игнорирует архивные.
func TestPersonBlacklist_FindSimilar(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, userCleanup := setupMWUser(t, db, true, false)
	defer userCleanup()
	svc := newPersonBlacklistService(db)
	ctx := context.Background()

	target, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Иванов", FirstName: "Иван", MiddleName: "Иванович", Reason: "обход",
	}, userID)
	require.NoError(t, err)
	_, err = svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Сидоров", FirstName: "Сидор", MiddleName: "Сидорович", Reason: "фон",
	}, userID)
	require.NoError(t, err)

	containsTarget := func(t *testing.T, res []models.BlacklistSimilarMatch) models.BlacklistSimilarMatch {
		t.Helper()
		for _, m := range res {
			if m.ID == target.ID {
				return m
			}
		}
		t.Fatalf("ожидали эталон id=%d среди похожих, получили %+v", target.ID, res)
		return models.BlacklistSimilarMatch{}
	}

	t.Run("латиница-гомоглиф в фамилии (обход) -> sim ~1.0", func(t *testing.T) {
		// "Ивaнов" с латинской 'a' нормализуется в "иванов" - сырой Check бы не сматчил.
		res, err := svc.FindSimilar(ctx, "Ивaнов", "Иван", "Иванович")
		require.NoError(t, err)
		m := containsTarget(t, res)
		assert.InDelta(t, 1.0, m.Similarity, 1e-9)
		assert.Equal(t, "обход", m.Reason)
		assert.Contains(t, m.MatchedValue, "Иванов")
	})

	t.Run("без отчества (обход) -> похоже, >=0.7", func(t *testing.T) {
		res, err := svc.FindSimilar(ctx, "Иванов", "Иван", "")
		require.NoError(t, err)
		m := containsTarget(t, res)
		assert.GreaterOrEqual(t, m.Similarity, 0.7)
	})

	t.Run("опечатка в имени -> похоже", func(t *testing.T) {
		res, err := svc.FindSimilar(ctx, "Иванов", "Иваи", "Иванович")
		require.NoError(t, err)
		_ = containsTarget(t, res)
	})

	t.Run("постороннее ФИО -> эталон не матчится", func(t *testing.T) {
		res, err := svc.FindSimilar(ctx, "Кузнецов", "Алексей", "Петрович")
		require.NoError(t, err)
		for _, m := range res {
			assert.NotEqual(t, target.ID, m.ID)
		}
	})

	t.Run("архивная запись не находится", func(t *testing.T) {
		require.NoError(t, svc.Archive(ctx, target.ID, userID))
		res, err := svc.FindSimilar(ctx, "Иванов", "Иван", "Иванович")
		require.NoError(t, err)
		for _, m := range res {
			assert.NotEqual(t, target.ID, m.ID)
		}
	})
}

// TestPersonBlacklist_Update покрывает правку ФИО/причины: лог истории, пересчёт нормали,
// дубль -> 409, запрет правки архивной.
func TestPersonBlacklist_Update(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, userCleanup := setupMWUser(t, db, true, false)
	defer userCleanup()
	svc := newPersonBlacklistService(db)
	ctx := context.Background()

	entry, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Иванов", FirstName: "Иван", MiddleName: "Иванович", Reason: "старая",
	}, userID)
	require.NoError(t, err)
	oldFIO := entry.NormalizedFIO

	t.Run("правка причины - сохраняется + история", func(t *testing.T) {
		_, err := svc.Update(ctx, entry.ID, models.UpdatePersonBlacklistRequest{
			LastName: "Иванов", FirstName: "Иван", MiddleName: "Иванович", Reason: "  новая  ",
		}, userID)
		require.NoError(t, err)
		stored, err := svc.GetByID(ctx, entry.ID)
		require.NoError(t, err)
		assert.Equal(t, "новая", stored.Reason)
	})

	t.Run("смена ФИО - пересчёт нормали", func(t *testing.T) {
		_, err := svc.Update(ctx, entry.ID, models.UpdatePersonBlacklistRequest{
			LastName: "Петров", FirstName: "Пётр", MiddleName: "Петрович", Reason: "новая",
		}, userID)
		require.NoError(t, err)
		stored, err := svc.GetByID(ctx, entry.ID)
		require.NoError(t, err)
		assert.Equal(t, "Петров", stored.LastName)
		assert.NotEqual(t, oldFIO, stored.NormalizedFIO, "нормаль должна пересчитаться")
		assert.NotEmpty(t, stored.NormalizedFIO)
	})

	t.Run("пустые фамилия/имя - 400", func(t *testing.T) {
		_, err := svc.Update(ctx, entry.ID, models.UpdatePersonBlacklistRequest{
			LastName: "  ", FirstName: "Пётр", Reason: "x",
		}, userID)
		assertHTTPStatus(t, err, 400)
	})

	t.Run("дубль активной записи - 409", func(t *testing.T) {
		other, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
			LastName: "Сидоров", FirstName: "Сидор", Reason: "вторая",
		}, userID)
		require.NoError(t, err)
		_, err = svc.Update(ctx, other.ID, models.UpdatePersonBlacklistRequest{
			LastName: "Петров", FirstName: "Пётр", MiddleName: "Петрович", Reason: "вторая",
		}, userID)
		assertHTTPStatus(t, err, 409)
	})

	t.Run("нельзя редактировать архивную - 400", func(t *testing.T) {
		require.NoError(t, svc.Archive(ctx, entry.ID, userID))
		_, err := svc.Update(ctx, entry.ID, models.UpdatePersonBlacklistRequest{
			LastName: "Петров", FirstName: "Пётр", MiddleName: "Петрович", Reason: "после архива",
		}, userID)
		assertHTTPStatus(t, err, 400)
	})
}
