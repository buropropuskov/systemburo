package services

import (
	"testing"

	"systemburo/internal/models"
)

func defByKey(defs []FieldDef, key string) (FieldDef, bool) {
	for _, d := range defs {
		if d.Key == key {
			return d, true
		}
	}
	return FieldDef{}, false
}

func mergedByKey(fields []models.MergedField, key string) (models.MergedField, bool) {
	for _, f := range fields {
		if f.Key == key {
			return f, true
		}
	}
	return models.MergedField{}, false
}

func TestFieldRegistryFor_GroupsByType(t *testing.T) {
	tests := []struct {
		attType   string
		wantKeys  []string // должны присутствовать
		absentKey []string // не должны
	}{
		{
			attType:   "people",
			wantKeys:  []string{"entry_date_from", "last_name", "passport", "target_tables"},
			absentKey: []string{"number", "mark", "item_name"},
		},
		{
			attType:   "cars",
			wantKeys:  []string{"entry_date_from", "number", "mark", "unloading_places", "passage_tables"},
			absentKey: []string{"last_name", "item_name"},
		},
		{
			attType:   "items",
			wantKeys:  []string{"entry_date_from", "item_name", "quantity"},
			absentKey: []string{"number", "last_name"},
		},
		{
			attType:   "unknown",
			wantKeys:  []string{"entry_date_from", "roof_access"},
			absentKey: []string{"number", "last_name", "item_name"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.attType, func(t *testing.T) {
			defs := FieldRegistryFor(tc.attType)
			for _, k := range tc.wantKeys {
				if _, ok := defByKey(defs, k); !ok {
					t.Errorf("FieldRegistryFor(%q): ожидался ключ %q", tc.attType, k)
				}
			}
			for _, k := range tc.absentKey {
				if _, ok := defByKey(defs, k); ok {
					t.Errorf("FieldRegistryFor(%q): не должно быть ключа %q", tc.attType, k)
				}
			}
		})
	}
}

func TestFieldRegistryFor_CommonFirst(t *testing.T) {
	defs := FieldRegistryFor("people")
	if len(defs) == 0 {
		t.Fatal("пустой реестр")
	}
	// common-поля идут до type-specific
	sawTypeGroup := false
	for _, d := range defs {
		if d.Group != FieldGroupCommon {
			sawTypeGroup = true
		} else if sawTypeGroup {
			t.Errorf("common-поле %q после type-specific - порядок нарушен", d.Key)
		}
	}
}

// Дефолты обязаны отражать ТЕКУЩЕЕ поведение форм (обязательство среза H-1).
func TestRegistryDefaults_MatchForms(t *testing.T) {
	cases := []struct {
		attType      string
		key          string
		wantVisible  bool
		wantRequired bool
		wantRequirbl bool
	}{
		// common: даты И время обязательны (CreateApplication.vue hasValidDates).
		{"people", "entry_date_from", true, true, true},
		{"people", "entry_time_from", true, true, true},
		{"people", "entry_time_to", true, true, true},
		// булевые чекбоксы - не обязуемы
		{"people", "roof_access", true, false, false},
		{"people", "free_parking", true, false, false},
		// people
		{"people", "last_name", true, true, true},
		{"people", "middle_name", true, false, true},
		{"people", "patent", true, false, true},
		{"people", "work_permission", true, false, true},
		// cars
		{"cars", "number", true, true, true},
		{"cars", "unloading_places", true, true, true},
		{"cars", "passage_tables", true, true, true},
		// items
		{"items", "quantity", true, true, true},
	}
	for _, c := range cases {
		t.Run(c.attType+"/"+c.key, func(t *testing.T) {
			d, ok := defByKey(FieldRegistryFor(c.attType), c.key)
			if !ok {
				t.Fatalf("нет поля %q у типа %q", c.key, c.attType)
			}
			if d.DefaultVisible != c.wantVisible {
				t.Errorf("%s: visible=%v, ожидалось %v", c.key, d.DefaultVisible, c.wantVisible)
			}
			if d.DefaultRequired != c.wantRequired {
				t.Errorf("%s: required=%v, ожидалось %v", c.key, d.DefaultRequired, c.wantRequired)
			}
			if d.Requirable != c.wantRequirbl {
				t.Errorf("%s: requirable=%v, ожидалось %v", c.key, d.Requirable, c.wantRequirbl)
			}
		})
	}
}

func TestMergeFieldConfig_DefaultsWhenNoOverrides(t *testing.T) {
	merged := MergeFieldConfig("cars", nil)
	if len(merged) != len(FieldRegistryFor("cars")) {
		t.Fatalf("merged=%d, реестр=%d", len(merged), len(FieldRegistryFor("cars")))
	}
	number, ok := mergedByKey(merged, "number")
	if !ok {
		t.Fatal("нет number")
	}
	if !number.Visible || !number.Required {
		t.Errorf("number дефолт: visible=%v required=%v, ожидалось true/true", number.Visible, number.Required)
	}
}

func TestMergeFieldConfig_OverrideApplied(t *testing.T) {
	overrides := []models.AttachmentFieldConfig{
		{FieldKey: "passport", Visible: false, Required: false},
		{FieldKey: "middle_name", Visible: true, Required: true},
	}
	merged := MergeFieldConfig("people", overrides)

	passport, _ := mergedByKey(merged, "passport")
	if passport.Visible || passport.Required {
		t.Errorf("passport оверрайд не применён: visible=%v required=%v", passport.Visible, passport.Required)
	}
	mid, _ := mergedByKey(merged, "middle_name")
	if !mid.Required {
		t.Errorf("middle_name required оверрайд не применён: required=%v", mid.Required)
	}
	// не затронутое поле остаётся дефолтным
	if lastName, _ := mergedByKey(merged, "last_name"); !lastName.Required {
		t.Error("last_name должен остаться required по дефолту")
	}
}

func TestMergeFieldConfig_NonRequirableForcedFalse(t *testing.T) {
	overrides := []models.AttachmentFieldConfig{
		{FieldKey: "roof_access", Visible: true, Required: true}, // required=true должен игнорироваться
	}
	merged := MergeFieldConfig("cars", overrides)
	roof, ok := mergedByKey(merged, "roof_access")
	if !ok {
		t.Fatal("нет roof_access")
	}
	if roof.Required {
		t.Error("roof_access не обязуем - required должен форситься в false даже с оверрайдом")
	}
	if !roof.Visible {
		t.Error("roof_access visible оверрайд должен примениться")
	}
}

func TestMergeFieldConfig_IgnoresOverrideForOtherType(t *testing.T) {
	// оверрайд по ключу не из реестра типа не должен ничего ломать
	overrides := []models.AttachmentFieldConfig{
		{FieldKey: "number", Visible: false, Required: false}, // number нет у people
	}
	merged := MergeFieldConfig("people", overrides)
	if _, ok := mergedByKey(merged, "number"); ok {
		t.Error("number не должен попасть в merged для people")
	}
	// чужой оверрайд не должен затронуть реальные поля people
	if lastName, _ := mergedByKey(merged, "last_name"); !lastName.Visible || !lastName.Required {
		t.Error("last_name должен остаться дефолтным при чужом оверрайде")
	}
	if pass, _ := mergedByKey(merged, "passport"); !pass.Visible || !pass.Required {
		t.Error("passport должен остаться дефолтным при чужом оверрайде")
	}
}

func TestBuildFieldConfigRows_DedupLastWins(t *testing.T) {
	items := []models.FieldConfigItem{
		{Key: "passport", Visible: true, Required: true},
		{Key: "last_name", Visible: true, Required: true},
		{Key: "passport", Visible: false, Required: false}, // дубль - побеждает он
	}
	rows := buildFieldConfigRows("people", 7, items)
	if len(rows) != 2 {
		t.Fatalf("ожидалось 2 строки после дедупа, got %d", len(rows))
	}
	var passport *models.AttachmentFieldConfig
	for i := range rows {
		if rows[i].FieldKey == "passport" {
			passport = &rows[i]
		}
		if rows[i].UniqueAttachmentID != 7 {
			t.Errorf("uaID не проставлен: %d", rows[i].UniqueAttachmentID)
		}
	}
	if passport == nil {
		t.Fatal("нет passport")
	}
	if passport.Visible || passport.Required {
		t.Errorf("last-wins не сработал: passport visible=%v required=%v", passport.Visible, passport.Required)
	}
}

func TestBuildFieldConfigRows_ForcesNonRequirableFalse(t *testing.T) {
	items := []models.FieldConfigItem{
		{Key: "roof_access", Visible: true, Required: true},
	}
	rows := buildFieldConfigRows("cars", 1, items)
	if len(rows) != 1 {
		t.Fatalf("ожидалась 1 строка, got %d", len(rows))
	}
	if rows[0].Required {
		t.Error("roof_access не обязуем - required должен форситься в false при сохранении")
	}
}

func TestRegistryDateTimeLocked(t *testing.T) {
	lockedKeys := []string{"entry_date_from", "entry_date_to", "entry_time_from", "entry_time_to"}
	defs := FieldRegistryFor("people")
	for _, k := range lockedKeys {
		d, ok := defByKey(defs, k)
		if !ok {
			t.Fatalf("нет поля %q", k)
		}
		if !d.Locked {
			t.Errorf("%s должен быть Locked", k)
		}
	}
	// прочие поля не залочены
	for _, k := range []string{"roof_access", "last_name", "passport"} {
		if d, _ := defByKey(defs, k); d.Locked {
			t.Errorf("%s не должен быть Locked", k)
		}
	}
}

func TestMergeFieldConfig_LockedForcedAndIgnoresOverride(t *testing.T) {
	// оверрайд пытается скрыть/снять обязательность даты - должен игнорироваться
	overrides := []models.AttachmentFieldConfig{
		{FieldKey: "entry_date_from", Visible: false, Required: false},
		{FieldKey: "entry_time_to", Visible: false, Required: false},
	}
	merged := MergeFieldConfig("cars", overrides)
	for _, k := range []string{"entry_date_from", "entry_time_to"} {
		f, ok := mergedByKey(merged, k)
		if !ok {
			t.Fatalf("нет %q", k)
		}
		if !f.Visible || !f.Required || !f.Locked {
			t.Errorf("%s залочен: visible=%v required=%v locked=%v, ожидалось true/true/true", k, f.Visible, f.Required, f.Locked)
		}
	}
}

func TestBuildFieldConfigRows_SkipsLocked(t *testing.T) {
	items := []models.FieldConfigItem{
		{Key: "entry_date_from", Visible: false, Required: false}, // залочен - не персистится
		{Key: "number", Visible: false, Required: false},
	}
	rows := buildFieldConfigRows("cars", 3, items)
	if len(rows) != 1 {
		t.Fatalf("ожидалась 1 строка (залоченное поле пропущено), got %d", len(rows))
	}
	if rows[0].FieldKey != "number" {
		t.Errorf("ожидался number, got %q", rows[0].FieldKey)
	}
}

func TestUnknownFieldKeys(t *testing.T) {
	tests := []struct {
		name    string
		attType string
		keys    []string
		want    []string
	}{
		{"все валидны people", "people", []string{"last_name", "passport", "entry_date_from"}, nil},
		{"ключ чужой группы", "people", []string{"last_name", "number"}, []string{"number"}},
		{"выдуманный ключ", "cars", []string{"number", "bogus"}, []string{"bogus"}},
		{"пустой список", "items", nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := UnknownFieldKeys(tc.attType, tc.keys)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}
