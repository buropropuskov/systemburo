package entityarchive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDecodeCell_ScalarsPassThrough: nil/bool/string идут в INSERT как есть - decodeCell
// не должен трогать то, что уже в подходящей для driver.Value форме.
func TestDecodeCell_ScalarsPassThrough(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want any
	}{
		{"nil", nil, nil},
		{"bool true", true, true},
		{"bool false", false, false},
		{"string", "Иванов", "Иванов"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := decodeCell(c.in)
			if err != nil {
				t.Fatalf("decodeCell(%v): %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("decodeCell(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// TestDecodeCell_NumberKeepsPrecision: json.Number обязан вернуться ТЕКСТОМ, а не пройти
// через float64 - иначе id/суммы вне диапазона точного представления float64 (>2^53)
// молча округлятся ещё до того, как уйдут параметром в INSERT. Число ниже нарочно за
// границей точности float64, чтобы поймать регресс на "удобный" float64(n).
func TestDecodeCell_NumberKeepsPrecision(t *testing.T) {
	const big = "9007199254740993" // 2^53 + 1: не представимо точно в float64
	got, err := decodeCell(json.Number(big))
	if err != nil {
		t.Fatalf("decodeCell: %v", err)
	}
	if got != big {
		t.Fatalf("decodeCell(json.Number(%s)) = %v, потеряна точность", big, got)
	}

	// Тот же путь, что loadPackageTables: строка jsonl -> декодер с UseNumber -> decodeCell.
	var row map[string]any
	dec := json.NewDecoder(strings.NewReader(`{"id":` + big + `}`))
	dec.UseNumber()
	if err := dec.Decode(&row); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, err = decodeCell(row["id"])
	if err != nil {
		t.Fatalf("decodeCell: %v", err)
	}
	if got != big {
		t.Fatalf("сквозной разбор строки дал %v, ожидалась точная строка %s", got, big)
	}
}

// TestDecodeCell_NestedJSONRoundTrips: объект/массив (jsonb-колонка) уезжает обратно
// JSON-текстом - Postgres приводит параметр без явного типа к целевой jsonb-колонке
// присваиванием (тем же приёмом, что использует сам gorm для json.RawMessage).
func TestDecodeCell_NestedJSONRoundTrips(t *testing.T) {
	obj := map[string]any{"a": json.Number("1"), "b": "x"}
	got, err := decodeCell(obj)
	if err != nil {
		t.Fatalf("decodeCell(object): %v", err)
	}
	s, ok := got.(string)
	if !ok {
		t.Fatalf("decodeCell(object) = %T, ожидалась строка", got)
	}
	var back map[string]any
	if err := json.Unmarshal([]byte(s), &back); err != nil {
		t.Fatalf("результат не разбирается как JSON: %v (%s)", err, s)
	}
	if back["b"] != "x" {
		t.Fatalf("объект потерял поле b: %v", back)
	}

	arr := []any{json.Number("1"), json.Number("2")}
	got, err = decodeCell(arr)
	if err != nil {
		t.Fatalf("decodeCell(array): %v", err)
	}
	if got != `[1,2]` {
		t.Fatalf("decodeCell(array) = %v, want [1,2]", got)
	}
}

// TestDecodeCell_RejectsUnknownType: пробой сторожа - подставляем тип, которого decodeCell
// не ждёт (то, во что json.Decoder.UseNumber никогда не разберёт значение), и проверяем,
// что он отказывает явно, а не пропускает мимо как nil.
func TestDecodeCell_RejectsUnknownType(t *testing.T) {
	_, err := decodeCell(struct{ X int }{X: 1})
	if err == nil {
		t.Fatal("decodeCell принял неожиданный тип без ошибки")
	}
	if !strings.Contains(err.Error(), "struct") {
		t.Fatalf("ошибка не называет тип: %v", err)
	}
}

func TestQuoteIdent(t *testing.T) {
	if got := quoteIdent("applications"); got != `"applications"` {
		t.Fatalf("quoteIdent(applications) = %s", got)
	}
	// Имя с кавычкой внутри (в схеме такого нет - проверяем экранирование, а не реальный кейс).
	if got := quoteIdent(`a"b`); got != `"a""b"` {
		t.Fatalf("quoteIdent не экранировал кавычку: %s", got)
	}
}

func TestRowID(t *testing.T) {
	id, err := rowID(map[string]any{"id": json.Number("42")})
	if err != nil || id != 42 {
		t.Fatalf("rowID = %d, %v; want 42, nil", id, err)
	}

	if _, err := rowID(map[string]any{"name": "x"}); err == nil {
		t.Fatal("строка без колонки id принята без ошибки")
	}
	if _, err := rowID(map[string]any{"id": "42"}); err == nil {
		t.Fatal("нечисловая колонка id принята без ошибки")
	}
}

// TestApplicationFileNames_And_ApplyLocalEncryption: пара функций, готовящих файлы заявок
// к записи на диск - строится ИМЕННО на строках application_files, чужую таблицу не трогает.
func TestApplicationFileNames_And_ApplyLocalEncryption(t *testing.T) {
	tables := []packageTable{
		{
			name:    applicationFilesTable,
			columns: []string{"id", "stored_name", "encrypted"},
			rows: []map[string]any{
				{"id": json.Number("5"), "stored_name": "a.bin", "encrypted": true},
				{"id": json.Number("6"), "stored_name": "b.bin", "encrypted": false},
			},
			ids: []int{5, 6},
		},
		{
			name:    "applications",
			columns: []string{"id"},
			rows:    []map[string]any{{"id": json.Number("1")}},
			ids:     []int{1},
		},
	}

	names := applicationFileNames(tables)
	if names[5] != "a.bin" || names[6] != "b.bin" {
		t.Fatalf("applicationFileNames = %v", names)
	}
	if _, ok := names[1]; ok {
		t.Fatal("applicationFileNames заглянул в чужую таблицу")
	}

	// Манифест мог нести encrypted=true/false вперемешку - applyLocalEncryption обязана
	// перезаписать ОБА факта установки, а не только те, что расходятся с целевым значением.
	applyLocalEncryption(tables, false)
	for _, row := range tables[0].rows {
		if row["encrypted"] != false {
			t.Fatalf("строка %v не переписана фактом установки", row)
		}
	}
	applyLocalEncryption(tables, true)
	for _, row := range tables[0].rows {
		if row["encrypted"] != true {
			t.Fatalf("строка %v не переписана фактом установки", row)
		}
	}
	// Соседняя таблица не должна была получить поле encrypted вовсе.
	if _, ok := tables[1].rows[0]["encrypted"]; ok {
		t.Fatal("applyLocalEncryption задела чужую таблицу")
	}
}

// TestAllowedNodeTables_MatchesGraph: множество из allowedNodeTables обязано совпадать с
// таблицами organizationNodes() один в один - ни лишних, ни пропущенных.
func TestAllowedNodeTables_MatchesGraph(t *testing.T) {
	allowed, err := allowedNodeTables(TypeOrganization)
	if err != nil {
		t.Fatalf("allowedNodeTables: %v", err)
	}
	nodes := organizationNodes()
	if len(allowed) != len(nodes) {
		t.Fatalf("allowedNodeTables даёт %d таблиц, organizationNodes - %d", len(allowed), len(nodes))
	}
	for _, n := range nodes {
		if !allowed[n.Table] {
			t.Fatalf("таблица узла графа %s не попала в allowedNodeTables", n.Table)
		}
	}
	// Таблица заведомо вне графа - настройки/роли/аудит, а не машины/сотрудники/заявки.
	if allowed["role_permission_grants"] {
		t.Fatal("allowedNodeTables пропустил таблицу вне графа организации")
	}
}

// TestAllowedNodeTables_RejectsUnsupportedType: v1 знает только organization - неизвестный
// тип обязан быть отказом здесь же, а не тихим пустым множеством (пустое множество означало
// бы "любая таблица манифеста лишняя", а не "тип не поддерживается" - разные причины отказа).
func TestAllowedNodeTables_RejectsUnsupportedType(t *testing.T) {
	if _, err := allowedNodeTables("company"); err == nil {
		t.Fatal("неподдерживаемый тип принят без ошибки")
	}
}

// TestCheckTablesInGraph: тот же сторож, что checkGraphMembership в verify.go, но со
// стороны Import - unit-тест на саму функцию, без сборки пакета на диске.
func TestCheckTablesInGraph(t *testing.T) {
	ok := []TableFile{{Table: "organizations"}, {Table: "applications"}}
	if err := checkTablesInGraph(TypeOrganization, ok); err != nil {
		t.Fatalf("таблицы графа отклонены: %v", err)
	}

	bad := []TableFile{{Table: "organizations"}, {Table: "role_permission_grants"}}
	err := checkTablesInGraph(TypeOrganization, bad)
	if err == nil {
		t.Fatal("таблица вне графа принята без ошибки")
	}
	if !strings.Contains(err.Error(), "role_permission_grants") {
		t.Fatalf("ошибка не называет таблицу вне графа: %v", err)
	}
}

// buildPackageWithExtraTable дописывает в уже собранный buildPackage-пакет ещё одну
// таблицу (со своим jsonl-файлом и корректным sha256) и перезаписывает манифест. Нужен
// только тестам белого списка графа - остальные структурные проверки покрывает
// verify_test.go на пакете из одной таблицы.
func buildPackageWithExtraTable(t *testing.T, dir string, m Manifest, table string, rows []string) Manifest {
	t.Helper()
	body := []byte(strings.Join(rows, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(dir, tablesDir, table+".jsonl"), body, 0o600); err != nil {
		t.Fatalf("write table %s: %v", table, err)
	}
	sum := sha256.Sum256(body)
	m.Tables = append(m.Tables, TableFile{
		Table:   table,
		Rows:    int64(len(rows)),
		Columns: []string{"id"},
		File:    "tables/" + table + ".jsonl",
		Bytes:   int64(len(body)),
		SHA256:  hex.EncodeToString(sum[:]),
	})
	writeManifestFile(t, dir, m)
	return m
}

// TestVerifyStructure_RejectsTableOutsideGraph: манифест ссылается на реальную по форме,
// внутренне непротиворечивую (верный sha256, верное число строк) таблицу, которой в графе
// organization нет. Сверка со схемой её бы пропустила (verifySchema здесь не участвует -
// verifyStructure DB не трогает); checkGraphMembership обязана отказать до неё.
func TestVerifyStructure_RejectsTableOutsideGraph(t *testing.T) {
	dir, m := buildPackage(t, []string{`{"id":1,"name":"Организация"}`})
	buildPackageWithExtraTable(t, dir, m, "role_permission_grants", []string{`{"id":1}`})

	res, loaded := verifyStructure(dir, nil)
	if !loaded {
		t.Fatal("манифест не разобрался")
	}
	if res.OK {
		t.Fatal("пакет с таблицей вне графа organization сочтён годным")
	}
	found := false
	for _, p := range res.Problems {
		if strings.Contains(p, "role_permission_grants") {
			found = true
		}
	}
	if !found {
		t.Fatalf("отказ не называет таблицу вне графа: %v", res.Problems)
	}
}

func TestConflictSummary(t *testing.T) {
	got := conflictSummary([]ConflictReport{
		{Table: "organizations", Total: 1, Examples: []int{5}},
		{Table: "applications", Total: 3, Examples: []int{10, 11, 12}},
	})
	want := "organizations (1, например 5); applications (3, например 10, 11, 12)"
	if got != want {
		t.Fatalf("conflictSummary = %q, want %q", got, want)
	}
}
