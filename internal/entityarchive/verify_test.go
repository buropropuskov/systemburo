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

// Проверка пакета (server entity verify) - чистые тесты без базы, живут здесь, а не в
// internal/handlers: сверка со схемой (единственная часть, которой нужна база) в
// verifyStructure не вызывается, дальше её проверяет отдельный DB-backed тест в
// internal/handlers/entity_verify_test.go.

// buildPackage кладёт на диск минимальный пакет вручную - без шифрования и без базы: ровно
// то, что нужно тестам структурных проверок. rows - готовые JSON-объекты, по одному в строке.
func buildPackage(t *testing.T, rows []string) (dir string, m Manifest) {
	t.Helper()
	dir = t.TempDir()

	body := []byte(strings.Join(rows, "\n") + "\n")
	if err := os.MkdirAll(filepath.Join(dir, tablesDir), 0o700); err != nil {
		t.Fatalf("mkdir tables: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, tablesDir, "organizations.jsonl"), body, 0o600); err != nil {
		t.Fatalf("write table: %v", err)
	}
	sum := sha256.Sum256(body)

	m = Manifest{
		Version: PackageVersion,
		Type:    TypeOrganization,
		ID:      1,
		Tables: []TableFile{{
			Table:   "organizations",
			Rows:    int64(len(rows)),
			Columns: []string{"id", "name"},
			File:    "tables/organizations.jsonl",
			Bytes:   int64(len(body)),
			SHA256:  hex.EncodeToString(sum[:]),
		}},
	}
	writeManifestFile(t, dir, m)
	return dir, m
}

func writeManifestFile(t *testing.T, dir string, m Manifest) {
	t.Helper()
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestName), body, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func TestVerifyStructure_ValidPackage(t *testing.T) {
	dir, _ := buildPackage(t, []string{`{"id":1,"name":"Организация"}`, `{"id":2,"name":"Вторая"}`})

	res, loaded := verifyStructure(dir, nil)
	if !loaded {
		t.Fatal("манифест не найден")
	}
	if !res.OK {
		t.Fatalf("годный пакет сочтён негодным: %v", res.Problems)
	}
	if len(res.Problems) != 0 {
		t.Fatalf("problems на годном пакете: %v", res.Problems)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("warnings на годном пакете: %v", res.Warnings)
	}
	if len(res.Files) != 1 || res.Files[0].Rows != 2 || !res.Files[0].OK {
		t.Fatalf("files: %+v", res.Files)
	}
}

// TestVerifyStructure_MissingManifest: пустой каталог - манифеста нет вовсе, и дальнейшие
// проверки не имеют смысла (loaded=false).
func TestVerifyStructure_MissingManifest(t *testing.T) {
	dir := t.TempDir()

	res, loaded := verifyStructure(dir, nil)
	if loaded {
		t.Fatal("манифест якобы найден в пустом каталоге")
	}
	if res.OK {
		t.Fatal("пустой каталог сочтён годным пакетом")
	}
	if len(res.Problems) == 0 {
		t.Fatal("нет причины отказа")
	}
}

// TestVerifyStructure_ManifestIsDirectory: рядом с пакетом оказалась ДИРЕКТОРИЯ с именем
// manifest.json, а не файл. os.Stat её не отличает от обычного файла, поэтому кандидат
// считается найденным - отказ обязана дать уже попытка прочитать содержимое (чтение из
// директории как из файла возвращает ошибку), а не паника и не ложное "манифест не найден".
func TestVerifyStructure_ManifestIsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, manifestName), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	res, loaded := verifyStructure(dir, nil)
	if loaded {
		t.Fatal("директория вместо манифеста якобы разобралась как манифест")
	}
	if res.OK {
		t.Fatal("директория вместо манифеста сочтена годным пакетом")
	}
	if len(res.Problems) == 0 {
		t.Fatal("нет причины отказа")
	}
}

// TestVerifyStructure_EncryptedWithoutKey: манифест лежит конвертом, а расшифровать нечем -
// частый операторский промах (забыли передать ключи). Отказ, а не попытка прочитать
// шифротекст как JSON.
func TestVerifyStructure_EncryptedWithoutKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, manifestName+ageSuffix), []byte("sealed-bytes"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	res, loaded := verifyStructure(dir, nil)
	if loaded {
		t.Fatal("манифест якобы разобран без ключа")
	}
	if res.OK {
		t.Fatal("зашифрованный пакет без ключа сочтён годным")
	}
}

// TestVerifyStructure_BothManifestsPresent: настоящий manifest.json и посторонний
// manifest.json.age присутствуют одновременно - подмена или повреждение пакета, отказ
// обязан сработать по самому факту двойного присутствия, раньше попытки что-то прочитать.
// До фикса перебор manifestCandidates по порядку читал бы manifest.json и принимал такой
// пакет как годный (ровно так ревью и воспроизвело обход: поддельный открытый манифест
// рядом с настоящим зашифрованным).
func TestVerifyStructure_BothManifestsPresent(t *testing.T) {
	dir, _ := buildPackage(t, []string{`{"id":1,"name":"Организация"}`})
	if err := os.WriteFile(filepath.Join(dir, manifestName+ageSuffix), []byte("посторонний файл"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	res, loaded := verifyStructure(dir, nil)
	if loaded {
		t.Fatal("пакет с двумя манифестами якобы разобран")
	}
	if res.OK {
		t.Fatal("пакет с двумя манифестами сочтён годным")
	}
}

// TestVerifyStructure_CorruptedByte: содержимое таблицы подменено, отпечаток в манифесте
// остался прежним - ровно то расхождение, которое и должна ловить проверка sha256.
func TestVerifyStructure_CorruptedByte(t *testing.T) {
	dir, _ := buildPackage(t, []string{`{"id":1,"name":"Организация"}`})

	path := filepath.Join(dir, tablesDir, "organizations.jsonl")
	if err := os.WriteFile(path, []byte(`{"id":1,"name":"Подменено"}`+"\n"), 0o600); err != nil {
		t.Fatalf("corrupt: %v", err)
	}

	res, loaded := verifyStructure(dir, nil)
	if !loaded {
		t.Fatal("манифест должен найтись")
	}
	if res.OK {
		t.Fatal("подменённое содержимое не поймано")
	}
	if !containsSubstring(res.Problems, "отпечаток") {
		t.Fatalf("нет сообщения про отпечаток: %v", res.Problems)
	}
}

// TestVerifyStructure_DeletedFile: файл, обещанный описью, пропал с диска.
func TestVerifyStructure_DeletedFile(t *testing.T) {
	dir, _ := buildPackage(t, []string{`{"id":1,"name":"Организация"}`})

	if err := os.Remove(filepath.Join(dir, tablesDir, "organizations.jsonl")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	res, loaded := verifyStructure(dir, nil)
	if !loaded {
		t.Fatal("манифест должен найтись")
	}
	if res.OK {
		t.Fatal("удалённый файл не пойман")
	}
}

// TestVerifyStructure_VersionMismatch: обе стороны несовпадения версии - пакет новее и
// пакет старше системы - обязаны давать отказ, а не молчаливое чтение.
func TestVerifyStructure_VersionMismatch(t *testing.T) {
	dir, m := buildPackage(t, []string{`{"id":1,"name":"Организация"}`})

	m.Version = PackageVersion + 1
	writeManifestFile(t, dir, m)
	newer, _ := verifyStructure(dir, nil)
	if newer.OK {
		t.Fatal("более новая версия формата не поймана")
	}

	m.Version = PackageVersion - 1
	writeManifestFile(t, dir, m)
	older, _ := verifyStructure(dir, nil)
	if older.OK {
		t.Fatal("более старая версия формата не поймана")
	}
}

// TestVerifyStructure_RowCountMismatch: опись обещает другое число строк, чем в файле.
func TestVerifyStructure_RowCountMismatch(t *testing.T) {
	dir, m := buildPackage(t, []string{`{"id":1,"name":"Организация"}`})
	m.Tables[0].Rows = 5
	writeManifestFile(t, dir, m)

	res, _ := verifyStructure(dir, nil)
	if res.OK {
		t.Fatal("несовпадение числа строк не поймано")
	}
}

// TestVerifyStructure_InvalidJSONLine: строка таблицы не разбирается как JSON-объект.
func TestVerifyStructure_InvalidJSONLine(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, tablesDir), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := []byte("не json\n")
	if err := os.WriteFile(filepath.Join(dir, tablesDir, "organizations.jsonl"), body, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	sum := sha256.Sum256(body)
	m := Manifest{
		Version: PackageVersion,
		Type:    TypeOrganization,
		ID:      1,
		Tables: []TableFile{{
			Table: "organizations", Rows: 1, Columns: []string{"id"},
			File: "tables/organizations.jsonl", Bytes: int64(len(body)), SHA256: hex.EncodeToString(sum[:]),
		}},
	}
	writeManifestFile(t, dir, m)

	res, _ := verifyStructure(dir, nil)
	if res.OK {
		t.Fatal("битая строка JSON не поймана")
	}
}

// TestVerifyStructure_ByteSizeMismatch: опись обещает другой размер файла, чем на диске -
// отдельная проверка от sha256, а не производная от неё (можно испортить именно её,
// оставив отпечаток формально совпадающим по другой причине - опись врёт о размере).
func TestVerifyStructure_ByteSizeMismatch(t *testing.T) {
	dir, m := buildPackage(t, []string{`{"id":1,"name":"Организация"}`})
	m.Tables[0].Bytes += 100
	writeManifestFile(t, dir, m)

	res, _ := verifyStructure(dir, nil)
	if res.OK {
		t.Fatal("несовпадение размера файла не поймано")
	}
	if !containsSubstring(res.Problems, "размер") {
		t.Fatalf("нет сообщения про размер: %v", res.Problems)
	}
}

// TestVerifyStructure_ExtraFile: файл в каталоге, которого нет в описи - предупреждение,
// но не отказ.
func TestVerifyStructure_ExtraFile(t *testing.T) {
	dir, _ := buildPackage(t, []string{`{"id":1,"name":"Организация"}`})
	if err := os.WriteFile(filepath.Join(dir, tablesDir, "surprise.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write extra: %v", err)
	}

	res, loaded := verifyStructure(dir, nil)
	if !loaded {
		t.Fatal("манифест должен найтись")
	}
	if !res.OK {
		t.Fatalf("лишний файл не должен ронять пакет: %v", res.Problems)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("нет предупреждения о лишнем файле")
	}
}

func containsSubstring(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
