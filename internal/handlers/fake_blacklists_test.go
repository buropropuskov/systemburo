package handlers_test

// Проверка среза наливки чёрных списков машин и людей (#1682, том 4): после прогона
// fakedata.Run записи реально созданы через сервисный слой, зарегистрированы в партии,
// причины блокировки разнообразны, повторный прогон не падает, и -- главное -- среди
// записей есть похожие (но не идентичные) на реальные записи реестров: FindSimilar
// находит их и подтверждает высокую близость, а не просто факт существования записи.
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

func TestFakeBlacklists_RunFillsBlacklistsWithSimilarPairs(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	// CleanDB здесь принципиален (в отличие от TestFakeDictionaries): без него в
	// vehicle_blacklists/person_blacklists могли остаться строки других тестов пакета, и
	// проверка "есть похожая пара" читала бы чужие данные вместо только что созданных
	// этим прогоном.
	testutil.CleanDB(t, db)

	admin := models.User{
		Username:     uniq("fake_bl_admin"),
		Password:     "x",
		TypeID:       1,
		IsSuperAdmin: true,
		IsActive:     true,
	}
	require.NoError(t, db.Create(&admin).Error)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)

	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-bl"), 5150, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 5150}))

	auditRecorder := services.NewAuditRecorder(db)
	vehicleSvc := services.NewVehicleBlacklistService(db, auditRecorder)
	personSvc := services.NewPersonBlacklistService(db, auditRecorder)

	// --- записи реально созданы через сервисный слой ---

	vehicleEntries, err := vehicleSvc.GetAll(ctx, false)
	require.NoError(t, err)
	require.Len(t, vehicleEntries, profile.Blacklists, "чёрный список машин должен получить ровно столько записей, сколько просит профиль")

	personEntries, err := personSvc.GetAll(ctx, false)
	require.NoError(t, err)
	require.Len(t, personEntries, profile.Blacklists, "чёрный список людей должен получить ровно столько записей, сколько просит профиль")

	// --- причины блокировки разнообразны, а не одна строка на все записи ---

	vehicleReasons := map[string]bool{}
	for _, e := range vehicleEntries {
		require.NotEmpty(t, e.Reason)
		vehicleReasons[e.Reason] = true
	}
	require.Greater(t, len(vehicleReasons), 1, "причины блокировки машин не должны быть одной строкой на все записи")

	personReasons := map[string]bool{}
	for _, e := range personEntries {
		require.NotEmpty(t, e.Reason)
		personReasons[e.Reason] = true
	}
	require.Greater(t, len(personReasons), 1, "причины блокировки людей не должны быть одной строкой на все записи")

	// --- всё созданное зарегистрировано в партии ---

	require.Equal(t, profile.Blacklists, batch.Counts()[models.AuditEntityVehicleBlacklist])
	require.Equal(t, profile.Blacklists, batch.Counts()[models.AuditEntityPersonBlacklist])

	var itemCount int64
	require.NoError(t, db.Model(&models.FakeBatchItem{}).
		Where("batch_id = ? AND entity IN ?", batch.ID(),
			[]string{models.AuditEntityVehicleBlacklist, models.AuditEntityPersonBlacklist}).
		Count(&itemCount).Error)
	require.Equal(t, int64(2*profile.Blacklists), itemCount, "число строк партии должно совпасть с суммой обеих сводок")

	// --- похожие пары реально похожи на реестр (проверено расстоянием, не фактом
	// существования): FindSimilar -- та же функция, что предупреждает оператора о
	// возможном обходе ЧС при подаче заявки (#481, detectBlacklistSimilarity) ---

	carSvc := services.NewUniqueCarService(db)
	cars, err := carSvc.GetAll(ctx, admin.Username, "all_system")
	require.NoError(t, err)
	require.Len(t, cars, profile.Cars)

	foundSimilarCar := false
	for _, c := range cars {
		if c.Number == nil {
			continue
		}
		matches, err := vehicleSvc.FindSimilar(ctx, *c.Number)
		require.NoError(t, err)
		for _, m := range matches {
			foundSimilarCar = true
			require.GreaterOrEqual(t, m.Similarity, 0.7,
				"похожая запись обязана пройти порог похожести FindSimilar, а не просто существовать")
			require.Less(t, m.Similarity, 1.0,
				"похожая пара должна отличаться от реестра номером, а не совпадать с ним точно")
		}
	}
	require.True(t, foundSimilarCar,
		"среди записей чёрного списка машин должна быть хотя бы одна похожая на запись реестра -- иначе detectBlacklistSimilarity нечем проверить")

	employeeSvc := services.NewUniqueEmployeeService(db)
	employees, err := employeeSvc.GetAll(ctx, admin.Username, "all_system")
	require.NoError(t, err)
	require.Len(t, employees, profile.Employees)

	foundSimilarPerson := false
	for _, e := range employees {
		if e.LastName == nil || e.FirstName == nil {
			continue
		}
		middle := ""
		if e.MiddleName != nil {
			middle = *e.MiddleName
		}
		matches, err := personSvc.FindSimilar(ctx, *e.LastName, *e.FirstName, middle)
		require.NoError(t, err)
		for _, m := range matches {
			foundSimilarPerson = true
			require.GreaterOrEqual(t, m.Similarity, 0.7,
				"похожая запись обязана пройти порог похожести FindSimilar, а не просто существовать")
			require.Less(t, m.Similarity, 1.0,
				"похожая пара должна отличаться от реестра фамилией, а не совпадать с ним точно")
		}
	}
	require.True(t, foundSimilarPerson,
		"среди записей чёрного списка людей должна быть хотя бы одна похожая на запись реестра -- иначе detectBlacklistSimilarity нечем проверить")

	// --- повторный прогон не падает на партиальных уникальных индексах ЧС ---

	batch2, err := fakedata.OpenBatch(ctx, db, uniq("fake-bl-2"), 6060, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch2, Profile: profile, Seed: 6060}))
	require.Equal(t, profile.Blacklists, batch2.Counts()[models.AuditEntityVehicleBlacklist])
	require.Equal(t, profile.Blacklists, batch2.Counts()[models.AuditEntityPersonBlacklist])
}

// Похожие пары строятся от реальных записей реестров -- пустой реестр (профиль без
// сотрудников/машин) означает, что строить их не от чего, и шаг обязан честно упасть, а не
// молча пропустить чёрные списки: без отказа "готово" на стенде с пустым чёрным списком
// заметили бы только открыв раздел вручную.
func TestFakeBlacklists_FailsWhenRegistriesEmpty(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile := fakedata.Profile{
		Name: "empty-registries", Organizations: 3, Companies: 3, Users: 10,
		Employees: 0, Cars: 0, Applications: 0, Blacklists: 5, DaysBack: 30,
	}

	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-bl-empty"), 7070, profile.Name)
	require.NoError(t, err)

	err = fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 7070})

	require.Error(t, err, "наливка без записей реестра обязана сообщить об отказе, а не пропустить чёрные списки молча")
	require.Contains(t, err.Error(), "реестр машин пуст")
}

// Запас до порога похожести тоньше всего на самых коротких ФИО словаря: триграммная
// близость падает с длиной строки. Сторож перебирает короткий хвост словаря, поэтому
// добавленная короткая фамилия сразу покажет, что «похожая» запись перестала ловиться
// детектором и пары стали бесполезными.
func TestFakeBlacklists_ShortestNameStillPassesThreshold(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	admin := seedFakeAdmin(t, db)

	personSvc := services.NewPersonBlacklistService(db, services.NewAuditRecorder(db))

	for _, name := range fakedata.ShortestFullNames(5) {
		// Мутация повторяет то, что делает шаг: замена последней буквы фамилии. Звать
		// приватную mutateLastRune через экспорт «для теста» не стоит -- проверяется
		// порог похожести, а не сама подстановка символа.
		runes := []rune(name.LastName)
		replacement := 'н'
		if runes[len(runes)-1] == replacement {
			replacement = 'к'
		}
		runes[len(runes)-1] = replacement
		mutated := string(runes)
		require.NotEqual(t, name.LastName, mutated)

		entry, err := personSvc.Create(ctx, models.CreatePersonBlacklistRequest{
			LastName:   mutated,
			FirstName:  name.FirstName,
			MiddleName: name.MiddleName,
			Reason:     "проверка порога похожести",
		}, admin.ID)
		require.NoError(t, err)

		matches, err := personSvc.FindSimilar(ctx, name.LastName, name.FirstName, name.MiddleName)
		require.NoError(t, err)
		require.NotEmpty(t, matches,
			"ФИО %q %q %q: похожая запись обязана находиться детектором, иначе пара бесполезна",
			name.LastName, name.FirstName, name.MiddleName)

		// Убираем запись из активных: следующая итерация проверяет свою пару, и чужая
		// похожая запись сделала бы проверку неотличимой от «нашлось что-то другое».
		require.NoError(t, personSvc.Archive(ctx, entry.ID, admin.ID))
	}
}
