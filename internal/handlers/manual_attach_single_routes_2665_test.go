package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestSingleManualAttach2665HTTPIdentityAndManagedGate(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPeriodRoutes2665World(t, false, true)
			id, source := w.employee, w.peopleAttachment
			if kind == services.ElementCar {
				id, source = w.car, w.carAttachment
			}
			var target models.Attachment
			require.NoError(t, w.db.First(&target, source).Error)
			target.ID = 0
			require.NoError(t, w.db.Create(&target).Error)
			require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", source).Updates(map[string]any{"application_id": nil, "is_manual": true}).Error)
			base := fmt.Sprintf("/%s/%d", kind, id)
			request := services.SingleManualAttachRequest{TargetAttachmentID: &target.ID, Reason: "HTTP single row attachment"}
			body := periodRoutes2665Body(t, request, w.otherActor)
			periodRoutes2665Envelope(t, testutil.GET(t, w.e, base+"/manual-attach-context", nil), http.StatusUnauthorized, nil)
			for _, suffix := range []string{"/attach-to-application/preview", "/attach-to-application"} {
				periodRoutes2665Envelope(t, testutil.POST(t, w.e, base+suffix, body, nil), http.StatusUnauthorized, nil)
			}
			headers := testutil.AuthHeader(w.token)
			var context services.SingleManualAttachContext
			periodRoutes2665Envelope(t, testutil.GET(t, w.e, base+"/manual-attach-context", headers), http.StatusOK, &context)
			require.Equal(t, id, context.EntityID)
			require.False(t, context.RequiresPeriodChoice)
			require.False(t, context.CanAssignPeriod, "finite records do not bypass detail deny; they need no new period")
			var preview services.SingleManualAttachResult
			periodRoutes2665Envelope(t, testutil.POST(t, w.e, base+"/attach-to-application/preview", body, headers), http.StatusOK, &preview)
			request.ExpectedRevision = preview.Revision
			var result services.SingleManualAttachResult
			periodRoutes2665Envelope(t, testutil.POST(t, w.e, base+"/attach-to-application", periodRoutes2665Body(t, request, w.otherActor), headers), http.StatusOK, &result)
			require.Equal(t, id, result.EntityID)
			require.Equal(t, target.ID, *result.DestinationAttachmentID)
			var audits []models.AuditLog
			require.NoError(t, w.db.Where("entity_id=? AND actor_user_id=?", id, w.actor).Find(&audits).Error)
			require.NotEmpty(t, audits, "the authenticated actor must be used instead of spoofed body fields")
			var spoofed int64
			require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_id=? AND actor_user_id=?", id, w.otherActor).Count(&spoofed).Error)
			require.Zero(t, spoofed)
			testutil.DenyPermission(t, w.actor, services.KeyPageAdmin)
			periodRoutes2665Envelope(t, testutil.GET(t, w.e, base+"/manual-attach-context", headers), http.StatusForbidden, nil)
		})
	}
}
