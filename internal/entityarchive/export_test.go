package entityarchive

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeEncryptor подменяет age-конверт узнаваемой обёрткой: тесту важно не то, как
// шифрует библиотека, а что выгрузка вообще пропускает содержимое через шифрование и
// пишет в опись имя с суффиксом, а не исходное.
type fakeEncryptor struct {
	off  bool
	fail bool
}

func (f fakeEncryptor) Enabled() bool { return !f.off }

func (f fakeEncryptor) FileName(name string) string {
	if f.off {
		return name
	}
	return name + ".age"
}

func (f fakeEncryptor) Encrypt(data []byte) ([]byte, error) {
	if f.fail {
		return nil, errors.New("ключ отвергнут")
	}
	if f.off {
		return data, nil
	}
	return append([]byte("SEALED:"), data...), nil
}

func TestEncodeValue_KeepsJSONAndText(t *testing.T) {
	// jsonb обязан остаться объектом: обёрнутый в строку, при импорте он стал бы текстом.
	v, err := encodeValue("attachments", "details", "JSONB", []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("jsonb: %v", err)
	}
	raw, ok := v.(json.RawMessage)
	if !ok {
		t.Fatalf("jsonb пришёл как %T, ожидался json.RawMessage", v)
	}
	out, err := json.Marshal(map[string]any{"details": raw})
	if err != nil {
		t.Fatalf("сериализация строки: %v", err)
	}
	if string(out) != `{"details":{"a":1}}` {
		t.Fatalf("jsonb уехал в пакет как %s", out)
	}

	if v, err := encodeValue("users", "last_name", "VARCHAR", []byte("Иванов")); err != nil || v != "Иванов" {
		t.Fatalf("текстовая колонка: %v, %v", v, err)
	}
	if v, err := encodeValue("users", "id", "INT8", int64(7)); err != nil || v != int64(7) {
		t.Fatalf("число: %v, %v", v, err)
	}
	if v, err := encodeValue("users", "deleted_at", "TIMESTAMP", nil); err != nil || v != nil {
		t.Fatalf("NULL: %v, %v", v, err)
	}
}

// TestEncodeValue_RejectsBinary: двоичная колонка должна остановить выгрузку. Молча
// превращать её в строку нельзя - пакет выглядел бы целым, а содержимое было бы битым.
func TestEncodeValue_RejectsBinary(t *testing.T) {
	_, err := encodeValue("documents", "body", "BYTEA", []byte{0x00, 0xff})
	if err == nil {
		t.Fatal("двоичная колонка принята без ошибки")
	}
	if !strings.Contains(err.Error(), "documents.body") {
		t.Fatalf("ошибка не называет колонку: %v", err)
	}
}

func TestWritePackageFile_SealsAndRenames(t *testing.T) {
	dir := t.TempDir()

	name, err := writePackageFile(dir, "tables/users.jsonl", []byte("{}\n"), fakeEncryptor{})
	if err != nil {
		t.Fatalf("запись зашифрованного файла: %v", err)
	}
	if name != "tables/users.jsonl.age" {
		t.Fatalf("в опись попало имя %q, ожидалось с суффиксом конверта", name)
	}
	body, err := os.ReadFile(filepath.Join(dir, "tables", "users.jsonl.age"))
	if err != nil {
		t.Fatalf("файл пакета не создан: %v", err)
	}
	if string(body) != "SEALED:{}\n" {
		t.Fatalf("содержимое не прошло через шифрование: %q", body)
	}

	// Без шифрования имя остаётся прежним, иначе импорт искал бы конверт там, где его нет.
	plain, err := writePackageFile(dir, "tables/cars.jsonl", []byte("{}\n"), fakeEncryptor{off: true})
	if err != nil {
		t.Fatalf("запись открытого файла: %v", err)
	}
	if plain != "tables/cars.jsonl" {
		t.Fatalf("имя открытого файла %q", plain)
	}

	// Пакет читает только тот, кто им владеет: права на файлы проверяются здесь, а не
	// подразумеваются - в них лежат персональные данные целиком.
	info, err := os.Stat(filepath.Join(dir, "tables", "cars.jsonl"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("права файла пакета %o, ожидались 600", perm)
	}
}

// TestWritePackageFile_FailureLeavesNothing: сбой шифрования обязан вернуться ошибкой, а
// не оставить на диске открытый файл вместо конверта.
func TestWritePackageFile_FailureLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	if _, err := writePackageFile(dir, "tables/users.jsonl", []byte("{}\n"), fakeEncryptor{fail: true}); err == nil {
		t.Fatal("сбой шифрования не вернул ошибку")
	}
	if _, err := os.Stat(filepath.Join(dir, "tables")); !os.IsNotExist(err) {
		t.Fatalf("после сбоя на диске остались файлы пакета: %v", err)
	}
}

// TestEncEnabled_NilInterface: выключенное шифрование приходит и nil-интерфейсом (пакет
// собирают без ключей), и такой вызов не должен ронять выгрузку.
func TestEncEnabled_NilInterface(t *testing.T) {
	if encEnabled(nil) {
		t.Fatal("nil-шифрование сочтено включённым")
	}
	if encEnabled(fakeEncryptor{off: true}) {
		t.Fatal("шифрование без ключей сочтено включённым")
	}
}
