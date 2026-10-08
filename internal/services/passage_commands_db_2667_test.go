package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http"
	"sync"
	"sync/atomic"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
	"testing"
	"time"
)

type passage2667World struct {
	db                           *gorm.DB
	kind                         services.ElementKind
	actor, id, table, attachment int
	entryID                      int64
	entered                      time.Time
}

func setupPassage2667World(t *testing.T, kind services.ElementKind) passage2667World {
	t.Helper()
	_, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	seed := testutil.SeedTestData(t, db)
	actor := models.User{Username: "passage2667_actor", Password: "x", TypeID: 1, OrganizationID: &seed.OrgID, IsActive: true, IsAdmin: true}
	require.NoError(t, db.Create(&actor).Error)
	now := time.Now().UTC()
	status, confirmation := models.StatusInWork, models.ConfirmationApproved
	app := models.Application{OrganizationID: seed.OrgID, SenderUserID: actor.ID, Status: &status, Confirmation: &confirmation, SendingDatetime: &now}
	require.NoError(t, db.Create(&app).Error)
	from, to := now.In(services.MoscowLocation()).AddDate(0, 0, -5).Format("2006-01-02"), now.In(services.MoscowLocation()).AddDate(0, 0, -1).Format("2006-01-02")
	inactive, inside := 0, 1
	attachmentType := models.TableTypePeople
	if kind == services.ElementCar {
		attachmentType = models.TableTypeCars
	}
	attachment := models.Attachment{ApplicationID: &app.ID, AttachmentType: attachmentType, Status: &inactive, EntryDateFrom: &from, EntryDateTo: &to}
	require.NoError(t, db.Create(&attachment).Error)
	table := models.SystemTable{Name: "passage2667_post", TableType: attachmentType, IsActive: true, Status: "active"}
	require.NoError(t, db.Create(&table).Error)
	w := passage2667World{db: db, kind: kind, actor: actor.ID, table: table.ID, attachment: attachment.ID, entered: now.Add(-49 * time.Hour)}
	entityType := models.AuditEntityEmployee
	if kind == services.ElementCar {
		plate := "TEST2667"
		entity := models.Car{AttachmentID: attachment.ID, CarNumber: &plate, Status: &inactive, TerritoryStatus: &inside}
		require.NoError(t, db.Create(&entity).Error)
		w.id = entity.ID
		require.NoError(t, db.Exec("INSERT INTO car_target_tables(car_id,table_id) VALUES(?,?)", w.id, w.table).Error)
		entityType = models.AuditEntityCar
	} else {
		name := "Fixture2667"
		entity := models.Employee{AttachmentID: &attachment.ID, LastName: &name, Status: &inactive, TerritoryStatus: &inside}
		require.NoError(t, db.Create(&entity).Error)
		w.id = entity.ID
		require.NoError(t, db.Exec("INSERT INTO employee_target_tables(employee_id,table_id) VALUES(?,?)", w.id, w.table).Error)
	}
	details, err := json.Marshal(map[string]any{"table_id": w.table})
	require.NoError(t, err)
	entry := models.AuditLog{EntityType: entityType, EntityID: &w.id, ActorUserID: &w.actor, Action: "entry", CreatedAt: w.entered, Details: details}
	require.NoError(t, db.Create(&entry).Error)
	w.entryID = int64(entry.ID)
	return w
}
func passage2667HTTP(t *testing.T, err error, status int) {
	t.Helper()
	var e *echo.HTTPError
	require.ErrorAs(t, err, &e)
	require.Equal(t, status, e.Code)
}
func (w passage2667World) snapshot(t *testing.T) string {
	t.Helper()
	var value string
	require.NoError(t, w.db.Raw("SELECT to_jsonb(e)::text FROM "+string(w.kind)+" e WHERE id=?", w.id).Scan(&value).Error)
	return value
}
func (w passage2667World) correction() services.PassageCorrectionRequest {
	return services.PassageCorrectionRequest{Source: "table", TableID: &w.table, ExpectedLastEventID: &w.entryID, Reason: "Проверка закрытия учёта"}
}

func TestPassage2667DBExpiredExitCorrectionAndReload(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPassage2667World(t, kind)
			ctx := context.Background()
			commands := services.NewPassageCommandService(w.db, services.NewAuditRecorder(w.db))
			list := services.NewPassageOpenService(w.db)
			var published atomic.Int32
			commands.SetAfterChange(func(context.Context, services.ElementKind, int) { published.Add(1) })
			open, err := list.List(ctx, w.actor, kind, &w.table, services.PassageOpenFilter{AttentionOnly: true})
			require.NoError(t, err)
			require.EqualValues(t, 1, open.Total)
			require.EqualValues(t, 1, open.Counts.Attention)
			require.Len(t, open.Items, 1)
			require.True(t, open.Items[0].Admission.CanExit)
			require.False(t, open.Items[0].Admission.CanEnter)
			_, err = commands.Mark(ctx, w.actor, kind, w.id, services.PassageCommandRequest{TableID: &w.table, TerritoryStatus: 1, ExpectedLastEventID: &w.entryID})
			passage2667HTTP(t, err, http.StatusUnprocessableEntity)
			require.EqualValues(t, 0, published.Load())
			exit, err := commands.Mark(ctx, w.actor, kind, w.id, services.PassageCommandRequest{TableID: &w.table, TerritoryStatus: 2, ExpectedLastEventID: &w.entryID})
			require.NoError(t, err)
			require.False(t, exit.PassageState.Open)
			require.True(t, exit.PassageState.InExitGrace)
			require.False(t, exit.Admission.CanEnter)
			restored, err := commands.Revert(ctx, w.actor, kind, w.id, services.PassageCommandRequest{TableID: &w.table, TerritoryStatus: 2, ExpectedLastEventID: &exit.PassageState.LastEventID, Reason: "Ошибочная отметка"})
			require.NoError(t, err)
			require.True(t, restored.PassageState.Open)
			require.WithinDuration(t, w.entered, *restored.PassageState.EntryAt, time.Microsecond)
			correction, err := commands.Correct(ctx, w.actor, kind, w.id, w.correction())
			require.NoError(t, err)
			require.False(t, correction.PassageState.Open)
			require.True(t, correction.PassageState.CanRevertCorrection)
			recent, err := list.List(ctx, w.actor, kind, &w.table, services.PassageOpenFilter{View: "corrections"})
			require.NoError(t, err)
			require.EqualValues(t, 1, recent.Total)
			require.Zero(t, recent.Counts.AllOpen)
			require.True(t, recent.Items[0].PassageState.CanRevertCorrection)
			req := w.correction()
			req.ExpectedLastEventID = &correction.PassageState.LastEventID
			restored, err = commands.RevertCorrection(ctx, w.actor, kind, w.id, req)
			require.NoError(t, err)
			require.True(t, restored.PassageState.Open)
			require.WithinDuration(t, w.entered, *restored.PassageState.EntryAt, time.Microsecond)
			require.EqualValues(t, 4, published.Load())
		})
	}
}

type failingPassage2667Audit struct{ services.AuditRecorder }

func (f failingPassage2667Audit) Record(context.Context, *gorm.DB, string, *int, string, *int, interface{}) error {
	return errors.New("synthetic audit failure")
}
func TestPassage2667DBRollbackAndFreshDeny(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPassage2667World(t, kind)
			ctx := context.Background()
			before := w.snapshot(t)
			var publishes atomic.Int32
			commands := services.NewPassageCommandService(w.db, failingPassage2667Audit{services.NewAuditRecorder(w.db)})
			commands.SetAfterChange(func(context.Context, services.ElementKind, int) { publishes.Add(1) })
			_, err := commands.Correct(ctx, w.actor, kind, w.id, w.correction())
			require.ErrorContains(t, err, "synthetic audit failure")
			require.Equal(t, before, w.snapshot(t))
			require.Zero(t, publishes.Load())
			commands = services.NewPassageCommandService(w.db, services.NewAuditRecorder(w.db))
			deny := models.UserPermissionOverride{UserID: w.actor, PermissionKey: "detail.passage.correct", Value: "deny", GrantedAt: time.Now().UTC()}
			require.NoError(t, w.db.Create(&deny).Error)
			_, err = commands.Correct(ctx, w.actor, kind, w.id, w.correction())
			passage2667HTTP(t, err, http.StatusForbidden)
			require.Equal(t, before, w.snapshot(t))
			require.NoError(t, w.db.Delete(&deny).Error)
			require.NoError(t, w.db.Model(&models.User{}).Where("id=?", w.actor).Update("is_active", false).Error)
			_, err = commands.Correct(ctx, w.actor, kind, w.id, w.correction())
			passage2667HTTP(t, err, http.StatusForbidden)
			require.Equal(t, before, w.snapshot(t))
		})
	}
}

func TestPassage2667DBConcurrentCorrection(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPassage2667World(t, kind)
			commands := services.NewPassageCommandService(w.db, services.NewAuditRecorder(w.db))
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			var published atomic.Int32
			commands.SetAfterChange(func(context.Context, services.ElementKind, int) { published.Add(1) })
			ready := make(chan struct{}, 2)
			release := make(chan struct{})
			results := make(chan error, 2)
			var workers sync.WaitGroup
			workers.Add(2)
			for i := 0; i < 2; i++ {
				go func() {
					defer workers.Done()
					ready <- struct{}{}
					select {
					case <-release:
					case <-ctx.Done():
						results <- ctx.Err()
						return
					}
					_, err := commands.Correct(ctx, w.actor, kind, w.id, w.correction())
					results <- err
				}()
			}
			<-ready
			<-ready
			close(release)
			workers.Wait()
			close(results)
			successes, conflicts := 0, 0
			for err := range results {
				if err == nil {
					successes++
				} else {
					var e *echo.HTTPError
					if errors.As(err, &e) && e.Code == http.StatusConflict {
						conflicts++
					} else {
						t.Errorf("unexpected concurrent outcome: %v", err)
					}
				}
			}
			require.Equal(t, 1, successes)
			require.Equal(t, 1, conflicts)
			require.EqualValues(t, 1, published.Load())
			var count int64
			require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_id=? AND action=?", w.id, services.PassageCorrectionAction).Count(&count).Error)
			require.EqualValues(t, 1, count)
		})
	}
}
