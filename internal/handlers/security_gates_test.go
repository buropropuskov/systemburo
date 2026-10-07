package handlers_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mw "systemburo/internal/middleware"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type securityGateWorld struct {
	e                      *echo.Echo
	db                     *gorm.DB
	gates                  *testutil.SecurityGateServices
	admin, user, refresh   string
	adminID, userID, orgID int
}

func newSecurityGateWorld(t *testing.T) securityGateWorld {
	t.Helper()
	e, db, gates, cleanup := testutil.SetupTestAppWithAllGates(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	for _, path := range []string{gates.UploadDir, gates.ArchiveDir} {
		require.NoError(t, os.MkdirAll(path, 0700))
	}
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "security_gate_user", testPassword, 1, td.OrgID, td.CompanyID)
	user, refresh := testutil.LoginUser(t, e, "security_gate_user", testPassword)
	return securityGateWorld{e, db, gates, admin, user, refresh,
		getUserID(t, db, "testadmin"), getUserID(t, db, "security_gate_user"), td.OrgID}
}

func (w securityGateWorld) change(t *testing.T, state string) {
	t.Helper()
	ctx := context.Background()
	switch state {
	case "maintenance":
		require.NoError(t, w.gates.Maintenance.Enable(ctx, w.adminID, "testadmin", services.MaintenanceParams{}))
	case "ban":
		require.NoError(t, w.gates.Bans.Ban(ctx, w.userID, w.adminID, "test gate"))
	case "archive":
		require.NoError(t, w.gates.Users.Delete(ctx, w.adminID, "security_gate_user"))
	case "consent":
		enableConsentSettings(t, w.e, w.admin, "<p>Test consent version</p>")
	case "password":
		setPasswordFlag(t, w.db, "security_gate_user", true)
	default:
		t.Fatalf("unknown state %q", state)
	}
}

func securityGateRequest(e *echo.Echo, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func assertSecurityGateDenied(t *testing.T, rec *httptest.ResponseRecorder, state string) {
	t.Helper()
	status, marker := http.StatusForbidden, ""
	switch state {
	case "maintenance":
		status, marker = http.StatusServiceUnavailable, "технические работы"
	case "ban":
		marker = "Учётная запись заблокирована"
	case "archive":
		marker = "Учётная запись отключена"
	case "consent":
		marker = `"consent_required":true`
		require.Equal(t, "1", rec.Header().Get("X-PD-Consent-Required"))
	case "password":
		marker = `"code":"PASSWORD_CHANGE_REQUIRED"`
		require.Equal(t, "1", rec.Header().Get("X-Password-Change-Required"))
	}
	require.Equal(t, status, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), marker)
}

// Снимок бизнес-данных и содержимого файлов. Аудит отказов и last_seen не входят:
// их запись не является выполнением запрещённого бизнес-запроса.
func (w securityGateWorld) snapshot(t *testing.T) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, table := range []string{"applications", "attachments", "employees", "cars", "application_files", "organizations", "companies"} {
		var rows string
		query := fmt.Sprintf("SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.id), '[]'::jsonb)::text FROM %s x", table)
		require.NoError(t, w.db.Raw(query).Scan(&rows).Error)
		result[table] = rows
	}
	var users string
	require.NoError(t, w.db.Raw(`SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.id), '[]'::jsonb)::text FROM
        (SELECT id, username, theme, is_active, is_banned, must_change_password FROM users) x`).Scan(&users).Error)
	result["users"] = users
	for _, root := range []string{w.gates.UploadDir, w.gates.ArchiveDir} {
		require.NoError(t, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			result[path] = fmt.Sprintf("%x", sha256.Sum256(data))
			return nil
		}))
	}
	return result
}

func TestSecurityGates_RouteMatrix(t *testing.T) {
	for _, state := range []string{"maintenance", "ban", "archive", "consent", "password"} {
		t.Run(state, func(t *testing.T) {
			w := newSecurityGateWorld(t)
			// Валидная мутация до перехода, тот же запрос после него.
			body := `{"theme":"dark"}`
			rec := securityGateRequest(w.e, "PUT", "/api/users/me/theme", body, w.user)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			// Прогреваем реальные кэши до штатного изменения состояния.
			require.Equal(t, http.StatusOK, testutil.GET(t, w.e, "/users/me", testutil.AuthHeader(w.user)).Code)
			w.change(t, state)
			before := w.snapshot(t)
			rec = securityGateRequest(w.e, "PUT", "/api/users/me/theme", body, w.user)
			assertSecurityGateDenied(t, rec, state)
			count := 0
			for _, key := range securityProtectedRoutes(t, w.e) {
				method, path, _ := strings.Cut(key, " ")
				safe := method == "GET" || method == "HEAD" || method == "OPTIONS"
				if (state == "ban" || state == "archive") && safe {
					continue
				}
				if state == "consent" && mw.PDConsentWhitelist[key] {
					continue
				}
				if state == "password" && mw.MustChangePasswordWhitelist[key] {
					continue
				}
				t.Run(key, func(t *testing.T) {
					rec := securityGateRequest(w.e, method, concreteRoutePath(path, w.userID), `{}`, w.user)
					assertSecurityGateDenied(t, rec, state)
				})
				count++
			}
			require.Equal(t, before, w.snapshot(t), "отклонённые запросы изменили бизнес-данные или файлы")
			t.Logf("%s: %d blocked registered routes", state, count)
		})
	}
}

func TestSecurityGates_CacheAndReadOnlyCabinet(t *testing.T) {
	for _, state := range []string{"ban", "archive", "maintenance"} {
		t.Run(state, func(t *testing.T) {
			w := newSecurityGateWorld(t)
			testutil.GrantPermission(t, w.userID, "page.admin.file_archive")
			require.Equal(t, http.StatusOK, testutil.GET(t, w.e, "/file-archive/settings", testutil.AuthHeader(w.user)).Code)
			w.change(t, state)
			rec := testutil.GET(t, w.e, "/users/me", testutil.AuthHeader(w.user))
			if state == "maintenance" {
				assertSecurityGateDenied(t, rec, state)
			} else {
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				rec = testutil.GET(t, w.e, "/file-archive/settings", testutil.AuthHeader(w.user))
				require.Equal(t, http.StatusForbidden, rec.Code, "заблокированный пользователь сохранил права: %s", rec.Body.String())
			}
			ctx := context.Background()
			switch state {
			case "ban":
				require.NoError(t, w.gates.Bans.Unban(ctx, w.userID, w.adminID))
			case "archive":
				require.NoError(t, w.gates.Users.Restore(ctx, w.adminID, "security_gate_user"))
			case "maintenance":
				require.NoError(t, w.gates.Maintenance.Disable(ctx, w.adminID, "testadmin"))
			}
			rec = testutil.PUT(t, w.e, "/users/me/theme", `{"theme":"light"}`, testutil.AuthHeader(w.user))
			require.Equal(t, http.StatusOK, rec.Code, "кэш не инвалидирован: %s", rec.Body.String())
		})
	}
}

func TestSecurityGates_SuperAdminAndOrder(t *testing.T) {
	w := newSecurityGateWorld(t)
	w.change(t, "consent")
	w.change(t, "password")
	rec := testutil.GET(t, w.e, "/citizenships", testutil.AuthHeader(w.user))
	assertSecurityGateDenied(t, rec, "consent")
	w.change(t, "ban")
	rec = testutil.POST(t, w.e, "/events/ticket", `{}`, testutil.AuthHeader(w.user))
	assertSecurityGateDenied(t, rec, "ban")
	w.change(t, "maintenance")
	rec = testutil.POST(t, w.e, "/events/ticket", `{}`, testutil.AuthHeader(w.user))
	assertSecurityGateDenied(t, rec, "maintenance")
	// Super-admin обходит maintenance/consent, но не обязательную смену пароля.
	rec = testutil.GET(t, w.e, "/citizenships", testutil.AuthHeader(w.admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	setPasswordFlag(t, w.db, "testadmin", true)
	rec = testutil.GET(t, w.e, "/citizenships", testutil.AuthHeader(w.admin))
	assertSecurityGateDenied(t, rec, "password")
	rec = testutil.GET(t, w.e, "/users/me", testutil.AuthHeader(w.admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var user models.User
	require.NoError(t, w.db.First(&user, w.userID).Error)
	require.True(t, user.IsBanned)
}
