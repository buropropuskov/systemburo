package database

import (
	"testing"

	"systemburo/internal/reportpresets"
)

// Переименование системных наборов (#2315) должно вести на реально существующий
// набор: промахнувшись именем, миграция оставила бы в базе запись-сироту, а рядом
// создала бы дубль - ровно то расхождение, ради устранения которого всё и делалось.
func TestRenamedSystemTemplates_TargetsExist(t *testing.T) {
	presets, err := reportpresets.All()
	if err != nil {
		t.Fatalf("наборы не разобрались: %v", err)
	}

	titles := make(map[string]bool, len(presets))
	for _, p := range presets {
		titles[p.Title] = true
	}

	for oldName, newName := range renamedSystemTemplates {
		if !titles[newName] {
			t.Errorf("переименование %q -> %q ведёт в никуда: такого набора нет в источнике", oldName, newName)
		}
		if titles[oldName] {
			t.Errorf("прежнее имя %q снова встречается среди наборов - переименование затрёт живую запись", oldName)
		}
	}
}
