package handlers_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"systemburo/internal/models"
	"systemburo/internal/realtime"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

type queueAsyncHTTPClient func(*http.Request) (*http.Response, error)

func (f queueAsyncHTTPClient) Do(r *http.Request) (*http.Response, error) { return f(r) }

func pushQueueResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}
}

func shutdownPushFixture(svc services.PushService, release func()) {
	if release != nil {
		release()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	svc.Shutdown(ctx)
}

func newAsyncQueueService(t *testing.T, db *gorm.DB, client queueAsyncHTTPClient) services.PushService {
	t.Helper()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	return services.NewPushService(db, pub, priv, testPushSubject,
		services.WithPushLocalEndpoints(), services.WithPushHTTPClient(client))
}

func awaitPushQueueSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("delivery did not reach controlled transport")
	}
}

// Saturating real asynchronous delivery must not lose persisted or SSE notifications.
func TestPushQueueAsync_OverflowPreservesNotifications(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	uid := seedPushUser(t, db, td, "queue_overflow")
	seedSubscription(t, db, uid, "http://127.0.0.1/queue-overflow")
	entered := make(chan struct{}, 30)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var calls atomic.Int64
	svc := newAsyncQueueService(t, db, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
			return pushQueueResponse(http.StatusCreated), nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	})
	defer shutdownPushFixture(svc, unblock)
	payload := services.PushPayload{Title: "Queue", Message: "Retained", Type: "application_created", NotificationID: 1}
	for i := 0; i < 30; i++ {
		svc.Send(context.Background(), uid, payload)
	}
	for i := 0; i < 30; i++ {
		awaitPushQueueSignal(t, entered)
	}
	for i := 0; i < 1024; i++ {
		svc.Send(context.Background(), uid, payload)
	}
	hub := realtime.NewHub()
	events, unsubscribe, subscribeErr := hub.Subscribe(uid, uid)
	require.NoError(t, subscribeErr)
	defer unsubscribe()
	notifications := services.NewNotificationService(db, services.WithNotificationPushSender(svc), services.WithNotificationRealtimePublisher(hub))
	title, message, typ := "Retained notification", "First", services.NotificationTypeApplicationApprovalRequired
	_, err := notifications.Create(context.Background(), models.CreateNotificationRequest{UserID: uid, Type: &typ, Title: &title, Message: &message})
	require.NoError(t, err)
	require.NoError(t, notifications.CreateForUserGrouped(context.Background(), uid, typ, title, "Second", nil, "queue-group"))
	require.NoError(t, notifications.CreateForUserGrouped(context.Background(), uid, typ, title, "Third", nil, "queue-group"))
	var rows []models.Notification
	require.NoError(t, db.Where("user_id = ?", uid).Find(&rows).Error)
	require.Len(t, rows, 2)
	var grouped models.Notification
	require.NoError(t, db.Where("user_id = ? AND group_key = ?", uid, "queue-group").First(&grouped).Error)
	require.Equal(t, 2, grouped.Count)
	require.NotNil(t, grouped.Message)
	require.Equal(t, "Есть обновление", *grouped.Message)
	require.Equal(t, "Есть обновление", *grouped.Title)
	require.Nil(t, grouped.Data)
	require.Nil(t, grouped.ProjectionRef)
	feed, err := notifications.GetByUserID(context.Background(), uid)
	require.NoError(t, err)
	require.Len(t, feed, 2)
	for _, notice := range feed {
		require.Equal(t, "Есть обновление", *notice.Message)
		require.Nil(t, notice.Data)
		require.Nil(t, notice.GroupKey)
	}
	for i := 0; i < 3; i++ {
		select {
		case ev := <-events:
			require.Equal(t, "notification.new", ev.Type)
		default:
			t.Fatal("persisted notification must publish its SSE signal")
		}
	}
	unblock()
	shutdownPushFixture(svc, nil)
	require.EqualValues(t, 1054, calls.Load(), "overflow must reject only new push jobs")
}

func TestPushQueueAsync_CancellationKeepsSubscription(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	uid := seedPushUser(t, db, td, "queue_cancel")
	seedSubscription(t, db, uid, "http://127.0.0.1/queue-cancel")
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("user_id = ?", uid).Updates(map[string]interface{}{"failed_count": 9, "last_error": "preserve"}).Error)
	entered, returned := make(chan struct{}), make(chan struct{})
	svc := newAsyncQueueService(t, db, func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-r.Context().Done()
		close(returned)
		return nil, r.Context().Err()
	})
	defer shutdownPushFixture(svc, nil)
	svc.Send(context.Background(), uid, services.PushPayload{Title: "Cancel", Type: "application_created"})
	awaitPushQueueSignal(t, entered)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.Shutdown(ctx)
	awaitPushQueueSignal(t, returned)
	shutdownPushFixture(svc, nil)
	var sub models.PushSubscription
	require.NoError(t, db.Where("user_id = ?", uid).First(&sub).Error)
	require.Equal(t, 9, sub.FailedCount)
	require.NotNil(t, sub.LastError)
	require.Equal(t, "preserve", *sub.LastError)
	require.Nil(t, sub.LastSuccessAt)
}

func TestPushQueueAsync_CallerCancellationDoesNotCancelAcceptedJob(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	uid := seedPushUser(t, db, td, "queue_caller_cancel")
	seedSubscription(t, db, uid, "http://127.0.0.1/caller-cancel")
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	svc := newAsyncQueueService(t, db, func(r *http.Request) (*http.Response, error) {
		close(entered)
		select {
		case <-release:
			return pushQueueResponse(http.StatusCreated), nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	})
	defer shutdownPushFixture(svc, unblock)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Send(ctx, uid, services.PushPayload{Title: "Retain", Type: "application_created"})
	awaitPushQueueSignal(t, entered)
	cancel()
	unblock()
	shutdownPushFixture(svc, nil)
	var sub models.PushSubscription
	require.NoError(t, db.Where("user_id = ?", uid).First(&sub).Error)
	require.NotNil(t, sub.LastSuccessAt)
}

func TestPushQueueAsync_PagesExcludeLateAndForeignSubscriptions(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	uid := seedPushUser(t, db, td, "queue_pages")
	foreign := seedPushUser(t, db, td, "queue_foreign")
	p256dh, auth := fakeSubscriberKeys(t)
	rows := make([]models.PushSubscription, 205)
	for i := range rows {
		rows[i] = models.PushSubscription{UserID: uid, Endpoint: fmt.Sprintf("http://127.0.0.1/page-%d", i), P256dh: p256dh, Auth: auth}
	}
	require.NoError(t, db.CreateInBatches(&rows, 100).Error)
	seedSubscription(t, db, foreign, "http://127.0.0.1/foreign")
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var mu sync.Mutex
	seen := make(map[string]int)
	deadlines := make([]time.Time, 0, 205)
	type pageQuery struct {
		sql   string
		limit int
		rows  int64
	}
	var queries []pageQuery
	const callback = "test:push_queue_pages"
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table != "push_subscriptions" {
			return
		}
		limit := 0
		if c, ok := tx.Statement.Clauses["LIMIT"].Expression.(clause.Limit); ok && c.Limit != nil {
			limit = *c.Limit
		}
		mu.Lock()
		queries = append(queries, pageQuery{sql: tx.Statement.SQL.String(), limit: limit, rows: tx.RowsAffected})
		mu.Unlock()
	}))
	defer db.Callback().Query().Remove(callback)
	svc := newAsyncQueueService(t, db, func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		first := len(seen) == 0
		seen[r.URL.Path]++
		deadline, _ := r.Context().Deadline()
		deadlines = append(deadlines, deadline)
		mu.Unlock()
		if first {
			close(entered)
			select {
			case <-release:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
		}
		switch r.URL.Path {
		case "/page-0":
			return pushQueueResponse(http.StatusNotFound), nil
		case "/page-99":
			return pushQueueResponse(http.StatusGone), nil
		}
		return pushQueueResponse(http.StatusCreated), nil
	})
	defer shutdownPushFixture(svc, unblock)
	svc.Send(context.Background(), uid, services.PushPayload{Title: "Pages", Type: "application_created"})
	awaitPushQueueSignal(t, entered)
	seedSubscription(t, db, uid, "http://127.0.0.1/late")
	unblock()
	shutdownPushFixture(svc, nil)
	require.NoError(t, db.Callback().Query().Remove(callback))
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, queries, 3, "205 subscriptions require three bounded page queries")
	for i, query := range queries {
		require.Equal(t, 100, query.limit)
		require.Contains(t, query.sql, "id >")
		require.Contains(t, query.sql, "id <=")
		require.Contains(t, query.sql, "user_id =")
		require.EqualValues(t, []int{100, 100, 5}[i], query.rows)
	}
	require.Len(t, seen, 205)
	for i := 0; i < 205; i++ {
		require.Equal(t, 1, seen[fmt.Sprintf("/page-%d", i)])
	}
	require.Zero(t, seen["/foreign"])
	require.Zero(t, seen["/late"])
	for _, deadline := range deadlines {
		require.False(t, deadline.IsZero())
		require.Equal(t, deadlines[0], deadline, "one processing deadline must span all pages")
	}
	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("user_id = ? AND last_success_at IS NOT NULL", uid).Count(&count).Error)
	require.EqualValues(t, 203, count)
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("id IN ?", []int{rows[0].ID, rows[99].ID}).Count(&count).Error)
	require.Zero(t, count)
}
