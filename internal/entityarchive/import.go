package entityarchive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"systemburo/internal/crypto"
	"systemburo/internal/models"

	"gorm.io/gorm"
)

// Разворот проверенного пакета выгрузки на текущий стенд (server entity import) -
// зеркало Export внутрь: читает то же дерево, что тот записал, и вставляет его в базу.
//
// Гейт проверки идёт первым и без исключений: Import сам зовёт Verify по тому же
// каталогу и отказывается разворачивать что угодно, кроме целого, читаемого пакета -
// "не сносить, пока копия не снята и не проверена" держится и с этой стороны тоже.
//
// Верности схеме мало: Verify сверяет манифест с information_schema (таблица/колонки
// РЕАЛЬНО существуют), а это доказывает лишь "такой объект где-то в этой базе есть" -
// подменённый манифест мог бы назвать любую реальную таблицу схемы (настройки, роли,
// права), и такая сверка её пропустит. Поэтому состав манифеста ДВАЖДЫ сверяется с картой
// графа (allowedNodeTables/organizationNodes, не со схемой) - внутри Verify и
// независимо ещё раз здесь, перед вставкой (checkTablesInGraph): защита не должна
// зависеть от того, что кто-то не переставит вызовы местами.
//
// Пакет разворачивается только на ЧИСТЫЙ стенд: перед вставкой Import сверяет
// идентификаторы каждой таблицы пакета с тем, что уже есть в базе, и отказывается при
// первом же совпадении. Слияние с существующими данными - другая задача с другими
// правилами (что делать при расхождении строк с одним id), и молча перезаписывать чужую
// строку здесь нельзя.
//
// Порядок вставки - РОДИТЕЛИ РАНЬШЕ ДЕТЕЙ, то есть обратный порядку organizationNodes().
// Отдельно этот порядок не пересчитывается: writeTable в export.go кладёт таблицы в
// manifest.Tables строго в порядке обхода organizationNodes() (только непустые), поэтому
// разворот в обратном порядке ЭТОГО списка даёт нужный результат без второго обращения к
// самому графу.
//
// Файлы заявок ложатся на диск ДО транзакции со строками: осиротевший файл без строки
// безопаснее строки, ссылающейся в пустоту (его подбирает штатный уборщик сирот). Поле
// encrypted вставляемой строки application_files описывает ФАКТ этой установки (задан ли
// здесь DATA_ENCRYPTION_KEY), а не то, что было в манифесте пакета - иначе строка
// разойдётся с тем, как файл реально лёг на диск, и не прочитается.
//
// Значения строк берутся из jsonl как есть; единственная декодировка - json.Number
// обратно в текст (без обхода через float64, который на больших id/суммах теряет
// точность) и обратная сериализация вложенных объектов/массивов (jsonb). Схема текущей
// базы содержит ноль json/jsonb колонок в графе организации, поэтому ветка вложенных
// значений сейчас не задействована, но decodeCell обязана остаться корректной на будущее:
// см. её комментарий про единственный неразрешимый на этом уровне случай.

const (
	// applicationFilesTable - единственная таблица графа, у строк которой есть файл на
	// диске (см. applicationFileRows в export.go). Список файлов пакета (manifest.Files)
	// ссылается сюда через RowID.
	applicationFilesTable = "application_files"
	// importConflictExamples - сколько занятых id показывать в отказе на таблицу: список
	// не обязан быть полным, оператору нужно только опознать конфликт и понять, где смотреть.
	importConflictExamples = 5
)

// AuditRecorder пишет след импорта в audit_log. Интерфейс объявлен здесь тем же приёмом,
// что Encryptor/Decryptor в export.go/verify.go: реализация уже готова (services.AuditRecorder),
// а тянуть ради одного метода весь пакет сервисов незачем.
type AuditRecorder interface {
	Record(ctx context.Context, exec *gorm.DB, entityType string, entityID *int, action string, actorID *int, details interface{}) error
}

// ImportOptions - параметры разворота пакета.
type ImportOptions struct {
	// UploadPath - тот же корень загрузок, что у выгрузки: под ним лежит application_files.
	UploadPath string
	// Decrypt открывает файлы ПАКЕТА (конверт архива), как и у Verify. nil - пакет без
	// шифрования или ключи не заданы: тогда зашифрованный файл честно откажет.
	Decrypt Decryptor
	// Recorder пишет audit_log об успешном импорте. Обязателен при Apply=true - импорт
	// без следа в журнале не считается штатным путём (см. Import).
	Recorder AuditRecorder
	// Apply - записать по-настоящему. Без него команда только проверяет пакет, считает
	// конфликты идентификаторов и отвечает, что случилось бы - не трогая ни базу, ни диск.
	Apply bool
}

// ConflictReport - одна таблица пакета, чьи идентификаторы уже заняты на этом стенде.
type ConflictReport struct {
	Table    string
	Total    int
	Examples []int
}

// ImportResult - что получилось (или получилось бы при пробном прогоне).
type ImportResult struct {
	Dir      string
	Apply    bool
	Manifest Manifest
	Rows     int64
	Files    int
	// Conflicts - непустой список останавливает импорт ещё до вставки (см. Import).
	Conflicts []ConflictReport
	// Problems - причины отказа гейта Verify. Пустой, если пакет прошёл проверку (тогда
	// импорт мог остановиться по другой причине - конфликтам - или дойти до записи).
	Problems []string
	Warnings []string
}

// Import проверяет пакет в dir (Verify) и разворачивает его граф на текущий стенд.
func Import(ctx context.Context, db *gorm.DB, dir string, opt ImportOptions) (ImportResult, error) {
	v, err := Verify(ctx, db, dir, opt.Decrypt, "", 0)
	if err != nil {
		return ImportResult{}, fmt.Errorf("проверка пакета: %w", err)
	}
	res := ImportResult{Dir: dir, Apply: opt.Apply, Manifest: v.Manifest, Warnings: append([]string(nil), v.Warnings...)}
	if !v.OK {
		res.Problems = v.Problems
		return res, fmt.Errorf("пакет не прошёл проверку (%d ошибок) - импорт не начинается", len(v.Problems))
	}
	// Сверка со схемой базы внутри Verify доказывает только "такая таблица где-то в этой
	// базе есть" - подменённый манифест мог бы назвать любую реальную таблицу схемы
	// (настройки, роли, права) и пройти её чисто. checkGraphMembership в verify.go уже
	// отсеивает такой пакет через v.OK выше, но здесь та же сверка повторяется НЕЗАВИСИМО:
	// защита вставки не должна зависеть от того, что кто-то не переставит вызовы местами
	// или ослабит чек внутри Verify в будущей правке. insertTable/findConflicts строят SQL
	// по имени таблицы через quoteIdent - оно обязано быть в белом списке графа, а не
	// просто существовать в базе, независимо от результата Verify.
	if err := checkTablesInGraph(v.Manifest.Type, v.Manifest.Tables); err != nil {
		res.Problems = append(res.Problems, err.Error())
		return res, err
	}
	if v.Manifest.FieldEncryption == "system_key" {
		// Определить точно, тот ли это ключ, отсюда нельзя - манифест несёт лишь признак
		// "закрыто ключом системы", не сам ключ. Единственный честный ответ - предупредить
		// оператора и дать проверить руками, а не промолчать и не гадать.
		res.Warnings = append(res.Warnings, "поля под ключом системы (паспорта, патенты): проверьте, что "+
			"DATA_ENCRYPTION_KEY на этом стенде совпадает с тем, которым собран пакет - иначе эти поля "+
			"лягут нечитаемым шифротекстом, и разворот об этом не узнает")
	}

	tables, err := loadPackageTables(dir, v.Manifest.Tables, opt.Decrypt)
	if err != nil {
		return res, err
	}
	for _, t := range tables {
		res.Rows += int64(len(t.rows))
	}
	res.Files = len(v.Manifest.Files)

	conflicts, err := findConflicts(ctx, db, tables)
	if err != nil {
		return res, err
	}
	res.Conflicts = conflicts
	if len(conflicts) > 0 {
		return res, fmt.Errorf("идентификаторы заняты в %d таблицах пакета - пакет разворачивается только на "+
			"чистый стенд, слияние с существующими данными не поддерживается: %s", len(conflicts), conflictSummary(conflicts))
	}

	if !opt.Apply {
		return res, nil
	}
	if opt.Recorder == nil {
		return res, errors.New("не задан журнал аудита (Recorder) - успешный импорт обязан оставить след в audit_log")
	}

	// Факт этой установки, а не признак манифеста: строка application_files должна
	// описывать, как файл ЗДЕСЬ и СЕЙЧАС лёг на диск.
	localEncrypted := len(crypto.GetGlobalKey()) > 0
	if err := writeDataFiles(dir, opt.UploadPath, v.Manifest.Files, applicationFileNames(tables), opt.Decrypt, localEncrypted); err != nil {
		return res, err
	}
	applyLocalEncryption(tables, localEncrypted)

	orgID := v.Manifest.ID
	details := importAuditDetails{Package: dir, Tables: len(tables), Rows: res.Rows, Files: res.Files}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := insertPackage(ctx, tx, tables); err != nil {
			return err
		}
		return opt.Recorder.Record(ctx, tx, v.Manifest.Type, &orgID, models.OrganizationActionImported, nil, details)
	})
	if err != nil {
		return res, fmt.Errorf("запись пакета в базу: %w", err)
	}
	return res, nil
}

// checkTablesInGraph отказывает, если манифест ссылается хоть на одну таблицу вне графа
// entityType (allowedNodeTables из registry.go - тот же список, что использует и Verify).
// Вызывается независимо от checkGraphMembership в verify.go - см. комментарий на месте
// вызова в Import.
func checkTablesInGraph(entityType string, tables []TableFile) error {
	allowed, err := allowedNodeTables(entityType)
	if err != nil {
		return err
	}
	var bad []string
	for _, t := range tables {
		if !allowed[t.Table] {
			bad = append(bad, t.Table)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("манифест ссылается на таблицы вне графа %s: %s", entityType, strings.Join(bad, ", "))
	}
	return nil
}

type importAuditDetails struct {
	Package string `json:"package"`
	Tables  int    `json:"tables"`
	Rows    int64  `json:"rows"`
	Files   int    `json:"files"`
}

// packageTable - одна распакованная таблица пакета: имя, состав колонок из описи и уже
// разобранные строки в порядке manifest.Tables (дети раньше родителей, как у export).
type packageTable struct {
	name    string
	columns []string
	rows    []map[string]any
	ids     []int
}

// loadPackageTables читает и разбирает jsonl каждой таблицы пакета. Файлы уже прошли
// Verify (отпечаток, число строк, валидный JSON на каждой строке) - второе чтение здесь
// не задваивает проверку, оно нужно, чтобы получить сами значения для вставки.
func loadPackageTables(dir string, files []TableFile, dec Decryptor) ([]packageTable, error) {
	out := make([]packageTable, 0, len(files))
	for _, f := range files {
		rc, err := openPackageFile(filepath.Join(dir, filepath.FromSlash(f.File)), dec)
		if err != nil {
			return nil, fmt.Errorf("таблица %s: %w", f.Table, err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("таблица %s: чтение: %w", f.Table, err)
		}

		t := packageTable{name: f.Table, columns: f.Columns}
		for i, line := range splitJSONLines(body) {
			jdec := json.NewDecoder(bytes.NewReader(line))
			// UseNumber бережёт целые id/суммы от прохода через float64: JSON-число вида
			// 9007199254740993 округлилось бы при разборе в float64 ещё до того, как мы
			// успеем вернуть его текстом в INSERT.
			jdec.UseNumber()
			var row map[string]any
			if err := jdec.Decode(&row); err != nil {
				return nil, fmt.Errorf("таблица %s: строка %d: %w", f.Table, i+1, err)
			}
			id, err := rowID(row)
			if err != nil {
				return nil, fmt.Errorf("таблица %s: строка %d: %w", f.Table, i+1, err)
			}
			t.rows = append(t.rows, row)
			t.ids = append(t.ids, id)
		}
		out = append(out, t)
	}
	return out, nil
}

// rowID достаёт значение колонки id из уже разобранной строки. Все таблицы графа
// организации - таблицы с serial-колонкой id (замок TestOrganizationRoots_* держит
// граф в согласии с моделями), поэтому её отсутствие или нечисловой тип - повреждённый
// пакет, а не штатный случай.
func rowID(row map[string]any) (int, error) {
	raw, ok := row["id"]
	if !ok {
		return 0, errors.New("в строке нет колонки id")
	}
	num, ok := raw.(json.Number)
	if !ok {
		return 0, fmt.Errorf("колонка id не число: %T", raw)
	}
	n, err := num.Int64()
	if err != nil {
		return 0, fmt.Errorf("колонка id: %w", err)
	}
	return int(n), nil
}

// findConflicts проверяет, заняты ли на этом стенде идентификаторы каждой таблицы
// пакета. Идёт ДО любой записи: список конфликтов должен увидеть оператор, а не первая
// ошибка дубликата ключа посреди частично вставленного графа.
func findConflicts(ctx context.Context, db *gorm.DB, tables []packageTable) ([]ConflictReport, error) {
	var out []ConflictReport
	for _, t := range tables {
		var existing []int
		q := `SELECT id FROM ` + quoteIdent(t.name) + ` WHERE id IN ? ORDER BY id`
		if err := db.WithContext(ctx).Raw(q, t.ids).Scan(&existing).Error; err != nil {
			return nil, fmt.Errorf("проверка занятых идентификаторов %s: %w", t.name, err)
		}
		if len(existing) == 0 {
			continue
		}
		examples := existing
		if len(examples) > importConflictExamples {
			examples = examples[:importConflictExamples]
		}
		out = append(out, ConflictReport{Table: t.name, Total: len(existing), Examples: examples})
	}
	return out, nil
}

func conflictSummary(cs []ConflictReport) string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		ids := make([]string, len(c.Examples))
		for j, id := range c.Examples {
			ids[j] = strconv.Itoa(id)
		}
		parts[i] = fmt.Sprintf("%s (%d, например %s)", c.Table, c.Total, strings.Join(ids, ", "))
	}
	return strings.Join(parts, "; ")
}

// applicationFileNames индексирует stored_name по id строки application_files - нужен,
// чтобы положить содержимое files/* на диск под ИСХОДНЫМ именем: сама DataFile из
// манифеста несёт только номер строки, имя на диске есть только в самой строке.
func applicationFileNames(tables []packageTable) map[int]string {
	out := map[int]string{}
	for _, t := range tables {
		if t.name != applicationFilesTable {
			continue
		}
		for i, row := range t.rows {
			if name, ok := row["stored_name"].(string); ok {
				out[t.ids[i]] = name
			}
		}
	}
	return out
}

// applyLocalEncryption переписывает encrypted-поле строк application_files фактом ЭТОЙ
// установки. Манифест мог быть собран с другим DATA_ENCRYPTION_KEY (или вовсе без него),
// а файл на диске здесь ляжет так, как задан ключ здесь - иначе строка разойдётся с тем,
// что реально можно прочитать.
func applyLocalEncryption(tables []packageTable, encrypted bool) {
	for _, t := range tables {
		if t.name != applicationFilesTable {
			continue
		}
		for _, row := range t.rows {
			row["encrypted"] = encrypted
		}
	}
}

// writeDataFiles кладёт файлы заявок на диск ДО транзакции со строками: осиротевший файл
// без строки безопаснее строки, ссылающейся в пустоту (штатный уборщик сирот его подберёт),
// а обратное - строка на несуществующий файл - незаметно до первого скачивания.
func writeDataFiles(dir, uploadPath string, files []DataFile, storedNames map[int]string, dec Decryptor, localEncrypted bool) error {
	if len(files) == 0 {
		return nil
	}
	destDir := filepath.Join(uploadPath, applicationFilesDir)
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return fmt.Errorf("каталог файлов заявок %s: %w", destDir, err)
	}
	var key []byte
	if localEncrypted {
		key = crypto.GetGlobalKey()
	}
	for _, f := range files {
		if f.Table != applicationFilesTable {
			// v1 графа не знает других узлов с файлами (см. applicationFileRows в
			// export.go) - незнакомый узел означает пакет новее этой версии кода.
			return fmt.Errorf("файл %s: пакет ссылается на неизвестный узел %s", f.File, f.Table)
		}
		name, ok := storedNames[f.RowID]
		if !ok || name == "" {
			return fmt.Errorf("файл %s (заявка %d): в строке application_files нет имени на диске", f.File, f.RowID)
		}
		dest := filepath.Join(destDir, name)
		if _, err := os.Stat(dest); err == nil {
			return fmt.Errorf("файл %s уже существует на этом стенде - разворот поверх существующих файлов не поддерживается", dest)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("проверка файла %s: %w", dest, err)
		}

		if err := copyPackageFile(dir, f.File, dest, dec, key); err != nil {
			return err
		}
	}
	return nil
}

func copyPackageFile(dir, pkgName, dest string, dec Decryptor, key []byte) error {
	rc, err := openPackageFile(filepath.Join(dir, filepath.FromSlash(pkgName)), dec)
	if err != nil {
		return fmt.Errorf("файл %s: %w", pkgName, err)
	}
	defer rc.Close()

	// O_EXCL - вторая линия защиты поверх Stat-проверки выше: между ними теоретически
	// мог появиться файл (параллельный импорт того же пакета), и молча перезаписывать
	// его нельзя так же, как и найденный заранее.
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("создание файла %s: %w", dest, err)
	}

	// done остаётся false до самого конца - сбой на любом шаге ниже убирает СВОЙ файл
	// сразу, а не оставляет огрызок ждать часового уборщика сирот: тот же приём, что
	// done/defer у Export (export.go), только уровнем ниже - на один файл, а не на весь
	// пакет. Без этого повторный import -apply того же пакета упирался бы в Stat-проверку
	// выше ("файл уже существует") на файле, который сам импорт и не дописал.
	done := false
	defer func() {
		if done {
			return
		}
		if rmErr := os.Remove(dest); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			slog.Error("недописанный файл импорта остался на диске", "path", dest, "error", rmErr)
		}
	}()

	w, err := crypto.NewStreamWriter(out, key)
	if err != nil {
		out.Close()
		return fmt.Errorf("шифрование файла %s: %w", dest, err)
	}
	if _, err := io.Copy(w, rc); err != nil {
		w.Close()
		out.Close()
		return fmt.Errorf("запись файла %s: %w", dest, err)
	}
	if err := w.Close(); err != nil {
		out.Close()
		return fmt.Errorf("завершение записи файла %s: %w", dest, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("закрытие файла %s: %w", dest, err)
	}
	done = true
	return nil
}

// insertPackage вставляет все таблицы пакета в порядке РОДИТЕЛИ РАНЬШЕ ДЕТЕЙ (обратном
// manifest.Tables - см. комментарий пакета) и правит затронутые последовательности.
func insertPackage(ctx context.Context, tx *gorm.DB, tables []packageTable) error {
	for i := len(tables) - 1; i >= 0; i-- {
		if err := insertTable(ctx, tx, tables[i]); err != nil {
			return err
		}
	}
	for _, t := range tables {
		if err := fixSequence(ctx, tx, t.name); err != nil {
			return err
		}
	}
	return nil
}

func insertTable(ctx context.Context, tx *gorm.DB, t packageTable) error {
	if len(t.rows) == 0 {
		return nil
	}
	cols := make([]string, len(t.columns))
	placeholders := make([]string, len(t.columns))
	for i, c := range t.columns {
		cols[i] = quoteIdent(c)
		placeholders[i] = "?"
	}
	stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", quoteIdent(t.name), strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	for i, row := range t.rows {
		args := make([]any, len(t.columns))
		for j, c := range t.columns {
			raw, ok := row[c]
			if !ok {
				return fmt.Errorf("таблица %s, id=%d: в строке нет колонки %s, хотя она есть в описи", t.name, t.ids[i], c)
			}
			v, err := decodeCell(raw)
			if err != nil {
				return fmt.Errorf("таблица %s, id=%d, колонка %s: %w", t.name, t.ids[i], c, err)
			}
			args[j] = v
		}
		if err := tx.WithContext(ctx).Exec(stmt, args...).Error; err != nil {
			return fmt.Errorf("вставка в %s (id=%d): %w", t.name, t.ids[i], err)
		}
	}
	return nil
}

// decodeCell превращает значение, разобранное из jsonl, в параметр для INSERT.
//
// nil/bool/string идут как есть - ровно то, что encodeValue в export.go положило в jsonl
// для NULL, булевых и текстовых колонок. json.Number (см. loadPackageTables) возвращается
// текстом, а не float64: так целые id и суммы переживают обратный путь без потери
// точности, а Postgres разбирает текстовую форму числа для любой числовой колонки сам.
// map/slice - развёрнутый jsonb-объект/массив, сериализуется обратно в JSON-текст;
// параметр без явного типа Postgres приводит к целевой колонке присваиванием (тот же
// приём, которым gorm сам пишет json.RawMessage).
//
// Нерешаемый на этом уровне случай: голая JSON-строка ВНУТРИ jsonb-колонки (например,
// значение "foo", а не объект) неотличима здесь от значения текстовой колонки - обе
// приходят как Go string. У графа организации сейчас нет ни одной jsonb-колонки (грепом
// по моделям), поэтому случай не возникает; появится такая колонка - решать по типу
// колонки в схеме, а не по форме значения.
func decodeCell(v any) (any, error) {
	switch val := v.(type) {
	case nil:
		return nil, nil
	case bool, string:
		return val, nil
	case json.Number:
		return val.String(), nil
	case map[string]any, []any:
		b, err := json.Marshal(val)
		if err != nil {
			return nil, fmt.Errorf("сериализация вложенного значения: %w", err)
		}
		return string(b), nil
	default:
		return nil, fmt.Errorf("неожиданный тип значения в строке пакета: %T", val)
	}
}

// fixSequence поднимает serial-последовательность таблицы до максимума ПОСЛЕ вставки
// строк с явными id: без этого первая же обычная вставка на этом стенде получит id,
// который уже занят импортированной строкой, и упадёт на дубле ключа. Таблицы без
// serial/identity id (pg_get_serial_sequence вернёт NULL) пропускаются - поправлять нечего.
func fixSequence(ctx context.Context, tx *gorm.DB, table string) error {
	var seq *string
	if err := tx.WithContext(ctx).Raw(`SELECT pg_get_serial_sequence(?, 'id')`, table).Scan(&seq).Error; err != nil {
		return fmt.Errorf("поиск последовательности %s: %w", table, err)
	}
	if seq == nil {
		return nil
	}
	// Таблица только что получила хотя бы одну строку пакета (insertTable пропускает
	// пустые), поэтому MAX(id) здесь не NULL - is_called=true безопасен без доп. проверки.
	q := `SELECT setval(?, (SELECT MAX(id) FROM ` + quoteIdent(table) + `), true)`
	if err := tx.WithContext(ctx).Exec(q, *seq).Error; err != nil {
		return fmt.Errorf("правка последовательности %s: %w", table, err)
	}
	return nil
}

// quoteIdent оборачивает имя таблицы/колонки в двойные кавычки. Инъекции здесь не
// источник риска: к этому месту доходят только имена, ДВАЖДЫ проверенные - Verify сверил
// каждое с information_schema текущей базы (verifySchema в verify.go, буквальное
// совпадение), а checkTablesInGraph выше в Import сверил сами таблицы с картой графа
// (allowedNodeTables) - "существует в схеме" и "принадлежит графу" разные гарантии, и
// нужны обе. Кавычки здесь нужны ради корректности с зарезервированными словами и
// смешанным регистром, а не ради экранирования чужого ввода.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
