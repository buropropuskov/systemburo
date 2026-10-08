package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"systemburo/internal/models"
	"systemburo/internal/services"
)

func TestSingleManualAttach2665DBMetadata(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, scenario := range []string{"allowed", "personal_deny", "banned", "archived_actor", "not_work", "invalid_table", "not_manual"} {
			t.Run(string(kind)+"/"+scenario, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				command := singleAttach2665Service(w)
				active, inactive := 1, 0
				items := models.Attachment{ApplicationID: &w.application, AttachmentType: "items", Status: &active}
				require.NoError(t, w.db.Create(&items).Error)
				for _, a := range []models.Attachment{
					{ApplicationID: &w.application, AttachmentType: "items", Status: &inactive},
					{ApplicationID: &w.application, AttachmentType: "items", Status: &active, IsManual: true},
				} {
					require.NoError(t, w.db.Create(&a).Error)
				}
				var tableID *int
				want := http.StatusForbidden
				switch scenario {
				case "allowed":
					// Admin is not a sender or participant in this application. The
					// narrow selector must not depend on the general detail permission.
					other := models.User{Username: "single_metadata_sender", Password: "x", TypeID: 1, IsActive: true}
					require.NoError(t, w.db.Create(&other).Error)
					require.NoError(t, w.db.Table("applications").Where("id=?", w.application).Update("sender_user_id", other.ID).Error)
					require.NoError(t, w.db.Where("application_id=? AND user_id=?", w.application, w.actor).Delete(&models.ApplicationResponsibleUser{}).Error)
				case "personal_deny":
					require.NoError(t, w.db.Create(&models.UserPermissionOverride{UserID: w.actor, PermissionKey: services.KeyPageAdmin, Value: "deny", GrantedAt: w.clock.UTC()}).Error)
				case "banned":
					require.NoError(t, w.db.Table("users").Where("id=?", w.actor).Update("is_banned", true).Error)
				case "archived_actor":
					require.NoError(t, w.db.Table("users").Where("id=?", w.actor).Update("is_active", false).Error)
				case "not_work":
					require.NoError(t, w.db.Table("applications").Where("id=?", w.application).Update("status", models.StatusCompleted).Error)
					want = http.StatusUnprocessableEntity
				case "invalid_table":
					invalid := 987654321
					tableID = &invalid
				case "not_manual":
					require.NoError(t, w.db.Table("attachments").Where("id=?", w.orphan).Update("is_manual", false).Error)
					want = http.StatusConflict
				}
				before := w.snapshot(t)
				rows, err := command.Attachments(context.Background(), w.actor, kind, w.entity, w.application, tableID)
				if scenario != "allowed" {
					period2665RequireHTTPError(t, err, want)
					require.Nil(t, rows)
					require.Equal(t, before, w.snapshot(t))
					return
				}
				require.NoError(t, err)
				require.Len(t, rows, 2)
				gotIDs := []int{}
				for _, row := range rows {
					gotIDs = append(gotIDs, row.ID)
					require.Equal(t, w.application, row.ApplicationID)
					require.Equal(t, 1, row.Status)
					require.False(t, row.IsManual)
					encoded, err := json.Marshal(row)
					require.NoError(t, err)
					var fields map[string]any
					require.NoError(t, json.Unmarshal(encoded, &fields))
					require.Len(t, fields, 11, "metadata DTO must not grow entity/participant payloads")
					for _, key := range []string{"employees", "cars", "documents", "sender", "responsible_users", "messages", "audit"} {
						require.NotContains(t, fields, key)
					}
				}
				require.ElementsMatch(t, []int{w.attachment, items.ID}, gotIDs)
				require.Equal(t, before, w.snapshot(t), "selector must not mutate rows or append audits")
			})
		}
	}
}
