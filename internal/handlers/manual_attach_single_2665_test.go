package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"systemburo/internal/models"
	"systemburo/internal/services"
)

func singleAttach2665Service(w manualAttach2665World) *services.SingleManualAttachCommandService {
	return services.NewSingleManualAttachCommandService(w.db, services.NewAuditRecorder(w.db), nil, nil)
}
func singleAttach2665Request(w manualAttach2665World, newAttachment bool) services.SingleManualAttachRequest {
	request := services.SingleManualAttachRequest{TargetAttachmentID: &w.attachment, PeriodChoice: "source", SourceAttachmentID: &w.attachment, Reason: "Тестовая привязка открытой записи"}
	if newAttachment {
		request.TargetAttachmentID = nil
		request.ApplicationID = &w.application
	}
	return request
}
func singleAttach2665JSON(t *testing.T, db *gorm.DB, table string, id int) string {
	t.Helper()
	var out string
	require.NoError(t, db.Raw("SELECT to_jsonb(r)::text FROM "+table+" r WHERE r.id=?", id).Scan(&out).Error)
	return out
}

func TestSingleManualAttach2665DBOnlySelectedRow(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, newAttachment := range []bool{false, true} {
			label := "existing"
			if newAttachment {
				label = "new"
			}
			t.Run(string(kind)+"/"+label, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				command := singleAttach2665Service(w)
				ctx := context.Background()
				orphanBefore := singleAttach2665JSON(t, w.db, "attachments", w.orphan)
				neighborBefore := singleAttach2665JSON(t, w.db, string(kind), w.individual)
				votesBefore := w.parentAndVotesSnapshot(t)
				before := w.snapshot(t)
				request := singleAttach2665Request(w, newAttachment)
				preview, err := command.Preview(ctx, w.actor, kind, w.entity, request)
				require.NoError(t, err)
				require.Len(t, preview.Revision, 64)
				require.Equal(t, before, w.snapshot(t), "preview may not mutate anything")
				if newAttachment {
					require.Nil(t, preview.DestinationAttachmentID)
				} else {
					require.Equal(t, w.attachment, *preview.DestinationAttachmentID)
				}
				request.ExpectedRevision = preview.Revision
				notifications := 0
				command.SetAfterAttach(func(_ context.Context, id int) { require.Equal(t, w.application, id); notifications++ })
				result, err := command.Attach(ctx, w.actor, kind, w.entity, request)
				require.NoError(t, err)
				require.NotNil(t, result.DestinationAttachmentID)
				dest, mode, own := w.readPeriod(t, w.entity)
				require.Equal(t, *result.DestinationAttachmentID, dest)
				require.Equal(t, models.PeriodInherit, mode)
				require.Equal(t, models.EntryPeriod{}, own)
				require.Equal(t, orphanBefore, singleAttach2665JSON(t, w.db, "attachments", w.orphan))
				require.Equal(t, neighborBefore, singleAttach2665JSON(t, w.db, string(kind), w.individual))
				require.Equal(t, votesBefore, w.parentAndVotesSnapshot(t))
				require.Equal(t, 1, notifications)
				entityType := models.AuditEntityEmployee
				if kind == services.ElementCar {
					entityType = models.AuditEntityCar
				}
				var dateAudit models.AuditLog
				require.NoError(t, w.db.Where("entity_type=? AND entity_id=? AND action=?", entityType, w.entity, models.AuditActionDatesChanged).Take(&dateAudit).Error)
				var periodDetails struct {
					FieldName string                 `json:"field_name"`
					OldValue  string                 `json:"old_value"`
					NewValue  string                 `json:"new_value"`
					Comment   string                 `json:"comment"`
					OldMode   models.PeriodMode      `json:"old_period_mode"`
					NewMode   models.PeriodMode      `json:"new_period_mode"`
					OldPeriod models.EffectivePeriod `json:"old_period"`
					NewPeriod models.EffectivePeriod `json:"new_period"`
				}
				require.NoError(t, json.Unmarshal(dateAudit.Details, &periodDetails))
				require.Equal(t, "period", periodDetails.FieldName)
				require.NotEmpty(t, periodDetails.OldValue)
				require.NotEmpty(t, periodDetails.NewValue)
				require.NotEqual(t, periodDetails.OldValue, periodDetails.NewValue)
				require.Equal(t, request.Reason, periodDetails.Comment)
				require.Equal(t, result.CurrentMode, periodDetails.OldMode)
				require.Equal(t, result.NewMode, periodDetails.NewMode)
				require.Equal(t, result.CurrentEffective, periodDetails.OldPeriod)
				require.Equal(t, result.NewEffective, periodDetails.NewPeriod)
				if newAttachment {
					require.NotEqual(t, w.orphan, dest)
					require.NotEqual(t, w.attachment, dest)
					var parent models.Attachment
					require.NoError(t, w.db.First(&parent, dest).Error)
					require.False(t, parent.IsManual)
					require.Equal(t, w.application, *parent.ApplicationID)
					require.Nil(t, parent.OrganizationID)
					require.Nil(t, parent.CompanyID)
					require.Equal(t, w.actor, *parent.CreatedByUserID)
				}
				_, err = command.Attach(ctx, w.actor, kind, w.entity, request)
				period2665RequireHTTPError(t, err, http.StatusConflict)
				require.Equal(t, 1, notifications, "failed repeat cannot publish")
			})
		}
	}
}

func TestSingleManualAttach2665DBFiniteWindows(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, scenario := range []string{"finite_inherit", "individual_unbounded_parent", "individual_new", "explicit_individual", "other_source"} {
			t.Run(string(kind)+"/"+scenario, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				command := singleAttach2665Service(w)
				ctx := context.Background()
				request := singleAttach2665Request(w, scenario == "individual_new")
				selected := w.entity
				var expected models.EntryPeriod
				wantMode := models.PeriodIndividual
				switch scenario {
				case "finite_inherit":
					input := w.request("", 1).Period
					expected = models.EntryPeriod{EntryDateFrom: &input.EntryDateFrom, EntryDateTo: &input.EntryDateTo, EntryTimeFrom: &input.EntryTimeFrom, EntryTimeTo: &input.EntryTimeTo}
					require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.orphan).Updates(map[string]any{"entry_date_from": expected.EntryDateFrom, "entry_date_to": expected.EntryDateTo, "entry_time_from": expected.EntryTimeFrom, "entry_time_to": expected.EntryTimeTo}).Error)
					request.PeriodChoice = ""
					request.SourceAttachmentID = nil
				case "individual_unbounded_parent", "individual_new":
					selected = w.individual
					_, _, expected = w.readPeriod(t, selected)
					request.PeriodChoice = ""
					request.SourceAttachmentID = nil
					require.NoError(t, w.db.Model(&models.UserPermissionOverride{}).Where("user_id=? AND permission_key=?", w.actor, services.KeyDetailPeriodChange).Update("value", "deny").Error)
				case "explicit_individual":
					request.PeriodChoice = "individual"
					request.SourceAttachmentID = nil
					request.Period = w.request("", 5).Period
					input := request.Period
					start, end := "08:00:00", "22:00:00"
					expected = models.EntryPeriod{EntryDateFrom: &input.EntryDateFrom, EntryDateTo: &input.EntryDateTo, EntryTimeFrom: &start, EntryTimeTo: &end}
				case "other_source":
					var source models.Attachment
					require.NoError(t, w.db.First(&source, w.attachment).Error)
					source.ID = 0
					end := w.clock.AddDate(0, 0, 5).Format("2006-01-02")
					source.EntryDateTo = &end
					require.NoError(t, w.db.Create(&source).Error)
					request.SourceAttachmentID = &source.ID
					expected = models.EntryPeriod{EntryDateFrom: source.EntryDateFrom, EntryDateTo: source.EntryDateTo, EntryTimeFrom: source.EntryTimeFrom, EntryTimeTo: source.EntryTimeTo}
				}
				contextResult, err := command.Context(ctx, w.actor, kind, selected, nil)
				require.NoError(t, err)
				if scenario == "individual_unbounded_parent" || scenario == "individual_new" {
					require.False(t, contextResult.RequiresPeriodChoice)
					require.False(t, contextResult.CanAssignPeriod)
				}
				orphanBefore := singleAttach2665JSON(t, w.db, "attachments", w.orphan)
				preview, err := command.Preview(ctx, w.actor, kind, selected, request)
				require.NoError(t, err)
				request.ExpectedRevision = preview.Revision
				result, err := command.Attach(ctx, w.actor, kind, selected, request)
				require.NoError(t, err)
				require.Equal(t, wantMode, result.NewMode)
				require.Equal(t, expected, result.NewEffective.EntryPeriod)
				entityType := models.AuditEntityEmployee
				if kind == services.ElementCar {
					entityType = models.AuditEntityCar
				}
				var dateCount int64
				require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id=? AND action=?", entityType, selected, models.AuditActionDatesChanged).Count(&dateCount).Error)
				if scenario == "individual_unbounded_parent" || scenario == "individual_new" {
					require.Zero(t, dateCount, "unchanged individual window/mode must not invent a date change")
				} else {
					require.EqualValues(t, 1, dateCount)
				}
				require.Equal(t, orphanBefore, singleAttach2665JSON(t, w.db, "attachments", w.orphan))
			})
		}
	}
}

func TestSingleManualAttach2665DBSecondAuditFailureRollsBackBothAudits(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, createNew := range []bool{false, true} {
			label := "existing"
			if createNew {
				label = "new"
			}
			t.Run(string(kind)+"/"+label, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				command := singleAttach2665Service(w)
				ctx := context.Background()
				request := singleAttach2665Request(w, createNew)
				preview, err := command.Preview(ctx, w.actor, kind, w.entity, request)
				require.NoError(t, err)
				request.ExpectedRevision = preview.Revision
				before := w.snapshot(t)
				failure := errors.New("synthetic second single attach audit failure")
				seenFirstAudit, seenMove, notifications := false, false, 0
				command.SetAfterAttach(func(context.Context, int) { notifications++ })
				entityType := models.AuditEntityEmployee
				if kind == services.ElementCar {
					entityType = models.AuditEntityCar
				}
				const callback = "single_attach2665_second_audit_failure"
				require.NoError(t, w.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
					entry, ok := tx.Statement.Dest.(*models.AuditLog)
					if !ok || entry.EntityType != entityType || entry.EntityID == nil || *entry.EntityID != w.entity || entry.ActorUserID == nil || *entry.ActorUserID != w.actor || entry.Action != models.AuditActionDatesChanged {
						return
					}
					transaction := tx.Session(&gorm.Session{NewDB: true})
					var count int64
					if err := transaction.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id=? AND actor_user_id=? AND action=?", entityType, w.entity, w.actor, "update").Count(&count).Error; err != nil {
						tx.AddError(err)
						return
					}
					seenFirstAudit = count == 1
					var parentID int
					if err := tx.Session(&gorm.Session{NewDB: true}).Table(string(kind)).Select("attachment_id").Where("id=?", w.entity).Scan(&parentID).Error; err != nil {
						tx.AddError(err)
						return
					}
					seenMove = parentID != w.orphan
					tx.AddError(failure)
				}))
				t.Cleanup(func() { require.NoError(t, w.db.Callback().Create().Remove(callback)) })
				_, err = command.Attach(ctx, w.actor, kind, w.entity, request)
				require.ErrorIs(t, err, failure)
				require.True(t, seenFirstAudit, "the first history write must precede the injected second failure")
				require.True(t, seenMove)
				require.Equal(t, before, w.snapshot(t), "entity move, new attachment and both audits roll back together")
				require.Zero(t, notifications)
			})
		}
	}
}

func TestSingleManualAttach2665DBByFactAndTargetPlacesRevision(t *testing.T) {
	for _, scenario := range []string{"by_fact_long", "by_fact_preserved", "target_places_stale"} {
		t.Run(scenario, func(t *testing.T) {
			w := setupManualAttach2665World(t, services.ElementCar)
			command := singleAttach2665Service(w)
			ctx := context.Background()
			request := singleAttach2665Request(w, false)
			selected := w.entity
			if scenario == "by_fact_preserved" {
				selected = w.individual
				request.PeriodChoice = ""
				request.SourceAttachmentID = nil
			}
			if scenario != "target_places_stale" {
				require.NoError(t, w.db.Model(&models.Car{}).Where("id=?", selected).Update("car_number", "По факту").Error)
			}
			if scenario == "by_fact_long" {
				request.PeriodChoice = "individual"
				request.SourceAttachmentID = nil
				request.Period = w.request("", 5).Period
				before := w.snapshot(t)
				_, err := command.Preview(ctx, w.actor, w.kind, selected, request)
				period2665RequireHTTPError(t, err, http.StatusBadRequest)
				require.Equal(t, before, w.snapshot(t))
				return
			}
			preview, err := command.Preview(ctx, w.actor, w.kind, selected, request)
			require.NoError(t, err)
			request.ExpectedRevision = preview.Revision
			if scenario == "target_places_stale" {
				place := models.UnloadPlace{Name: "Тестовое новое место цели", IsActive: true}
				require.NoError(t, w.db.Create(&place).Error)
				require.NoError(t, w.db.Create(&models.AttachmentUnloadPlace{AttachmentID: w.attachment, UnloadPlaceID: place.ID}).Error)
				before := w.snapshot(t)
				_, err = command.Attach(ctx, w.actor, w.kind, selected, request)
				period2665RequireHTTPError(t, err, http.StatusConflict)
				require.Equal(t, before, w.snapshot(t))
				return
			}
			result, err := command.Attach(ctx, w.actor, w.kind, selected, request)
			require.NoError(t, err)
			require.Equal(t, models.PeriodIndividual, result.NewMode)
			require.Equal(t, w.individualEnd, *result.NewEffective.EntryDateTo)
		})
	}
}

func TestSingleManualAttach2665DBFreshDeniedAndStale(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, scenario := range []string{"page_deny", "detail_deny", "banned", "archived", "stale_parent", "stale_target", "stale_entity", "foreign_table", "inactive", "purged", "flags", "foreign_source", "no_choice"} {
			t.Run(string(kind)+"/"+scenario, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				command := singleAttach2665Service(w)
				ctx := context.Background()
				request := singleAttach2665Request(w, false)
				preview, err := command.Preview(ctx, w.actor, kind, w.entity, request)
				require.NoError(t, err)
				request.ExpectedRevision = preview.Revision
				want := http.StatusForbidden
				switch scenario {
				case "page_deny":
					require.NoError(t, w.db.Create(&models.UserPermissionOverride{UserID: w.actor, PermissionKey: services.KeyPageAdmin, Value: "deny", GrantedAt: w.clock.UTC()}).Error)
				case "detail_deny":
					require.NoError(t, w.db.Model(&models.UserPermissionOverride{}).Where("user_id=? AND permission_key=?", w.actor, services.KeyDetailPeriodChange).Update("value", "deny").Error)
				case "banned":
					require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Update("is_banned", true).Error)
				case "archived":
					require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Update("is_active", false).Error)
				case "stale_parent":
					want = http.StatusConflict
					require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.orphan).UpdateColumn("free_parking", true).Error)
				case "stale_target":
					want = http.StatusConflict
					require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).UpdateColumn("entry_time_to", "23:00:00").Error)
				case "stale_entity":
					want = http.StatusConflict
					require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.entity).UpdateColumn("updated_at", w.clock.AddDate(0, 0, 1)).Error)
				case "foreign_table":
					invalid := 987654
					request.TableID = &invalid
				case "inactive":
					want = http.StatusUnprocessableEntity
					require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.entity).Update("status", 0).Error)
				case "purged":
					want = http.StatusUnprocessableEntity
					require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.entity).Update("is_purged", true).Error)
				case "flags":
					want = http.StatusConflict
					require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).UpdateColumn("roof_access", true).Error)
				case "foreign_source":
					want = http.StatusUnprocessableEntity
					request.SourceAttachmentID = &w.orphan
				case "no_choice":
					want = http.StatusBadRequest
					request.PeriodChoice = ""
					request.SourceAttachmentID = nil
				}
				before := w.snapshot(t)
				_, err = command.Attach(ctx, w.actor, kind, w.entity, request)
				// Pointing at the manual orphan is rejected by request validation first.
				if scenario == "foreign_source" {
					want = http.StatusBadRequest
				}
				period2665RequireHTTPError(t, err, want)
				require.Equal(t, before, w.snapshot(t))
			})
		}
	}
}

func TestSingleManualAttach2665DBAuditFailureRollsBack(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, newAttachment := range []bool{false, true} {
			label := "existing"
			if newAttachment {
				label = "new"
			}
			t.Run(string(kind)+"/"+label, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				command := singleAttach2665Service(w)
				ctx := context.Background()
				request := singleAttach2665Request(w, newAttachment)
				preview, err := command.Preview(ctx, w.actor, kind, w.entity, request)
				require.NoError(t, err)
				request.ExpectedRevision = preview.Revision
				before := w.snapshot(t)
				failure := errors.New("synthetic single attach audit failure")
				seenMove := false
				notifications := 0
				command.SetAfterAttach(func(context.Context, int) { notifications++ })
				const callback = "single_attach2665_audit_failure"
				require.NoError(t, w.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
					entry, ok := tx.Statement.Dest.(*models.AuditLog)
					if !ok || entry.EntityID == nil || *entry.EntityID != w.entity || entry.ActorUserID == nil || *entry.ActorUserID != w.actor {
						return
					}
					var parentID int
					readErr := tx.Session(&gorm.Session{NewDB: true}).Table(string(kind)).Select("attachment_id").Where("id=?", w.entity).Scan(&parentID).Error
					if readErr != nil {
						tx.AddError(readErr)
						return
					}
					seenMove = parentID != w.orphan
					tx.AddError(failure)
				}))
				t.Cleanup(func() { require.NoError(t, w.db.Callback().Create().Remove(callback)) })
				_, err = command.Attach(ctx, w.actor, kind, w.entity, request)
				require.ErrorIs(t, err, failure)
				require.True(t, seenMove)
				require.Equal(t, before, w.snapshot(t))
				require.Zero(t, notifications)
			})
		}
	}
}

func TestSingleManualAttach2665DBCarAdoptsTargetPlacesOnly(t *testing.T) {
	for _, newAttachment := range []bool{false, true} {
		label := "existing"
		if newAttachment {
			label = "new"
		}
		t.Run(label, func(t *testing.T) {
			w := setupManualAttach2665World(t, services.ElementCar)
			command := singleAttach2665Service(w)
			ctx := context.Background()
			own := models.UnloadPlace{Name: "Тестовое место выбранной машины", IsActive: true}
			other := models.UnloadPlace{Name: "Тестовое место соседней машины", IsActive: true}
			target := models.UnloadPlace{Name: "Тестовое место вложения", IsActive: true}
			for _, place := range []*models.UnloadPlace{&own, &other, &target} {
				require.NoError(t, w.db.Create(place).Error)
			}
			ownLink := models.CarUnloadPlace{CarID: w.entity, UnloadPlaceID: own.ID}
			otherLink := models.CarUnloadPlace{CarID: w.individual, UnloadPlaceID: other.ID}
			require.NoError(t, w.db.Create(&ownLink).Error)
			require.NoError(t, w.db.Create(&otherLink).Error)
			for _, place := range []models.AttachmentUnloadPlace{{AttachmentID: w.orphan, UnloadPlaceID: own.ID}, {AttachmentID: w.orphan, UnloadPlaceID: other.ID}, {AttachmentID: w.attachment, UnloadPlaceID: target.ID}} {
				p := place
				require.NoError(t, w.db.Create(&p).Error)
			}
			var orphanBefore, targetBefore string
			require.NoError(t, w.db.Raw("SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM attachment_unload_places p WHERE attachment_id=?", w.orphan).Scan(&orphanBefore).Error)
			require.NoError(t, w.db.Raw("SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM attachment_unload_places p WHERE attachment_id=?", w.attachment).Scan(&targetBefore).Error)
			guard := models.User{Username: "single_attach_guard", Password: "x", TypeID: 1, IsActive: true}
			require.NoError(t, w.db.Create(&guard).Error)
			require.NoError(t, w.db.Create(&models.SecurityUserUnloadPlace{UserID: guard.ID, UnloadPlaceID: own.ID}).Error)
			apps := services.NewApplicationService(w.db, nil, nil, nil, nil, services.NewAuditRecorder(w.db))
			beforeVisible, err := apps.CanSecurityViewAttachment(ctx, guard.ID, false, w.attachment)
			require.NoError(t, err)
			require.False(t, beforeVisible)
			request := singleAttach2665Request(w, newAttachment)
			preview, err := command.Preview(ctx, w.actor, w.kind, w.entity, request)
			require.NoError(t, err)
			request.ExpectedRevision = preview.Revision
			result, err := command.Attach(ctx, w.actor, w.kind, w.entity, request)
			require.NoError(t, err)
			expected := target.ID
			if newAttachment {
				expected = own.ID
			}
			var ids []int
			require.NoError(t, w.db.Model(&models.CarUnloadPlace{}).Where("car_id=?", w.entity).Order("unload_place_id").Pluck("unload_place_id", &ids).Error)
			require.Equal(t, []int{expected}, ids)
			ids = nil
			require.NoError(t, w.db.Model(&models.CarUnloadPlace{}).Where("car_id=?", w.individual).Pluck("unload_place_id", &ids).Error)
			require.Equal(t, []int{other.ID}, ids)
			ids = nil
			require.NoError(t, w.db.Model(&models.AttachmentUnloadPlace{}).Where("attachment_id=?", *result.DestinationAttachmentID).Pluck("unload_place_id", &ids).Error)
			require.Equal(t, []int{expected}, ids)
			var orphanAfter, targetAfter string
			require.NoError(t, w.db.Raw("SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM attachment_unload_places p WHERE attachment_id=?", w.orphan).Scan(&orphanAfter).Error)
			require.NoError(t, w.db.Raw("SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM attachment_unload_places p WHERE attachment_id=?", w.attachment).Scan(&targetAfter).Error)
			require.Equal(t, orphanBefore, orphanAfter)
			require.Equal(t, targetBefore, targetAfter)
			afterVisible, err := apps.CanSecurityViewAttachment(ctx, guard.ID, false, w.attachment)
			require.NoError(t, err)
			require.Equal(t, beforeVisible, afterVisible, "existing target audience never expands")
			if newAttachment {
				visible, err := apps.CanSecurityViewAttachment(ctx, guard.ID, false, *result.DestinationAttachmentID)
				require.NoError(t, err)
				require.True(t, visible)
			}
		})
	}
}
