package reportpresets

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Источник наборов один - файл фронта; копия здесь нужна лишь потому, что образ
// фронтенда собирается из каталога frontend и Go-пакет туда не попадает. Тест
// стережёт их совпадение: разойдясь однажды, списки живут врозь годами (#2315).
func TestReportPresets_FrontAndBackInSync(t *testing.T) {
	frontPath := filepath.Join("..", "..", "frontend", "src", "components", "statistics", "reportPresets.json")
	front, err := os.ReadFile(frontPath)
	if err != nil {
		t.Fatalf("не прочитан источник наборов %s: %v", frontPath, err)
	}
	if !bytes.Equal(bytes.TrimSpace(front), bytes.TrimSpace(Raw())) {
		t.Fatalf("копия наборов разошлась с источником - выполните `make sync-presets`")
	}
}

func TestReportPresets_Parsed(t *testing.T) {
	presets, err := All()
	if err != nil {
		t.Fatalf("наборы не разобрались: %v", err)
	}
	if len(presets) < 5 {
		t.Fatalf("ожидался непустой список наборов, got %d", len(presets))
	}
	seen := map[string]bool{}
	for _, p := range presets {
		if p.Title == "" || p.Description == "" || len(p.Form) == 0 {
			t.Errorf("набор %q неполон: title=%q description=%q form=%s", p.ID, p.Title, p.Description, p.Form)
		}
		if seen[p.Title] {
			t.Errorf("название набора %q повторяется - в базе они различаются по имени", p.Title)
		}
		seen[p.Title] = true
	}
}
