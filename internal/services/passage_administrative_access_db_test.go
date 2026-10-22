package services_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

func TestPassageAdministrativeAccessRejectsReceivingRoleWithGrants(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPassage2667World(t, kind)
			user := models.User{Username: "passage_receiver_access", Password: "x", TypeID: 1, IsActive: true}
			require.NoError(t, w.db.Create(&user).Error)
			require.NoError(t, w.db.Create(&models.ApplicationApprover{UserID: user.ID}).Error)
			for _, verb := range []string{"view", "entry", "exit"} {
				testutil.GrantTableVerb(t, user.ID, "passage2667_post", verb)
			}
			testutil.GrantPermission(t, user.ID, services.KeyDetailPassageCorrect)
			ctx := context.Background()
			list := services.NewPassageOpenService(w.db)
			commands := services.NewPassageCommandService(w.db, services.NewAuditRecorder(w.db))
			before := w.snapshot(t)
			for _, source := range []string{"table", "admin_summary"} {
				req := w.correction()
				req.Source = source
				if source == "admin_summary" {
					req.TableID = nil
				}
				_, err := list.List(ctx, user.ID, kind, req.TableID, services.PassageOpenFilter{})
				passage2667HTTP(t, err, http.StatusForbidden)
				_, err = commands.Correct(ctx, user.ID, kind, w.id, req)
				passage2667HTTP(t, err, http.StatusForbidden)
				_, err = commands.RevertCorrection(ctx, user.ID, kind, w.id, req)
				passage2667HTTP(t, err, http.StatusForbidden)
				require.Equal(t, before, w.snapshot(t))
			}
			// Ordinary table reads and exit admission survive, but do not advertise correction.
			rows, err := services.LoadTablePassageResults(ctx, w.db, user.ID, kind, []int{w.id}, w.table, time.Time{})
			require.NoError(t, err)
			require.Contains(t, rows, w.id)
			require.True(t, rows[w.id].Admission.CanExit)
			require.False(t, rows[w.id].PassageState.CanCorrect)
			require.False(t, rows[w.id].PassageState.CanRevertCorrection)
		})
	}
}
