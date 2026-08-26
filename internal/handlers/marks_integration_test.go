package handlers_test

// Интеграционные тесты MarkService (#185).
// Запускаются с реальной PG через testutil.SetupTestApp.

import (
	"context"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
)

func uniqMarkName(prefix string) string {
	return prefix + "-mark-" + intStr(int(time.Now().UnixNano()%100000))
}

func TestMarkService_CRUD(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewMarkService(db)

	userID, _, cleanup := setupMWUser(t, db, true, false)
	defer cleanup()
	ctx := context.Background()

	name := uniqMarkName("e2e")
	mark, err := svc.Create(ctx, models.CreateMarkRequest{Name: name}, userID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !mark.IsActive || mark.Name != name {
		t.Errorf("unexpected mark: %+v", mark)
	}

	if _, err := svc.Create(ctx, models.CreateMarkRequest{Name: name}, userID); err == nil {
		t.Error("expected conflict on duplicate name")
	}

	newName := name + "-updated"
	if err := svc.Update(ctx, mark.ID, models.UpdateMarkRequest{Name: newName}, userID); err != nil {
		t.Fatalf("update: %v", err)
	}

	hist, err := svc.GetHistory(ctx, mark.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(hist) != 2 {
		t.Errorf("expected 2 history entries (created+renamed), got %d", len(hist))
	}
	if hist[0].ActionType != models.MarkActionRenamed {
		t.Errorf("expected newest event=renamed, got %s", hist[0].ActionType)
	}

	if err := svc.Archive(ctx, mark.ID, userID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	got, _ := svc.GetByID(ctx, mark.ID)
	if got.IsActive {
		t.Error("expected is_active=false after archive")
	}

	all, _ := svc.GetAll(ctx, false)
	for _, m := range all {
		if m.ID == mark.ID {
			t.Error("archived mark не должен возвращаться без include_archived")
		}
	}
	allWithArchive, _ := svc.GetAll(ctx, true)
	found := false
	for _, m := range allWithArchive {
		if m.ID == mark.ID {
			found = true
		}
	}
	if !found {
		t.Error("archived mark должен возвращаться с include_archived=true")
	}

	if err := svc.Restore(ctx, mark.ID, userID); err != nil {
		t.Fatalf("restore: %v", err)
	}
	got, _ = svc.GetByID(ctx, mark.ID)
	if !got.IsActive {
		t.Error("expected is_active=true after restore")
	}

	hist, _ = svc.GetHistory(ctx, mark.ID)
	if len(hist) != 4 {
		t.Errorf("expected 4 history entries, got %d", len(hist))
	}

	db.Delete(&models.Mark{}, mark.ID)
}

func TestMarkService_PartialUnique_ArchivedNameReusable(t *testing.T) {
	// #FF-marks: partial-unique только среди активных - архивную марку можно
	// пересоздать активной с тем же именем; восстановление при конфликте даёт 409.
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewMarkService(db)
	userID, _, cleanup := setupMWUser(t, db, true, false)
	defer cleanup()
	ctx := context.Background()

	name := uniqMarkName("partial")
	first, err := svc.Create(ctx, models.CreateMarkRequest{Name: name}, userID)
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if err := svc.Archive(ctx, first.ID, userID); err != nil {
		t.Fatalf("archive: %v", err)
	}

	// Создание активной с тем же именем при архивной - должно пройти (был баг 409).
	second, err := svc.Create(ctx, models.CreateMarkRequest{Name: name}, userID)
	if err != nil {
		t.Fatalf("expected reuse of archived name to succeed, got: %v", err)
	}
	if second.ID == first.ID {
		t.Error("expected a new mark, got same id")
	}

	// Восстановление архивной при существующей активной - 409, а не 500.
	err = svc.Restore(ctx, first.ID, userID)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusConflict {
		t.Errorf("expected 409 conflict on restore with active duplicate, got %v", err)
	}

	db.Delete(&models.Mark{}, []int{first.ID, second.ID})
}

func TestMarkService_UpdateSameNameIsNoop(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewMarkService(db)

	userID, _, cleanup := setupMWUser(t, db, true, false)
	defer cleanup()
	ctx := context.Background()

	name := uniqMarkName("noop")
	mark, _ := svc.Create(ctx, models.CreateMarkRequest{Name: name}, userID)

	if err := svc.Update(ctx, mark.ID, models.UpdateMarkRequest{Name: name}, userID); err != nil {
		t.Fatalf("update same name: %v", err)
	}

	hist, _ := svc.GetHistory(ctx, mark.ID)
	if len(hist) != 1 {
		t.Errorf("expected 1 history entry (only created), got %d", len(hist))
	}

	db.Delete(&models.Mark{}, mark.ID)
}
