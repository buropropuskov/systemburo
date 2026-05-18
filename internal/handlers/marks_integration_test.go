package handlers_test

// Интеграционные тесты MarkService (#185).
// Запускаются с реальной PG через testutil.SetupTestApp.

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
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

	db.Where("mark_id = ?", mark.ID).Delete(&models.MarkHistory{})
	db.Delete(&models.Mark{}, mark.ID)
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

	db.Where("mark_id = ?", mark.ID).Delete(&models.MarkHistory{})
	db.Delete(&models.Mark{}, mark.ID)
}
