package handlers_test

// Проверка среза проходов через посты (#1682, том 8): после прогона fakedata.Run
// часть машин/сотрудников принятых в работу заявок отмечена и остающейся на
// территории, и выехавшей, время въезда исторически согласовано с принятием заявки
// в работу (не раньше него, не позже "сейчас"), у выехавших выезд не раньше въезда,
// в истории есть записи entry/exit с теми же датами, суточный отчёт поста
// (DailyPassReportService) после CatchUp даёт непустой результат, повторный прогон
// не падает. testutil.SetupTestApp поднимает базу -- по правилу проекта такие тесты
// живут только в internal/handlers. Профиль "small" выбран нарочно маленьким.

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/fakedata"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// passageEntityRow -- снимок состояния машины/сотрудника после прохода вместе с
// моментом принятия заявки в работу, к которому историчность даты и сверяется.
type passageEntityRow struct {
	ID                 int
	TerritoryStatus    *int
	TerritoryEntryTime *time.Time
	UpdatedAt          time.Time
	AcceptedAt         *time.Time
}

func readBatchCarPassages(t *testing.T, db *gorm.DB, batchID int) []passageEntityRow {
	t.Helper()
	var rows []passageEntityRow
	require.NoError(t, db.Raw(`
		SELECT c.id, c.territory_status, c.territory_entry_time, c.updated_at, app.accepted_at
		FROM cars c
		JOIN attachments att ON att.id = c.attachment_id
		JOIN applications app ON app.id = att.application_id
		JOIN fake_batch_items fbi ON fbi.entity_id = app.id
		WHERE fbi.batch_id = ? AND fbi.entity = ? AND c.territory_status IS NOT NULL`,
		batchID, models.AuditEntityApplication).Scan(&rows).Error)
	return rows
}

func readBatchEmployeePassages(t *testing.T, db *gorm.DB, batchID int) []passageEntityRow {
	t.Helper()
	var rows []passageEntityRow
	require.NoError(t, db.Raw(`
		SELECT e.id, e.territory_status, e.territory_entry_time, e.updated_at, app.accepted_at
		FROM employees e
		JOIN attachments att ON att.id = e.attachment_id
		JOIN applications app ON app.id = att.application_id
		JOIN fake_batch_items fbi ON fbi.entity_id = app.id
		WHERE fbi.batch_id = ? AND fbi.entity = ? AND e.territory_status IS NOT NULL`,
		batchID, models.AuditEntityApplication).Scan(&rows).Error)
	return rows
}

type passageHistoryRow struct {
	EntityID  int
	Action    string
	CreatedAt time.Time
}

func readPassageHistory(t *testing.T, db *gorm.DB, entityType string, ids []int) []passageHistoryRow {
	t.Helper()
	if len(ids) == 0 {
		return nil
	}
	var rows []passageHistoryRow
	require.NoError(t, db.Raw(`
		SELECT entity_id, action, created_at FROM audit_log
		WHERE entity_type = ? AND entity_id IN ? AND action IN ('entry', 'exit')`,
		entityType, ids).Scan(&rows).Error)
	return rows
}

func TestFakePassages_RunMarksEntryAndExit(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-passages"), 9191, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 9191}))

	now := time.Now().UTC()

	cars := readBatchCarPassages(t, db, batch.ID())
	employees := readBatchEmployeePassages(t, db, batch.ID())
	require.NotEmpty(t, cars, "должна найтись хотя бы одна отмеченная машина")

	// Проходы обязаны быть разложены по прошлому вместе с заявками, а не собраться в
	// день прогона. Проверка «не позже сейчас» этого не доказывает: время прогона ей
	// удовлетворяет по определению, и сломанный перенос дат прошёл бы незамеченным.
	var passagesInPast int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM cars c
		JOIN attachments t ON t.id = c.attachment_id
		JOIN applications a ON a.id = t.application_id
		JOIN fake_batch_items i ON i.entity = 'application' AND i.entity_id = a.id AND i.batch_id = ?
		WHERE c.territory_entry_time IS NOT NULL
		  AND c.territory_entry_time < DATE_TRUNC('day', NOW())`, batch.ID()).Scan(&passagesInPast).Error)
	require.Positive(t, passagesInPast,
		"ни один проход не попал в прошлое: похоже, даты остались временем прогона наливки")
	require.NotEmpty(t, employees, "должен найтись хотя бы один отмеченный сотрудник")

	// --- у машин и у сотрудников по отдельности встречаются оба состояния:
	// остался на территории (status=1) и выехал (status=2) ---

	assertBothStates := func(rows []passageEntityRow, what string) {
		var onTerritory, exited int
		for _, r := range rows {
			require.NotNil(t, r.TerritoryStatus, "%s %d: territory_status не заполнен", what, r.ID)
			require.NotNil(t, r.AcceptedAt, "%s %d: у заявки нет accepted_at", what, r.ID)
			require.NotNil(t, r.TerritoryEntryTime, "%s %d: нет времени въезда/входа", what, r.ID)

			require.False(t, r.TerritoryEntryTime.Before(*r.AcceptedAt),
				"%s %d: въезд раньше принятия заявки в работу", what, r.ID)
			require.False(t, r.TerritoryEntryTime.After(now),
				"%s %d: въезд в будущем", what, r.ID)

			switch *r.TerritoryStatus {
			case 1:
				onTerritory++
			case 2:
				exited++
				// Строго After, не просто "не раньше" -- слабая проверка "не раньше"
				// осталась бы зелёной и при совпавших въезде/выезде (ровно та ловушка,
				// что уже ловила "не позже сейчас" в срезе стадий: граница-тавтология
				// молчит именно там, где должна сработать).
				require.True(t, r.UpdatedAt.After(*r.TerritoryEntryTime),
					"%s %d: выезд не строго позже въезда", what, r.ID)
				require.False(t, r.UpdatedAt.After(now), "%s %d: выезд в будущем", what, r.ID)
			default:
				t.Fatalf("%s %d: неожиданный territory_status %d", what, r.ID, *r.TerritoryStatus)
			}
		}
		require.Positive(t, onTerritory, "%s: должен остаться хотя бы один на территории", what)
		require.Positive(t, exited, "%s: должен быть хотя бы один выехавший", what)
	}
	assertBothStates(cars, "машина")
	assertBothStates(employees, "сотрудник")

	// --- история: у каждой отмеченной машины/сотрудника есть entry (и у выехавших --
	// exit), с датами, согласованными с самим переходом, не только с "не позже сейчас" ---

	carIDs := make([]int, len(cars))
	for i, r := range cars {
		carIDs[i] = r.ID
	}
	empIDs := make([]int, len(employees))
	for i, r := range employees {
		empIDs[i] = r.ID
	}
	carHistory := readPassageHistory(t, db, models.AuditEntityCar, carIDs)
	empHistory := readPassageHistory(t, db, models.AuditEntityEmployee, empIDs)
	require.NotEmpty(t, carHistory, "у отмеченных машин должна быть история entry/exit")
	require.NotEmpty(t, empHistory, "у отмеченных сотрудников должна быть история entry/exit")

	byEntity := func(rows []passageEntityRow) map[int]passageEntityRow {
		m := make(map[int]passageEntityRow, len(rows))
		for _, r := range rows {
			m[r.ID] = r
		}
		return m
	}
	carByID := byEntity(cars)
	empByID := byEntity(employees)

	checkHistoryDates := func(history []passageHistoryRow, byID map[int]passageEntityRow, what string) {
		for _, h := range history {
			row, ok := byID[h.EntityID]
			require.True(t, ok, "%s: история ссылается на неизвестную запись %d", what, h.EntityID)
			switch h.Action {
			case "entry":
				require.WithinDuration(t, *row.TerritoryEntryTime, h.CreatedAt, 0,
					"%s %d: дата entry в истории не совпадает с датой въезда", what, h.EntityID)
			case "exit":
				require.Equal(t, 2, *row.TerritoryStatus, "%s %d: exit в истории у не выехавшей записи", what, h.EntityID)
				require.WithinDuration(t, row.UpdatedAt, h.CreatedAt, 0,
					"%s %d: дата exit в истории не совпадает с датой выезда", what, h.EntityID)
			}
			require.False(t, h.CreatedAt.Before(*row.AcceptedAt), "%s %d: история прохода раньше принятия заявки в работу", what, h.EntityID)
			require.False(t, h.CreatedAt.After(now), "%s %d: история прохода в будущем", what, h.EntityID)
		}
	}
	checkHistoryDates(carHistory, carByID, "машина")
	checkHistoryDates(empHistory, empByID, "сотрудник")

	// --- в самой истории (не только в итоговых полях записи) exit датирован СТРОГО
	// позже entry той же сущности -- независимая от territory_entry_time/updated_at
	// проверка того же факта, ровно там, где его требует задача ---
	assertExitAfterEntryInHistory := func(history []passageHistoryRow, what string) {
		entryAt := make(map[int]time.Time)
		exitAt := make(map[int]time.Time)
		for _, h := range history {
			switch h.Action {
			case "entry":
				entryAt[h.EntityID] = h.CreatedAt
			case "exit":
				exitAt[h.EntityID] = h.CreatedAt
			}
		}
		checked := 0
		for id, exit := range exitAt {
			entry, ok := entryAt[id]
			require.True(t, ok, "%s %d: в истории есть exit без entry", what, id)
			require.True(t, exit.After(entry), "%s %d: в истории exit не строго позже entry", what, id)
			checked++
		}
		require.Positive(t, checked, "%s: должна найтись хотя бы одна пара entry/exit для сверки", what)
	}
	assertExitAfterEntryInHistory(carHistory, "машина")
	assertExitAfterEntryInHistory(empHistory, "сотрудник")

	// --- автор отметки -- реальный администратор (is_admin/is_super_admin), а не
	// призрачный actor_user_id=0 ---

	var actorIDs []int
	require.NoError(t, db.Raw(`
		SELECT DISTINCT actor_user_id FROM audit_log
		WHERE entity_type IN (?, ?) AND action IN ('entry', 'exit') AND entity_id IN ?`,
		models.AuditEntityCar, models.AuditEntityEmployee, append(append([]int{}, carIDs...), empIDs...)).
		Pluck("actor_user_id", &actorIDs).Error)
	require.NotEmpty(t, actorIDs)
	for _, id := range actorIDs {
		var isAdmin, isSuperAdmin bool
		require.NoError(t, db.Raw(`SELECT is_admin, is_super_admin FROM users WHERE id = ?`, id).
			Row().Scan(&isAdmin, &isSuperAdmin))
		require.True(t, isAdmin || isSuperAdmin, "actor_user_id %d не администратор", id)
	}

	// --- суточный отчёт по посту: после CatchUp закрытые окна непустые ---

	reportSvc := services.NewDailyPassReportService(db)
	require.NoError(t, reportSvc.CatchUp(ctx))

	var reportTotal int64
	require.NoError(t, db.Raw(`
		SELECT COALESCE(SUM(car_entries + car_exits + people_entries + people_exits), 0)
		FROM daily_pass_reports`).Scan(&reportTotal).Error)
	require.Positive(t, reportTotal, "суточный отчёт по постам должен показать хотя бы одно событие")

	// --- повторный прогон не падает и размечает проходы на СВОЕЙ, независимой партии ---

	batch2, err := fakedata.OpenBatch(ctx, db, uniq("fake-passages-2"), 9292, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch2, Profile: profile, Seed: 9292}))
	cars2 := readBatchCarPassages(t, db, batch2.ID())
	require.NotEmpty(t, cars2, "повторный прогон тоже должен отметить проходы")
}

// Профиль без заявок означает, что applicationsStep/stagesStep ничего не подали --
// проходам нечего отмечать, и шаг обязан молча выйти, а не упасть на пустой партии.
func TestFakePassages_NoOpWhenNoApplications(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile := fakedata.Profile{
		Name: "passages-no-apps", Organizations: 3, Companies: 3, Users: 10,
		Employees: 10, Cars: 10, Applications: 0, Blacklists: 0, DaysBack: 30,
	}

	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-passages-empty"), 8181, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 8181}))

	require.Empty(t, readBatchCarPassages(t, db, batch.ID()))
	require.Empty(t, readBatchEmployeePassages(t, db, batch.ID()))
}

// Проход обязан укладываться в окно допуска вложения: пропуск выдают на недели, а
// заявки на стенде разложены на месяцы назад, поэтому у большинства принятых заявок
// окно давно закрыто. Въезд по просроченному пропуску охрана бы не пропустила, и на
// стенде такого быть не должно.
func TestFakePassages_StayWithinPermitWindow(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-window"), 2211, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 2211}))

	var outsideWindow int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM cars c
		JOIN attachments t ON t.id = c.attachment_id
		JOIN applications a ON a.id = t.application_id
		JOIN fake_batch_items i ON i.entity = 'application' AND i.entity_id = a.id AND i.batch_id = ?
		WHERE c.territory_entry_time IS NOT NULL
		  AND c.territory_entry_time::date > t.entry_date_to::date`, batch.ID()).Scan(&outsideWindow).Error)
	require.Zero(t, outsideWindow, "машина въехала после окончания действия пропуска")

	// На территории могут оставаться только те, чей пропуск ещё действует: иначе на
	// посту висел бы человек с просроченным допуском.
	var staleInside int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM cars c
		JOIN attachments t ON t.id = c.attachment_id
		JOIN applications a ON a.id = t.application_id
		JOIN fake_batch_items i ON i.entity = 'application' AND i.entity_id = a.id AND i.batch_id = ?
		WHERE c.territory_status = 1 AND t.entry_date_to::date < CURRENT_DATE`, batch.ID()).Scan(&staleInside).Error)
	require.Zero(t, staleInside, "на территории осталась машина с истёкшим пропуском")
}

// Отчёт о наливке обязан показать отметки прохода числом. Раньше их не было ни в одной
// строке отчёта: предпоказ обещал отметки, а «Создано» о них молчало -- разница между
// обещанным и показанным объяснялась только чтением исходников.
func TestFakePassages_ReportedAsMarksNotRecords(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-passage-marks"), 9292, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 9292}))

	marks := batch.Marks()
	require.Positive(t, marks[models.AuditEntityCar], "отмеченные машины должны попасть в отчёт")
	require.Positive(t, marks[models.AuditEntityEmployee], "отмеченные сотрудники должны попасть в отчёт")

	// Отметка -- действие над записью заявки, а не новая запись: в перечень партии
	// (и, значит, в объём удаления) она попадать не должна.
	counts := batch.Counts()
	require.Zero(t, counts[models.AuditEntityCar])
	require.Zero(t, counts[models.AuditEntityEmployee])

	require.Equal(t, len(readBatchCarPassages(t, db, batch.ID())), marks[models.AuditEntityCar],
		"в отчёте столько же отмеченных машин, сколько их реально в базе")
	require.Equal(t, len(readBatchEmployeePassages(t, db, batch.ID())), marks[models.AuditEntityEmployee],
		"в отчёте столько же отмеченных сотрудников, сколько их реально в базе")
}
