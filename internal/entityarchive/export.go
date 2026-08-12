package entityarchive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/models"

	"gorm.io/gorm"
)

// Выгрузка графа сущности в пакет.
//
// Пакет - каталог <тип>-<id>-<время> в ENTITY_EXPORT_PATH:
//
//	manifest.json         опись: версия формата, цель, время, колонки и счётчики строк
//	                      каждой таблицы, sha256 каждого файла пакета
//	tables/<table>.jsonl  по объекту-строке на запись, таблицы в порядке карты графа
//	files/<номер>         файлы, приложенные к заявкам организации
//
// При включённом шифровании каждый файл пакета лежит age-конвертом (суффикс .age) на
// тех же ключах, что и файловый архив бланков - ARCHIVE_AGE_RECIPIENT и
// ARCHIVE_AGE_IDENTITY. Отдельного ключа для выгрузки не заводим: он хранился бы там же,
// где уже лежит архивный, и добавлял бы не защиту, а ещё одну сущность для потери.
//
// Имена файлов внутри пакета безличные (числовые). Имя файла на диске конвертом не
// закрывается, а «Паспорт Иванова.pdf» в листинге каталога - это ровно те данные, ради
// которых пакет и шифруют. Настоящее имя лежит в манифесте, то есть под конвертом.
//
// Что в пакет НЕ входит:
//   - общие справочники (гражданства, марки, посты, места разгрузки): организации они не
//     принадлежат, на приёмной стороне свои;
//   - файловый архив бланков (ARCHIVE_PATH): отдельное хранилище со своей выгрузкой,
//     бланк восстановим из заявки.
//
// Поля, зашифрованные ключом системы (паспорт, патент), уезжают в пакет как лежат в
// базе: строки читаются сырым SELECT, мимо AfterFind. Развернуть их на другом стенде
// можно только с тем же DATA_ENCRYPTION_KEY, и манифест отмечает это признаком
// field_encryption - чтобы импорт не принял шифротекст за данные.

// PackageVersion - версия формата пакета. Импорт сверяет её и отказывается разворачивать
// пакет, собранный форматом, которого не знает.
const PackageVersion = 1

const (
	manifestName = "manifest.json"
	tablesDir    = "tables"
	filesDir     = "files"

	// applicationFilesDir - подкаталог загрузок, где лежат файлы заявок
	// (services.NewApplicationFileService собирает его так же).
	applicationFilesDir = "application_files"
)

// Encryptor - шифрование пакета age-конвертом. Интерфейс объявлен здесь, а реализация
// берётся готовая (services.ArchiveCrypto): иначе сборщик графа потянул бы за собой весь
// пакет сервисов ради трёх методов.
type Encryptor interface {
	Enabled() bool
	FileName(name string) string
	Encrypt(data []byte) ([]byte, error)
}

// Manifest - опись пакета. Её же читает импорт: содержимое пакета описано здесь, а не
// выводится из имён файлов в каталоге.
type Manifest struct {
	Version   int       `json:"version"`
	Type      string    `json:"type"`
	ID        int       `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	// Encrypted - файлы пакета закрыты age-конвертами.
	Encrypted bool `json:"encrypted"`
	// FieldEncryption - чем закрыты отдельные поля внутри строк: system_key означает,
	// что паспорта и патенты лежат под ключом системы и без него не читаются.
	FieldEncryption string      `json:"field_encryption"`
	Tables          []TableFile `json:"tables"`
	Files           []DataFile  `json:"files"`
}

// TableFile - выгруженная таблица графа.
type TableFile struct {
	Table   string   `json:"table"`
	Rows    int64    `json:"rows"`
	Columns []string `json:"columns"`
	// File - путь внутри пакета, уже с суффиксом конверта, если он есть.
	File   string `json:"file"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// DataFile - файл, приложенный к записи графа. SHA256 и Bytes считаются по открытому
// содержимому: проверять после разворота конверта надо именно исходный файл.
type DataFile struct {
	Table        string `json:"table"`
	RowID        int    `json:"row_id"`
	File         string `json:"file"`
	OriginalName string `json:"original_name"`
	Bytes        int64  `json:"bytes"`
	SHA256       string `json:"sha256"`
}

// ExportOptions - что нужно выгрузке помимо самого графа.
type ExportOptions struct {
	// Root - корень пакетов, ENTITY_EXPORT_PATH.
	Root string
	// UploadPath - корень загрузок: под ним лежат файлы заявок.
	UploadPath string
	// Crypto - шифрование пакета. Выключенное (nil или без ключей) пишет открытым текстом.
	Crypto Encryptor
	// Recorder пишет audit_log о снятой копии. Обязателен при DryRun=false - выгрузка без
	// следа в журнале не считается штатным путём (см. Export). Пробный прогон Recorder не
	// требует: он ничего не пишет ни в пакет, ни в журнал.
	Recorder AuditRecorder
	// Now - момент пакета: попадает в имя каталога и в манифест.
	Now time.Time
	// DryRun - только посчитать, ничего не записывать.
	DryRun bool
	// AfterSnapshotSuccessForTest - НЕ для прода, только для тестов. Коллбэк вызывается
	// сразу после того, как снимок собрал пакет успешно (manifest.json на диске), но ДО
	// того, как Transaction() ниже попытается его закоммитить.
	//
	// Единственное назначение - тест пути "commit снимка упал уже после полной записи
	// пакета" (см. Export, ветка written): между возвратом exportInSnapshot и вызовом
	// Commit() нет ни одной точки синхронизации - голая цепочка вызовов одной горутины,
	// поэтому сбой ровно на коммите иначе не воспроизвести. Внешний обрыв по таймингу
	// (закрыть соединение из другой горутины, отменить контекст снаружи) эту гонку
	// проигрывает чаще, чем выигрывает: сам обрыв требует либо сетевого round-trip, либо
	// побудки чужой горутины, а путь до Commit() - нет. На этом пути держится инвариант
	// «нет записи в журнале - нет пакета»: не заполнено (обычный случай) - никакого
	// эффекта; заполнено - коллбэк получает tx именно этой транзакции и может честно
	// оборвать СВОЁ соединение (например, SELECT pg_terminate_backend(pg_backend_pid())),
	// не имитацией на уровне драйвера, а настоящим обрывом.
	AfterSnapshotSuccessForTest func(tx *gorm.DB)
}

// ExportResult - что получилось (или получилось бы при пробном прогоне).
type ExportResult struct {
	Dir       string
	DryRun    bool
	Manifest  Manifest
	Rows      int64
	Files     int
	FileBytes int64
	// ManifestSHA256 - sha256 от манифеста ДО шифрования (тем же приёмом, что и у
	// TableFile.SHA256 - отпечаток описывает содержимое, а не то, во что оно завёрнуто).
	// Уходит в audit_log вместе с фактом выгрузки: единственный внешний якорь, по которому
	// позже можно отличить подменённый пакет от настоящего. Пусто при DryRun - манифест
	// тогда на диск не пишется вовсе.
	ManifestSHA256 string
	// Warnings - то, что оператор обязан увидеть, но что не мешает выгрузке.
	Warnings []string
}

// Export собирает пакет по графу сущности. Базу не меняет: только SELECT.
//
// Ошибка на любом шаге прерывает выгрузку и уносит недописанный каталог: обрубок с
// готовым манифестом легко принять за целый пакет, а на пакете держится инвариант
// «не сносить, пока копия не снята и не проверена».
//
// Реальная (не пробная) выгрузка обязана оставить след в audit_log: снятие копии всех
// персональных данных организации иначе прошло бы бесследно. Сама выгрузка идёт в
// read-only снимке базы (см. ниже) - писать в него нельзя, поэтому запись в журнал делает
// ОТДЕЛЬНЫЙ шаг после того, как снимок закрыт и пакет уже на диске. Если эта запись не
// удалась, пакет удаляется: «нет записи в журнале - нет пакета», а не наоборот.
func Export(ctx context.Context, db *gorm.DB, entityType string, id int, opt ExportOptions) (ExportResult, error) {
	if strings.TrimSpace(opt.Root) == "" {
		return ExportResult{}, fmt.Errorf("каталог выгрузки не задан: укажите ENTITY_EXPORT_PATH")
	}
	if !opt.DryRun && opt.Recorder == nil {
		// Отказ здесь же, до единого SELECT: тихая выгрузка без следа в audit_log
		// запрещена владельцем, а не просто нежелательна. Пробный прогон Recorder не
		// требует - см. комментарий поля.
		return ExportResult{}, errors.New("не задан журнал аудита (Recorder) - запись пакета без следа в audit_log запрещена")
	}
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}

	// Весь пакет собирается в ОДНОМ снимке базы: repeatable read держит все выборки на
	// одном моменте, поэтому список файлов, строки таблиц и счётчики описывают одно и то
	// же состояние. Без снимка выгрузка растянута на десятки запросов, и поданная в этот
	// момент заявка попадала бы в один файл пакета и отсутствовала в другом - неполный,
	// но с виду целый пакет, а на нём держится право сносить данные из живой системы.
	// Read only здесь не украшение: он запрещает записи на уровне самой базы, а не только
	// обещанием в комментарии.
	//
	// Цена снимка: пока идёт выгрузка, база держит горизонт очистки мёртвых строк. Для
	// команды, которую запускают при офбординге организации, это дешевле, чем пакет,
	// собранный из двух разных состояний.
	var res ExportResult
	txErr := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		res, err = exportInSnapshot(ctx, tx, entityType, id, opt)
		if err == nil && opt.AfterSnapshotSuccessForTest != nil {
			// Точка встраивания для теста (см. комментарий поля) - в продакшене поле
			// всегда nil, сюда не заходим.
			opt.AfterSnapshotSuccessForTest(tx)
		}
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})

	// written отличает "снимок не собрался" (exportInSnapshot вернула ошибку - сама
	// возвращает ExportResult{} и уже снесла свой обрубок defer'ом) от "снимок собрался
	// целиком, упал только COMMIT" (exportInSnapshot вернула res, nil - весь пакет,
	// включая manifest.json, уже на диске, а Transaction() поймала ошибку САМОГО commit,
	// например обрыв соединения или рестарт базы). packageWritten проверяет заполненный
	// манифест - его успевает получить только res второго случая, в первом res == ExportResult{}.
	written := !opt.DryRun && res.Dir != "" && packageWritten(res)
	if txErr != nil && !written {
		return ExportResult{}, txErr
	}
	if opt.DryRun {
		if txErr != nil {
			return ExportResult{}, txErr
		}
		return res, nil
	}

	// С этой точки пакет ФИЗИЧЕСКИ на диске - снимок закрылся штатно, либо (written==true)
	// упал только commit уже ПОСЛЕ того, как пакет дописан. В обоих случаях запись в
	// журнал обязана произойти: она и так идёт не через транзакцию снимка (та либо уже
	// закоммичена, либо мертва - неважно, recordExport открывает своё соединение через
	// r.Record(ctx, nil, ...)), поэтому сбой commit'а снимка ей не помеха.
	if err := recordExport(ctx, opt.Recorder, entityType, id, res); err != nil {
		// Запись в журнал не удалась - пакет с персональными данными остался бы на диске
		// без единого следа в audit_log, ровно то, что запрещено (для обоих путей сюда,
		// не только для штатного): удаляем пакет и возвращаем ошибку наверх.
		if rmErr := os.RemoveAll(res.Dir); rmErr != nil {
			// Уборка сама не удалась - молчать нельзя: на диске остался пакет со всеми
			// персональными данными организации, а журнал о его существовании не знает.
			slog.Error("пакет выгрузки не удалось снести после сбоя записи в audit_log",
				"dir", res.Dir, "error", rmErr)
			return ExportResult{}, fmt.Errorf("запись в журнал аудита: %w (пакет %s НЕ снесён, снесите вручную)", err, res.Dir)
		}
		return ExportResult{}, fmt.Errorf("запись в журнал аудита: %w (пакет %s снесён с диска)", err, res.Dir)
	}
	if txErr != nil {
		// Пакет записан и теперь зафиксирован в audit_log несмотря на то, что commit
		// снимка упал - оператору незачем повторять команду: она бы просто сняла вторую
		// такую же копию рядом с первой.
		return ExportResult{}, fmt.Errorf("%w (пакет %s записан и зафиксирован в audit_log - повторный запуск не требуется)", txErr, res.Dir)
	}
	return res, nil
}

// recordExport пишет в audit_log факт снятия пакета. Details не несёт персональных
// данных - ни имён файлов, ни содержимого строк, только метаданные пакета и отпечаток
// манифеста. Отпечаток - единственный внешний якорь, по которому позже можно отличить
// подменённый пакет от настоящего: опись внутри пакета сверяется сама с собой, а
// получатель age публичный, то есть переупаковать весь пакет целиком (вместе с
// манифестом) может кто угодно, у кого есть доступ к диску.
func recordExport(ctx context.Context, r AuditRecorder, entityType string, id int, res ExportResult) error {
	entityID := id
	details := exportAuditDetails{
		Package:        res.Dir,
		Rows:           res.Rows,
		Files:          res.Files,
		Encrypted:      res.Manifest.Encrypted,
		ManifestSHA256: res.ManifestSHA256,
	}
	return r.Record(ctx, nil, entityType, &entityID, models.OrganizationActionExported, nil, details)
}

// exportAuditDetails - подробности записи audit_log о снятой выгрузке.
type exportAuditDetails struct {
	Package        string `json:"package"`
	Rows           int64  `json:"rows"`
	Files          int    `json:"files"`
	Encrypted      bool   `json:"encrypted"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

// packageWritten отвечает, успела ли выгрузка дописать пакет до сбоя. Опись собирается
// последней, поэтому заполненный манифест и есть признак готовности.
func packageWritten(res ExportResult) bool {
	return len(res.Manifest.Tables) > 0 && res.Manifest.Tables[0].SHA256 != ""
}

// exportInSnapshot делает всю работу внутри снимка: db здесь - транзакция, открытая
// вызывающим, и другого источника данных у выгрузки быть не должно.
func exportInSnapshot(ctx context.Context, db *gorm.DB, entityType string, id int, opt ExportOptions) (ExportResult, error) {
	graph, err := Collect(ctx, db, entityType, id)
	if err != nil {
		return ExportResult{}, err
	}
	if graph.Total() == 0 {
		return ExportResult{}, fmt.Errorf("%s #%d: связанных записей нет, выгружать нечего", entityType, id)
	}

	files, err := applicationFileRows(ctx, db, id)
	if err != nil {
		return ExportResult{}, err
	}

	res := ExportResult{
		DryRun:   opt.DryRun,
		Rows:     graph.Total(),
		Files:    len(files),
		Warnings: exportWarnings(graph),
		Dir: filepath.Join(opt.Root, fmt.Sprintf("%s-%d-%s",
			graph.Type, graph.ID, opt.Now.UTC().Format("20060102-150405"))),
	}
	for _, f := range files {
		res.FileBytes += f.Size
	}
	res.Manifest = Manifest{
		Version:         PackageVersion,
		Type:            graph.Type,
		ID:              graph.ID,
		CreatedAt:       opt.Now.UTC(),
		Encrypted:       encEnabled(opt.Crypto),
		FieldEncryption: fieldEncryptionMode(),
	}
	if opt.DryRun {
		// Оператор решает по этому списку, поэтому состав таблиц показываем и без
		// записи; отпечатков и имён файлов здесь нет - их даёт только сама выгрузка.
		for _, t := range graph.Tables {
			res.Manifest.Tables = append(res.Manifest.Tables, TableFile{Table: t.Table, Rows: t.Rows})
		}
		return res, nil
	}

	if err := os.MkdirAll(opt.Root, 0o700); err != nil {
		return ExportResult{}, fmt.Errorf("каталог выгрузки %s: %w", opt.Root, err)
	}
	// Именно Mkdir, а не MkdirAll: существующий каталог с тем же именем означает
	// пакет, снятый в ту же секунду, и дописывать его поверх нельзя.
	if err := os.Mkdir(res.Dir, 0o700); err != nil {
		return ExportResult{}, fmt.Errorf("каталог пакета %s: %w", res.Dir, err)
	}
	done := false
	defer func() {
		if done {
			return
		}
		// Уборка сама может не удаться (права, переполненный раздел). Промолчать здесь
		// нельзя: оператор увидел бы только исходную ошибку и не узнал, что на диске
		// остался обрубок с персональными данными.
		if err := os.RemoveAll(res.Dir); err != nil {
			slog.Error("недописанный пакет выгрузки остался на диске", "dir", res.Dir, "error", err)
		}
	}()

	ordered, err := tablesWithID(ctx, db)
	if err != nil {
		return ExportResult{}, err
	}
	for _, node := range organizationNodes() {
		entry, err := writeTable(ctx, db, node, id, res.Dir, opt.Crypto, ordered[node.Table])
		if err != nil {
			return ExportResult{}, err
		}
		if entry != nil {
			res.Manifest.Tables = append(res.Manifest.Tables, *entry)
		}
	}
	for _, row := range files {
		entry, err := writeDataFile(res.Dir, opt.UploadPath, row, opt.Crypto)
		if err != nil {
			return ExportResult{}, err
		}
		res.Manifest.Files = append(res.Manifest.Files, entry)
	}

	body, err := json.MarshalIndent(res.Manifest, "", "  ")
	if err != nil {
		return ExportResult{}, fmt.Errorf("сборка манифеста: %w", err)
	}
	// Отпечаток считается с ОТКРЫТОГО тела манифеста, тем же приёмом, что и SHA256 у
	// TableFile (writeTable выше) - до возможного шифрования writePackageFile. Так
	// отпечаток в audit_log остаётся проверяемым независимо от того, включено шифрование
	// пакета или нет.
	manifestSum := sha256.Sum256(body)
	res.ManifestSHA256 = hex.EncodeToString(manifestSum[:])
	if _, err := writePackageFile(res.Dir, manifestName, body, opt.Crypto); err != nil {
		return ExportResult{}, err
	}

	done = true
	return res, nil
}

// writeTable выгружает один узел графа в tables/<table>.jsonl. Пустой узел файла не
// получает и в манифест не попадает: пустой файл в описи читался бы как «данные были и
// потерялись», а не «данных нет».
func writeTable(ctx context.Context, db *gorm.DB, node Node, id int, dir string, enc Encryptor, ordered bool) (*TableFile, error) {
	q := "SELECT * FROM " + node.Table + " WHERE " + node.Where
	if ordered {
		// Порядок строк должен быть воспроизводимым: два снимка одних и тех же данных
		// обязаны дать одинаковый sha256, иначе сверка пакета превращается в гадание.
		q += " ORDER BY id"
	}
	rows, err := db.WithContext(ctx).Raw(q, sql.Named("org", id)).Rows()
	if err != nil {
		return nil, fmt.Errorf("выборка %s: %w", node.Table, err)
	}
	defer rows.Close()

	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("колонки %s: %w", node.Table, err)
	}
	columns := make([]string, len(types))
	for i, ct := range types {
		columns[i] = ct.Name()
	}

	var buf bytes.Buffer
	writer := json.NewEncoder(&buf)
	// Данные читает человек и разбирает импорт: экранировать в них угловые скобки
	// незачем, а читаемость это портит.
	writer.SetEscapeHTML(false)

	var count int64
	for rows.Next() {
		values := make([]any, len(types))
		ptrs := make([]any, len(types))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("чтение строки %s: %w", node.Table, err)
		}
		record := make(map[string]any, len(types))
		for i, ct := range types {
			v, err := encodeValue(node.Table, ct.Name(), ct.DatabaseTypeName(), values[i])
			if err != nil {
				return nil, err
			}
			record[ct.Name()] = v
		}
		if err := writer.Encode(record); err != nil {
			return nil, fmt.Errorf("запись строки %s: %w", node.Table, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("обход строк %s: %w", node.Table, err)
	}
	if count == 0 {
		return nil, nil
	}

	sum := sha256.Sum256(buf.Bytes())
	name, err := writePackageFile(dir, path.Join(tablesDir, node.Table+".jsonl"), buf.Bytes(), enc)
	if err != nil {
		return nil, err
	}
	return &TableFile{
		Table:   node.Table,
		Rows:    count,
		Columns: columns,
		File:    name,
		Bytes:   int64(buf.Len()),
		SHA256:  hex.EncodeToString(sum[:]),
	}, nil
}

// encodeValue приводит значение колонки к тому, что переживёт JSON и обратный разбор.
//
// Драйвер отдаёт нераспознанные типы куском байт, и разница между «текст» и «двоичное»
// на этом уровне уже не видна - её приходится брать из типа колонки.
func encodeValue(table, column, dbType string, v any) (any, error) {
	raw, ok := v.([]byte)
	if !ok {
		return v, nil
	}
	switch strings.ToUpper(dbType) {
	case "JSON", "JSONB":
		// Кладём как есть: обёрнутый в строку json стал бы при импорте текстом,
		// а не объектом.
		return json.RawMessage(raw), nil
	case "BYTEA":
		// Двоичных колонок в схеме сейчас нет (файлы лежат на диске). Появится такая -
		// пусть выгрузка честно откажется, а не превратит содержимое в битую строку.
		return nil, fmt.Errorf("%s.%s: двоичные колонки (%s) формат пакета не поддерживает", table, column, dbType)
	default:
		return string(raw), nil
	}
}

// appFileRow - строка файла заявки: что искать на диске и как назвать в описи.
type appFileRow struct {
	ID         int
	StoredName string
	FileName   string
	Encrypted  bool
	Size       int64
}

// applicationFileRows - файлы, приложенные к заявкам организации.
//
// Это единственный источник файлов в графе организации: employee_files числится в
// схеме, но ни один путь кода в неё не пишет, а бланки и слепки заявок живут в
// отдельном файловом архиве (ARCHIVE_PATH) со своей выгрузкой.
func applicationFileRows(ctx context.Context, db *gorm.DB, id int) ([]appFileRow, error) {
	var rows []appFileRow
	q := `SELECT id, stored_name, file_name, encrypted, file_size AS size
		FROM application_files WHERE application_id IN (` + orgApps + `) ORDER BY id`
	if err := db.WithContext(ctx).Raw(q, sql.Named("org", id)).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("файлы заявок: %w", err)
	}
	return rows, nil
}

// writeDataFile кладёт в пакет один файл заявки, снимая шифрование ключом системы.
//
// Именно снимая: файл на диске закрыт ключом этой установки, и пакет с ним внутри
// развернулся бы только здесь же. В конверте пакета он поедет закрытым заново - ключами
// получателя, как это уже делает выгрузка файлового архива.
func writeDataFile(dir, uploadPath string, row appFileRow, enc Encryptor) (DataFile, error) {
	if row.StoredName == "" {
		return DataFile{}, fmt.Errorf("файл заявки %d (%s): в базе нет имени файла на диске", row.ID, row.FileName)
	}
	if row.Encrypted && len(crypto.GetGlobalKey()) == 0 {
		return DataFile{}, fmt.Errorf("файл заявки %d закрыт ключом системы, а DATA_ENCRYPTION_KEY не задан: "+
			"без него в пакет уехал бы шифротекст", row.ID)
	}

	src := filepath.Join(uploadPath, applicationFilesDir, row.StoredName)
	f, err := os.Open(src)
	if err != nil {
		return DataFile{}, fmt.Errorf("файл заявки %d (%s): %w", row.ID, row.FileName, err)
	}
	defer f.Close()

	var key []byte
	if row.Encrypted {
		key = crypto.GetGlobalKey()
	}
	reader, err := crypto.NewStreamReader(f, key)
	if err != nil {
		return DataFile{}, fmt.Errorf("файл заявки %d: %w", row.ID, err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return DataFile{}, fmt.Errorf("чтение файла заявки %d: %w", row.ID, err)
	}

	sum := sha256.Sum256(data)
	name, err := writePackageFile(dir, path.Join(filesDir, fmt.Sprintf("%06d", row.ID)), data, enc)
	if err != nil {
		return DataFile{}, err
	}
	return DataFile{
		Table:        "application_files",
		RowID:        row.ID,
		File:         name,
		OriginalName: row.FileName,
		Bytes:        int64(len(data)),
		SHA256:       hex.EncodeToString(sum[:]),
	}, nil
}

// writePackageFile записывает файл пакета, при включённом шифровании - конвертом.
// Возвращает имя внутри пакета (уже с суффиксом): по нему импорт находит файл в описи,
// а не угадывает по настройке, с которой пакет собирали.
func writePackageFile(dir, name string, data []byte, enc Encryptor) (string, error) {
	if encEnabled(enc) {
		sealed, err := enc.Encrypt(data)
		if err != nil {
			return "", fmt.Errorf("шифрование %s: %w", name, err)
		}
		data = sealed
		name = enc.FileName(name)
	}
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		return "", fmt.Errorf("каталог для %s: %w", name, err)
	}
	if err := os.WriteFile(full, data, 0o600); err != nil {
		return "", fmt.Errorf("запись %s: %w", name, err)
	}
	return name, nil
}

// tablesWithID - таблицы, у которых есть столбец id: по нему выгрузка сортирует строки.
func tablesWithID(ctx context.Context, db *gorm.DB) (map[string]bool, error) {
	var names []string
	q := `SELECT table_name FROM information_schema.columns
		WHERE table_schema = current_schema() AND column_name = 'id'`
	if err := db.WithContext(ctx).Raw(q).Scan(&names).Error; err != nil {
		return nil, fmt.Errorf("список таблиц с id: %w", err)
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set, nil
}

// exportWarnings - то, о чём оператор должен узнать до того, как решит, что пакет полон.
func exportWarnings(g Graph) []string {
	var out []string
	for _, t := range g.Tables {
		if t.Table == "employee_files" && t.Rows > 0 {
			out = append(out, fmt.Sprintf(
				"employee_files: %d строк уедут в пакет, но сами файлы - нет: ни один путь кода "+
					"эту таблицу не заполняет, и где лежит содержимое, система не знает", t.Rows))
		}
	}
	return out
}

// encEnabled отвечает, шифруется ли пакет. Отдельная функция потому, что выключенное
// шифрование приходит и как nil-интерфейс, и как реализация с пустыми ключами.
func encEnabled(enc Encryptor) bool { return enc != nil && enc.Enabled() }

// fieldEncryptionMode - чем закрыты поля внутри строк на этой установке.
func fieldEncryptionMode() string {
	if len(crypto.GetGlobalKey()) > 0 {
		return "system_key"
	}
	return "none"
}
