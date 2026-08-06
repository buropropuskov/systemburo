package handlers_test

// Проверка среза стадий обработки заявок (#1682, том 7): после прогона fakedata.Run в
// партии реально встречаются все шесть состояний Центра заявок (непрочитано, в
// обработке+согласована, в обработке+отклонена, в работе, возвращена из работы,
// отозвана), большинство -- принятые в работу, у решённых заявок проставлены голоса
// согласующих с реальным actor'ом, история переходов записана, все даты переходов
// исторически согласованы (позже подачи, не в будущем), повторный прогон не падает.
// testutil.SetupTestApp поднимает базу -- по правилу проекта такие тесты живут только в
// internal/handlers. Профиль "small" выбран нарочно маленьким: пакет handlers и так на
// грани CI-таймаута под -race.

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/fakedata"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// stageAppRow -- снимок полей applications, которые стадии обязаны проставить/сдвинуть.
type stageAppRow struct {
	ID                   int
	Status               *string
	Confirmation         *string
	AcceptedAt           *time.Time
	ConfirmationDatetime *time.Time
	ReadingDatetime      *time.Time
	WithdrawnAt          *time.Time
	StatusUpdatedAt      *time.Time
	SendingDatetime      time.Time
}

func readStageAppRows(t *testing.T, db *gorm.DB, ids []int) []stageAppRow {
	t.Helper()
	var rows []stageAppRow
	require.NoError(t, db.Raw(`
		SELECT id, status, confirmation, accepted_at, confirmation_datetime, reading_datetime,
			withdrawn_at, status_updated_at, sending_datetime
		FROM applications WHERE id IN ?`, ids).Scan(&rows).Error)
	return rows
}

func TestFakeStages_RunCoversAllApplicationStates(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile, err := fakedata.ProfileByName("small")
	require.NoError(t, err)
	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-stages"), 4242, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 4242}))

	// Стадии не заводят новых сущностей -- партия не приросла ничем сверх того, что уже
	// посчитал applicationsStep (заявки + шаблоны вложений).
	require.Equal(t, profile.Applications, batch.Counts()[models.AuditEntityApplication])

	var appIDs []int
	require.NoError(t, db.Table("fake_batch_items").
		Where("batch_id = ? AND entity = ?", batch.ID(), models.AuditEntityApplication).
		Pluck("entity_id", &appIDs).Error)
	require.Len(t, appIDs, profile.Applications)

	rows := readStageAppRows(t, db, appIDs)
	require.Len(t, rows, profile.Applications)

	now := time.Now().UTC()

	var unread, inWork, approvedOnly, rejected, revoked, withdrawn int
	for _, r := range rows {
		status := ""
		if r.Status != nil {
			status = *r.Status
		}
		confirmation := ""
		if r.Confirmation != nil {
			confirmation = *r.Confirmation
		}

		switch {
		case status == models.StatusWithdrawn:
			withdrawn++
			require.NotNil(t, r.WithdrawnAt, "у отозванной заявки %d должен быть withdrawn_at", r.ID)
		case status == models.StatusInWork:
			inWork++
			require.NotNil(t, r.AcceptedAt, "у принятой в работу заявки %d должен быть accepted_at", r.ID)
			require.Equal(t, models.ConfirmationApproved, confirmation, "принятая в работу заявка %d обязана быть согласована", r.ID)
		case status == models.StatusProcessing && r.AcceptedAt != nil:
			// Принята в работу и возвращена обратно (RevokeApplicationFromWork) -- отличается
			// от "просто согласована" непустым accepted_at (COALESCE его не стирает).
			revoked++
			require.Equal(t, models.ConfirmationApproved, confirmation, "возвращённая из работы заявка %d обязана остаться согласованной", r.ID)
		case status == models.StatusProcessing && confirmation == models.ConfirmationApproved:
			approvedOnly++
		case status == models.StatusProcessing && confirmation == models.ConfirmationRejected:
			rejected++
		case status == models.StatusUnread:
			unread++
		default:
			t.Fatalf("заявка %d в неожиданном сочетании статус=%q согласование=%q", r.ID, status, confirmation)
		}

		for _, moment := range []*time.Time{r.AcceptedAt, r.ConfirmationDatetime, r.ReadingDatetime, r.WithdrawnAt, r.StatusUpdatedAt} {
			if moment == nil {
				continue
			}
			require.False(t, moment.Before(r.SendingDatetime),
				"заявка %d: момент стадии %v раньше подачи %v", r.ID, moment, r.SendingDatetime)
			require.False(t, moment.After(now),
				"заявка %d: момент стадии %v в будущем относительно %v", r.ID, moment, now)
		}
	}

	require.Positive(t, unread, "должна остаться хотя бы одна непрочитанная заявка")
	require.Positive(t, approvedOnly, "должна быть хотя бы одна согласованная, но не принятая в работу заявка")
	require.Positive(t, rejected, "должна быть хотя бы одна отклонённая заявка")
	require.Positive(t, revoked, "должна быть хотя бы одна заявка, возвращённая из работы")
	require.Positive(t, withdrawn, "должна быть хотя бы одна отозванная заявка")
	require.Positive(t, inWork, "большинство заявок партии должно быть принято в работу")
	require.Greater(t, inWork, unread+approvedOnly+rejected+revoked+withdrawn,
		"принятые в работу обязаны оставаться большинством партии")

	// --- у решённых заявок (не "непрочитано") есть хотя бы один реальный голос
	// согласующего, датированный между подачей и "сейчас" ---

	decided := profile.Applications - unread
	require.Positive(t, decided)

	type voteRow struct {
		ApplicationID    int
		ApprovalStatus   *string
		ApprovalDatetime *time.Time
		SendingDatetime  time.Time
	}
	var votes []voteRow
	require.NoError(t, db.Raw(`
		SELECT aru.application_id, aru.approval_status, aru.approval_datetime, a.sending_datetime
		FROM application_responsible_users aru
		JOIN applications a ON a.id = aru.application_id
		JOIN fake_batch_items fbi ON fbi.entity_id = a.id
		WHERE fbi.batch_id = ? AND fbi.entity = ? AND aru.approval_status IN ('approved', 'rejected')`,
		batch.ID(), models.AuditEntityApplication).Scan(&votes).Error)
	require.NotEmpty(t, votes, "у решённых заявок должны быть реальные голоса согласующих")

	votedApps := make(map[int]bool, len(votes))
	for _, v := range votes {
		votedApps[v.ApplicationID] = true
		require.NotNil(t, v.ApprovalDatetime, "заявка %d: голос %q без approval_datetime", v.ApplicationID, *v.ApprovalStatus)
		require.False(t, v.ApprovalDatetime.Before(v.SendingDatetime),
			"заявка %d: голос согласующего раньше подачи заявки", v.ApplicationID)
		require.False(t, v.ApprovalDatetime.After(now),
			"заявка %d: голос согласующего в будущем", v.ApplicationID)
	}
	require.GreaterOrEqual(t, len(votedApps), approvedOnly+rejected+inWork+revoked,
		"у каждой согласованной/отклонённой/принятой/возвращённой заявки должен быть хотя бы один голос")

	// --- история переходов записана: у решённых заявок есть записи audit_log помимо
	// самого создания ---

	type historyCount struct {
		ApplicationID int
		Actions       int64
	}
	var history []historyCount
	require.NoError(t, db.Raw(`
		SELECT entity_id AS application_id, COUNT(*) AS actions
		FROM audit_log
		WHERE entity_type = ? AND entity_id IN ? AND action NOT IN ('create', 'assigned_responsible')
		GROUP BY entity_id`, models.AuditEntityApplication, appIDs).Scan(&history).Error)
	require.GreaterOrEqual(t, len(history), decided,
		"у каждой заявки, кроме непрочитанных, должна быть запись истории о переходе стадии")

	// --- history "принят в работу" реально сдвинута: истории машин/сотрудников
	// (added_to_table), рождённые принятием в работу, не остаются с датой прогона ---

	type addedRow struct {
		CreatedAt       time.Time
		SendingDatetime time.Time
	}
	var addedCars []addedRow
	require.NoError(t, db.Raw(`
		SELECT l.created_at, a.sending_datetime
		FROM audit_log l
		JOIN cars c ON c.id = l.entity_id
		JOIN attachments att ON att.id = c.attachment_id
		JOIN applications a ON a.id = att.application_id
		JOIN fake_batch_items fbi ON fbi.entity_id = a.id
		WHERE fbi.batch_id = ? AND fbi.entity = ? AND l.entity_type = ? AND l.action = ?`,
		batch.ID(), models.AuditEntityApplication, models.AuditEntityCar, models.AuditActionAddedToTable).
		Scan(&addedCars).Error)
	for _, r := range addedCars {
		require.False(t, r.CreatedAt.Before(r.SendingDatetime), "добавление машины в таблицу проходной раньше подачи заявки")
		require.False(t, r.CreatedAt.After(now), "добавление машины в таблицу проходной в будущем")
	}

	// --- повторный прогон не падает и разыгрывает стадии на СВОЕЙ, независимой партии ---

	batch2, err := fakedata.OpenBatch(ctx, db, uniq("fake-stages-2"), 5252, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch2, Profile: profile, Seed: 5252}))
	require.Equal(t, profile.Applications, batch2.Counts()[models.AuditEntityApplication])
}

// Профиль без заявок (Applications: 0) означает, что applicationsStep ничего не подал --
// стадиям нечего прогонять, и шаг обязан молча выйти, а не упасть на пустой партии.
func TestFakeStages_NoOpWhenNoApplications(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	ctx := context.Background()
	testutil.CleanDB(t, db)
	seedFakeAdmin(t, db)

	profile := fakedata.Profile{
		Name: "stages-no-apps", Organizations: 3, Companies: 3, Users: 10,
		Employees: 10, Cars: 10, Applications: 0, Blacklists: 0, DaysBack: 30,
	}

	batch, err := fakedata.OpenBatch(ctx, db, uniq("fake-stages-empty"), 7777, profile.Name)
	require.NoError(t, err)
	require.NoError(t, fakedata.Run(ctx, &fakedata.Env{DB: db, Batch: batch, Profile: profile, Seed: 7777}))
	require.Zero(t, batch.Counts()[models.AuditEntityApplication])
}
