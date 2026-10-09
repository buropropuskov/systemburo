package services_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"systemburo/internal/models"
	"systemburo/internal/services"
)

func TestPassage2667DBMissingRecordAfterTableAuthorization(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPassage2667World(t, kind)
			commands := services.NewPassageCommandService(w.db, services.NewAuditRecorder(w.db))
			before := w.snapshot(t)
			var auditBefore, auditAfter int64
			require.NoError(t, w.db.Model(&models.AuditLog{}).Count(&auditBefore).Error)
			req := services.PassageCommandRequest{TableID: &w.table, TerritoryStatus: 1}
			_, err := commands.Mark(context.Background(), w.actor, kind, 999999, req)
			passage2667HTTP(t, err, http.StatusNotFound)
			require.Equal(t, before, w.snapshot(t))
			binding, column := "employee_target_tables", "employee_id"
			if kind == services.ElementCar {
				binding, column = "car_target_tables", "car_id"
			}
			require.NoError(t, w.db.Exec("DELETE FROM "+binding+" WHERE "+column+"=?", w.id).Error)
			_, err = commands.Mark(context.Background(), w.actor, kind, w.id, req)
			passage2667HTTP(t, err, http.StatusForbidden)
			require.Equal(t, before, w.snapshot(t))
			deny := models.UserPermissionOverride{UserID: w.actor, PermissionKey: "table.passage2667_post.view", Value: "deny", GrantedAt: time.Now().UTC()}
			require.NoError(t, w.db.Create(&deny).Error)
			_, err = commands.Mark(context.Background(), w.actor, kind, 999999, req)
			passage2667HTTP(t, err, http.StatusForbidden)
			require.Equal(t, before, w.snapshot(t))
			require.NoError(t, w.db.Model(&models.AuditLog{}).Count(&auditAfter).Error)
			require.Equal(t, auditBefore, auditAfter)
		})
	}
}
