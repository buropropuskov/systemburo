package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestS6History_ActorMaskDisplaySearchAndRole(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementCar, services.ElementEmployee} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPassage2667World(t, kind)
			last, first, middle := "ActorHiddenS6", "ActorFirstS6", "ActorMiddleS6"
			require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.actor).
				Updates(map[string]any{"last_name": last, "first_name": first, "middle_name": middle}).Error)
			label := "Оператор 'бюро' \\ проверка"
			require.NoError(t, w.db.Create(&models.ApplicationApprover{UserID: w.actor, DisplayName: &label}).Error)
			viewer := models.User{Username: "s6_history_viewer", Password: "x", TypeID: 1, IsActive: true}
			require.NoError(t, w.db.Create(&viewer).Error)
			ctx := services.WithSessionIdentity(context.Background(), models.SessionIdentity{EffectiveUserID: viewer.ID})
			cars := services.NewCarService(w.db, services.NewAuditRecorder(w.db))
			employees := services.NewEmployeesHistoryService(w.db)
			for _, role := range []string{"regular", "admin", "super", "revoked", "support-as-regular"} {
				t.Run(role, func(t *testing.T) {
					ctx := ctx
					if role == "support-as-regular" {
						ctx = services.WithSessionIdentity(ctx, models.SessionIdentity{EffectiveUserID: viewer.ID, OwnerUserID: w.actor, ImpersonatedBy: &w.actor})
					}
					require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", viewer.ID).
						Updates(map[string]any{"is_admin": role == "admin", "is_super_admin": role == "super"}).Error)
					wantReal := role == "admin" || role == "super"
					for _, query := range []string{last, label} {
						var total int64
						var err error
						q := models.PassageHistoryQuery{Search: query, Page: 1, PerPage: 1}
						if kind == services.ElementCar {
							_, total, err = cars.GetCarsHistoryByTable(ctx, w.table, q)
						} else {
							_, total, err = employees.GetByTable(ctx, w.table, q)
						}
						require.NoError(t, err)
						matched := (query == last) == wantReal
						if matched {
							require.EqualValues(t, 1, total)
						} else {
							require.Zero(t, total)
						}
					}
					var rows any
					var options any
					if kind == services.ElementCar {
						items, err := cars.GetCarHistory(ctx, w.id)
						require.NoError(t, err)
						require.Len(t, items, 1)
						rows = items
						opts, err := cars.GetCarsHistoryFilterOptions(ctx, &w.table)
						require.NoError(t, err)
						options = opts
					} else {
						items, err := employees.GetByEmployee(ctx, w.id)
						require.NoError(t, err)
						require.Len(t, items, 1)
						rows = items
						opts, err := employees.GetFilterOptions(ctx, &w.table)
						require.NoError(t, err)
						options = opts
					}
					for _, response := range []any{rows, options} {
						raw, err := json.Marshal(response)
						require.NoError(t, err)
						if wantReal {
							require.Contains(t, string(raw), last)
						} else {
							require.NotContains(t, string(raw), last)
							require.NotContains(t, string(raw), first)
							require.NotContains(t, string(raw), middle)
							require.Contains(t, string(raw), "Оператор")
						}
					}
				})
			}
			// Consent is independent: neither administrator role can reveal it.
			for _, setting := range [][2]string{{"legal.pd_consent_required", "true"}, {"legal.pd_consent_text", "<p>Consent fixture</p>"}} {
				require.NoError(t, w.db.Exec("INSERT INTO system_settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value", setting[0], setting[1]).Error)
			}
			// An arbitrary receiver alias can itself contain the real surname.
			// Consent must dominate that alias even for ordinary viewers.
			require.NoError(t, w.db.Model(&models.ApplicationApprover{}).Where("user_id=?", w.actor).Update("display_name", last).Error)
			for _, role := range []string{"regular", "admin", "super"} {
				require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", viewer.ID).Updates(map[string]any{"is_admin": role == "admin", "is_super_admin": role == "super"}).Error)
				var rows any
				if kind == services.ElementCar {
					items, err := cars.GetCarHistory(ctx, w.id)
					require.NoError(t, err)
					require.Len(t, items, 1)
					rows = items
				} else {
					items, err := employees.GetByEmployee(ctx, w.id)
					require.NoError(t, err)
					require.Len(t, items, 1)
					rows = items
				}
				raw, err := json.Marshal(rows)
				require.NoError(t, err)
				require.Contains(t, string(raw), "@passage2667_actor")
				require.NotContains(t, string(raw), last)
			}
		})
	}
}

func TestS6History_MaskDependencyFailureIsUnavailable(t *testing.T) {
	w := setupPassage2667World(t, services.ElementCar)
	for _, setting := range [][2]string{{"legal.pd_consent_required", "true"}, {"legal.pd_consent_text", "<p>Consent fixture</p>"}} {
		require.NoError(t, w.db.Exec("INSERT INTO system_settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value", setting[0], setting[1]).Error)
	}
	for _, target := range []string{"system_settings", "pd_consents", "application_approvers", "viewer"} {
		t.Run(target, func(t *testing.T) {
			name := "s6_mask_dependency"
			table := target
			if target == "pd_consents" {
				// Consent lookup scans users with a NOT EXISTS consent subquery.
				table = "users"
			}
			injected := false
			callbacks := w.db.Callback().Row()
			before := "gorm:row"
			ctx := context.Background()
			if target == "viewer" {
				table = "users"
				callbacks = w.db.Callback().Query()
				before = "gorm:query"
				ctx = services.WithSessionIdentity(ctx, models.SessionIdentity{EffectiveUserID: w.actor})
			}
			require.NoError(t, callbacks.Before(before).Register(name, func(tx *gorm.DB) {
				// This callback runs before GORM builds the SELECT SQL.
				if tx.Statement.Table == table {
					injected = true
					tx.AddError(errors.New("synthetic dependency failure"))
				}
			}))
			defer callbacks.Remove(name)
			rows, err := services.NewCarService(w.db, services.NewAuditRecorder(w.db)).GetCarHistory(ctx, w.id)
			require.True(t, injected, "dependency failure must actually be injected")
			require.Nil(t, rows)
			var unavailable *echo.HTTPError
			require.ErrorAs(t, err, &unavailable)
			require.Equal(t, http.StatusServiceUnavailable, unavailable.Code)
		})
	}
}

func TestS6History_MixedActorsKeepPagesAndTotals(t *testing.T) {
	w := setupPassage2667World(t, services.ElementCar)
	label, private, visible := "OperatorPageS6", "PrivatePageS6", "VisiblePageS6"
	require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Update("last_name", private).Error)
	require.NoError(t, w.db.Create(&models.ApplicationApprover{UserID: w.actor, DisplayName: &label}).Error)
	control := models.User{Username: "s6_page_control", Password: "x", TypeID: 1, LastName: &visible, IsActive: true}
	require.NoError(t, w.db.Create(&control).Error)
	require.NoError(t, w.db.Create(&models.AuditLog{EntityType: models.AuditEntityCar, EntityID: &w.id, ActorUserID: &control.ID, Action: "exit", CreatedAt: w.entered.Add(time.Second)}).Error)
	svc := services.NewCarService(w.db, services.NewAuditRecorder(w.db))
	seen := map[int]bool{}
	for page := 1; page <= 2; page++ {
		rows, total, err := svc.GetCarsHistoryByTable(context.Background(), w.table, models.PassageHistoryQuery{Page: page, PerPage: 1})
		require.NoError(t, err)
		require.EqualValues(t, 2, total)
		require.Len(t, rows, 1)
		require.NotNil(t, rows[0].UserID)
		seen[*rows[0].UserID] = true
		if *rows[0].UserID == w.actor {
			require.Equal(t, label, rows[0].UserName)
		} else {
			require.Contains(t, rows[0].UserName, visible)
		}
	}
	require.Len(t, seen, 2)
}

func TestS6History_MaskFailurePreventsWrite(t *testing.T) {
	w := setupPassage2667World(t, services.ElementCar)
	var before int64
	require.NoError(t, w.db.Model(&models.News{}).Count(&before).Error)
	name := "s6_mask_before_write"
	injected := false
	require.NoError(t, w.db.Callback().Row().Before("gorm:row").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "system_settings" {
			injected = true
			tx.AddError(errors.New("synthetic mask failure"))
		}
	}))
	defer w.db.Callback().Row().Remove(name)
	row, err := services.NewNewsService(w.db).CreateNews(context.Background(), w.actor, models.CreateNewsRequest{Title: "Synthetic write must not happen"})
	require.True(t, injected)
	require.Nil(t, row)
	var unavailable *echo.HTTPError
	require.ErrorAs(t, err, &unavailable)
	require.Equal(t, http.StatusServiceUnavailable, unavailable.Code)
	var after int64
	require.NoError(t, w.db.Model(&models.News{}).Count(&after).Error)
	require.Equal(t, before, after)
}
