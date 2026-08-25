package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// TestUpdatePreferences_MandatoryType400: попытка выключить mandatory-тип (безопасность)
// через PUT /notifications/preferences должна отбиваться 400, а не молча пройти (#1748).
func TestUpdatePreferences_MandatoryType400(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "pref_user1", "pass123", 1, td.OrgID, td.CompanyID)

	body := fmt.Sprintf(`{"items":[{"type_code":"%s","enabled":false}]}`, services.NotificationTypePasswordChanged)
	rec := testutil.PUT(t, e, "/notifications/preferences", body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestUpdatePreferences_UnknownType400: код вне каталога тоже отбивается 400 - иначе
// строка с опечаткой тихо легла бы в базу и никогда не читалась гейтом.
func TestUpdatePreferences_UnknownType400(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "pref_user2", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/notifications/preferences", `{"items":[{"type_code":"nonexistent_type","enabled":false}]}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestPreferences_UpdateThenGet_ReflectsOverride: сохранённый override должен сразу
// читаться обратно как эффективное состояние переключателя этого типа.
func TestPreferences_UpdateThenGet_ReflectsOverride(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "pref_user3", "pass123", 1, td.OrgID, td.CompanyID)

	body := fmt.Sprintf(`{"items":[{"type_code":"%s","enabled":false}]}`, services.NotificationTypeNewsPublished)
	rec := testutil.PUT(t, e, "/notifications/preferences", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, "/notifications/preferences", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	categories := testutil.ParseResponse[[]models.NotificationPreferenceCategory](t, rec)

	found := false
	for _, cat := range categories {
		for _, item := range cat.Items {
			if item.TypeCode == services.NotificationTypeNewsPublished {
				found = true
				assert.False(t, item.Enabled, "выключенный тип должен отдавать enabled=false")
			}
		}
	}
	assert.True(t, found, "news_published должен присутствовать в каталоге настроек")
}

// TestGetNotifications_WithoutLimit_ReturnsFlatArray - guard обратной совместимости
// (#1748): без limit ответ остаётся плоским массивом БЕЗ meta, как до этого среза -
// фронт колокольчика/ленты не должен сломаться до среза S7.
func TestGetNotifications_WithoutLimit_ReturnsFlatArray(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "page_user1", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "page_user1")

	svc := services.NewNotificationService(db)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationCreated, "T", fmt.Sprintf("M%d", i), nil))
	}

	rec := testutil.GET(t, e, "/notifications", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	notifications := testutil.ParseSlice(t, rec)
	assert.Len(t, notifications, 3)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	_, hasMeta := raw["meta"]
	assert.False(t, hasMeta, "legacy-режим (без limit) не должен добавлять meta")
}

// TestGetNotifications_WithLimit_ReturnsPageAndUnreadCount: с limit ответ - страница с
// total в meta, а unread_count считается по ВСЕМ уведомлениям пользователя, а не по
// текущей странице (#1748).
func TestGetNotifications_WithLimit_ReturnsPageAndUnreadCount(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "page_user2", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "page_user2")

	svc := services.NewNotificationService(db)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationCreated, "T", fmt.Sprintf("M%d", i), nil))
	}
	// Одно прочитанное, чтобы unread_count заведомо отличался от total.
	var first models.Notification
	require.NoError(t, db.Where("user_id = ?", userID).Order("id ASC").First(&first).Error)
	require.NoError(t, db.Model(&first).Update("is_read", true).Error)

	rec := testutil.GET(t, e, "/notifications?limit=2", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	var env struct {
		Success bool                     `json:"success"`
		Data    []map[string]interface{} `json:"data"`
		Meta    struct {
			Total       int64 `json:"total"`
			Page        int   `json:"page"`
			PerPage     int   `json:"per_page"`
			UnreadCount int64 `json:"unread_count"`
		} `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	assert.Len(t, env.Data, 2)
	assert.Equal(t, int64(5), env.Meta.Total)
	assert.Equal(t, 2, env.Meta.PerPage)
	assert.Equal(t, int64(4), env.Meta.UnreadCount)
}

// TestGetNotifications_InvalidLimit400: отрицательный/нечисловой limit должен отбиваться
// 400, а не тихо откатываться на дефолт или падать 500.
func TestGetNotifications_InvalidLimit400(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "page_user3", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/notifications?limit=-5", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = testutil.GET(t, e, "/notifications?limit=abc", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = testutil.GET(t, e, "/notifications?limit=10&filter=bogus", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestMarkAllRead_ClearsUnreadAndReturnsCount: read-all - один UPDATE по всем непрочитанным
// уведомлениям пользователя, ответ несёт их число (#1748).
func TestMarkAllRead_ClearsUnreadAndReturnsCount(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "readall_user1", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "readall_user1")

	svc := services.NewNotificationService(db)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationCreated, "T", "M", nil))
	}

	rec := testutil.PUT(t, e, "/notifications/read-all", "", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	result := testutil.ParseMap(t, rec)
	assert.Equal(t, float64(4), result["updated"])

	var unread int64
	require.NoError(t, db.Model(&models.Notification{}).Where("user_id = ? AND is_read = ?", userID, false).Count(&unread).Error)
	assert.Equal(t, int64(0), unread)
}
