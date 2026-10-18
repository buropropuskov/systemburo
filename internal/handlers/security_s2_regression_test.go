package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"systemburo/internal/handlers"
	"systemburo/internal/middleware"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

func TestSecurityS2_BanCheckCurrentState(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	userID, _, remove := setupMWUser(t, db, true, false)
	defer remove()
	svc := services.NewBanCheckService(db, time.Hour)
	_, active, err := svc.Status(context.Background(), userID)
	require.NoError(t, err)
	require.True(t, active)
	for _, tc := range []struct {
		name, method, route, request string
		banned, active               bool
		actor, want                  int
	}{
		{"active baseline", "GET", "/api/applications", "/api/applications", false, true, userID, 204},
		{"banned cabinet", "GET", "/api/users/me", "/api/users/me", true, true, userID, 204},
		{"banned own object gate follows", "GET", "/api/applications/:id/details", "/api/applications/12/details", true, true, userID, 204},
		{"banned service read", "GET", "/api/applications", "/api/applications", true, true, userID, 403},
		{"banned files", "GET", "/api/applications/:id/files", "/api/applications/12/files", true, true, userID, 403},
		{"banned notifications", "GET", "/api/notifications", "/api/notifications", true, true, userID, 403},
		{"banned mutation", "POST", "/api/applications", "/api/applications", true, true, userID, 403},
		{"banned head", "HEAD", "/api/users/me", "/api/users/me", true, true, userID, 403},
		{"banned options", "OPTIONS", "/api/users/me", "/api/users/me", true, true, userID, 403},
		{"archive cabinet", "GET", "/api/users/me", "/api/users/me", false, false, userID, 403},
		{"archive super", "GET", "/api/applications", "/api/applications", false, false, userID, 403},
		{"archive logout", "POST", "/api/logout", "/api/logout", false, false, userID, 204},
		{"archive logout all", "POST", "/api/logout-all", "/api/logout-all", false, false, userID, 204},
		{"missing cabinet", "GET", "/api/users/me", "/api/users/me", false, true, -1, 403},
		{"missing logout", "POST", "/api/logout", "/api/logout", false, true, -1, 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{"is_active": tc.active, "is_banned": tc.banned}).Error)
			e := echo.New()
			called := false
			inject := func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("user_id", tc.actor); return next(c) }
			}
			e.Add(tc.method, tc.route, func(c echo.Context) error { called = true; return c.NoContent(http.StatusNoContent) }, inject, middleware.BanCheck(svc))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.request, nil))
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
			require.Equal(t, tc.want == http.StatusNoContent, called)
		})
	}
}

func TestSecurityS2_BannedOwnApplicationsOnly(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()
	// A fixed clock controls both the change and the comparison: no dependency on today's date.
	point := time.Date(2026, time.January, 12, 9, 0, 0, 0, time.UTC)
	status, confirmation, message, privateNote, approval := "Непрочитано", "Ожидает", "submitted fixture", "service-only fixture", "true"
	ownNumber, foreignNumber := "S2-OWN", "S2-ORG-PEER"
	own := models.Application{ApplicationNumber: &ownNumber, OrganizationID: w.orgID, SenderUserID: w.senderID,
		SendingDatetime: &point, StatusUpdatedAt: &point, Status: &status, Confirmation: &confirmation,
		Message: &message, ResponsibleUserID: &w.guardID, ResponsibleComment: &privateNote, BureauNote: &privateNote, DataApproval: &approval}
	foreign := models.Application{ApplicationNumber: &foreignNumber, OrganizationID: w.orgID, SenderUserID: w.guardID,
		SendingDatetime: &point, StatusUpdatedAt: &point, Status: &status, Confirmation: &confirmation}
	require.NoError(t, w.db.Create(&own).Error)
	require.NoError(t, w.db.Create(&foreign).Error)
	require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.guardID).Update("first_name", "HiddenReceiverFixture").Error)
	approvalNote := "HiddenApprovalFixture"
	require.NoError(t, w.db.Create(&models.ApplicationResponsibleUser{ApplicationID: own.ID, UserID: w.guardID, ApprovalComment: &approvalNote}).Error)

	baseline, err := w.svc.GetUserApplications(ctx, "sender1", services.ApplicationFilter{})
	require.NoError(t, err)
	require.Len(t, baseline, 2, "active organization scope is preserved")
	require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.senderID).Updates(map[string]interface{}{"is_banned": true, "is_super_admin": true}).Error)
	// Persisted approver assignment and super flag cannot expand a banned user's scope.
	require.NoError(t, w.db.Create(&models.ApplicationApprover{UserID: w.senderID}).Error)
	rows, err := w.svc.GetUserApplications(ctx, "sender1", services.ApplicationFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, own.ID, rows[0].ID)
	require.True(t, rows[0].HasStatusUpdate)
	require.Nil(t, rows[0].ResponsibleUserID)
	require.Nil(t, rows[0].ResponsibleComment)
	require.Empty(t, rows[0].ResponsibleName)
	paged, total, err := w.svc.GetUserApplicationsPaginated(ctx, "sender1", services.ApplicationFilter{}, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, paged, 1)
	require.Equal(t, own.ID, paged[0].ID)
	empty, total, err := w.svc.GetUserApplicationsPaginated(ctx, "sender1", services.ApplicationFilter{}, 2, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Empty(t, empty)
	count, err := w.svc.GetUserStatusUpdatesCount(ctx, "sender1", services.ApplicationFilter{})
	require.NoError(t, err)
	require.Equal(t, 1, count.StatusUpdates)
	allowed, err := services.CheckApplicationAccess(ctx, w.svc, foreign.ID, "sender1", true)
	require.NoError(t, err)
	require.False(t, allowed)

	for _, search := range []string{privateNote, "HiddenReceiverFixture", approvalNote, "submitted fixture"} {
		filter := services.ApplicationFilter{SearchQuery: &search}
		legacy, err := w.svc.GetUserApplications(ctx, "sender1", filter)
		require.NoError(t, err)
		page, total, err := w.svc.GetUserApplicationsPaginated(ctx, "sender1", filter, 1, 10)
		require.NoError(t, err)
		want := 0
		if search == "submitted fixture" {
			want = 1
		}
		require.Len(t, legacy, want)
		require.Len(t, page, want)
		require.EqualValues(t, want, total)
		statusCount, err := w.svc.GetUserStatusUpdatesCount(ctx, "sender1", filter)
		require.NoError(t, err)
		require.Equal(t, 1, statusCount.StatusUpdates, "status chip ignores search, consistently with active accounts")
	}

	// Exercise both actual HTTP detail handlers, including their incidental write hooks.
	e := echo.New()
	inject := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("username", "sender1")
			c.Set("user_id", w.senderID)
			c.Set("is_super_admin", true)
			return next(c)
		}
	}
	h := handlers.NewApplicationHandler(w.svc, services.NewPermissionResolver(w.db))
	e.GET("/api/applications/:id", h.GetApplicationByID, inject)
	e.GET("/api/applications/:id/details", h.GetApplicationDetails, inject)
	for _, suffix := range []string{"", "/details"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/applications/%d%s", own.ID, suffix), nil))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		body := testutil.ParseResponse[map[string]interface{}](t, rec)
		require.Equal(t, message, body["message"])
		for _, key := range []string{"responsible_user_id", "responsible_comment", "responsible_users", "bureau_note", "blacklist_flags_count", "files"} {
			require.NotContains(t, body, key)
		}
		other := httptest.NewRecorder()
		e.ServeHTTP(other, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/applications/%d%s", foreign.ID, suffix), nil))
		require.Equal(t, http.StatusForbidden, other.Code, other.Body.String())
	}
	var reads, views int64
	require.NoError(t, w.db.Table("application_reads").Where("application_id = ? AND user_id = ?", own.ID, w.senderID).Count(&reads).Error)
	require.NoError(t, w.db.Table("application_status_views").Where("application_id = ? AND user_id = ?", own.ID, w.senderID).Count(&views).Error)
	require.Zero(t, reads)
	require.Zero(t, views)
	var readsInHistory int64
	require.NoError(t, w.db.Table("audit_log").Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, own.ID, "read").Count(&readsInHistory).Error)
	require.Zero(t, readsInHistory)

	var after models.Application
	require.NoError(t, w.db.First(&after, own.ID).Error)
	require.Equal(t, status, *after.Status)
	require.Nil(t, after.ReadingDatetime)
}

func TestSecurityS2_FreshPermissionAfterPeerRevocation(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	userID, _, _, remove := seedRoleGroupUser(t, db)
	defer remove()
	ctx := context.Background()
	peer := services.NewPermissionResolver(db)
	cached, err := peer.Resolve(ctx, userID)
	require.NoError(t, err)
	require.True(t, cached.Has("page.center"))
	// A different backend writes the authoritative deny without invalidating peer's memory.
	require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: userID, PermissionKey: "page.center", Value: "deny"}).Error)
	cached, err = peer.Resolve(ctx, userID)
	require.NoError(t, err)
	require.True(t, cached.Has("page.center"), "test must actually retain the stale reference cache")
	allowed, err := peer.HasPermission(ctx, userID, "page.center")
	require.NoError(t, err)
	require.False(t, allowed)
	e := echo.New()
	called := false
	inject := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("user_id", userID); return next(c) }
	}
	e.GET("/protected", func(c echo.Context) error { called = true; return c.NoContent(http.StatusNoContent) }, inject,
		middleware.RequirePermissionV2(peer, services.NewAccessDenialService(db), "page.center"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/protected", nil))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	require.False(t, called)
}

func TestSecurityS2_ArchivedScopeAndRegistries(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()
	resolver := services.NewPermissionResolver(w.db)
	scopes := services.NewElementScopeResolver(w.db, resolver)
	scope, err := scopes.Resolve(ctx, w.senderID)
	require.NoError(t, err)
	require.Equal(t, w.orgID, scope.OrgID)
	cars := services.NewUniqueCarService(w.db)
	employees := services.NewUniqueEmployeeService(w.db)
	_, err = cars.GetOwnerInfo(ctx, "sender1")
	require.NoError(t, err)
	_, err = employees.GetOwnerInfo(ctx, "sender1")
	require.NoError(t, err)
	for _, state := range []string{"archive", "ban"} {
		t.Run(state, func(t *testing.T) {
			require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.senderID).Updates(map[string]interface{}{
				"is_active": state != "archive", "is_banned": state == "ban", "is_admin": true,
			}).Error)
			got, err := scopes.Resolve(ctx, w.senderID)
			require.NoError(t, err)
			require.Equal(t, services.ElementScope{}, got)
			_, err = cars.GetAll(ctx, "sender1", "all_system")
			s2RequireHTTPStatus(t, err, http.StatusForbidden)
			_, err = employees.GetAll(ctx, "sender1", "all_system")
			s2RequireHTTPStatus(t, err, http.StatusForbidden)
		})
	}
}

func s2RequireHTTPStatus(t *testing.T, err error, status int) {
	t.Helper()
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, status, httpErr.Code)
}

// A healthy BanCheck must not mask a later permission-source failure as 401 or 500.
func TestSecurityS2_ProtectiveDBErrorsAfterBanCheck(t *testing.T) {
	w := setupSecurityWorld(t)
	failed := s2ClosedDB(t)
	guides := handlers.NewGuideHandler(services.NewGuideService(failed, services.NewPermissionResolver(failed)), nil, 0)
	e := echo.New()
	e.HTTPErrorHandler = handlers.CustomHTTPErrorHandler
	passedBan := 0
	inject := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("user_id", w.senderID); return next(c) }
	}
	observed := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { passedBan++; return next(c) }
	}
	e.GET("/api/guide/sections", guides.ListSections, inject, middleware.BanCheck(services.NewBanCheckService(w.db, time.Hour)), observed)
	e.GET("/api/guide/sections/:role/download", guides.Download, inject, middleware.BanCheck(services.NewBanCheckService(w.db, time.Hour)), observed)
	for _, path := range []string{"/api/guide/sections", "/api/guide/sections/user/download"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
		require.NotContains(t, rec.Body.String(), "postgres")
		require.NotContains(t, rec.Body.String(), "sql:")
	}
	require.Equal(t, 2, passedBan)
	ctx := context.Background()
	_, err := services.NewUniqueCarService(failed).GetOwnerInfo(ctx, "sender1")
	s2RequireHTTPStatus(t, err, http.StatusServiceUnavailable)
	_, err = services.NewUniqueEmployeeService(failed).GetOwnerInfo(ctx, "sender1")
	s2RequireHTTPStatus(t, err, http.StatusServiceUnavailable)
	_, err = services.NewElementScopeResolver(failed, services.NewPermissionResolver(failed)).Resolve(ctx, w.senderID)
	s2RequireHTTPStatus(t, err, http.StatusServiceUnavailable)
	apps := services.NewApplicationService(failed, nil, nil, nil, nil, services.NewAuditRecorder(failed))
	allowed, err := services.CheckApplicationAccess(ctx, apps, 1, "sender1", true)
	require.False(t, allowed)
	s2RequireHTTPStatus(t, err, http.StatusServiceUnavailable)
	require.False(t, apps.CanAccessApplication(ctx, 1, "sender1", true), "legacy error contract is fail-closed")
}

func s2ClosedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "postgres://nobody:nobody@127.0.0.1:1/nonexistent"}),
		&gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, pool.Close())
	return db
}

func TestSecurityS2_AvailableScopeRejectsRevokedJWTSuper(t *testing.T) {
	w := setupSecurityWorld(t)
	require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.senderID).Update("is_super_admin", true).Error)
	resolver := services.NewPermissionResolver(w.db)
	set, err := resolver.Resolve(context.Background(), w.senderID)
	require.NoError(t, err)
	require.True(t, set.IsSuperAdmin(), "warm the old peer cache")
	e := echo.New()
	inject := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("user_id", w.senderID)
			c.Set("username", "sender1")
			c.Set("is_super_admin", true)
			return next(c)
		}
	}
	h := handlers.NewApplicationHandler(w.svc, resolver)
	e.GET("/api/applications/available-attachments", h.GetAvailableAttachments, inject)
	baseline := httptest.NewRecorder()
	e.ServeHTTP(baseline, httptest.NewRequest(http.MethodGet, "/api/applications/available-attachments", nil))
	require.Equal(t, http.StatusOK, baseline.Code, baseline.Body.String())
	require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.senderID).Update("is_super_admin", false).Error)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/applications/available-attachments", nil))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

type securityS2ActorLockKey struct{}

func TestSecurityS2_DetailPinsActorModeThroughProjection(t *testing.T) {
	w := setupSecurityWorld(t)
	point := time.Date(2026, time.January, 12, 9, 0, 0, 0, time.UTC)
	status := "В обработке"
	number := "S2-LOCK"
	app := models.Application{ApplicationNumber: &number, OrganizationID: w.orgID, SenderUserID: w.senderID, SendingDatetime: &point, Status: &status}
	require.NoError(t, w.db.Create(&app).Error)
	observed := false
	var lockErr error
	callback := "security_s2_actor_mode_lock"
	require.NoError(t, w.db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if observed || tx.Statement.Table != "users" || tx.Statement.Context.Value(securityS2ActorLockKey{}) != true {
			return
		}
		observed = true
		locking, ok := tx.Statement.Clauses["FOR"].Expression.(clause.Locking)
		require.True(t, ok, "detail must choose projection under an actor row lock")
		require.Equal(t, "SHARE", locking.Strength)
		// A peer ban must wait for this read decision. A DB lock timeout provides a
		// deterministic barrier; there is no goroutine scheduling or wall-clock sleep.
		lockErr = w.db.Transaction(func(peer *gorm.DB) error {
			if err := peer.Exec("SET LOCAL lock_timeout = '100ms'").Error; err != nil {
				return err
			}
			return peer.Model(&models.User{}).Where("id = ?", w.senderID).Update("is_banned", true).Error
		})
	}))
	defer w.db.Callback().Query().Remove(callback)
	ctx := context.WithValue(context.Background(), securityS2ActorLockKey{}, true)
	_, err := w.svc.GetApplicationByID(ctx, "sender1", app.ID, false)
	require.NoError(t, err)
	require.True(t, observed)
	require.Error(t, lockErr, "peer must not switch actor mode while full projection is being read")
	require.Contains(t, lockErr.Error(), "55P03", "failure must be a row-lock timeout, not an unrelated query failure")
	require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.senderID).Update("is_banned", true).Error)
	safe, err := w.svc.GetApplicationByID(context.Background(), "sender1", app.ID, false)
	require.NoError(t, err)
	require.NotContains(t, safe, "responsible_users")
}
