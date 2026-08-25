package entityarchive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

// Проверка снятого пакета (server entity verify) - гейт, на котором держится инвариант
// «не сносить, пока копия не снята и не проверена». Ничего не меняет: ни базу, ни сам
// пакет. Зелёный ответ обязан означать «пакет полон и разворачиваем», а не «файлы на
// месте» - поэтому каждая проверка описана ниже с внятной причиной отказа, а не молчаливым
// пропуском файла, который не открылся.

// Decryptor открывает файл пакета на чтение, расшифровывая при необходимости. Интерфейс
// объявлен здесь, а не в internal/services, тем же приёмом, что и Encryptor в export.go:
// иначе проверка потянула бы весь пакет сервисов ради одного метода. Контракт зеркалит
// services.ArchiveCrypto.Open - признак шифрования берётся из суффикса имени файла, а не
// из настройки, и реализация уже готова.
type Decryptor interface {
	Open(path string) (io.ReadCloser, error)
}

// ageSuffix повторяет services.EncryptedSuffix буквально. Заводить зависимость от
// services ради одной константы означало бы тянуть весь пакет сервисов; суффикс формата
// конверта не меняется без смены самого формата, и дублирование литерала здесь дешевле.
const ageSuffix = ".age"

// manifestCandidates - как манифест может называться на диске: открытым текстом или под
// конвертом. Легитимный пакет производит ровно одно из двух имён.
var manifestCandidates = []string{manifestName, manifestName + ageSuffix}

// FileCheck - один файл пакета после проверки: строка будущей таблицы отчёта оператору.
type FileCheck struct {
	// Name - путь внутри пакета, как в описи (с суффиксом конверта, если он есть).
	Name string
	// Rows - строк в файле по факту. У файлов заявок (не таблиц) остаётся нулём: у них
	// нет построчного формата, для них проверяется только целостность содержимого.
	Rows int64
	// OK - файл прочитан, отпечаток и размер сошлись с описью (для таблиц - и число строк).
	OK bool
	// State - короткая причина для колонки отчёта: "ок" или что именно не сошлось.
	State string
}

// VerifyResult - итог проверки: годен ли пакет к развороту.
//
// Печать живёт в cmd, здесь только данные: то, что нужно, чтобы построить таблицу файлов,
// список проблем и список предупреждений, не заново вычисляя их из Manifest.
type VerifyResult struct {
	// OK - манифест разобран, версия формата поддерживается, все файлы описи целы и
	// совпадают со схемой текущей базы. Предупреждения на OK не влияют.
	OK       bool
	Manifest Manifest
	// ManifestEncrypted - находился ли САМ файл манифеста реально под age-конвертом на
	// диске (имя manifest.json.age), а не то, что заявляет Manifest.Encrypted. Последнее -
	// поле ИЗ ТЕЛА манифеста, то есть заявление пакета о самом себе: открытый manifest.json
	// с "encrypted": true внутри разбирается ничуть не хуже настоящего. Снос (Purge) обязан
	// опираться на ManifestEncrypted, а не на Manifest.Encrypted - иначе гейт "пакет обязан
	// быть зашифрован" проверяет утверждение, а не факт. checkEncryptionConsistency ниже
	// дополнительно валит OK при любом расхождении - защита не должна зависеть от того, что
	// вызывающий (Purge) не перепутает поле.
	ManifestEncrypted bool
	Files             []FileCheck
	// Problems - причины отказа. Пустой список при OK=false не бывает.
	Problems []string
	// Warnings - то, что оператор должен увидеть, но что не мешает развороту.
	Warnings []string
}

func (r *VerifyResult) fail(format string, args ...any) {
	r.OK = false
	r.Problems = append(r.Problems, fmt.Sprintf(format, args...))
}

func (r *VerifyResult) warn(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// Verify проверяет пакет выгрузки в dir. dec расшифровывает файлы конверта; для пакета
// без шифрования и для чтения открытых файлов внутри частично зашифрованного каталога
// (такого пакет не производит, но проверка не полагается на это) можно передать nil - тогда
// зашифрованный файл честно даёт отказ вместо попытки разобрать конверт как текст.
//
// wantType и wantID - ожидаемая личность пакета (пустая строка и 0 отключают сверку
// соответствующего поля). Без них пакет с манифестом другой сущности проходит проверку
// целостности чисто, если он внутренне непротиворечив - подмена содержимого манифеста на
// чужую организацию не даёт расхождений в отпечатках, ведь они считаются от того же
// (поддельного) манифеста. Снос (следующий срез) получает от оператора и путь к пакету, и
// id сносимой сущности одновременно - сверка личности живёт здесь, а не заводится второй
// проверкой поверх готового результата.
func Verify(ctx context.Context, db *gorm.DB, dir string, dec Decryptor, wantType string, wantID int) (VerifyResult, error) {
	res, loaded := verifyStructure(dir, dec)
	if !loaded {
		// Манифест не нашёлся или не разобрался - сверять со схемой базы и с личностью
		// нечего, дальнейшие проверки только повторили бы одну и ту же причину отказа.
		return res, nil
	}
	checkIdentity(&res, wantType, wantID)
	if err := verifySchema(ctx, db, &res); err != nil {
		return VerifyResult{}, fmt.Errorf("сверка со схемой базы: %w", err)
	}
	return res, nil
}

// checkIdentity сверяет тип и id манифеста с тем, что ожидал вызывающий. Обе части
// проверяются независимо - несовпадение любой даёт отказ со своим сообщением.
func checkIdentity(res *VerifyResult, wantType string, wantID int) {
	if wantType != "" && res.Manifest.Type != wantType {
		res.fail("пакет собран для сущности %q, а проверяется как %q - это не тот пакет",
			res.Manifest.Type, wantType)
	}
	if wantID != 0 && res.Manifest.ID != wantID {
		res.fail("пакет собран для #%d, а проверяется как #%d - это не тот пакет",
			res.Manifest.ID, wantID)
	}
}

// verifyStructure делает всё, что проверяется без обращения к базе: манифест, версию,
// целостность файлов, число строк и лишние файлы в каталоге. loaded=false значит, что
// манифеста нет вовсе - VerifyResult в этом случае уже несёт причину отказа.
func verifyStructure(dir string, dec Decryptor) (VerifyResult, bool) {
	res := VerifyResult{OK: true}

	m, name, err := readManifest(dir, dec)
	if err != nil {
		res.fail("%v", err)
		return res, false
	}
	res.Manifest = m
	res.ManifestEncrypted = strings.HasSuffix(name, ageSuffix)
	checkVersion(&res, m.Version)
	checkGraphMembership(&res, m)
	checkEncryptionConsistency(&res, m, res.ManifestEncrypted)

	// Опись определяет, чего в каталоге ждать: имя манифеста плюс все файлы таблиц и
	// вложений. Всё остальное на диске - лишнее (проверяется ниже).
	expected := map[string]bool{name: true}
	for _, t := range m.Tables {
		expected[t.File] = true
		res.Files = append(res.Files, verifyTableFile(t, dir, dec, &res))
	}
	for _, f := range m.Files {
		expected[f.File] = true
		res.Files = append(res.Files, verifyDataFile(f, dir, dec, &res))
	}
	checkExtraFiles(dir, expected, &res)

	return res, true
}

// readManifest находит манифест на диске (открытым текстом или под конвертом),
// расшифровывает при необходимости и разбирает. Возвращает и имя файла - оно нужно
// дальше, чтобы не принять сам манифест за «лишний файл» каталога.
//
// Легитимный пакет производит РОВНО ОДНО из двух имён (export пишет манифест либо
// открытым текстом, либо конвертом - никогда оба сразу). Если на диске нашлись оба, это
// не повод довериться более защищённому варианту: ревью показало ровно обратный сценарий -
// поддельный открытый manifest.json рядом с настоящим manifest.json.age. Перебор кандидатов
// по порядку читал бы подделку, а настоящий манифест тонул бы в списке предупреждений как
// «лишний файл». Единственный безопасный ответ на противоречивое состояние - отказ.
func readManifest(dir string, dec Decryptor) (Manifest, string, error) {
	var present []string
	// statErr копит первую ошибку доступа (не "файла нет"), чтобы не путать её с
	// отсутствием манифеста: EACCES на каталоге пакета выглядит для os.Stat так же, как
	// ENOENT, но означает совсем другую причину отказа и требует другого действия от
	// оператора - поправить права, а не искать пропавший файл.
	var statErr error
	for _, candidate := range manifestCandidates {
		_, err := os.Stat(filepath.Join(dir, candidate))
		switch {
		case err == nil:
			present = append(present, candidate)
		case errors.Is(err, fs.ErrPermission):
			if statErr == nil {
				statErr = err
			}
		}
	}
	switch len(present) {
	case 0:
		if statErr != nil {
			return Manifest{}, "", fmt.Errorf("манифест: нет доступа к каталогу пакета: %w", statErr)
		}
		return Manifest{}, "", fmt.Errorf("манифест не найден: нет ни %s, ни %s", manifestName, manifestName+ageSuffix)
	case 1:
		// единственный кандидат - штатный случай, идём дальше
	default:
		return Manifest{}, "", fmt.Errorf("в каталоге пакета одновременно %s - легитимный пакет производит "+
			"только один из них, оба сразу означают подмену или повреждение пакета", strings.Join(present, " и "))
	}
	name := present[0]

	rc, err := openPackageFile(filepath.Join(dir, name), dec)
	if err != nil {
		return Manifest{}, "", fmt.Errorf("манифест %s: %w", name, err)
	}
	defer rc.Close()
	body, err := io.ReadAll(rc)
	if err != nil {
		return Manifest{}, "", fmt.Errorf("манифест %s: чтение: %w", name, err)
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return Manifest{}, "", fmt.Errorf("манифест %s: не разбирается как JSON: %w", name, err)
	}
	return m, name, nil
}

// checkVersion сравнивает версию формата пакета с той, что понимает текущая система.
// Разные тексты отказа под "новее"/"старше" не декоративны: оператору нужно разное
// действие - обновить систему или развернуть пакет там, где он ещё поддерживается.
func checkVersion(res *VerifyResult, version int) {
	switch {
	case version > PackageVersion:
		res.fail("пакет собран более новой версией формата (%d) - обновите систему: "+
			"текущая поддерживаемая версия %d", version, PackageVersion)
	case version < PackageVersion:
		res.fail("пакет собран устаревшей версией формата (%d, текущая версия системы - %d) - "+
			"разверните его на стенде с той версией системы, которой он был собран", version, PackageVersion)
	}
}

// checkGraphMembership сверяет состав манифеста с картой графа (allowedNodeTables), а не
// со схемой базы. Сверка со схемой (verifySchema ниже) доказывает только "такая таблица
// где-то в этой базе есть" - подменённый манифест мог бы назвать любую реальную таблицу
// (настройки, роли, права) и пройти её чисто. Этот якорь независим от схемы: то, что
// реально принадлежит графу entityType, задано кодом (organizationNodes), а не тем, что
// нашлось в information_schema.
func checkGraphMembership(res *VerifyResult, m Manifest) {
	allowed, err := allowedNodeTables(m.Type)
	if err != nil {
		res.fail("%v", err)
		return
	}
	for _, t := range m.Tables {
		if !allowed[t.Table] {
			res.fail("таблица %s не входит в граф %s - манифест ссылается на объект вне ожидаемого состава пакета",
				t.Table, m.Type)
		}
	}
}

// checkEncryptionConsistency сверяет заявленный Manifest.Encrypted (часть ТЕЛА манифеста,
// то есть заявление пакета о самом себе) с тем, как манифест РЕАЛЬНО лежал на диске
// (manifestFileEncrypted - имя файла, а не содержимое), и с тем, как названы файлы описи.
// Расхождение - само по себе повод отказать, а не только сигнал для Purge: открытый
// manifest.json с телом "encrypted": true иначе проходил бы любую проверку целостности
// чисто (отпечатки в нём сходятся сами с собой) и обманывал бы гейт "пакет обязан быть
// зашифрован" (см. ManifestEncrypted и Purge в purge.go), оставаясь при этом читаемым без
// единого ключа. Та же сверка идёт и по каждому файлу таблиц/вложений - иначе конверт
// достаточно было бы натянуть только на манифест, оставив содержимое таблиц открытым.
func checkEncryptionConsistency(res *VerifyResult, m Manifest, manifestFileEncrypted bool) {
	if m.Encrypted != manifestFileEncrypted {
		res.fail("манифест заявляет encrypted=%v, но сам файл манифеста реально лежит %s - "+
			"расхождение между описанием и действительностью, пакет считается изменённым",
			m.Encrypted, sealedLabel(manifestFileEncrypted))
		return
	}
	for _, t := range m.Tables {
		if strings.HasSuffix(t.File, ageSuffix) != m.Encrypted {
			res.fail("таблица %s: файл %s не соответствует заявленному в манифесте encrypted=%v",
				t.Table, t.File, m.Encrypted)
		}
	}
	for _, f := range m.Files {
		if strings.HasSuffix(f.File, ageSuffix) != m.Encrypted {
			res.fail("файл %s (заявка %d): не соответствует заявленному в манифесте encrypted=%v",
				f.File, f.RowID, m.Encrypted)
		}
	}
}

func sealedLabel(sealed bool) string {
	if sealed {
		return "конвертом"
	}
	return "открытым текстом"
}

// verifyTableFile проверяет один файл tables/*.jsonl: отпечаток, размер, число строк из
// описи и что каждая строка разбирается как JSON-объект. Расхождение по любому пункту -
// отказ, а не предупреждение: развернуть такую таблицу означало бы разойтись с описью
// молча.
func verifyTableFile(t TableFile, dir string, dec Decryptor, res *VerifyResult) FileCheck {
	check := FileCheck{Name: t.File}

	rc, err := openPackageFile(filepath.Join(dir, filepath.FromSlash(t.File)), dec)
	if err != nil {
		res.fail("таблица %s (%s): файл не читается: %v", t.Table, t.File, err)
		check.State = "нет файла"
		return check
	}
	defer rc.Close()
	body, err := io.ReadAll(rc)
	if err != nil {
		res.fail("таблица %s (%s): чтение: %v", t.Table, t.File, err)
		check.State = "ошибка чтения"
		return check
	}

	ok := true
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != t.SHA256 {
		res.fail("таблица %s (%s): отпечаток не совпадает с описью", t.Table, t.File)
		ok = false
	}
	if int64(len(body)) != t.Bytes {
		res.fail("таблица %s (%s): размер %d байт, в описи %d", t.Table, t.File, len(body), t.Bytes)
		ok = false
	}

	lines := splitJSONLines(body)
	check.Rows = int64(len(lines))
	if int64(len(lines)) != t.Rows {
		res.fail("таблица %s: строк %d, в описи заявлено %d", t.Table, len(lines), t.Rows)
		ok = false
	}
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			res.fail("таблица %s: строка %d не разбирается как JSON-объект", t.Table, i+1)
			ok = false
		}
	}

	check.OK = ok
	check.State = stateLabel(ok)
	return check
}

// verifyDataFile проверяет один файл заявки в files/: отпечаток и размер против описи.
// У вложений нет построчного формата, поэтому в отличие от таблиц здесь нет счётчика строк.
func verifyDataFile(f DataFile, dir string, dec Decryptor, res *VerifyResult) FileCheck {
	check := FileCheck{Name: f.File}

	rc, err := openPackageFile(filepath.Join(dir, filepath.FromSlash(f.File)), dec)
	if err != nil {
		res.fail("файл %s (заявка %d, %s): файл не читается: %v", f.File, f.RowID, f.Table, err)
		check.State = "нет файла"
		return check
	}
	defer rc.Close()
	body, err := io.ReadAll(rc)
	if err != nil {
		res.fail("файл %s: чтение: %v", f.File, err)
		check.State = "ошибка чтения"
		return check
	}

	ok := true
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != f.SHA256 {
		res.fail("файл %s (заявка %d): отпечаток не совпадает с описью", f.File, f.RowID)
		ok = false
	}
	if int64(len(body)) != f.Bytes {
		res.fail("файл %s: размер %d байт, в описи %d", f.File, len(body), f.Bytes)
		ok = false
	}

	check.OK = ok
	check.State = stateLabel(ok)
	return check
}

func stateLabel(ok bool) string {
	if ok {
		return "ок"
	}
	return "ошибка"
}

// splitJSONLines режет тело файла на строки, отбрасывая завершающий перевод строки:
// writeTable в export.go пишет объект-per-line через json.Encoder, и каждая строка (в том
// числе последняя) заканчивается "\n" - без обрезки подсчёт строк был бы на одну больше.
func splitJSONLines(body []byte) [][]byte {
	trimmed := strings.TrimRight(string(body), "\n")
	if trimmed == "" {
		return nil
	}
	raw := strings.Split(trimmed, "\n")
	out := make([][]byte, len(raw))
	for i, line := range raw {
		out[i] = []byte(line)
	}
	return out
}

// checkExtraFiles ищет в каталоге пакета файлы, которых нет в описи. Это предупреждение,
// не отказ (спека прямо разделяет два случая) - но провал самого обхода каталога молчать
// не должен: иначе оператор решит, что лишних файлов нет, хотя каталог попросту не
// досмотрели.
func checkExtraFiles(dir string, expected map[string]bool, res *VerifyResult) {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !expected[rel] {
			res.warn("лишний файл в пакете: %s", rel)
		}
		return nil
	})
	if err != nil {
		res.warn("не удалось полностью проверить каталог на лишние файлы: %v", err)
	}
}

// verifySchema сверяет колонки каждой таблицы описи со схемой ТЕКУЩЕЙ базы: годный пакет
// обязан разворачиваться именно сюда, а не абстрактно куда-то.
func verifySchema(ctx context.Context, db *gorm.DB, res *VerifyResult) error {
	for _, t := range res.Manifest.Tables {
		cols, err := tableColumns(ctx, db, t.Table)
		if err != nil {
			return err
		}
		if len(cols) == 0 {
			res.fail("таблица %s: в текущей схеме базы такой таблицы нет - разворачивать некуда", t.Table)
			continue
		}
		for _, c := range t.Columns {
			if !containsString(cols, c) {
				res.fail("таблица %s: колонка %s есть в пакете, но отсутствует в текущей схеме базы - "+
					"разворачивать некуда", t.Table, c)
			}
		}
		for _, c := range cols {
			if !containsString(t.Columns, c) {
				res.warn("таблица %s: колонка %s есть в схеме базы, но отсутствует в пакете - "+
					"при развороте получит значение по умолчанию", t.Table, c)
			}
		}
	}
	return nil
}

func tableColumns(ctx context.Context, db *gorm.DB, table string) ([]string, error) {
	var cols []string
	q := `SELECT column_name FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = ? ORDER BY column_name`
	if err := db.WithContext(ctx).Raw(q, table).Scan(&cols).Error; err != nil {
		return nil, fmt.Errorf("колонки %s: %w", table, err)
	}
	return cols, nil
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// openPackageFile открывает файл пакета на чтение. Расшифровку (если файл под конвертом)
// делает сам dec - контракт зеркалит services.ArchiveCrypto.Open: признак шифрования
// берётся из суффикса имени, а не из настройки. dec может быть nil (пакет без шифрования
// или проверка запущена без ключей) - тогда зашифрованный файл честно отказывает, вместо
// того чтобы попытаться прочитать конверт как открытый текст.
func openPackageFile(full string, dec Decryptor) (io.ReadCloser, error) {
	if dec != nil {
		return dec.Open(full)
	}
	if strings.HasSuffix(full, ageSuffix) {
		return nil, fmt.Errorf("файл закрыт конвертом, а ключ для расшифровки не передан")
	}
	return os.Open(full)
}
