package handlers_test

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

// Ban/archive проверяются при использовании старого билета. Остальные гейты
// сохраняют текущий контракт: новая выдача ограничена, старый билет действителен.
func TestSecurityGates_ArchiveTicketStateTransitions(t *testing.T) {
	for _, state := range []string{"ban", "archive", "maintenance", "consent", "password"} {
		t.Run(state, func(t *testing.T) {
			w := newSecurityGateWorld(t)
			securityGrantArchive(t, w)
			date := time.Now().Format("2006-01-02")
			bucket, err := time.Parse("2006-01-02", date)
			require.NoError(t, err)
			const name = "security-gate-fixture.txt"
			const contents = "synthetic archive fixture"
			require.NoError(t, os.WriteFile(filepath.Join(w.gates.ArchiveDir, name), []byte(contents), 0600))
			require.NoError(t, w.db.Create(&models.BlankExport{
				ApplicationID: 1, AttachmentID: 1, BucketDate: bucket,
				FileName: name, SizeBytes: int64(len(contents)), Status: models.BlankExportOK,
				QueuedAt: time.Now(),
			}).Error)
			ticket := securityArchiveTicket(t, w, date)
			w.change(t, state)
			// Новая выдача уже закрыта соответствующим protected-гейтом.
			rec := testutil.POST(t, w.e, "/file-archive/download-ticket",
				`{"date_from":"`+date+`","date_to":"`+date+`"}`, testutil.AuthHeader(w.user))
			assertSecurityGateDenied(t, rec, state)
			rec = testutil.GET(t, w.e, "/file-archive/download?ticket="+ticket, nil)
			if state == "ban" || state == "archive" {
				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				require.NotContains(t, rec.Body.String(), contents)
				require.NotContains(t, rec.Header().Get("Content-Type"), "application/zip")
			} else {
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
				require.NoError(t, err)
				require.Len(t, zr.File, 1)
				require.Equal(t, name, zr.File[0].Name)
				r, err := zr.File[0].Open()
				require.NoError(t, err)
				data, err := io.ReadAll(r)
				require.NoError(t, r.Close())
				require.NoError(t, err)
				require.Equal(t, contents, string(data))
			}
			rec = testutil.GET(t, w.e, "/file-archive/download?ticket="+ticket, nil)
			require.Equal(t, http.StatusUnauthorized, rec.Code)
			if state == "ban" || state == "archive" {
				if state == "ban" {
					require.NoError(t, w.gates.Bans.Unban(context.Background(), w.userID, w.adminID))
				} else {
					require.NoError(t, w.gates.Users.Restore(context.Background(), w.adminID, "security_gate_user"))
				}
				// Восстановление допускает новый билет, но не оживляет погашенный.
				restoredTicket := securityArchiveTicket(t, w, date)
				rec = testutil.GET(t, w.e, "/file-archive/download?ticket="+restoredTicket, nil)
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestSecurityGates_SSETicketStateTransitions(t *testing.T) {
	for _, state := range []string{"ban", "archive", "maintenance", "consent", "password"} {
		t.Run(state, func(t *testing.T) {
			w := newSecurityGateWorld(t)
			rec := testutil.POST(t, w.e, "/events/ticket", `{}`, testutil.AuthHeader(w.user))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			ticket, ok := testutil.ParseMap(t, rec)["ticket"].(string)
			require.True(t, ok)
			require.NotEmpty(t, ticket)
			w.change(t, state)
			rec = testutil.POST(t, w.e, "/events/ticket", `{}`, testutil.AuthHeader(w.user))
			if state == "consent" {
				// Штатный whitelist разрешает SSE для уведомлений окна согласия.
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			} else {
				assertSecurityGateDenied(t, rec, state)
			}
			server := httptest.NewServer(w.e)
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/events?ticket="+ticket, nil)
			require.NoError(t, err)
			response, err := server.Client().Do(req)
			require.NoError(t, err)
			defer response.Body.Close()
			if state == "ban" || state == "archive" {
				require.Equal(t, http.StatusForbidden, response.StatusCode)
				body, err := io.ReadAll(response.Body)
				require.NoError(t, err)
				require.NotContains(t, string(body), ": connected")
				require.NotContains(t, response.Header.Get("Content-Type"), "text/event-stream")
				rec = testutil.GET(t, w.e, "/events?ticket="+ticket, nil)
				require.Equal(t, http.StatusUnauthorized, rec.Code)
				if state == "ban" {
					require.NoError(t, w.gates.Bans.Unban(context.Background(), w.userID, w.adminID))
				} else {
					require.NoError(t, w.gates.Users.Restore(context.Background(), w.adminID, "security_gate_user"))
				}
				rec = testutil.POST(t, w.e, "/events/ticket", `{}`, testutil.AuthHeader(w.user))
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				restoredTicket, ok := testutil.ParseMap(t, rec)["ticket"].(string)
				require.True(t, ok)
				req, err = http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/events?ticket="+restoredTicket, nil)
				require.NoError(t, err)
				restoredResponse, err := server.Client().Do(req)
				require.NoError(t, err)
				defer restoredResponse.Body.Close()
				require.Equal(t, http.StatusOK, restoredResponse.StatusCode)
				require.Contains(t, securityReadFrame(t, bufio.NewReader(restoredResponse.Body)), ": connected")
			} else {
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.Contains(t, securityReadFrame(t, bufio.NewReader(response.Body)), ": connected")
			}
		})
	}
}
