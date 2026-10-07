package handlers_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/realtime"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestSecurityGates_RefreshRevocation(t *testing.T) {
	for _, state := range []string{"ban", "archive", "maintenance"} {
		t.Run(state, func(t *testing.T) {
			w := newSecurityGateWorld(t)
			refresh := func(cookie string) *httptest.ResponseRecorder {
				h := http.Header{}
				h.Set("Cookie", "refresh_token="+cookie)
				return testutil.POST(t, w.e, "/refresh-token", `{}`, h)
			}
			rec := refresh(w.refresh)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			current := ""
			for _, cookie := range rec.Result().Cookies() {
				if cookie.Name == "refresh_token" {
					current = cookie.Value
				}
			}
			require.NotEmpty(t, current)
			w.change(t, state)
			for _, cookie := range []string{current, w.refresh} {
				rec = refresh(cookie)
				want := http.StatusUnauthorized
				if state == "archive" {
					want = http.StatusForbidden
				}
				require.Equal(t, want, rec.Code, rec.Body.String())
				var failure map[string]any
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &failure))
				require.Equal(t, false, failure["success"])
				require.Nil(t, failure["data"])
				for _, next := range rec.Result().Cookies() {
					require.True(t, next.Name != "refresh_token" || next.Value == "", "отказ создал сессию")
				}
			}
			var active int64
			require.NoError(t, w.db.Model(&models.RefreshToken{}).Where("user_id = ? AND is_revoked = false", w.userID).Count(&active).Error)
			require.Zero(t, active)
		})
	}
}

// Reader привязан к HTTP-запросу с deadline: чтение оборвётся вместе с ним.
func securityReadFrame(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	var frame strings.Builder
	for {
		line, err := reader.ReadString('\n')
		require.NoError(t, err)
		if line == "\n" || line == "\r\n" {
			return frame.String()
		}
		frame.WriteString(line)
	}
}

func TestSecurityGates_SSETicketLifecycle(t *testing.T) {
	w := newSecurityGateWorld(t)
	rec := testutil.POST(t, w.e, "/events/ticket", `{}`, testutil.AuthHeader(w.user))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	ticket, ok := testutil.ParseMap(t, rec)["ticket"].(string)
	require.True(t, ok)
	require.NotEmpty(t, ticket)
	server := httptest.NewServer(w.e)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/events?ticket="+ticket, nil)
	require.NoError(t, err)
	response, err := server.Client().Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	reader := bufio.NewReader(response.Body)
	require.Contains(t, securityReadFrame(t, reader), ": connected")
	w.gates.Events.Publish(w.userID, realtime.Event{Type: "test.signal", Scope: fmt.Sprintf("user:%d", w.userID)})
	require.Contains(t, securityReadFrame(t, reader), "test.signal")
	rec = testutil.GET(t, w.e, "/events?ticket="+ticket, nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	w.change(t, "ban")
	rec = testutil.POST(t, w.e, "/events/ticket", `{}`, testutil.AuthHeader(w.user))
	assertSecurityGateDenied(t, rec, "ban")
	// Текущий контракт допускает жизнь уже открытого потока до переподключения.
	// Проверяем реальную доставку сигнала блокировки от штатного сервиса.
	require.Contains(t, securityReadFrame(t, reader), "user.banned")
}

func securityGrantArchive(t *testing.T, w securityGateWorld) {
	t.Helper()
	for _, key := range []string{"page.admin.file_archive", "action.download.file_archive", services.KeyDetailDocuments, services.KeyDetailDocumentsExport} {
		testutil.GrantPermission(t, w.userID, key)
	}
}

func securityArchiveTicket(t *testing.T, w securityGateWorld, date string) string {
	t.Helper()
	rec := testutil.POST(t, w.e, "/file-archive/download-ticket",
		fmt.Sprintf(`{"date_from":%q,"date_to":%q}`, date, date), testutil.AuthHeader(w.user))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return testutil.ParseResponse[models.ArchiveDownloadTicketResponse](t, rec).Ticket
}

func TestSecurityGates_ArchiveTicketLifecycle(t *testing.T) {
	w := newSecurityGateWorld(t)
	securityGrantArchive(t, w)
	date := time.Now().Format("2006-01-02")
	ticket := securityArchiveTicket(t, w, date)
	require.NotEmpty(t, ticket)
	rec := testutil.GET(t, w.e, "/file-archive/download?ticket="+ticket, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "application/zip", rec.Header().Get("Content-Type"))
	rec = testutil.GET(t, w.e, "/file-archive/download?ticket="+ticket, nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	ticket = securityArchiveTicket(t, w, date)
	// Время передаётся настоящему потребителю; ожидание TTL в минуту не нужно.
	_, _, _, err := w.gates.Downloads.ConsumeAndCollect(context.Background(), ticket, time.Now().Add(2*time.Minute))
	require.Error(t, err)
	rec = testutil.GET(t, w.e, "/file-archive/download?ticket="+ticket, nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
