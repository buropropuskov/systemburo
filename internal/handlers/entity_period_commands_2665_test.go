package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type period2665World struct {
	db                                               *gorm.DB
	commands                                         *services.EntityPeriodCommandService
	kind                                             services.ElementKind
	actor, target, neighbor, attachment, application int
	clock                                            time.Time
}

func setupPeriod2665World(t *testing.T, kind services.ElementKind) period2665World {
	t.Helper()
	_, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	seed := testutil.SeedTestData(t, db)
	now := time.Now().In(services.MoscowLocation())
	actor := models.User{Username: "period2665_actor", Password: "x", TypeID: 1, OrganizationID: &seed.OrgID, IsActive: true}
	voter := models.User{Username: "period2665_voter", Password: "x", TypeID: 1, OrganizationID: &seed.OrgID, IsActive: true}
	require.NoError(t, db.Create(&actor).Error)
	require.NoError(t, db.Create(&voter).Error)
	require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: actor.ID, PermissionKey: services.KeyDetailPeriodChange, Value: "allow", GrantedAt: now.UTC()}).Error)
	status, confirmation := models.StatusInWork, models.ConfirmationApproved
	app := models.Application{OrganizationID: seed.OrgID, SenderUserID: actor.ID, Status: &status, Confirmation: &confirmation, SendingDatetime: &now}
	require.NoError(t, db.Create(&app).Error)
	from, to := now.AddDate(0, 0, -1).Format("2006-01-02"), now.AddDate(0, 0, 2).Format("2006-01-02")
	timeFrom, timeTo, active := "08:00:00", "22:00:00", 1
	attachmentType := "people"
	if kind == services.ElementCar {
		attachmentType = "cars"
	}
	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: attachmentType, Status: &active, EntryDateFrom: &from, EntryDateTo: &to, EntryTimeFrom: &timeFrom, EntryTimeTo: &timeTo}
	require.NoError(t, db.Create(&att).Error)
	w := period2665World{db: db, commands: services.NewEntityPeriodCommandService(db, services.NewAuditRecorder(db)), kind: kind, actor: actor.ID, attachment: att.ID, application: app.ID, clock: now}
	if kind == services.ElementCar {
		for i := 0; i < 2; i++ {
			plate := fmt.Sprintf("TEST2665-%d", i)
			car := models.Car{AttachmentID: att.ID, CarNumber: &plate, Status: &active, EntryDateFrom: &from, EntryDateTo: &to, EntryTimeFrom: &timeFrom, EntryTimeTo: &timeTo}
			require.NoError(t, db.Create(&car).Error)
			if i == 0 {
				w.target = car.ID
			} else {
				w.neighbor = car.ID
			}
		}
	} else {
		for i := 0; i < 2; i++ {
			employee := models.Employee{AttachmentID: &att.ID, Status: &active}
			require.NoError(t, db.Create(&employee).Error)
			if i == 0 {
				w.target = employee.ID
			} else {
				w.neighbor = employee.ID
			}
		}
	}
	approved, pending := "approved", "pending"
	for i, vote := range []*string{&approved, &pending} {
		user := actor.ID
		if i == 1 {
			user = voter.ID
		}
		require.NoError(t, db.Create(&models.ApplicationResponsibleUser{ApplicationID: app.ID, UserID: user, RequiredApproval: true, ApprovalStatus: vote, ApprovalDatetime: &now}).Error)
	}
	return w
}

func (w period2665World) request(revision string, extraDays int) services.ChangeEntityPeriodRequest {
	return services.ChangeEntityPeriodRequest{PeriodMode: models.PeriodIndividual, Period: &services.EntityPeriodInput{
		EntryDateFrom: w.clock.AddDate(0, 0, -1).Format("2006-01-02"), EntryDateTo: w.clock.AddDate(0, 0, extraDays).Format("2006-01-02"), EntryTimeFrom: "08:00", EntryTimeTo: "22:00",
	}, Reason: "Продление тестовой записи", ExpectedRevision: revision}
}

func (w period2665World) rowSnapshot(t *testing.T, id int) string {
	t.Helper()
	var snapshot string
	require.NoError(t, w.db.Raw("SELECT to_jsonb(e)::text FROM "+string(w.kind)+" e WHERE e.id=?", id).Scan(&snapshot).Error)
	return snapshot
}

func (w period2665World) parentAndVotesSnapshot(t *testing.T) string {
	t.Helper()
	var snapshot string
	require.NoError(t, w.db.Raw(`SELECT jsonb_build_object(
		'attachment', (SELECT to_jsonb(a) FROM attachments a WHERE a.id=?),
		'application', (SELECT to_jsonb(app) FROM applications app WHERE app.id=?),
		'votes', (SELECT jsonb_agg(to_jsonb(v) ORDER BY v.id) FROM application_responsible_users v WHERE v.application_id=?)
	)::text`, w.attachment, w.application, w.application).Scan(&snapshot).Error)
	return snapshot
}

func period2665RequireHTTPError(t *testing.T, err error, status int) {
	t.Helper()
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, status, httpErr.Code)
}

func TestEntityPeriod2665DBRoundtripAndIsolation(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPeriod2665World(t, kind)
			ctx := context.Background()
			parentBefore, neighborBefore := w.parentAndVotesSnapshot(t), w.rowSnapshot(t, w.neighbor)
			preview, err := w.commands.Inspect(ctx, w.actor, kind, w.target, nil)
			require.NoError(t, err)
			first, err := w.commands.Change(ctx, w.actor, kind, w.target, w.request(preview.Revision, 5))
			require.NoError(t, err)
			require.Equal(t, models.PeriodIndividual, first.PeriodMode)
			require.False(t, first.ApprovalsReset)
			require.Equal(t, w.clock.AddDate(0, 0, 5).Format("2006-01-02"), *first.Effective.EntryDateTo)
			inspected, err := w.commands.Inspect(ctx, w.actor, kind, w.target, nil)
			require.NoError(t, err)
			require.Equal(t, first.Revision, inspected.Revision, "real PostgreSQL timestamp roundtrip must not change revision")
			var updated struct{ UpdatedAt time.Time }
			require.NoError(t, w.db.Table(string(kind)).Select("updated_at").Where("id=?", w.target).Scan(&updated).Error)
			require.Zero(t, updated.UpdatedAt.Nanosecond()%1000)
			second, err := w.commands.Change(ctx, w.actor, kind, w.target, w.request(first.Revision, 6))
			require.NoError(t, err, "returned revision must be usable by the next real command")
			require.NotEqual(t, first.Revision, second.Revision)
			require.Equal(t, neighborBefore, w.rowSnapshot(t, w.neighbor))
			require.Equal(t, parentBefore, w.parentAndVotesSnapshot(t), "parent, app and complete vote rows must remain unchanged")
			inherited, err := w.commands.Change(ctx, w.actor, kind, w.target, services.ChangeEntityPeriodRequest{PeriodMode: models.PeriodInherit, Reason: "Явный возврат к сроку вложения", ExpectedRevision: second.Revision})
			require.NoError(t, err)
			require.Nil(t, inherited.Individual)
			require.Equal(t, models.PeriodInherit, inherited.PeriodMode)
			require.Equal(t, w.clock.AddDate(0, 0, 2).Format("2006-01-02"), *inherited.Effective.EntryDateTo)
			var ownFields int64
			require.NoError(t, w.db.Table(string(kind)).Where("id=? AND (entry_date_from IS NOT NULL OR entry_date_to IS NOT NULL OR entry_time_from IS NOT NULL OR entry_time_to IS NOT NULL)", w.target).Count(&ownFields).Error)
			require.Zero(t, ownFields)
			require.Equal(t, parentBefore, w.parentAndVotesSnapshot(t))
		})
	}
}

func TestEntityPeriod2665DBRealPermissionAndAccountGates(t *testing.T) {
	for _, scenario := range []string{"personal_deny_admin", "banned_super", "archived_super"} {
		t.Run(scenario, func(t *testing.T) {
			w := setupPeriod2665World(t, services.ElementEmployee)
			ctx := context.Background()
			preview, err := w.commands.Inspect(ctx, w.actor, w.kind, w.target, nil)
			require.NoError(t, err)
			before := w.rowSnapshot(t, w.target)
			switch scenario {
			case "personal_deny_admin":
				require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Update("is_admin", true).Error)
				require.NoError(t, w.db.Model(&models.UserPermissionOverride{}).Where("user_id=? AND permission_key=?", w.actor, services.KeyDetailPeriodChange).Update("value", "deny").Error)
			case "banned_super":
				require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Updates(map[string]any{"is_super_admin": true, "is_banned": true}).Error)
			case "archived_super":
				require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Updates(map[string]any{"is_super_admin": true, "is_active": false}).Error)
			}
			_, err = w.commands.Change(ctx, w.actor, w.kind, w.target, w.request(preview.Revision, 5))
			period2665RequireHTTPError(t, err, http.StatusForbidden)
			require.Equal(t, before, w.rowSnapshot(t, w.target))
			_, err = w.commands.Inspect(ctx, w.actor, w.kind, w.target, nil)
			period2665RequireHTTPError(t, err, http.StatusForbidden)
		})
	}
}

func TestEntityPeriod2665DBStaleParentRevision(t *testing.T) {
	w := setupPeriod2665World(t, services.ElementCar)
	ctx := context.Background()
	preview, err := w.commands.Inspect(ctx, w.actor, w.kind, w.target, nil)
	require.NoError(t, err)
	before := w.rowSnapshot(t, w.target)
	// A change of values, even without a parent timestamp update, invalidates
	// the old edit form. The stale request may not partially write its entity.
	require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).UpdateColumn("entry_time_to", "23:00:00").Error)
	_, err = w.commands.Change(ctx, w.actor, w.kind, w.target, w.request(preview.Revision, 5))
	period2665RequireHTTPError(t, err, http.StatusConflict)
	require.Equal(t, before, w.rowSnapshot(t, w.target))
	latest, err := w.commands.Inspect(ctx, w.actor, w.kind, w.target, nil)
	require.NoError(t, err)
	require.NotEqual(t, preview.Revision, latest.Revision)
}

func TestEntityPeriod2665DBAuditFailureRollsBackUpdate(t *testing.T) {
	w := setupPeriod2665World(t, services.ElementEmployee)
	ctx := context.Background()
	preview, err := w.commands.Inspect(ctx, w.actor, w.kind, w.target, nil)
	require.NoError(t, err)
	before, parentBefore := w.rowSnapshot(t, w.target), w.parentAndVotesSnapshot(t)
	request := w.request(preview.Revision, 5)
	failure := errors.New("synthetic period audit storage failure")
	const callback = "period2665_fail_target_audit"
	seenWrittenPeriod := false
	// Keep the real recorder and SQL transaction. Inject a narrow storage error
	// only at this synthetic audit create, after the actual entity UPDATE.
	require.NoError(t, w.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		entry, ok := tx.Statement.Dest.(*models.AuditLog)
		if !ok || entry.EntityType != models.AuditEntityEmployee || entry.EntityID == nil || *entry.EntityID != w.target || entry.ActorUserID == nil || *entry.ActorUserID != w.actor || entry.Action != models.AuditActionDatesChanged {
			return
		}
		var written string
		readErr := tx.Session(&gorm.Session{NewDB: true}).Table("employees").Select("entry_date_to").Where("id=?", w.target).Scan(&written).Error
		if readErr != nil {
			tx.AddError(readErr)
			return
		}
		seenWrittenPeriod = written == request.Period.EntryDateTo
		tx.AddError(failure)
	}))
	t.Cleanup(func() { require.NoError(t, w.db.Callback().Create().Remove(callback)) })
	_, err = w.commands.Change(ctx, w.actor, w.kind, w.target, request)
	require.ErrorIs(t, err, failure)
	require.True(t, seenWrittenPeriod, "failure injection must occur after the real entity UPDATE")
	require.Equal(t, before, w.rowSnapshot(t, w.target), "audit failure must atomically roll back mode, dates and timestamp")
	require.Equal(t, parentBefore, w.parentAndVotesSnapshot(t))
	var audits int64
	require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id=? AND action=?", models.AuditEntityEmployee, w.target, models.AuditActionDatesChanged).Count(&audits).Error)
	require.Zero(t, audits)
}
