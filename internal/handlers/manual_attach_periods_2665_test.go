package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type manualAttach2665World struct {
	period2665World
	service                    services.ManualAttachService
	orphan, entity, individual int
	individualEnd              string
}

func TestManualAttach2665DBFiniteInheritMaterializesBeforeMove(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, scenario := range []string{"wider_target", "narrow_target", "adopt"} {
			t.Run(string(kind)+"/"+scenario, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				from, to := w.clock.AddDate(0, 0, -1).Format("2006-01-02"), w.clock.AddDate(0, 0, 1).Format("2006-01-02")
				start, end := "09:00:00", "18:00:00"
				if scenario == "narrow_target" {
					to = w.clock.AddDate(0, 0, 4).Format("2006-01-02")
				}
				require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.orphan).Updates(map[string]any{"entry_date_from": from, "entry_date_to": to, "entry_time_from": start, "entry_time_to": end}).Error)
				parentBefore := w.parentAndVotesSnapshot(t)
				_, _, individualBefore := w.readPeriod(t, w.individual)
				before := w.snapshot(t)
				request := services.AttachToApplicationRequest{TargetAttachmentID: &w.attachment}
				if scenario == "adopt" {
					request = services.AttachToApplicationRequest{ApplicationID: &w.application}
				}
				_, err := w.service.AttachToApplication(context.Background(), w.orphan, request, w.actor)
				if scenario == "narrow_target" {
					period2665RequireHTTPError(t, err, http.StatusUnprocessableEntity)
					require.Equal(t, before, w.snapshot(t))
					return
				}
				require.NoError(t, err)
				attachmentID, mode, own := w.readPeriod(t, w.entity)
				if scenario == "adopt" {
					require.Equal(t, w.orphan, attachmentID)
					require.Equal(t, models.PeriodInherit, mode)
					require.Equal(t, models.EntryPeriod{}, own)
				} else {
					require.Equal(t, w.attachment, attachmentID)
					require.Equal(t, models.PeriodIndividual, mode)
					require.Equal(t, models.EntryPeriod{EntryDateFrom: &from, EntryDateTo: &to, EntryTimeFrom: &start, EntryTimeTo: &end}, own)
				}
				_, keptMode, individualAfter := w.readPeriod(t, w.individual)
				require.Equal(t, models.PeriodIndividual, keptMode)
				require.Equal(t, individualBefore, individualAfter)
				require.Equal(t, parentBefore, w.parentAndVotesSnapshot(t))
			})
		}
	}
}

func TestManualAttach2665DBByFactRejectsChangedLongWindowBeforeMutation(t *testing.T) {
	for _, scenario := range []string{"reattach_source", "reattach_individual", "adopt_source", "adopt_individual", "allowed_source", "preserved_individual"} {
		t.Run(scenario, func(t *testing.T) {
			w := setupManualAttach2665World(t, services.ElementCar)
			plate := " По Факту "
			require.NoError(t, w.db.Model(&models.Car{}).Where("id=?", w.entity).Update("car_number", plate).Error)
			to := w.clock.AddDate(0, 0, 5).Format("2006-01-02")
			if scenario == "allowed_source" {
				to = services.ByFactMaxDate(w.clock)
			}
			require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).Update("entry_date_to", to).Error)
			if scenario == "preserved_individual" {
				_, _, kept := w.readPeriod(t, w.individual)
				require.NoError(t, w.db.Model(&models.Car{}).Where("id=?", w.entity).Updates(map[string]any{"period_mode": models.PeriodIndividual, "entry_date_from": kept.EntryDateFrom, "entry_date_to": kept.EntryDateTo, "entry_time_from": kept.EntryTimeFrom, "entry_time_to": kept.EntryTimeTo}).Error)
			}
			request := services.AttachToApplicationRequest{TargetAttachmentID: &w.attachment, PeriodChoice: "source", SourceAttachmentID: &w.attachment}
			if scenario == "reattach_individual" || scenario == "adopt_individual" {
				input := w.request("", 5).Period
				request.PeriodChoice, request.SourceAttachmentID, request.Period = "individual", nil, input
			}
			if scenario == "adopt_source" || scenario == "adopt_individual" {
				request.TargetAttachmentID, request.ApplicationID = nil, &w.application
			}
			before := w.snapshot(t)
			writes := 0
			const callback = "manual_attach2665_count_writes"
			require.NoError(t, w.db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) { writes++ }))
			t.Cleanup(func() { require.NoError(t, w.db.Callback().Update().Remove(callback)) })
			_, err := w.service.AttachToApplication(context.Background(), w.orphan, request, w.actor)
			if scenario == "allowed_source" || scenario == "preserved_individual" {
				require.NoError(t, err)
				if scenario == "preserved_individual" {
					_, mode, own := w.readPeriod(t, w.entity)
					require.Equal(t, models.PeriodIndividual, mode)
					require.Equal(t, w.individualEnd, *own.EntryDateTo)
				}
				return
			}
			period2665RequireHTTPError(t, err, http.StatusBadRequest)
			require.Zero(t, writes, "ByFact validation precedes the first mutation")
			require.Equal(t, before, w.snapshot(t))
		})
	}
}

func setupManualAttach2665World(t *testing.T, kind services.ElementKind) manualAttach2665World {
	t.Helper()
	w := setupPeriod2665World(t, kind)
	require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Update("is_admin", true).Error)
	// Real resolver: the administrator has page.admin, and this fixture grants
	// detail.period.change explicitly through its existing personal override.
	resolved, err := services.NewPermissionResolver(w.db).Resolve(context.Background(), w.actor)
	require.NoError(t, err)
	require.True(t, resolved.Has(services.KeyPageAdmin))
	require.True(t, resolved.Has(services.KeyDetailPeriodChange))
	active := 1
	attachmentType := "people"
	if kind == services.ElementCar {
		attachmentType = "cars"
	}
	orphan := models.Attachment{AttachmentType: attachmentType, IsManual: true, Status: &active}
	require.NoError(t, w.db.Create(&orphan).Error)
	from, to := w.clock.AddDate(0, 0, -1).Format("2006-01-02"), w.clock.AddDate(0, 0, 8).Format("2006-01-02")
	start, end := "08:00:00", "22:00:00"
	result := manualAttach2665World{period2665World: w, service: services.NewManualAttachService(w.db, services.NewAuditRecorder(w.db), nil, nil), orphan: orphan.ID, individualEnd: to}
	if kind == services.ElementCar {
		row := models.Car{AttachmentID: orphan.ID, Status: &active, PeriodMode: models.PeriodInherit}
		kept := models.Car{AttachmentID: orphan.ID, Status: &active, PeriodMode: models.PeriodIndividual, EntryDateFrom: &from, EntryDateTo: &to, EntryTimeFrom: &start, EntryTimeTo: &end}
		require.NoError(t, w.db.Create(&row).Error)
		require.NoError(t, w.db.Create(&kept).Error)
		result.entity, result.individual = row.ID, kept.ID
	} else {
		row := models.Employee{AttachmentID: &orphan.ID, Status: &active, PeriodMode: models.PeriodInherit}
		kept := models.Employee{AttachmentID: &orphan.ID, Status: &active, PeriodMode: models.PeriodIndividual, EntryDateFrom: &from, EntryDateTo: &to, EntryTimeFrom: &start, EntryTimeTo: &end}
		require.NoError(t, w.db.Create(&row).Error)
		require.NoError(t, w.db.Create(&kept).Error)
		result.entity, result.individual = row.ID, kept.ID
	}
	return result
}

func (w manualAttach2665World) snapshot(t *testing.T) string {
	t.Helper()
	var result string
	require.NoError(t, w.db.Raw(`SELECT jsonb_build_object(
		'attachments', (SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM attachments a),
		'applications', (SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM applications a),
		'cars', (SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM cars c),
		'employees', (SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM employees e),
		'votes', (SELECT jsonb_agg(to_jsonb(v) ORDER BY v.id) FROM application_responsible_users v),
		'audit', (SELECT jsonb_agg(to_jsonb(h) ORDER BY h.id) FROM audit_log h)
	)::text`).Scan(&result).Error)
	return result
}

func (w manualAttach2665World) readPeriod(t *testing.T, id int) (int, models.PeriodMode, models.EntryPeriod) {
	t.Helper()
	var row struct {
		AttachmentID                                           int
		PeriodMode                                             models.PeriodMode
		EntryDateFrom, EntryDateTo, EntryTimeFrom, EntryTimeTo *string
	}
	require.NoError(t, w.db.Table(string(w.kind)).Where("id=?", id).Take(&row).Error)
	return row.AttachmentID, row.PeriodMode, models.EntryPeriod{EntryDateFrom: row.EntryDateFrom, EntryDateTo: row.EntryDateTo, EntryTimeFrom: row.EntryTimeFrom, EntryTimeTo: row.EntryTimeTo}
}

func TestManualAttach2665DBExplicitFiniteChoices(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, scenario := range []string{"reattach_same_source", "reattach_other_source", "reattach_individual", "adopt_source", "adopt_individual"} {
			t.Run(string(kind)+"/"+scenario, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				request := services.AttachToApplicationRequest{TargetAttachmentID: &w.attachment, PeriodChoice: "source", SourceAttachmentID: &w.attachment}
				var source models.Attachment
				require.NoError(t, w.db.First(&source, w.attachment).Error)
				wantEnd, wantMode := *source.EntryDateTo, models.PeriodInherit
				if scenario == "reattach_other_source" {
					source.ID = 0
					to := w.clock.AddDate(0, 0, 5).Format("2006-01-02")
					source.EntryDateTo = &to
					require.NoError(t, w.db.Create(&source).Error)
					request.SourceAttachmentID = &source.ID
					wantEnd, wantMode = to, models.PeriodIndividual
				}
				if scenario == "adopt_source" || scenario == "adopt_individual" {
					request.TargetAttachmentID, request.ApplicationID = nil, &w.application
				}
				if scenario == "reattach_individual" || scenario == "adopt_individual" {
					request.PeriodChoice, request.SourceAttachmentID = "individual", nil
					request.Period = w.request("", 5).Period
					wantEnd = request.Period.EntryDateTo
					wantMode = models.PeriodIndividual
				}
				_, _, keptBefore := w.readPeriod(t, w.individual)
				parentBefore := w.parentAndVotesSnapshot(t)
				result, err := w.service.AttachToApplication(context.Background(), w.orphan, request, w.actor)
				require.NoError(t, err)
				require.True(t, result.Success)
				attachmentID, mode, own := w.readPeriod(t, w.entity)
				var parent models.Attachment
				require.NoError(t, w.db.First(&parent, attachmentID).Error)
				if request.TargetAttachmentID != nil {
					require.Equal(t, w.attachment, attachmentID)
					var count int64
					require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.orphan).Count(&count).Error)
					require.Zero(t, count)
				} else {
					require.Equal(t, w.orphan, attachmentID)
					require.False(t, parent.IsManual)
					require.Equal(t, w.application, *parent.ApplicationID)
				}
				// Adoption with an explicit own choice intentionally stays individual
				// even when the newly adopted parent gets the same chosen dates.
				require.Equal(t, wantMode, mode)
				effective, err := models.ResolveEntityPeriod(mode, own, models.EntryPeriod{EntryDateFrom: parent.EntryDateFrom, EntryDateTo: parent.EntryDateTo, EntryTimeFrom: parent.EntryTimeFrom, EntryTimeTo: parent.EntryTimeTo}, parent.IsManual)
				require.NoError(t, err)
				require.Equal(t, wantEnd, *effective.EntryDateTo)
				require.Equal(t, w.clock.AddDate(0, 0, -1).Format("2006-01-02"), *effective.EntryDateFrom)
				require.Equal(t, "08:00:00", *effective.EntryTimeFrom)
				require.Equal(t, "22:00:00", *effective.EntryTimeTo)
				if wantMode == models.PeriodInherit {
					require.Equal(t, models.EntryPeriod{}, own)
				}
				keptAttachment, keptMode, keptAfter := w.readPeriod(t, w.individual)
				require.Equal(t, attachmentID, keptAttachment)
				require.Equal(t, models.PeriodIndividual, keptMode)
				require.Equal(t, keptBefore, keptAfter)
				require.Equal(t, w.individualEnd, *keptAfter.EntryDateTo)
				require.Equal(t, parentBefore, w.parentAndVotesSnapshot(t), "target, application and complete votes remain unchanged")
				var audits int64
				entityType := models.AuditEntityEmployee
				if kind == services.ElementCar {
					entityType = models.AuditEntityCar
				}
				require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id=? AND action=?", entityType, w.entity, models.AuditActionDatesChanged).Count(&audits).Error)
				require.EqualValues(t, 1, audits)
			})
		}
	}
}

func TestManualAttach2665DBDeniedHasNoChanges(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, scenario := range []string{"no_choice", "foreign_source", "inactive_source", "page_deny", "detail_deny", "banned", "archived"} {
			t.Run(string(kind)+"/"+scenario, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				request := services.AttachToApplicationRequest{TargetAttachmentID: &w.attachment, PeriodChoice: "source", SourceAttachmentID: &w.attachment}
				wantStatus := http.StatusForbidden
				switch scenario {
				case "no_choice":
					request.PeriodChoice, request.SourceAttachmentID = "", nil
					wantStatus = http.StatusBadRequest
				case "foreign_source", "inactive_source":
					var source models.Attachment
					require.NoError(t, w.db.First(&source, w.attachment).Error)
					source.ID = 0
					if scenario == "foreign_source" {
						status, conf := models.StatusInWork, models.ConfirmationApproved
						var foreign models.Application
						require.NoError(t, w.db.First(&foreign, w.application).Error)
						foreign.ID = 0
						foreign.Status, foreign.Confirmation = &status, &conf
						require.NoError(t, w.db.Create(&foreign).Error)
						source.ApplicationID = &foreign.ID
					} else {
						inactive := 0
						source.Status = &inactive
					}
					require.NoError(t, w.db.Create(&source).Error)
					request.SourceAttachmentID = &source.ID
					wantStatus = http.StatusUnprocessableEntity
				case "page_deny":
					require.NoError(t, w.db.Create(&models.UserPermissionOverride{UserID: w.actor, PermissionKey: services.KeyPageAdmin, Value: "deny", GrantedAt: w.clock.UTC()}).Error)
				case "detail_deny":
					require.NoError(t, w.db.Model(&models.UserPermissionOverride{}).Where("user_id=? AND permission_key=?", w.actor, services.KeyDetailPeriodChange).Update("value", "deny").Error)
				case "banned":
					require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Update("is_banned", true).Error)
				case "archived":
					require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Update("is_active", false).Error)
				}
				before := w.snapshot(t)
				_, err := w.service.AttachToApplication(context.Background(), w.orphan, request, w.actor)
				period2665RequireHTTPError(t, err, wantStatus)
				require.Equal(t, before, w.snapshot(t))
			})
		}
	}
}

func TestManualAttach2665DBAuditFailureRollsBack(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupManualAttach2665World(t, kind)
			before := w.snapshot(t)
			failure := errors.New("synthetic manual attach period audit failure")
			const callback = "manual_attach2665_fail_period_audit"
			seenMove := false
			entityType := models.AuditEntityEmployee
			if kind == services.ElementCar {
				entityType = models.AuditEntityCar
			}
			require.NoError(t, w.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
				entry, ok := tx.Statement.Dest.(*models.AuditLog)
				if !ok || entry.EntityType != entityType || entry.EntityID == nil || *entry.EntityID != w.entity || entry.ActorUserID == nil || *entry.ActorUserID != w.actor || entry.Action != models.AuditActionDatesChanged {
					return
				}
				var attachmentID int
				err := tx.Session(&gorm.Session{NewDB: true}).Table(string(w.kind)).Select("attachment_id").Where("id=?", w.entity).Scan(&attachmentID).Error
				if err != nil {
					tx.AddError(err)
					return
				}
				seenMove = attachmentID == w.attachment
				tx.AddError(failure)
			}))
			t.Cleanup(func() { require.NoError(t, w.db.Callback().Create().Remove(callback)) })
			_, err := w.service.AttachToApplication(context.Background(), w.orphan, services.AttachToApplicationRequest{TargetAttachmentID: &w.attachment, PeriodChoice: "source", SourceAttachmentID: &w.attachment}, w.actor)
			require.ErrorIs(t, err, failure)
			require.True(t, seenMove, "real row move occurs before the narrow audit fault")
			require.Equal(t, before, w.snapshot(t), "row, original orphan, dates, votes and audit must all roll back")
		})
	}
}
