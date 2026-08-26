package handlers_test

// Проверка среза наливки заявок с вложениями (#1682, том 6): после прогона fakedata.Run
// заявки реально созданы через applicationService.SubmitCompleteApplication (видны через
// сервис списка отправителю, имеют согласующих и историю), вложения всех трёх типов несут
// реальных людей и машины из реестров, номер заявки после сдвига дат соответствует новой
// дате, чужая (созданная до наливки) заявка не тронута, всё зарегистрировано в партии,
// повторный прогон не падает. testutil.SetupTestApp поднимает базу -- по правилу проекта
// такие тесты живут только в internal/handlers. Профиль "small" выбран нарочно маленьким:
// пакет handlers и так на грани CI-таймаута под -race.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"systemburo/internal/fakedata"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFakeApplications_RunCreatesApplicationsWithAttachments(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	admin := seedFakeAdmin(t, db)

	recorder := services.NewAuditRecorder(db)
	appSvc := services.NewApplicationService(
		db,
		services.NewPermissionService(db),
		services.NewNotificationService(db),
		services.NewVehicleBlacklistService(db, recorder),
		services.NewPersonBlacklistService(db, recorder),
		recorder,
	)

	// --- чужая заявка, поданная ДО наливки -- сдвиг дат обязан её не тронуть ---

	orgSvc := services.NewOrganizationService(db)
	orgType := models.OrgTypeValues[0]
	foreignOrg, err := orgSvc.Create(ctx, admin.ID, services.CreateOrganizationRequest{
		Name: uniq("Чужая орг"), Type: &orgType,
	})
	require.NoError(t, err)

	userSvc := services.NewUserService(db, services.NewNotificationService(db))
	foreignUsername := uniq("fake_app_foreign")
	require.NoError(t, userSvc.Create(ctx, admin.ID, models.RegisterRequest{
		Username: foreignUsername, Password: fakedata.DefaultUserPassword,
		OrganizationID: foreignOrg.ID, TypeID: 1,
	}))

	foreignOrgID := foreignOrg.ID
	foreignResp, err := appSvc.CreateApplication(ctx, foreignUsername, services.ApplicationCreateRequest{
		OrganizationID: &foreignOrgID, DataApproval: true,
	}, false)
	require.NoError(t, err)

	foreignBefore := readApplicationSnapshot(t, db, foreignResp.ApplicationID)

	// --- наливка ---

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-apps"), 8181, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 8181}))

	// --- всё созданное зарегистрировано в партии ---

	require.Equal(t, profile.Applications, batch.Counts()[models.AuditEntityApplication])

	var appIDs []int
	require.NoError(t, db.Table("fake_batch_items").
		Where("batch_id = ? AND entity = ?", batch.ID(), models.AuditEntityApplication).
		Pluck("entity_id", &appIDs).Error)
	require.Len(t, appIDs, profile.Applications)

	// --- заявка реально видна через сервис списка её отправителю ---

	var sample struct {
		ID       int
		Username string
	}
	require.NoError(t, db.Raw(`
		SELECT a.id, u.username FROM applications a
		JOIN users u ON u.id = a.sender_user_id
		WHERE a.id = ?`, appIDs[0]).Row().Scan(&sample.ID, &sample.Username))

	userApps, err := appSvc.GetUserApplications(ctx, sample.Username, services.ApplicationFilter{})
	require.NoError(t, err)
	found := false
	for _, a := range userApps {
		if a.ID == sample.ID {
			found = true
			break
		}
	}
	require.True(t, found, "заявка %d должна быть видна отправителю %q через сервис списка", sample.ID, sample.Username)

	// --- у каждой заявки есть хотя бы один согласующий и хотя бы одна запись истории ---

	for _, id := range appIDs {
		var responsibleCount int64
		require.NoError(t, db.Table("application_responsible_users").Where("application_id = ?", id).Count(&responsibleCount).Error)
		require.Positive(t, responsibleCount, "у заявки %d должен быть хотя бы один согласующий", id)

		var historyCount int64
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ?", models.AuditEntityApplication, id).
			Count(&historyCount).Error)
		require.Positive(t, historyCount, "у заявки %d должна быть хотя бы одна запись истории", id)
	}

	// --- вложения всех трёх типов реально созданы, с людьми и машинами ---

	var attachmentTypes []string
	require.NoError(t, db.Raw(`SELECT DISTINCT attachment_type FROM attachments WHERE application_id IN ?`, appIDs).
		Scan(&attachmentTypes).Error)
	require.ElementsMatch(t, []string{"cars", "people", "items"}, attachmentTypes,
		"среди заявок партии должны встретиться вложения всех трёх типов")

	var carsCount, employeesCount, itemsCount int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM cars c JOIN attachments a ON a.id = c.attachment_id
		WHERE a.application_id IN ?`, appIDs).Scan(&carsCount).Error)
	require.Positive(t, carsCount, "во вложениях cars должны быть реальные машины")

	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM employees e JOIN attachments a ON a.id = e.attachment_id
		WHERE a.application_id IN ?`, appIDs).Scan(&employeesCount).Error)
	require.Positive(t, employeesCount, "во вложениях people должны быть реальные сотрудники")

	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM items i JOIN attachments a ON a.id = i.attachment_id
		WHERE a.application_id IN ?`, appIDs).Scan(&itemsCount).Error)
	require.Positive(t, itemsCount, "во вложениях items должны быть реальные ТМЦ")

	// --- часть заявок с одним вложением, часть -- с несколькими ---

	var attachmentCounts []int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM attachments WHERE application_id IN ? GROUP BY application_id`, appIDs).
		Scan(&attachmentCounts).Error)
	require.Len(t, attachmentCounts, profile.Applications)
	hasSingle, hasMultiple := false, false
	for _, c := range attachmentCounts {
		if c == 1 {
			hasSingle = true
		}
		if c > 1 {
			hasMultiple = true
		}
	}
	require.True(t, hasSingle, "должны быть заявки ровно с одним вложением")
	require.True(t, hasMultiple, "должны быть заявки с несколькими вложениями")

	// --- номер заявки после сдвига соответствует новой дате отправки ---

	rows := readApplicationSnapshots(t, db, appIDs)
	require.Len(t, rows, profile.Applications)
	for _, r := range rows {
		wantPrefix := fmt.Sprintf("№ %s/", r.Sending.UTC().Format("20060102"))
		require.True(t, strings.HasPrefix(r.Number, wantPrefix),
			"номер заявки %d (%q) должен начинаться с даты отправки после сдвига (%q)", r.ID, r.Number, wantPrefix)
	}

	// --- чужая заявка не сдвинута ---

	foreignAfter := readApplicationSnapshot(t, db, foreignResp.ApplicationID)
	require.Equal(t, foreignBefore.Number, foreignAfter.Number, "чужая заявка не должна поменять номер")
	require.True(t, foreignBefore.Sending.Equal(foreignAfter.Sending), "чужая заявка не должна поменять дату отправки")

	// --- повторный прогон не падает ---

	batch2, err := fakedata.OpenBatch(ctx, db, uniq("fake-apps-2"), 9292, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch2, Profile: profile, Seed: 9292}))
	require.Equal(t, profile.Applications, batch2.Counts()[models.AuditEntityApplication])
}

// Заявки собираются из реестров сотрудников и машин -- пустые реестры (профиль без
// Employees/Cars) означают, что вложения строить не из чего, и шаг обязан честно упасть, а
// не подать заявки без людей и машин молча.
func TestFakeApplications_FailsWhenRegistriesEmpty(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile := fakedata.Profile{
		Name: "apps-no-registries", Organizations: 3, Companies: 3, Users: 10,
		Employees: 0, Cars: 0, Applications: 5, Blacklists: 0, DaysBack: 30,
	}

	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-apps-empty-reg"), 1111, profile.Name)
	require.NoError(t, err)

	err = fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 1111})
	require.Error(t, err, "заявки без реестра машин обязаны сообщить об отказе, а не подать заявки без вложений")
	require.Contains(t, err.Error(), "реестр машин пуст")
}

// Заявки подаются от пользователей, налитых usersStep -- профиль без пользователей
// (Users: 0) означает, что подавать не от чьего имени, и шаг обязан честно упасть.
func TestFakeApplications_FailsWhenNoApplicants(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile := fakedata.Profile{
		Name: "apps-no-users", Organizations: 3, Companies: 3, Users: 0,
		Employees: 5, Cars: 5, Applications: 5, Blacklists: 0, DaysBack: 30,
	}

	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-apps-empty-users"), 2222, profile.Name)
	require.NoError(t, err)

	err = fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 2222})
	require.Error(t, err, "заявки без активных пользователей партии обязаны сообщить об отказе")
	require.Contains(t, err.Error(), "не налила ни одного активного пользователя")
}

// applicationSnapshot -- номер и дата отправки заявки, снятые до и после сдвига дат, чтобы
// сравнить их между собой.
type applicationSnapshot struct {
	ID      int
	Number  string
	Sending time.Time
}

func readApplicationSnapshot(t *testing.T, db *gorm.DB, id int) applicationSnapshot {
	t.Helper()
	var snap applicationSnapshot
	require.NoError(t, db.Raw(`
		SELECT id, application_number as number, sending_datetime as sending
		FROM applications WHERE id = ?`, id).Scan(&snap).Error)
	return snap
}

func readApplicationSnapshots(t *testing.T, db *gorm.DB, ids []int) []applicationSnapshot {
	t.Helper()
	var snaps []applicationSnapshot
	require.NoError(t, db.Raw(`
		SELECT id, application_number as number, sending_datetime as sending
		FROM applications WHERE id IN ?`, ids).Scan(&snaps).Error)
	return snaps
}

// Заявка, перенесённая в прошлое, обязана быть непротиворечивой целиком: вложения,
// машины, сотрудники, состав согласующих и записи истории не могут остаться с датой
// прогона наливки, иначе июльская заявка показывает историю «создана сегодня».
//
// Строка истории проверяется через "раньше даты подачи", а не "отличается от неё": с
// #1682 тома 7 (стадии обработки, internal/fakedata/stages.go) у заявки легитимно
// появляются записи ПОЗЖЕ дня подачи (принята в работу/согласована/отклонена и т.п.) --
// это не регрессия, а сама суть стадий. Инвариант этого теста остаётся прежним: запись
// не должна остаться датированной днём ПРОГОНА наливки, когда заявка сдвинута в прошлое.
//
// "Отметка изменения" машин/сотрудников с #1682 тома 8 (проходы через посты,
// internal/fakedata/passages.go) законно уходит ПОЗЖЕ последнего перехода самой
// заявки: принятая в работу машина/сотрудник может ещё и въехать/выехать, а это
// отдельная, более поздняя историческая отметка со своим действием (entry/exit) в
// audit_log конкретной машины/сотрудника, а не заявки. Верхняя граница поэтому берёт
// GREATEST из перехода заявки И собственной отметки прохода сущности -- иначе тест
// проверял бы инвариант, переставший быть правдой с появлением проходов.
func TestFakeApplications_ChildRecordsShiftWithApplication(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-shift"), 606, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 606}))

	type row struct {
		Label string
		Diff  int64
	}
	var rows []row
	require.NoError(t, db.Raw(`
		WITH mine AS (
			SELECT a.id, a.sending_datetime
			FROM applications a
			JOIN fake_batch_items i ON i.entity = 'application' AND i.entity_id = a.id AND i.batch_id = ?
		)
		SELECT 'вложения' AS label, COUNT(*) AS diff FROM attachments t JOIN mine ON mine.id = t.application_id
			WHERE DATE(t.created_at) <> DATE(mine.sending_datetime)
		UNION ALL
		SELECT 'машины', COUNT(*) FROM cars c JOIN attachments t ON t.id = c.attachment_id JOIN mine ON mine.id = t.application_id
			WHERE DATE(c.created_at) <> DATE(mine.sending_datetime)
		UNION ALL
		-- Отметку изменения переписывает принятие в работу и снятие с неё, поэтому она
		-- сверяется с моментом последнего перехода этой заявки, а не с днём подачи.
		-- Сравнение с «не позже сейчас» тут бесполезно: время прогона ему удовлетворяет,
		-- и непереносенная отметка прошла бы незамеченной.
		SELECT 'отметка изменения машин', COUNT(*) FROM cars c
			JOIN attachments t ON t.id = c.attachment_id
			JOIN mine ON mine.id = t.application_id
			WHERE c.updated_at > GREATEST(
				(SELECT MAX(l.created_at) FROM audit_log l
					WHERE l.entity_type = 'application' AND l.entity_id = mine.id),
				COALESCE((SELECT MAX(l2.created_at) FROM audit_log l2
					WHERE l2.entity_type = 'car' AND l2.entity_id = c.id AND l2.action IN ('entry', 'exit')),
					'-infinity'::timestamptz)
			) + INTERVAL '1 second'
		UNION ALL
		SELECT 'отметка изменения сотрудников', COUNT(*) FROM employees e
			JOIN attachments t ON t.id = e.attachment_id
			JOIN mine ON mine.id = t.application_id
			WHERE e.updated_at > GREATEST(
				(SELECT MAX(l.created_at) FROM audit_log l
					WHERE l.entity_type = 'application' AND l.entity_id = mine.id),
				COALESCE((SELECT MAX(l2.created_at) FROM audit_log l2
					WHERE l2.entity_type = 'employee' AND l2.entity_id = e.id AND l2.action IN ('entry', 'exit')),
					'-infinity'::timestamptz)
			) + INTERVAL '1 second'
		UNION ALL
		SELECT 'сотрудники', COUNT(*) FROM employees e JOIN attachments t ON t.id = e.attachment_id JOIN mine ON mine.id = t.application_id
			WHERE DATE(e.created_at) <> DATE(mine.sending_datetime)
		UNION ALL
		SELECT 'согласующие', COUNT(*) FROM application_responsible_users r JOIN mine ON mine.id = r.application_id
			WHERE DATE(r.created_at) <> DATE(mine.sending_datetime)
		UNION ALL
		-- Запись о СОЗДАНИИ заявки обязана совпадать с датой подачи день в день: именно
		-- она показывает, что история не осталась с датой прогона наливки. Переходы по
		-- стадиям датируются позже, поэтому для них проверяется только, что они не
		-- раньше подачи и не в будущем.
		SELECT 'история создания', COUNT(*) FROM audit_log l JOIN mine ON mine.id = l.entity_id
			WHERE l.entity_type = 'application' AND l.action = 'create'
			  AND DATE(l.created_at) <> DATE(mine.sending_datetime)
		UNION ALL
		SELECT 'история переходов', COUNT(*) FROM audit_log l JOIN mine ON mine.id = l.entity_id
			WHERE l.entity_type = 'application' AND l.action <> 'create'
			  AND (l.created_at < mine.sending_datetime OR l.created_at > NOW())
	`, batch.ID()).Scan(&rows).Error)

	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Zero(t, r.Diff, "%s: %d записей остались с датой прогона вместо даты заявки", r.Label, r.Diff)
	}
}

// Повторная наливка не должна выдать заявке номер, уже занятый прошлой партией: номер
// пересчитывается при переносе даты, и если бы он считался только по своей партии,
// вторая партия начала бы нумерацию тех же дней заново.
func TestFakeApplications_RepeatedRunKeepsNumbersUnique(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)

	first, err := fakedata.OpenBatch(ctx, db, uniq("fake-num-1"), 11, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: first, Profile: profile, Seed: 11}))

	second, err := fakedata.OpenBatch(ctx, db, uniq("fake-num-2"), 22, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: second, Profile: profile, Seed: 22}))

	var duplicates int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT application_number FROM applications
			WHERE application_number IS NOT NULL
			GROUP BY application_number HAVING COUNT(*) > 1
		) d`).Scan(&duplicates).Error)
	require.Zero(t, duplicates, "номера заявок обязаны остаться уникальными после повторной наливки")
}
