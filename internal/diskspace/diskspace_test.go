package diskspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUsage_UsedBytesAndPercent(t *testing.T) {
	cases := []struct {
		name        string
		usage       Usage
		wantUsed    int64
		wantPercent float64
	}{
		{"половина занята", Usage{TotalBytes: 1000, FreeBytes: 500}, 500, 50},
		{"всё свободно", Usage{TotalBytes: 1000, FreeBytes: 1000}, 0, 0},
		{"total неизвестен", Usage{TotalBytes: 0, FreeBytes: 0}, 0, 0},
		{"free больше total (гонка снимков)", Usage{TotalBytes: 100, FreeBytes: 200}, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.usage.UsedBytes(); got != tc.wantUsed {
				t.Errorf("UsedBytes() = %d, want %d", got, tc.wantUsed)
			}
			if got := tc.usage.UsedPercent(); got != tc.wantPercent {
				t.Errorf("UsedPercent() = %v, want %v", got, tc.wantPercent)
			}
		})
	}
}

func TestStatfs_ExistingDir(t *testing.T) {
	dir := t.TempDir()
	usage, err := Statfs(dir)
	if err != nil {
		t.Fatalf("Statfs() error = %v", err)
	}
	if usage.TotalBytes <= 0 {
		t.Errorf("TotalBytes = %d, want > 0", usage.TotalBytes)
	}
	if usage.FreeBytes <= 0 {
		t.Errorf("FreeBytes = %d, want > 0", usage.FreeBytes)
	}
	if usage.Device == 0 {
		t.Error("Device = 0, want a real device id")
	}
}

// TestStatfs_MissingDirFallsBackToAncestor проверяет, что каталог, который ещё не
// создан (архив до первой выгрузки), не роняет вызов - используется статистика
// ближайшего существующего предка, а раздел от этого не меняется.
func TestStatfs_MissingDirFallsBackToAncestor(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "2026", "07", "31")

	usage, err := Statfs(missing)
	if err != nil {
		t.Fatalf("Statfs() error = %v", err)
	}
	rootUsage, err := Statfs(root)
	if err != nil {
		t.Fatalf("Statfs(root) error = %v", err)
	}
	if usage.Device != rootUsage.Device {
		t.Errorf("Device = %d, want %d (тот же раздел, что и у существующего предка)", usage.Device, rootUsage.Device)
	}
}

func TestCollect_DedupesSameDevice(t *testing.T) {
	root := t.TempDir()
	archiveDir := filepath.Join(root, "archive")
	uploadsDir := filepath.Join(root, "uploads")
	if err := os.MkdirAll(archiveDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(uploadsDir, 0o750); err != nil {
		t.Fatal(err)
	}

	partitions := Collect([]Dir{
		{Label: "Архив", Path: archiveDir},
		{Label: "Загрузки", Path: uploadsDir},
	})

	if len(partitions) != 1 {
		t.Fatalf("len(partitions) = %d, want 1 (оба каталога на одном разделе теста)", len(partitions))
	}
	if got := partitions[0].Labels; len(got) != 2 || got[0] != "Архив" || got[1] != "Загрузки" {
		t.Errorf("Labels = %v, want [Архив Загрузки]", got)
	}
}

// TestCollect_SkipsEmptyPath - незаданный каталог (например, ЛОГИ не настроены,
// LOG_FILE_PATH пуст) не должен попадать в список разделов вовсе, а не всплывать
// как "раздел корня" через откат к ближайшему предку.
func TestCollect_SkipsEmptyPath(t *testing.T) {
	partitions := Collect([]Dir{{Label: "Логи", Path: ""}})
	if len(partitions) != 0 {
		t.Fatalf("len(partitions) = %d, want 0", len(partitions))
	}
}

// TestCollect_MissingButResolvableDirCounts - каталог, которого ещё нет на диске,
// но чей предок существует (архив до первой записи), попадает в список разделов
// через откат к предку, а не пропускается: диск под ним реален уже сейчас.
func TestCollect_MissingButResolvableDirCounts(t *testing.T) {
	root := t.TempDir()
	partitions := Collect([]Dir{{Label: "Архив", Path: filepath.Join(root, "2026", "07")}})
	if len(partitions) != 1 {
		t.Fatalf("len(partitions) = %d, want 1", len(partitions))
	}
}

func TestDirSize_SumsRegularFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("12345"), 0o640); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("1234567890"), 0o640); err != nil {
		t.Fatal(err)
	}

	got, err := DirSize(dir)
	if err != nil {
		t.Fatalf("DirSize() error = %v", err)
	}
	if want := int64(5 + 10); got != want {
		t.Errorf("DirSize() = %d, want %d", got, want)
	}
}

func TestDirSize_MissingDirIsZeroNotError(t *testing.T) {
	got, err := DirSize(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("DirSize() error = %v, want nil", err)
	}
	if got != 0 {
		t.Errorf("DirSize() = %d, want 0", got)
	}
}
