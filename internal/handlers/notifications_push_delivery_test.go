package handlers_test

import (
	"context"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	webpush "github.com/SherClockHolmes/webpush-go"
	"gorm.io/gorm"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// Доставка Web Push (#974) через реальный webpush.SendNotificationWithContext -
// шифрование и подпись VAPID настоящие, конечная точка подменена httptest-сервером.
// WithPushSyncSend делает Send() синхронным, чтобы проверять результат сразу после
// вызова, без гонки с фоновой горутиной прод-режима. Пользователь заводится напрямую
// через db.Create, без HTTP-регистрации - эти тесты вообще не трогают Echo-приложение.

// fakeSubscriberKeys генерирует валидную пару ключей подписчика (P-256 публичный ключ +
// 16-байтный auth secret), как их отдал бы PushSubscription.getKey() в браузере -
// webpush-go проверяет, что p256dh лежит на кривой, и падает раньше HTTP-запроса на
// случайной строке.
func fakeSubscriberKeys(t *testing.T) (p256dh, auth string) {
	t.Helper()
	curve := elliptic.P256()
	_, x, y, err := elliptic.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)
	pub := elliptic.Marshal(curve, x, y)
	secret := make([]byte, 16)
	_, err = rand.Read(secret)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(pub), base64.RawURLEncoding.EncodeToString(secret)
}

// newTestPushService поднимает pushService с настоящей VAPID-парой (иначе подпись
// запроса не соберётся) и синхронной отправкой, готовый писать в db из testutil.
func newTestPushService(t *testing.T, db *gorm.DB) services.PushService {
	t.Helper()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	return services.NewPushService(db, pub, priv, testPushSubject, services.WithPushSyncSend())
}

// testPushSubject -- адрес контакта в том виде, в каком он лежит в VAPID_SUBJECT: со
// схемой mailto:. Ровно такой же он обязан оказаться и в подписи запроса.
const testPushSubject = "mailto:bureau@example.com"

// seedPushUser заводит пользователя напрямую через db.Create - этим тестам не нужна
// настоящая аутентификация, только валидный user_id для FK подписки.
func seedPushUser(t *testing.T, db *gorm.DB, td testutil.TestData, username string) int {
	t.Helper()
	u := models.User{
		Username:       username,
		Password:       "x",
		TypeID:         1,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&u).Error)
	return u.ID
}

func seedSubscription(t *testing.T, db *gorm.DB, userID int, endpoint string) {
	t.Helper()
	p256dh, auth := fakeSubscriberKeys(t)
	sub := models.PushSubscription{UserID: userID, Endpoint: endpoint, P256dh: p256dh, Auth: auth}
	require.NoError(t, db.Create(&sub).Error)
}

// TestPushService_Send_DeliversAndRecordsSuccess: push-сервис отвечает 201 - подписка
// получает LastSuccessAt, счётчик неудач остаётся нулевым.
func TestPushService_Send_DeliversAndRecordsSuccess(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	userID := seedPushUser(t, db, td, "push_deliver")
	seedSubscription(t, db, userID, srv.URL)

	svc := newTestPushService(t, db)
	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	var sub models.PushSubscription
	require.NoError(t, db.Where("user_id = ?", userID).First(&sub).Error)
	assert.NotNil(t, sub.LastSuccessAt)
	assert.Equal(t, 0, sub.FailedCount)
}

// TestPushService_Send_SetsHighUrgency: запрос уходит с Urgency: high (RFC 8030).
// Без заголовка сообщение считается "normal", и телефон с низким зарядом придержит
// его до следующего пробуждения - уведомление о заявке приедет с опозданием.
func TestPushService_Send_SetsHighUrgency(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	var gotUrgency, gotTTL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUrgency = r.Header.Get("Urgency")
		gotTTL = r.Header.Get("TTL")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	userID := seedPushUser(t, db, td, "push_urgency")
	seedSubscription(t, db, userID, srv.URL)

	svc := newTestPushService(t, db)
	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	assert.Equal(t, "high", gotUrgency)
	assert.Equal(t, "86400", gotTTL, "сутки хранения недоставленного сообщения")
}

// TestPushService_Send_SubjectHasSingleMailtoScheme: в подписи VAPID адрес контакта
// должен стоять ровно с одной схемой mailto:. webpush-go приписывает её сама всему, что
// не начинается с https:, поэтому готовое значение VAPID_SUBJECT давало
// "mailto:mailto:адрес" - Apple отвергала такие запросы с 403, и на iPhone не доходило
// ни одно уведомление, тогда как Google и Mozilla то же самое принимали (#974).
func TestPushService_Send_SubjectHasSingleMailtoScheme(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	userID := seedPushUser(t, db, td, "push_subject")
	seedSubscription(t, db, userID, srv.URL)

	svc := newTestPushService(t, db)
	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	require.True(t, strings.HasPrefix(auth, "vapid t="), "ожидался заголовок схемы vapid, получено: %q", auth)
	claims := decodeJWTClaims(t, auth)
	sub, _ := claims["sub"].(string)
	assert.Equal(t, testPushSubject, sub, "адрес контакта в подписи должен совпадать с настроенным")
	assert.False(t, strings.HasPrefix(sub, "mailto:mailto:"), "схема mailto: продублирована - Apple ответит 403")
}

// decodeJWTClaims достаёт полезную нагрузку JWT из заголовка "vapid t=<jwt>, k=<key>".
// Подпись здесь не проверяется: тест смотрит на содержимое утверждений, а корректность
// подписи стерегут сами push-сервисы.
func decodeJWTClaims(t *testing.T, authHeader string) map[string]any {
	t.Helper()
	token := strings.TrimPrefix(authHeader, "vapid t=")
	if i := strings.Index(token, ","); i >= 0 {
		token = token[:i]
	}
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "JWT должен состоять из трёх частей")
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(raw, &claims))
	return claims
}

// TestPushService_Send_410RemovesSubscription: push-сервис подтверждает 410 Gone -
// подписка удаляется сразу, без накопления счётчика неудач (#974).
func TestPushService_Send_410RemovesSubscription(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()

	userID := seedPushUser(t, db, td, "push_gone")
	seedSubscription(t, db, userID, srv.URL)

	svc := newTestPushService(t, db)
	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("user_id = ?", userID).Count(&count).Error)
	assert.Equal(t, int64(0), count, "410 должен удалить подписку немедленно")
}

// TestPushService_Send_404RemovesSubscription: то же самое для 404 - оба кода по RFC 8030
// значат "endpoint больше не существует".
func TestPushService_Send_404RemovesSubscription(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	userID := seedPushUser(t, db, td, "push_404")
	seedSubscription(t, db, userID, srv.URL)

	svc := newTestPushService(t, db)
	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("user_id = ?", userID).Count(&count).Error)
	assert.Equal(t, int64(0), count, "404 должен удалить подписку немедленно")
}

// TestPushService_Send_TracksFailureAndRemovesAfterLimit: push-сервис стабильно отвечает
// 500 (не 404/410) - подписка не удаляется на первой неудаче, счётчик растёт, а после
// pushSubscriptionFailureLimit подряд идущих неудач строка убирается (#974).
func TestPushService_Send_TracksFailureAndRemovesAfterLimit(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	userID := seedPushUser(t, db, td, "push_flaky")
	seedSubscription(t, db, userID, srv.URL)

	svc := newTestPushService(t, db)
	payload := services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1}

	// Первая неудача: подписка остаётся, счётчик = 1.
	svc.Send(context.Background(), userID, payload)
	var sub models.PushSubscription
	require.NoError(t, db.Where("user_id = ?", userID).First(&sub).Error)
	assert.Equal(t, 1, sub.FailedCount)
	require.NotNil(t, sub.LastError)

	// Ещё восемь неудач подряд (итого девять) - подписка ещё жива.
	for i := 0; i < 8; i++ {
		svc.Send(context.Background(), userID, payload)
	}
	require.NoError(t, db.Where("user_id = ?", userID).First(&sub).Error)
	assert.Equal(t, 9, sub.FailedCount)

	// Десятая неудача переваливает порог - подписка удаляется.
	svc.Send(context.Background(), userID, payload)
	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("user_id = ?", userID).Count(&count).Error)
	assert.Equal(t, int64(0), count, "подписка должна быть удалена после серии неудач")
	assert.Equal(t, int32(10), atomic.LoadInt32(&calls))
}

// TestPushService_Send_VAPIDDisabled_NoOp: пустые VAPID-ключи (штатный режим "push
// выключен") - Send() ничего не отправляет и не трогает подписку.
func TestPushService_Send_VAPIDDisabled_NoOp(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	userID := seedPushUser(t, db, td, "push_off")
	seedSubscription(t, db, userID, srv.URL)

	svc := services.NewPushService(db, "", "", "", services.WithPushSyncSend())
	assert.False(t, svc.Configured())
	assert.Empty(t, svc.PublicKey())

	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	assert.Equal(t, int32(0), atomic.LoadInt32(&calls), "выключенный push не должен делать сетевых запросов")
	var sub models.PushSubscription
	require.NoError(t, db.Where("user_id = ?", userID).First(&sub).Error)
	assert.Nil(t, sub.LastSuccessAt)
	assert.Equal(t, 0, sub.FailedCount)
}

// TestPushService_Send_NoSubscriptions_NoOp: пользователь без подписок - Send() не падает
// и не создаёт лишних запросов.
func TestPushService_Send_NoSubscriptions_NoOp(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userID := seedPushUser(t, db, td, "push_nosub")
	svc := newTestPushService(t, db)
	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("user_id = ?", userID).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

// pushMaxConcurrentDeliveries дублирует одноимённую неэкспортированную константу
// push_service.go (пул одновременных отправок #974, разбор team-lead пункт 2) - тест
// живёт в другом пакете и не может сослаться на неё напрямую.
const pushMaxConcurrentDeliveries = 30

// TestPushService_Send_RespectsConcurrencyLimit защищает общий пул отправок (#974,
// разбор team-lead пункт 2): рассылка на много пользователей не должна открывать
// больше pushMaxConcurrentDeliveries одновременных запросов к push-сервису разом,
// сколько бы получателей ни было. Использует НАСТОЯЩИЙ асинхронный путь Send()
// (без WithPushSyncSend) - именно там раньше не было никакого ограничения.
func TestPushService_Send_RespectsConcurrencyLimit(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const totalUsers = pushMaxConcurrentDeliveries + 20 // заведомо больше пула - иначе тест не застанет насыщение

	var (
		current int32
		mu      sync.Mutex
		peak    int32
	)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseHandlers := func() { releaseOnce.Do(func() { close(release) }) }
	// defer СРАЗУ после создания канала, а не только явный close() ниже по телу теста:
	// если require.Eventually упадёт по таймауту (FailNow), явный close() ниже так и
	// не выполнится, обработчики останутся висеть на <-release навсегда, а
	// httptest.Server.Close() (тоже defer) будет ждать их вечно и повесит весь пакет
	// (найдено ревью team-lead на предыдущей версии теста).
	defer releaseHandlers()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&current, 1)
		mu.Lock()
		if n > peak {
			peak = n
		}
		mu.Unlock()
		<-release // держим соединение открытым до сигнала теста - так пул успевает насытиться
		atomic.AddInt32(&current, -1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	priv, pub, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	svc := services.NewPushService(db, pub, priv, "mailto:test@example.com") // без WithPushSyncSend - боевой асинхронный путь

	for i := 0; i < totalUsers; i++ {
		uid := seedPushUser(t, db, td, fmt.Sprintf("push_concurrency_%d", i))
		// Endpoint ОБЯЗАН быть уникальным на пользователя: одинаковый srv.URL у всех
		// схлопнул бы полсотни подписок в одну строку через ON CONFLICT (endpoint) в
		// Subscribe (намеренное поведение "endpoint переезжает на нового владельца",
		// см. push_service.go) - тест тогда проверял бы одну одновременную доставку
		// вместо пятидесяти (найдено ревью team-lead).
		seedSubscription(t, db, uid, fmt.Sprintf("%s/ep-%d", srv.URL, i))
		svc.Send(context.Background(), uid, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})
	}

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&current) >= pushMaxConcurrentDeliveries
	}, 5*time.Second, 10*time.Millisecond, "пул должен насытиться под нагрузкой в pushMaxConcurrentDeliveries запросов")

	releaseHandlers()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&current) == 0
	}, 5*time.Second, 10*time.Millisecond, "все отправки должны завершиться после освобождения")

	mu.Lock()
	gotPeak := peak
	mu.Unlock()
	assert.LessOrEqual(t, gotPeak, int32(pushMaxConcurrentDeliveries), "пик одновременных запросов не должен превышать пул")
}

// TestPushService_Shutdown_WaitsForInFlightSend защищает drain при остановке сервера
// (#974, разбор team-lead пункт 1): Shutdown обязан дождаться уже запущенной отправки, а
// не бросить её на середине - иначе отметка об успешной доставке в БД теряется молча.
func TestPushService_Shutdown_WaitsForInFlightSend(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // отправка "в процессе" на момент старта Shutdown
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	priv, pub, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	svc := services.NewPushService(db, pub, priv, "mailto:test@example.com")

	userID := seedPushUser(t, db, td, "push_shutdown_wait")
	seedSubscription(t, db, userID, srv.URL)
	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	// Достаточный запас на срок остановки - подтверждает, что Shutdown реально
	// дожидается, а не просто мгновенно возвращается, не дав отправке уйти.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	svc.Shutdown(shutdownCtx)

	var sub models.PushSubscription
	require.NoError(t, db.Where("user_id = ?", userID).First(&sub).Error)
	assert.NotNil(t, sub.LastSuccessAt, "Shutdown должен был дождаться завершения отправки и записи успеха")
}

// TestPushService_Shutdown_GivesUpAfterGracePeriod: если отправка виснет дольше
// отведённого срока (зависший push-сервис), Shutdown не ждёт вечно - возвращается по
// истечении ctx, как и предупреждал team-lead ("иначе это будет хуже потерянной
// доставки").
//
// Обработчик освобождается СВОИМ каналом stop, а не только через r.Context().Done():
// первая версия теста полагалась на srv.CloseClientConnections() перед Close(), но
// httptest не гарантирует, что отмена соединения со стороны сервера мгновенно отменяет
// контекст УЖЕ ПРИНЯТОГО запроса - на практике обработчик иногда оставался висеть, и
// httptest.Server.Close() (тоже defer) ждал его вечно, вешая пакет целиком (поймано
// ревью team-lead: "httptest.Server blocked in Close after 5 seconds"). defer close(stop)
// зарегистрирован ПОСЛЕ defer srv.Close() - выполняется первым (LIFO) и гарантированно
// освобождает обработчик до того, как Close начнёт его ждать, независимо от таймингов
// HTTP-слоя.
func TestPushService_Shutdown_GivesUpAfterGracePeriod(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	stop := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-stop:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(stop)

	priv, pub, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	svc := services.NewPushService(db, pub, priv, "mailto:test@example.com")

	userID := seedPushUser(t, db, td, "push_shutdown_giveup")
	seedSubscription(t, db, userID, srv.URL)
	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	started := time.Now()
	svc.Shutdown(shutdownCtx)
	elapsed := time.Since(started)

	assert.Less(t, elapsed, 2*time.Second, "Shutdown обязан вернуться по истечении ctx, а не ждать зависший push-сервис")
}

// TestPushService_ListDevices_ScopedToUser проверяет ListDevices напрямую (без HTTP) -
// сервисный уровень контракта, которым пользуется PushHandler.GetStatus.
func TestPushService_ListDevices_ScopedToUser(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userA := seedPushUser(t, db, td, "push_list_a")
	userB := seedPushUser(t, db, td, "push_list_b")
	for i := 0; i < 2; i++ {
		seedSubscription(t, db, userA, fmt.Sprintf("https://push.example.com/ep-list-a-%d", i))
	}
	seedSubscription(t, db, userB, "https://push.example.com/ep-list-b")

	svc := newTestPushService(t, db)
	devicesA, err := svc.ListDevices(context.Background(), userA)
	require.NoError(t, err)
	assert.Len(t, devicesA, 2)

	devicesB, err := svc.ListDevices(context.Background(), userB)
	require.NoError(t, err)
	assert.Len(t, devicesB, 1)
}
