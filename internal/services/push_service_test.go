package services

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
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

func TestNullableString(t *testing.T) {
	t.Parallel()
	if got := nullableString(""); got != nil {
		t.Errorf("пустая строка должна давать nil, получено %v", *got)
	}
	if got := nullableString("Chrome"); got == nil || *got != "Chrome" {
		t.Errorf("непустая строка должна вернуться как есть, получено %v", got)
	}
}
