package services_test

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"net/http"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"testing"
	"time"
)

func TestTableHistory2676ExpiredManualCard(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementCar, services.ElementEmployee} {
		for _, scenario := range []string{"expired", "removed", "purged", "detached", "denied", "missing", "wrong_type", "inactive_table"} {
			t.Run(string(kind)+"/"+scenario, func(t *testing.T) {
				w := setupPassage2667World(t, kind)
				ctx := context.Background()
				require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).Updates(map[string]any{"application_id": nil, "is_manual": true}).Error)
				at := time.Now().UTC().Add(-10 * time.Minute)
				details, err := json.Marshal(map[string]any{"table_id": w.table})
				require.NoError(t, err)
				entityType := models.AuditEntityEmployee
				removed, binding, column := "date_deleted", "employee_target_tables", "employee_id"
				if kind == services.ElementCar {
					entityType = models.AuditEntityCar
					removed, binding, column = "date_removed", "car_target_tables", "car_id"
				}
				require.NoError(t, w.db.Create(&models.AuditLog{EntityType: entityType, EntityID: &w.id, Action: "exit", CreatedAt: at, Details: details}).Error)
				id := w.id
				switch scenario {
				case "removed":
					require.NoError(t, w.db.Table(string(kind)).Where("id=?", id).Update(removed, at).Error)
				case "purged":
					require.NoError(t, w.db.Table(string(kind)).Where("id=?", id).Update("is_purged", true).Error)
				case "detached":
					require.NoError(t, w.db.Exec("DELETE FROM "+binding+" WHERE "+column+"=?", id).Error)
				case "denied":
					require.NoError(t, w.db.Create(&models.UserPermissionOverride{UserID: w.actor, PermissionKey: "table.passage2667_post.view", Value: "deny", GrantedAt: time.Now().UTC()}).Error)
				case "missing":
					id += 100000
				case "wrong_type":
					require.NoError(t, w.db.Model(&models.SystemTable{}).Where("id=?", w.table).Update("table_type", "other").Error)
				case "inactive_table":
					require.NoError(t, w.db.Model(&models.SystemTable{}).Where("id=?", w.table).Update("is_active", false).Error)
				}
				var readErr error
				if kind == services.ElementCar {
					svc := services.NewCarService(w.db, services.NewAuditRecorder(w.db))
					row, err := svc.GetHistoryCarForTable(ctx, w.actor, w.table, id)
					readErr = err
					if scenario == "expired" {
						require.NoError(t, err)
						require.Equal(t, id, row.ID)
						require.False(t, row.Admission.CanEnter)
						require.False(t, row.Admission.CanExit)
						active, err := svc.GetActiveCarsForTable(ctx, w.table)
						require.NoError(t, err)
						require.Empty(t, active)
					}
				} else {
					svc := services.NewEmployeeService(w.db, services.NewAuditRecorder(w.db))
					row, err := svc.GetHistoryEmployeeForTable(ctx, w.actor, w.table, id)
					readErr = err
					if scenario == "expired" {
						require.NoError(t, err)
						require.Equal(t, id, row.ID)
						require.False(t, row.Admission.CanEnter)
						require.False(t, row.Admission.CanExit)
						active, err := svc.GetActiveEmployeesForTable(ctx, w.table)
						require.NoError(t, err)
						require.Empty(t, active)
					}
				}
				if scenario != "expired" {
					status := http.StatusNotFound
					if scenario == "denied" || scenario == "wrong_type" || scenario == "inactive_table" {
						status = http.StatusForbidden
					}
					passage2667HTTP(t, readErr, status)
				}
			})
		}
	}
}
