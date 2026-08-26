package handlers_test

// Интеграционные тесты для RequirePermissionV2 middleware и UserBanService.
// Лежат в handlers_test чтобы не race-иться с CleanDB других пакетов
// (см. permission_resolver_integration_test.go).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/middleware"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// uniqMW -- собственный счётчик для middleware-тестов, изолирован от resolver-тестов.
var uniqMW = func() func(string) string {
	var n int64
	return func(prefix string) string {
		n++
		return prefix + "-mw-" + time.Now().Format("150405") + "-" + intStr(int(n))
	}
}()

func intStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func setupMWUser(t *testing.T, db *gorm.DB, isSuperAdmin, isBanned bool) (int, models.UserType, func()) {
	t.Helper()
	ut := models.UserType{Name: uniqMW("type"), Code: uniqMW("c")}
	if err := db.Create(&ut).Error; err != nil {
		t.Fatalf("create type: %v", err)
	}
	user := models.User{
		Username:     uniqMW("user"),
		Password:     "x",
		TypeID:       ut.ID,
		IsSuperAdmin: isSuperAdmin,
		IsBanned:     isBanned,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user.ID, ut, func() {
		db.Delete(&user)
		db.Delete(&ut)
	}
}

func TestRequirePermissionV2_AllowsForSuperAdmin(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	resolver := services.NewPermissionResolver(db)
	denials := services.NewAccessDenialService(db)

	userID, _, cleanup := setupMWUser(t, db, true, false)
	defer cleanup()

	e := echo.New()
	e.GET("/test", func(c echo.Context) error { return c.String(http.StatusOK, "ok") },
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Set("user_id", userID)
				return next(c)
			}
		},
		middleware.RequirePermissionV2(resolver, denials, "any.key"),
	)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for super-admin, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestRequirePermissionV2_DeniesAndLogsForRegularUser(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	resolver := services.NewPermissionResolver(db)
	denials := services.NewAccessDenialService(db)

	userID, _, cleanup := setupMWUser(t, db, false, false)
	defer cleanup()

	e := echo.New()
	e.GET("/test", func(c echo.Context) error { return c.String(http.StatusOK, "ok") },
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Set("user_id", userID)
				return next(c)
			}
		},
		middleware.RequirePermissionV2(resolver, denials, "page.center"),
	)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["banned"] != false && body["banned"] != nil {
		t.Errorf("expected banned=false in response, got %v", body["banned"])
	}

	// Лог пишется асинхронно: ждём недолго и проверяем.
	time.Sleep(100 * time.Millisecond)
	var count int64
	db.Model(&models.AccessDenial{}).
		Where("user_id = ? AND reason = ?", userID, models.DenialReasonPermission).
		Count(&count)
	if count == 0 {
		t.Errorf("expected denial log entry, got none")
	}
	defer db.Where("user_id = ?", userID).Delete(&models.AccessDenial{})
}

func TestRequirePermissionV2_LogsBannedReason(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	resolver := services.NewPermissionResolver(db)
	denials := services.NewAccessDenialService(db)

	userID, _, cleanup := setupMWUser(t, db, false, true)
	defer cleanup()

	e := echo.New()
	e.GET("/test", func(c echo.Context) error { return c.String(http.StatusOK, "ok") },
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Set("user_id", userID)
				return next(c)
			}
		},
		middleware.RequirePermissionV2(resolver, denials, "page.center"),
	)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}

	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["banned"] != true {
		t.Errorf("expected banned=true, got %v", body["banned"])
	}

	time.Sleep(100 * time.Millisecond)
	var count int64
	db.Model(&models.AccessDenial{}).
		Where("user_id = ? AND reason = ?", userID, models.DenialReasonBanned).
		Count(&count)
	if count == 0 {
		t.Errorf("expected banned-reason denial log entry")
	}
	defer db.Where("user_id = ?", userID).Delete(&models.AccessDenial{})
}

func TestUserBanService_BanRevokesRefreshTokens(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	resolver := services.NewPermissionResolver(db)
	banSvc := services.NewUserBanService(db, resolver, nil, services.NewAuditRecorder(db))

	targetID, _, cleanup := setupMWUser(t, db, false, false)
	defer cleanup()
	actorID, _, cleanupActor := setupMWUser(t, db, true, false)
	defer cleanupActor()

	rt := models.RefreshToken{
		UserID:    targetID,
		FamilyID:  uniqMW("fam"),
		TokenHash: uniqMW("hash"),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := db.Create(&rt).Error; err != nil {
		t.Fatalf("create rt: %v", err)
	}
	defer db.Delete(&rt)

	if err := banSvc.Ban(context.Background(), targetID, actorID, ""); err != nil {
		t.Fatalf("ban: %v", err)
	}

	var revoked bool
	db.Model(&models.RefreshToken{}).Select("is_revoked").Where("id = ?", rt.ID).Row().Scan(&revoked)
	if !revoked {
		t.Errorf("expected refresh token revoked after ban")
	}

	var u models.User
	db.Select("is_banned").First(&u, targetID)
	if !u.IsBanned {
		t.Errorf("expected user marked banned")
	}
}

func TestUserBanService_CannotBanSelf(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	resolver := services.NewPermissionResolver(db)
	banSvc := services.NewUserBanService(db, resolver, nil, services.NewAuditRecorder(db))

	userID, _, cleanup := setupMWUser(t, db, true, false)
	defer cleanup()

	err := banSvc.Ban(context.Background(), userID, userID, "")
	if err == nil {
		t.Error("expected error when banning self")
	}
}

func TestUserBanService_CannotBanSuperAdmin(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	resolver := services.NewPermissionResolver(db)
	banSvc := services.NewUserBanService(db, resolver, nil, services.NewAuditRecorder(db))

	targetID, _, cleanup := setupMWUser(t, db, true, false)
	defer cleanup()
	actorID, _, cleanupActor := setupMWUser(t, db, true, false)
	defer cleanupActor()

	err := banSvc.Ban(context.Background(), targetID, actorID, "")
	if err == nil {
		t.Error("expected error when banning super-admin")
	}
}

func TestBanCheck_AllowsActiveUser(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewBanCheckService(db, time.Minute)

	userID, _, cleanup := setupMWUser(t, db, false, false)
	defer cleanup()

	e := echo.New()
	e.GET("/test", func(c echo.Context) error { return c.String(http.StatusOK, "ok") },
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Set("user_id", userID)
				return next(c)
			}
		},
		middleware.BanCheck(svc),
	)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for active user, got %d", rec.Code)
	}
}

// banCheckHarness строит echo с GET и POST /test под BanCheck для заданного юзера.
func banCheckHarness(userID int, svc *services.BanCheckService) *echo.Echo {
	e := echo.New()
	inject := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("user_id", userID)
			return next(c)
		}
	}
	ok := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }
	e.GET("/test", ok, inject, middleware.BanCheck(svc))
	e.POST("/test", ok, inject, middleware.BanCheck(svc))
	return e
}

func TestBanCheck_DeniesBannedUser(t *testing.T) {
	// Забаненный: чтение (GET) проходит read-only, мутация (POST) -- 403.
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewBanCheckService(db, time.Minute)

	userID, _, cleanup := setupMWUser(t, db, false, true)
	defer cleanup()

	e := banCheckHarness(userID, svc)

	recGet := httptest.NewRecorder()
	e.ServeHTTP(recGet, httptest.NewRequest(http.MethodGet, "/test", nil))
	if recGet.Code != http.StatusOK {
		t.Errorf("expected 200 GET for banned user (read-only), got %d (body: %s)", recGet.Code, recGet.Body.String())
	}

	recPost := httptest.NewRecorder()
	e.ServeHTTP(recPost, httptest.NewRequest(http.MethodPost, "/test", nil))
	if recPost.Code != http.StatusForbidden {
		t.Errorf("expected 403 POST for banned user, got %d (body: %s)", recPost.Code, recPost.Body.String())
	}
}

func TestBanCheck_DeniesArchivedUser(t *testing.T) {
	// Архивный (is_active=false) с живым access-токеном: read-only, мутации -- 403.
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewBanCheckService(db, time.Minute)

	userID, _, cleanup := setupMWUser(t, db, false, false)
	defer cleanup()
	if err := db.Model(&models.User{}).Where("id = ?", userID).Update("is_active", false).Error; err != nil {
		t.Fatalf("archive user: %v", err)
	}

	e := banCheckHarness(userID, svc)

	recGet := httptest.NewRecorder()
	e.ServeHTTP(recGet, httptest.NewRequest(http.MethodGet, "/test", nil))
	if recGet.Code != http.StatusOK {
		t.Errorf("expected 200 GET for archived user (read-only), got %d (body: %s)", recGet.Code, recGet.Body.String())
	}

	recPost := httptest.NewRecorder()
	e.ServeHTTP(recPost, httptest.NewRequest(http.MethodPost, "/test", nil))
	if recPost.Code != http.StatusForbidden {
		t.Errorf("expected 403 POST for archived user, got %d (body: %s)", recPost.Code, recPost.Body.String())
	}
}

func TestBanCheck_InvalidationAfterBanReflectsImmediately(t *testing.T) {
	// Регресс на issue #271: после Ban() кэш ban-чекера должен инвалидироваться,
	// чтобы следующий же запрос забаненного юзера получил 403 без ожидания TTL.
	_, db, _ := testutil.SetupTestApp(t)
	banCache := services.NewBanCheckService(db, time.Hour)
	resolver := services.NewPermissionResolver(db)
	banSvc := services.NewUserBanService(db, resolver, banCache, services.NewAuditRecorder(db))

	targetID, _, cleanup := setupMWUser(t, db, false, false)
	defer cleanup()
	actorID, _, cleanupActor := setupMWUser(t, db, true, false)
	defer cleanupActor()

	// 1. До бана IsBanned=false, заполняется кэш.
	banned, err := banCache.IsBanned(context.Background(), targetID)
	if err != nil {
		t.Fatalf("ban check: %v", err)
	}
	if banned {
		t.Fatalf("expected not banned before Ban()")
	}

	// 2. Баним - должен инвалидировать кэш.
	if err := banSvc.Ban(context.Background(), targetID, actorID, ""); err != nil {
		t.Fatalf("ban: %v", err)
	}

	// 3. Сразу читаем - должен быть true (кэш сброшен, идём в БД).
	banned, err = banCache.IsBanned(context.Background(), targetID)
	if err != nil {
		t.Fatalf("ban check after ban: %v", err)
	}
	if !banned {
		t.Errorf("expected banned=true after Ban() even with long TTL")
	}
}

func TestBanCheck_InvalidationAfterArchiveReflectsImmediately(t *testing.T) {
	// Архив через userService должен инвалидировать кэш блокировок, чтобы
	// следующий же запрос архивного юзера дал 403 без ожидания TTL.
	_, db, _ := testutil.SetupTestApp(t)
	banCache := services.NewBanCheckService(db, time.Hour)
	userSvc := services.NewUserService(db, nil)
	userSvc.SetBanCache(banCache)

	targetID, _, cleanup := setupMWUser(t, db, false, false)
	defer cleanup()
	var target models.User
	if err := db.First(&target, targetID).Error; err != nil {
		t.Fatalf("load target: %v", err)
	}

	var adminTypeID int
	if err := db.Table("user_types").Select("id").Where("code = ?", "buropropuskov").Row().Scan(&adminTypeID); err != nil {
		t.Fatalf("find admin type: %v", err)
	}

	// 1. Прогреваем кэш - active=true.
	_, active, err := banCache.Status(context.Background(), targetID)
	if err != nil || !active {
		t.Fatalf("expected active before archive (active=%v err=%v)", active, err)
	}

	// 2. Архивируем через сервис - должен сбросить кэш.
	if err := userSvc.Delete(context.Background(), adminTypeID, target.Username); err != nil {
		t.Fatalf("archive: %v", err)
	}

	// 3. Сразу читаем - active=false (кэш сброшен, идём в БД) даже при часовом TTL.
	_, active, err = banCache.Status(context.Background(), targetID)
	if err != nil {
		t.Fatalf("status after archive: %v", err)
	}
	if active {
		t.Errorf("expected active=false immediately after archive (cache invalidated)")
	}
}

func TestAccessDenialService_ArchiveOlderThan(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewAccessDenialService(db)

	userID, _, cleanup := setupMWUser(t, db, false, false)
	defer cleanup()

	old := models.AccessDenial{
		UserID: &userID, Resource: "GET /old", Reason: models.DenialReasonPermission,
		CreatedAt: time.Now().AddDate(0, -4, 0),
	}
	fresh := models.AccessDenial{
		UserID: &userID, Resource: "GET /fresh", Reason: models.DenialReasonPermission,
		CreatedAt: time.Now(),
	}
	if err := db.Create(&old).Error; err != nil {
		t.Fatalf("create old: %v", err)
	}
	if err := db.Create(&fresh).Error; err != nil {
		t.Fatalf("create fresh: %v", err)
	}
	defer db.Where("user_id = ?", userID).Delete(&models.AccessDenial{})
	defer db.Where("user_id = ?", userID).Delete(&models.AccessDenialArchive{})

	cutoff := time.Now().AddDate(0, -3, 0)
	moved, err := svc.ArchiveOlderThan(context.Background(), cutoff)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if moved < 1 {
		t.Errorf("expected at least 1 record moved, got %d", moved)
	}

	var archived int64
	db.Model(&models.AccessDenialArchive{}).Where("user_id = ?", userID).Count(&archived)
	if archived < 1 {
		t.Errorf("expected archive record, got %d", archived)
	}

	var leftActive int64
	db.Model(&models.AccessDenial{}).Where("user_id = ? AND resource = ?", userID, "GET /old").Count(&leftActive)
	if leftActive != 0 {
		t.Errorf("expected old record removed from active table, got %d", leftActive)
	}

	var freshLeft int64
	db.Model(&models.AccessDenial{}).Where("user_id = ? AND resource = ?", userID, "GET /fresh").Count(&freshLeft)
	if freshLeft != 1 {
		t.Errorf("expected fresh record kept, got %d", freshLeft)
	}
}
