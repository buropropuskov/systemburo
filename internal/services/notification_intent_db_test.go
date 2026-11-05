package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/realtime"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type intentDeliveryProbe struct {
	mu                       sync.Mutex
	realtimeCalls, pushCalls int
	events                   []realtime.Event
	payloads                 []services.PushPayload
}

func (p *intentDeliveryProbe) Publish(_ int, event realtime.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.realtimeCalls++
	p.events = append(p.events, event)
}
func (p *intentDeliveryProbe) PublishMany(ids []int, event realtime.Event) {
	for _, id := range ids {
		p.Publish(id, event)
	}
}
func (p *intentDeliveryProbe) Send(_ context.Context, _ int, payload services.PushPayload) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pushCalls++
	p.payloads = append(p.payloads, payload)
}
func (p *intentDeliveryProbe) counts() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.realtimeCalls, p.pushCalls
}

type intentFixture struct {
	db               *gorm.DB
	author, approver models.User
	orgID            int
	app              services.ApplicationService
	notifications    services.NotificationService
	worker           *services.NotificationIntentWorker
	probe            *intentDeliveryProbe
}

func setupIntentFixture(t *testing.T) intentFixture {
	t.Helper()
	_, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	author := models.User{Username: "intent-author", Password: "unused", OrganizationID: &td.OrgID, CompanyID: &td.CompanyID, TypeID: 1, IsActive: true}
	approver := models.User{Username: "intent-approver", Password: "unused", OrganizationID: &td.OrgID, CompanyID: &td.CompanyID, TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&author).Error)
	require.NoError(t, db.Create(&approver).Error)
	probe := &intentDeliveryProbe{}
	notifications := services.NewNotificationService(db, services.WithNotificationRealtimePublisher(probe), services.WithNotificationPushSender(probe))
	app := services.NewApplicationService(db, nil, notifications, nil, nil, nil)
	worker, err := services.NewNotificationIntentWorker(db, app, notifications)
	require.NoError(t, err)
	return intentFixture{db: db, author: author, approver: approver, orgID: td.OrgID, app: app, notifications: notifications, worker: worker, probe: probe}
}

func (f intentFixture) createApplication(t *testing.T, tx *gorm.DB) models.Application {
	t.Helper()
	number := "INTENT-1"
	app := models.Application{OrganizationID: f.orgID, SenderUserID: f.author.ID, ApplicationNumber: &number}
	require.NoError(t, tx.Create(&app).Error)
	require.NoError(t, tx.Create(&models.ApplicationResponsibleUser{ApplicationID: app.ID, UserID: f.approver.ID, RequiredApproval: true}).Error)
	return app
}

func intentCount(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(model).Count(&count).Error)
	return count
}

func TestNotificationIntentBusinessRollbackAndRestartDedup(t *testing.T) {
	f := setupIntentFixture(t)
	ctx := context.Background()
	rollback := errors.New("synthetic business rollback")
	err := f.db.Transaction(func(tx *gorm.DB) error {
		app := f.createApplication(t, tx)
		require.NoError(t, services.EnqueueApplicationSubmittedNotificationTx(ctx, tx, app.ID, f.author.ID))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	require.Zero(t, intentCount(t, f.db, &models.NotificationIntent{}))
	require.Zero(t, intentCount(t, f.db, &models.Notification{}))
	var app models.Application
	require.NoError(t, f.db.Transaction(func(tx *gorm.DB) error {
		app = f.createApplication(t, tx)
		return services.EnqueueApplicationSubmittedNotificationTx(ctx, tx, app.ID, f.author.ID)
	}))
	var committed models.NotificationIntent
	require.NoError(t, f.db.Where("event_key = ?", fmt.Sprintf("submit:%d", app.ID)).Take(&committed).Error)
	require.Positive(t, committed.ID)
	require.Zero(t, intentCount(t, f.db, &models.Notification{}), "no in-app rows before materialization")
	r, p := f.probe.counts()
	require.Zero(t, r)
	require.Zero(t, p)
	// A new worker instance recovers the committed intent without an HTTP request.
	restarted, err := services.NewNotificationIntentWorker(f.db, f.app, f.notifications)
	require.NoError(t, err)
	processed, failed := restarted.Process(ctx)
	require.Equal(t, 1, processed)
	require.Zero(t, failed)
	require.EqualValues(t, 2, intentCount(t, f.db, &models.Notification{}))
	r, p = f.probe.counts()
	require.Equal(t, 2, r)
	require.Equal(t, 2, p)
	require.NoError(t, services.EnqueueApplicationSubmittedNotificationTx(ctx, f.db, app.ID, f.author.ID))
	var deduplicated models.NotificationIntent
	require.NoError(t, f.db.Where("event_key = ?", committed.EventKey).Take(&deduplicated).Error)
	require.Equal(t, committed.ID, deduplicated.ID, "unique event key must retain the original durable row")
	require.Equal(t, "done", deduplicated.State)
	require.EqualValues(t, 1, intentCount(t, f.db, &models.NotificationIntent{}))
	processed, failed = f.worker.Process(ctx)
	require.Zero(t, processed)
	require.Zero(t, failed)
	require.EqualValues(t, 2, intentCount(t, f.db, &models.Notification{}), "ACK key prevents re-enqueue duplication")
}

func TestNotificationIntentFanoutFailureRollsBackAndRetriesOnce(t *testing.T) {
	f := setupIntentFixture(t)
	ctx := context.Background()
	app := f.createApplication(t, f.db)
	require.NoError(t, services.EnqueueApplicationSubmittedNotificationTx(ctx, f.db, app.ID, f.author.ID))
	var reject atomic.Bool
	reject.Store(true)
	const callback = "fixture:notification-intent-failure"
	require.NoError(t, f.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if n, ok := tx.Statement.Dest.(*models.Notification); ok && n.UserID == f.approver.ID && reject.Load() {
			tx.AddError(errors.New("synthetic notification write failure"))
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.db.Callback().Create().Remove(callback)) })
	processed, failed := f.worker.Process(ctx)
	require.Zero(t, processed)
	require.Equal(t, 1, failed)
	require.Zero(t, intentCount(t, f.db, &models.Notification{}), "first recipient must roll back with failed fanout")
	r, p := f.probe.counts()
	require.Zero(t, r)
	require.Zero(t, p)
	var intent models.NotificationIntent
	require.NoError(t, f.db.Where("application_id = ?", app.ID).Take(&intent).Error)
	require.Equal(t, "pending", intent.State)
	require.Equal(t, 1, intent.Attempts)
	require.True(t, intent.NextAttemptAt.After(intent.OccurredAt))
	reject.Store(false)
	// Deterministically make it due relative to its own event clock; no sleeps.
	require.NoError(t, f.db.Model(&intent).Update("next_attempt_at", intent.OccurredAt.Add(-time.Second)).Error)
	processed, failed = f.worker.Process(ctx)
	require.Equal(t, 1, processed)
	require.Zero(t, failed)
	require.EqualValues(t, 2, intentCount(t, f.db, &models.Notification{}))
	processed, failed = f.worker.Process(ctx)
	require.Zero(t, processed)
	require.Zero(t, failed)
}

func TestNotificationIntentPreferencesMandatoryAndConcurrentClaim(t *testing.T) {
	f := setupIntentFixture(t)
	ctx := context.Background()
	app := f.createApplication(t, f.db)
	require.NoError(t, f.db.Create(&models.UserNotificationPreference{UserID: f.author.ID, TypeCode: services.NotificationTypeApplicationCreated, Enabled: false}).Error)
	require.NoError(t, f.db.Create(&models.UserNotificationPreference{UserID: f.approver.ID, TypeCode: services.NotificationTypeApplicationApprovalRequired, Enabled: false}).Error)
	require.NoError(t, services.EnqueueApplicationSubmittedNotificationTx(ctx, f.db, app.ID, f.author.ID))
	other, err := services.NewNotificationIntentWorker(f.db, f.app, f.notifications)
	require.NoError(t, err)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); f.worker.Process(ctx) }()
	go func() { defer wg.Done(); other.Process(ctx) }()
	wg.Wait()
	var rows []models.Notification
	require.NoError(t, f.db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, f.approver.ID, rows[0].UserID)
	require.Equal(t, 1, rows[0].Count, "concurrent workers must not collapse the same event twice")
}

func TestNotificationIntentPurgeAndCanceledHTTPDoNotLoseOrResurrectRows(t *testing.T) {
	f := setupIntentFixture(t)
	app := f.createApplication(t, f.db)
	request, cancel := context.WithCancel(context.Background())
	require.NoError(t, services.EnqueueApplicationSubmittedNotificationTx(request, f.db, app.ID, f.author.ID))
	cancel()
	processed, failed := f.worker.Process(context.Background())
	require.Equal(t, 1, processed)
	require.Zero(t, failed)
	require.EqualValues(t, 2, intentCount(t, f.db, &models.Notification{}))
	require.NoError(t, f.db.Exec("DELETE FROM notifications").Error)
	second := f.createApplication(t, f.db)
	require.NoError(t, services.EnqueueApplicationSubmittedNotificationTx(context.Background(), f.db, second.ID, f.author.ID))
	require.NoError(t, f.db.Delete(&second).Error)
	processed, failed = f.worker.Process(context.Background())
	require.Zero(t, processed)
	require.Zero(t, failed)
	require.Zero(t, intentCount(t, f.db, &models.Notification{}))
}

func TestNotificationIntentSupplementUsesSnapshotAndSkipsDisabledAccount(t *testing.T) {
	f := setupIntentFixture(t)
	ctx := context.Background()
	app := f.createApplication(t, f.db)
	require.NoError(t, f.db.Model(&models.ApplicationResponsibleUser{}).Where("application_id = ?", app.ID).Update("required_approval", false).Error)
	supplement := models.ApplicationSupplement{ApplicationID: app.ID, Number: 1, Status: models.SupplementPending, CreatedByUserID: f.author.ID, CreatedAt: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)}
	require.NoError(t, f.db.Create(&supplement).Error)
	require.NoError(t, f.db.Create(&models.ApplicationSupplementApproval{SupplementID: supplement.ID, UserID: f.approver.ID, RequiredApproval: true}).Error)
	require.NoError(t, services.EnqueueApplicationSupplementNotificationTx(ctx, f.db, app.ID, supplement.ID, f.author.ID, true))
	processed, failed := f.worker.Process(ctx)
	require.Equal(t, 1, processed)
	require.Zero(t, failed)
	var row models.Notification
	require.NoError(t, f.db.Take(&row).Error)
	require.Equal(t, f.approver.ID, row.UserID)
	require.Contains(t, *row.Message, "дополнение №1")
	require.NotNil(t, row.Data)
	var payload struct {
		SupplementNumber int `json:"supplement_number"`
	}
	require.NoError(t, json.Unmarshal([]byte(*row.Data), &payload))
	require.Equal(t, 1, payload.SupplementNumber)
	require.NoError(t, f.db.Exec("DELETE FROM notifications").Error)
	require.NoError(t, f.db.Model(&f.approver).Update("is_banned", true).Error)
	second := f.createApplication(t, f.db)
	require.NoError(t, services.EnqueueApplicationSubmittedNotificationTx(ctx, f.db, second.ID, f.author.ID))
	processed, failed = f.worker.Process(ctx)
	require.Equal(t, 1, processed)
	require.Zero(t, failed)
	var rows []models.Notification
	require.NoError(t, f.db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, f.author.ID, rows[0].UserID)
}

func TestNotificationIntentBatchIsBoundedAndRemainingWorkSurvives(t *testing.T) {
	f := setupIntentFixture(t)
	ctx := context.Background()
	for i := 0; i < 26; i++ {
		app := f.createApplication(t, f.db)
		require.NoError(t, services.EnqueueApplicationSubmittedNotificationTx(ctx, f.db, app.ID, f.author.ID))
	}
	processed, failed := f.worker.Process(ctx)
	require.Equal(t, 25, processed)
	require.Zero(t, failed)
	var pending int64
	require.NoError(t, f.db.Model(&models.NotificationIntent{}).Where("state = ?", "pending").Count(&pending).Error)
	require.EqualValues(t, 1, pending)
	restarted, err := services.NewNotificationIntentWorker(f.db, f.app, f.notifications)
	require.NoError(t, err)
	processed, failed = restarted.Process(ctx)
	require.Equal(t, 1, processed)
	require.Zero(t, failed)
	require.EqualValues(t, 52, intentCount(t, f.db, &models.Notification{}))
}

func TestNotificationIntentJobDeadlineUsesFreshRetryAndDoesNotStarveNext(t *testing.T) {
	f := setupIntentFixture(t)
	ctx := context.Background()
	first, second := f.createApplication(t, f.db), f.createApplication(t, f.db)
	require.NoError(t, services.EnqueueApplicationSubmittedNotificationTx(ctx, f.db, first.ID, f.author.ID))
	require.NoError(t, services.EnqueueApplicationSubmittedNotificationTx(ctx, f.db, second.ID, f.author.ID))
	var jobs []context.Context
	var retrySeen bool
	const queryCallback = "fixture:intent-job-context"
	const createCallback = "fixture:intent-job-timeout"
	const updateCallback = "fixture:intent-fresh-retry"
	require.NoError(t, f.db.Callback().Query().Before("gorm:query").Register(queryCallback, func(tx *gorm.DB) {
		if tx.Statement.Table == "applications" {
			deadline, ok := tx.Statement.Context.Deadline()
			require.True(t, ok, "each job must have its own deadline")
			require.LessOrEqual(t, time.Until(deadline), 120*time.Second)
			jobs = append(jobs, tx.Statement.Context)
		}
	}))
	var poisoned bool
	require.NoError(t, f.db.Callback().Create().Before("gorm:create").Register(createCallback, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*models.Notification); ok && !poisoned {
			poisoned = true
			// Controlled driver deadline outcome, without a real 120-second wait.
			expired, cancel := context.WithDeadline(tx.Statement.Context, time.Unix(0, 0))
			cancel()
			tx.Statement.Context = expired
			tx.AddError(context.DeadlineExceeded)
		}
	}))
	require.NoError(t, f.db.Callback().Update().Before("gorm:update").Register(updateCallback, func(tx *gorm.DB) {
		if tx.Statement.Table != "notification_intents" {
			return
		}
		fields, ok := tx.Statement.Dest.(map[string]any)
		if !ok {
			return
		}
		if _, ok := fields["attempts"]; !ok {
			return
		}
		retrySeen = true
		require.NoError(t, tx.Statement.Context.Err(), "expired job must not poison its retry context")
		deadline, ok := tx.Statement.Context.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 5*time.Second)
	}))
	t.Cleanup(func() {
		require.NoError(t, f.db.Callback().Query().Remove(queryCallback))
		require.NoError(t, f.db.Callback().Create().Remove(createCallback))
		require.NoError(t, f.db.Callback().Update().Remove(updateCallback))
	})
	processed, failed := f.worker.Process(ctx)
	require.Equal(t, 1, processed)
	require.Equal(t, 1, failed)
	require.True(t, retrySeen)
	require.Len(t, jobs, 2)
	require.False(t, jobs[0] == jobs[1], "one job budget must not be shared across the batch")
	var pending models.NotificationIntent
	require.NoError(t, f.db.Where("application_id = ?", first.ID).Take(&pending).Error)
	require.Equal(t, 1, pending.Attempts)
	require.True(t, pending.NextAttemptAt.After(pending.OccurredAt))
	require.EqualValues(t, 2, intentCount(t, f.db, &models.Notification{}), "later event must materialize after a poison event")
}

func TestNotificationIntentRevokedConsentSkipsRawAuthorNameQuery(t *testing.T) {
	f := setupIntentFixture(t)
	ctx := context.Background()
	const privateName = "SyntheticPrivateSurname"
	require.NoError(t, f.db.Model(&f.author).Updates(map[string]any{"last_name": privateName, "first_name": "SyntheticPrivateFirst"}).Error)
	for key, value := range map[string]string{"legal.pd_consent_required": "true", "legal.pd_consent_text": "<p>Synthetic consent</p>"} {
		require.NoError(t, f.db.Exec("INSERT INTO system_settings (key,value,type) VALUES (?,?,'string') ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value", key, value).Error)
	}
	consentTime := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	revokedAt := consentTime.Add(time.Minute)
	require.NoError(t, f.db.Create(&models.PDConsent{UserID: f.author.ID, ConsentType: services.ConsentTypePDProcessing, Granted: true, GrantedAt: consentTime, RevokedAt: &revokedAt, CreatedAt: consentTime}).Error)
	app := f.createApplication(t, f.db)
	require.NoError(t, services.EnqueueApplicationSubmittedNotificationTx(ctx, f.db, app.ID, f.author.ID))
	const callback = "fixture:forbid-revoked-raw-name"
	var rawQuery bool
	require.NoError(t, f.db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "users" && strings.Contains(strings.Join(tx.Statement.Selects, ","), "last_name") {
			rawQuery = true
			tx.AddError(errors.New("raw author name query forbidden after consent revoke"))
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.db.Callback().Query().Remove(callback)) })
	processed, failed := f.worker.Process(ctx)
	require.Equal(t, 1, processed)
	require.Zero(t, failed)
	require.False(t, rawQuery)
	var rows []models.Notification
	require.NoError(t, f.db.Find(&rows).Error)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.NotNil(t, row.Data)
		require.Contains(t, *row.Data, "@"+f.author.Username)
		require.NotContains(t, *row.Data, privateName)
		if row.Message != nil {
			require.NotContains(t, *row.Message, privateName)
		}
	}
	f.probe.mu.Lock()
	defer f.probe.mu.Unlock()
	for _, event := range f.probe.events {
		require.Equal(t, "notification.new", event.Type)
		require.Equal(t, "notifications", event.Scope)
	}
	for _, payload := range f.probe.payloads {
		require.NotContains(t, payload.Title, privateName)
		require.NotContains(t, payload.Message, privateName)
	}
}
