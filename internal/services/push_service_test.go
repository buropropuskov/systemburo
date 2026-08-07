package services

import (
	"context"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/models"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Чистые юниты push_service.go: сборка payload, обрезка длинного текста, решение об
// удалении подписки по коду ответа. Без БД - живёт рядом с DB-тестами доставки
// (Subscribe/Send через httptest) в internal/handlers, как и остальные сервисы (#974).

func TestBuildPushMessage_TruncatesLongMessage(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("я", pushPayloadMaxMessageLen+50)
	body := buildPushMessage(PushPayload{Title: "Заголовок", Message: long, Type: "application_created", NotificationID: 1})

	var decoded struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("payload не разобран: %v", err)
	}
	runes := []rune(decoded.Message)
	if len(runes) != pushPayloadMaxMessageLen+1 { // +1 за многоточие
		t.Errorf("ожидалась длина %d, получено %d (%q)", pushPayloadMaxMessageLen+1, len(runes), decoded.Message)
	}
	if !strings.HasSuffix(decoded.Message, "…") {
		t.Errorf("обрезанный текст должен заканчиваться многоточием, получено %q", decoded.Message)
	}
}

func TestBuildPushMessage_ShortMessageUntouched(t *testing.T) {
	t.Parallel()
	body := buildPushMessage(PushPayload{Title: "Заголовок", Message: "Короткий текст", Type: "application_created", NotificationID: 1})

	var decoded struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("payload не разобран: %v", err)
	}
	if decoded.Message != "Короткий текст" {
		t.Errorf("короткий текст не должен меняться, получено %q", decoded.Message)
	}
}

// TestBuildPushMessage_TruncatesLongTitle защищает явную обрезку заголовка (#974,
// разбор team-lead - пункт 3): раньше обрезался только Message, а Title молча полагался
// на то, что notifications.title в БД varchar(255) не пустит длиннее. Это ограничение
// схемы ленты уведомлений, а не push-сервиса - buildPushMessage обязан резать сам.
func TestBuildPushMessage_TruncatesLongTitle(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("я", pushPayloadMaxTitleLen+50)
	body := buildPushMessage(PushPayload{Title: long, Message: "M", Type: "application_created", NotificationID: 1})

	var decoded struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("payload не разобран: %v", err)
	}
	runes := []rune(decoded.Title)
	if len(runes) != pushPayloadMaxTitleLen+1 { // +1 за многоточие
		t.Errorf("ожидалась длина %d, получено %d (%q)", pushPayloadMaxTitleLen+1, len(runes), decoded.Title)
	}
}

func TestTruncatePushText_RespectsRuneBoundary(t *testing.T) {
	t.Parallel()
	s := "привет"
	got := truncatePushText(s, 3)
	want := "при…"
	if got != want {
		t.Errorf("ожидалось %q, получено %q", want, got)
	}
}

func TestTruncatePushText_ShortStringUnchanged(t *testing.T) {
	t.Parallel()
	if got := truncatePushText("привет", 100); got != "привет" {
		t.Errorf("строка короче предела не должна меняться, получено %q", got)
	}
}

func TestShouldDropSubscription(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status int
		drop   bool
	}{
		{http.StatusNotFound, true},
		{http.StatusGone, true},
		{http.StatusOK, false},
		{http.StatusCreated, false},
		{http.StatusInternalServerError, false},
		{http.StatusForbidden, false},
		{http.StatusTooManyRequests, false},
	}
	for _, c := range cases {
		if got := shouldDropSubscription(c.status); got != c.drop {
			t.Errorf("status=%d: ожидалось drop=%v, получено %v", c.status, c.drop, got)
		}
	}
}

// testSubscriberKeys генерирует валидную пару ключей подписчика (P-256 публичный ключ +
// 16-байтный auth secret), как их отдал бы PushSubscription.getKey() в браузере -
// webpush-go проверяет, что p256dh лежит на кривой, и падает раньше проверки размера
// на случайной строке.
func testSubscriberKeys(t *testing.T) (p256dh, auth string) {
	t.Helper()
	curve := elliptic.P256()
	_, x, y, err := elliptic.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatalf("generate EC key: %v", err)
	}
	pub := elliptic.Marshal(curve, x, y)
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		t.Fatalf("generate auth secret: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(pub), base64.RawURLEncoding.EncodeToString(secret)
}

// TestDeliver_PayloadTooLarge_DoesNotTouchDB защищает разбор team-lead (пункт 3):
// payload, не влезающий в бюджет webpush-go (ErrMaxPadExceeded возвращается ДО сетевого
// запроса), не должен наказывать подписку - её счётчик неудач и БД вообще не трогаются.
// db у сервиса намеренно nil: если бы код всё же дошёл до recordFailure/recordSuccess,
// тест упал бы паникой на nil-указателе, а не тихо прошёл - это и есть доказательство,
// что ветка ErrMaxPadExceeded возвращается раньше любого обращения к базе.
func TestDeliver_PayloadTooLarge_DoesNotTouchDB(t *testing.T) {
	t.Parallel()
	p256dh, auth := testSubscriberKeys(t)
	privKey, pubKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("generate VAPID keys: %v", err)
	}
	svc := &pushService{publicKey: pubKey, privateKey: privKey, subscriber: "mailto:test@example.com"}
	sub := models.PushSubscription{ID: 1, UserID: 1, Endpoint: "https://push.example.invalid/ep", P256dh: p256dh, Auth: auth}

	// Заведомо больше бюджета webpush-go (около 3993 байт при MaxRecordSize=4096) -
	// сообщения такого размера buildPushMessage никогда не соберёт (обрезка держит
	// worst case на ~1.6 КБ), но deliver() должен пережить его и без обрезки, не упав.
	huge := make([]byte, 5000)

	svc.deliver(context.Background(), sub, huge)
}

// Shutdown() чистая логика без БД/HTTP - управляет напрямую s.wg, минуя Send()/sendNow.
// Защищает разбор team-lead (#974, пункт 1): drain при остановке сервера.

// TestPushService_Shutdown_WaitsForPendingWork: Shutdown должен дождаться работы,
// зарегистрированной в wg, а не вернуться немедленно.
func TestPushService_Shutdown_WaitsForPendingWork(t *testing.T) {
	t.Parallel()
	s := &pushService{}
	s.wg.Add(1)
	var finished atomic.Bool
	go func() {
		time.Sleep(50 * time.Millisecond)
		finished.Store(true)
		s.wg.Done()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s.Shutdown(ctx)

	if !finished.Load() {
		t.Error("Shutdown должен был дождаться завершения работы в wg, а не вернуться раньше")
	}
}

// TestPushService_Shutdown_ReturnsOnContextDeadline: зависшая работа (никогда не
// вызывает wg.Done) не должна держать Shutdown дольше отведённого ctx - иначе один
// зависший push-сервис не дал бы процессу завершиться вовсе.
func TestPushService_Shutdown_ReturnsOnContextDeadline(t *testing.T) {
	t.Parallel()
	s := &pushService{}
	s.wg.Add(1)
	// Done() в конце теста подчищает внутреннюю горутину Shutdown (она ждёт s.wg.Wait()
	// в фоне даже после того, как сам Shutdown уже вернулся по ctx) - иначе она
	// протекла бы на весь процесс тестового бинаря.
	t.Cleanup(func() { s.wg.Done() })

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	started := time.Now()
	s.Shutdown(ctx)
	elapsed := time.Since(started)

	if elapsed > 500*time.Millisecond {
		t.Errorf("Shutdown должен был вернуться по истечении ctx (50мс), реально занял %s", elapsed)
	}
	if !s.shuttingDown.Load() {
		t.Error("Shutdown обязан взвести shuttingDown до начала ожидания")
	}
}

func TestNullableString(t *testing.T) {
	t.Parallel()
	if got := nullableString(""); got != nil {
		t.Errorf("пустая строка должна давать nil, получено %v", *got)
	}
	if got := nullableString("Chrome"); got == nil || *got != "Chrome" {
		t.Errorf("непустая строка должна вернуться как есть, получено %v", got)
	}
}
