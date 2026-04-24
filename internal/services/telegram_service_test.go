package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testReport() *models.BugReport {
	return &models.BugReport{
		BugHash:    "abc123de",
		UserID:     42,
		Route:      "POST /applications",
		HTTPStatus: 500,
		Message:    "Internal Server Error",
		UserAgent:  "Mozilla/5.0",
		CreatedAt:  time.Now(),
	}
}

// newTestTelegramService - сервис с подменённым API URL и ускоренным backoff
// для тестов. Воспроизводит NewTelegramService, но не делает реальных запросов
// к api.telegram.org.
func newTestTelegramService(apiURL, token, chatID string) *telegramService {
	return &telegramService{
		botToken:    token,
		chatID:      chatID,
		apiBaseURL:  apiURL,
		backoffBase: 5 * time.Millisecond,
		httpClient:  &http.Client{Timeout: 2 * time.Second},
	}
}

// Пустой bot token - сервис не должен делать сетевой запрос и возвращать nil.
func TestTelegramService_EmptyConfig_NoRequest(t *testing.T) {
	calls := int32(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewTelegramService("", "")
	err := s.SendBugReport(context.Background(), testReport(), "alice")
	require.NoError(t, err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&calls))
}

// Happy path - успешно отправляется с первого раза.
func TestTelegramService_Success(t *testing.T) {
	calls := int32(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		assert.Equal(t, "/bottest-token/sendMessage", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestTelegramService(srv.URL, "test-token", "test-chat")
	err := s.SendBugReport(context.Background(), testReport(), "alice")
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

// 5xx ответы: 2 попытки 500, третья 200 - считаем успехом, три вызова.
func TestTelegramService_RetryOn5xx_ThenSucceeds(t *testing.T) {
	calls := int32(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestTelegramService(srv.URL, "t", "c")
	err := s.SendBugReport(context.Background(), testReport(), "alice")
	require.NoError(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

// Все 3 попытки 500 - возвращаем ошибку.
func TestTelegramService_AllRetriesFail(t *testing.T) {
	calls := int32(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newTestTelegramService(srv.URL, "t", "c")
	err := s.SendBugReport(context.Background(), testReport(), "alice")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "after 3 attempts")
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

// Проверка, что format содержит критичные поля.
func TestFormatBugReport_ContainsFields(t *testing.T) {
	r := testReport()
	text := formatBugReport(r, "alice")
	assert.Contains(t, text, r.BugHash)
	assert.Contains(t, text, r.Route)
	assert.Contains(t, text, "500")
	assert.Contains(t, text, "alice")
}

// Sanitize: backtick и звёздочки замещаются.
func TestSanitizeForMarkdown(t *testing.T) {
	out := sanitizeForMarkdown("hello `backtick` *star*")
	assert.NotContains(t, out, "`")
	assert.NotContains(t, out, "*")
}
