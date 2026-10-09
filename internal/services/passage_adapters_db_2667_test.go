package services_test

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http"
	"sync/atomic"
	"systemburo/internal/models"
	"systemburo/internal/realtime"
	"systemburo/internal/services"
	"testing"
	"time"
)

type passage2667Publisher struct {
	count   int
	onEvent func()
}

func (p *passage2667Publisher) Publish(_ int, event realtime.Event) { p.PublishMany(nil, event) }
func (p *passage2667Publisher) PublishMany(_ []int, event realtime.Event) {
	if event.Type == "tables.refresh" {
		p.count++
		if p.onEvent != nil {
			p.onEvent()
		}
	}
}

func TestPassage2667DBTableCorrectionRejectsUnavailableFoundation(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, state := range []string{"withdrawn", "archived", "removed"} {
			t.Run(string(kind)+"/"+state, func(t *testing.T) {
				w := setupPassage2667World(t, kind)
				ctx := context.Background()
				var attachment models.Attachment
				require.NoError(t, w.db.First(&attachment, w.attachment).Error)
				switch state {
				case "withdrawn":
					require.NoError(t, w.db.Model(&models.Application{}).Where("id=?", attachment.ApplicationID).Update("status", models.StatusWithdrawn).Error)
				case "archived":
					date := time.Now().In(services.MoscowLocation()).AddDate(0, -3, 0).Format("2006-01-02")
					require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).Updates(map[string]any{"entry_date_from": date, "entry_date_to": date}).Error)
					require.NoError(t, w.db.Model(&models.Application{}).Where("id=?", attachment.ApplicationID).Update("status", models.StatusCompleted).Error)
				case "removed":
					column := "date_deleted"
					if kind == services.ElementCar {
						column = "date_removed"
					}
					require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.id).Update(column, time.Now().UTC()).Error)
				}
				before := w.snapshot(t)
				var publishes atomic.Int32
				commands := services.NewPassageCommandService(w.db, services.NewAuditRecorder(w.db))
				commands.SetAfterChange(func(context.Context, services.ElementKind, int) { publishes.Add(1) })
				_, err := commands.Correct(ctx, w.actor, kind, w.id, w.correction())
				passage2667HTTP(t, err, http.StatusForbidden)
				require.Equal(t, before, w.snapshot(t))
				require.Zero(t, publishes.Load())
				req := w.correction()
				req.Source = "admin_summary"
				req.TableID = nil
				closed, err := commands.Correct(ctx, w.actor, kind, w.id, req)
				require.NoError(t, err)
				require.False(t, closed.PassageState.Open)
				require.False(t, closed.Admission.CanEnter)
				require.False(t, closed.Admission.CanExit)
				require.EqualValues(t, 1, publishes.Load())
				req.ExpectedLastEventID = &closed.PassageState.LastEventID
				req.Source = "table"
				req.TableID = &w.table
				_, err = commands.RevertCorrection(ctx, w.actor, kind, w.id, req)
				passage2667HTTP(t, err, http.StatusForbidden)
				require.EqualValues(t, 1, publishes.Load())
			})
		}
	}
}

// Insert a committed event immediately before the first permission SELECT. A
// clock captured before that SELECT necessarily predates this visible event.
func TestPassage2667DBListClockFollowsSnapshot(t *testing.T) {
	w := setupPassage2667World(t, services.ElementEmployee)
	var inserted atomic.Bool
	var writeErr error
	var eventAt time.Time
	name := "passage2667_clock_boundary"
	require.NoError(t, w.db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table != "users" || !inserted.CompareAndSwap(false, true) {
			return
		}
		eventAt = time.Now().UTC()
		writeErr = w.db.Model(&models.AuditLog{}).Where("id=?", w.entryID).Update("created_at", eventAt).Error
	}))
	t.Cleanup(func() { w.db.Callback().Query().Remove(name) })
	list, err := services.NewPassageOpenService(w.db).List(context.Background(), w.actor, w.kind, &w.table, services.PassageOpenFilter{})
	require.True(t, inserted.Load())
	require.NoError(t, writeErr)
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.False(t, list.ServerNow.Before(eventAt))
	require.WithinDuration(t, eventAt, *list.Items[0].PassageState.EntryAt, time.Microsecond)
}

func TestPassage2667DBLegacyAdaptersFirstEntryRevertAndFactPass(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPassage2667World(t, kind)
			ctx := context.Background()
			require.NoError(t, w.db.Delete(&models.AuditLog{}, w.entryID).Error)
			require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.id).Updates(map[string]any{"territory_status": 0, "status": 1}).Error)
			clock := time.Now().UTC()
			from, to := clock.In(services.MoscowLocation()).AddDate(0, 0, -1).Format("2006-01-02"), clock.In(services.MoscowLocation()).AddDate(0, 0, 1).Format("2006-01-02")
			require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.attachment).Updates(map[string]any{"status": 1, "entry_date_from": from, "entry_date_to": to}).Error)
			req := services.UpdateTerritoryStatusRequest{UserID: &w.actor, TableID: &w.table, TerritoryStatus: 1}
			recorder := services.NewAuditRecorder(w.db)
			publisher := &passage2667Publisher{}
			publisher.onEvent = func() {
				var count int64
				require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_id=? AND action IN ('entry','entry_revert')", w.id).Count(&count).Error)
				require.EqualValues(t, publisher.count, count)
			}
			producer := services.NewTablesRefreshPublisher(w.db, services.NewPermissionResolver(w.db), publisher)
			var err error
			if kind == services.ElementCar {
				err = services.NewCarService(w.db, recorder, services.WithCarTablesProducer(producer)).UpdateCarTerritoryStatus(ctx, w.id, services.UpdateCarTerritoryStatusRequest{UpdateTerritoryStatusRequest: req, Pass: &services.FactPassData{Number: "FIXTURE2667"}})
			} else {
				err = services.NewEmployeeService(w.db, recorder, services.WithEmployeeTablesProducer(producer)).UpdateEmployeeTerritoryStatus(ctx, w.id, req)
			}
			require.NoError(t, err)
			require.Equal(t, 1, publisher.count)
			var audit models.AuditLog
			require.NoError(t, w.db.Where("entity_id=? AND action='entry'", w.id).First(&audit).Error)
			if kind == services.ElementCar {
				var details map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(audit.Details, &details))
				var fact services.FactPassData
				require.NoError(t, json.Unmarshal(details["metadata"], &fact))
				require.Equal(t, "FIXTURE2667", fact.Number)
			}
			expected := int64(audit.ID)
			revert := services.RevertPassageRequest{ActorUserID: w.actor, TableID: &w.table, TerritoryStatus: 1, ExpectedLastEventID: &expected, Reason: "Ошибочный вход"}
			if kind == services.ElementCar {
				err = services.NewCarService(w.db, recorder, services.WithCarTablesProducer(producer)).RevertCarPassage(ctx, w.id, revert)
			} else {
				err = services.NewEmployeeService(w.db, recorder, services.WithEmployeeTablesProducer(producer)).RevertEmployeePassage(ctx, w.id, revert)
			}
			require.NoError(t, err)
			require.Equal(t, 2, publisher.count)
			var cached struct{ TerritoryStatus *int }
			require.NoError(t, w.db.Table(string(kind)).Select("territory_status").Where("id=?", w.id).Scan(&cached).Error)
			require.Nil(t, cached.TerritoryStatus)
			states, err := services.LoadPassageStates(ctx, w.db, kind, []int{w.id}, time.Now().UTC())
			require.NoError(t, err)
			require.False(t, states[w.id].Open)
			require.False(t, states[w.id].HasEvent)
			scope := services.ElementScope{All: true}
			if kind == services.ElementCar {
				rows, e := services.NewCarService(w.db, recorder).GetCarsCurrentStatus(ctx, w.actor, scope)
				require.NoError(t, e)
				require.Len(t, rows, 1)
				require.False(t, rows[0].PassageState.Open)
				require.False(t, rows[0].CanRevert)
				require.Zero(t, rows[0].TerritoryStatus)
			} else {
				rows, e := services.NewEmployeesHistoryService(w.db).GetCurrentStatus(ctx, w.actor, scope)
				require.NoError(t, e)
				require.Len(t, rows, 1)
				require.False(t, rows[0].PassageState.Open)
				require.False(t, rows[0].CanRevert)
				require.Zero(t, rows[0].TerritoryStatus)
			}
		})
	}
}

func TestPassage2667DBEmployeeReaderRetainsPresenceAcrossDedup(t *testing.T) {
	w := setupPassage2667World(t, services.ElementEmployee)
	ctx := context.Background()
	recorder := services.NewAuditRecorder(w.db)
	employees := services.NewEmployeeService(w.db, recorder)
	var orphan models.Attachment
	require.NoError(t, w.db.First(&orphan, w.attachment).Error)
	clock := time.Now().UTC()
	from, to := clock.In(services.MoscowLocation()).Format("2006-01-02"), clock.In(services.MoscowLocation()).AddDate(0, 0, 2).Format("2006-01-02")
	active := 1
	newer := models.Attachment{ApplicationID: orphan.ApplicationID, AttachmentType: models.TableTypePeople, Status: &active, EntryDateFrom: &from, EntryDateTo: &to}
	require.NoError(t, w.db.Create(&newer).Error)
	name := "Fixture2667"
	other := models.Employee{AttachmentID: &newer.ID, LastName: &name, Status: &active}
	require.NoError(t, w.db.Create(&other).Error)
	require.NoError(t, w.db.Exec("INSERT INTO employee_target_tables(employee_id,table_id) VALUES(?,?)", other.ID, w.table).Error)
	require.NoError(t, w.db.Table("employees").Where("id IN ?", []int{w.id, other.ID}).Update("passport_series_number_hmac", "synthetic2667duplicate").Error)
	rows, err := employees.GetActiveEmployeesForTable(ctx, w.table)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	var retained *services.TableEmployeeResponse
	for i := range rows {
		if rows[i].ID == w.id {
			retained = &rows[i]
		}
	}
	require.NotNil(t, retained)
	require.True(t, retained.PassageState.Open)
	require.True(t, retained.PassageState.NeedsAttention)
	require.EqualValues(t, 1, *retained.TerritoryStatus)
	require.True(t, *rows[0].EffectivePeriod.EntryDateTo == to || *rows[1].EffectivePeriod.EntryDateTo == to)
	// The former open row stays for exactly the departure grace, never deduped
	// against a newer admission for the same passport.
	closed, err := services.NewPassageCommandService(w.db, recorder).Mark(ctx, w.actor, w.kind, w.id, services.PassageCommandRequest{TableID: &w.table, TerritoryStatus: 2, ExpectedLastEventID: &w.entryID})
	require.NoError(t, err)
	rows, err = employees.GetActiveEmployeesForTable(ctx, w.table)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.NoError(t, w.db.Model(&models.AuditLog{}).Where("id=?", closed.PassageState.LastEventID).Update("created_at", time.Now().UTC().Add(-6*time.Minute)).Error)
	rows, err = employees.GetActiveEmployeesForTable(ctx, w.table)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, other.ID, rows[0].ID)
	require.NoError(t, w.db.Model(&models.Application{}).Where("id=?", orphan.ApplicationID).Update("status", models.StatusWithdrawn).Error)
	rows, err = employees.GetActiveEmployeesForTable(ctx, w.table)
	require.NoError(t, err)
	require.Empty(t, rows)
}
