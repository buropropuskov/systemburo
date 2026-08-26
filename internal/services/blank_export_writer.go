package services

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"systemburo/internal/blankpath"
)

// Режимы файлового архива (#1615). Каталоги 0750, файлы 0640: содержимое читает
// процесс под uid/gid 1001 и группа, отданная в сетевую папку только на чтение.
// os.Chown не делаем - процесс не root и получил бы EPERM; владельца готовит
// администратор при создании каталога, это описано в руководстве по развёртыванию.
const (
	archiveDirMode  fs.FileMode = 0o750
	archiveFileMode fs.FileMode = 0o640
)

// archiveTempPrefix - префикс временных файлов записи. С точки, чтобы проводник
// Windows их прятал: оператор ходит в эту папку мышкой и недописанный бланк ему
// показывать незачем.
const archiveTempPrefix = ".tmp-"

// ErrArchiveSourceMissing - каталог, который значится в реестре, на диске не найден.
// Штатная ситуация, а не поломка: папку могли удалить руками. Вызывающий очищает
// фактический путь и пишет файлы заново, поэтому ошибка отделена от прочих.
var ErrArchiveSourceMissing = errors.New("archive source directory is missing")

// ArchiveWriter - запись бланков в каталог файлового архива. Знает только про диск:
// какие файлы писать и куда, решает сервис выгрузки, а писатель отвечает за то,
// чтобы на диске никогда не оказалось наполовину записанного .xlsx и чтобы ни одна
// запись не ушла за пределы корня архива.
type ArchiveWriter struct {
	// root - абсолютный путь корня (ARCHIVE_PATH). Относительные пути реестра
	// разрешаются от него, и результат каждый раз проверяется на выход за корень.
	root string
	// crypto - шифрование файлов архива. nil означает прежний режим: файлы
	// пишутся как есть, и площадка без ключей продолжает работать.
	crypto *ArchiveCrypto
}

// NewArchiveWriter создаёт писатель поверх корня архива. Путь приводится к
// абсолютному сразу: рабочий каталог процесса меняться не должен, но проверка
// «не вышли за корень» на относительном пути дала бы ложное срабатывание.
// SetCrypto включает шифрование записываемых файлов. nil оставляет прежний режим.
func (w *ArchiveWriter) SetCrypto(c *ArchiveCrypto) { w.crypto = c }

// Crypto отдаёт шифрование архива: читающей стороне нужен тот же ключ.
func (w *ArchiveWriter) Crypto() *ArchiveCrypto { return w.crypto }

func NewArchiveWriter(root string) (*ArchiveWriter, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("archive root is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve archive root %q: %w", root, err)
	}
	return &ArchiveWriter{root: filepath.Clean(abs)}, nil
}

// Root возвращает абсолютный корень архива.
func (w *ArchiveWriter) Root() string { return w.root }

// Resolve собирает абсолютный путь из уровней и проверяет, что он остался под
// корнем. Второй рубеж после санитайзера blankpath: тот чистит значения, а здесь
// ловится уже собранный путь, каким бы образом в него ни попал "..".
func (w *ArchiveWriter) Resolve(levels ...string) (string, error) {
	if err := validatePathComponents(levels); err != nil {
		return "", err
	}
	return blankpath.JoinUnder(w.root, levels...)
}

// validatePathComponents требует, чтобы каждый уровень был ровно одним элементом
// пути. Проверки «не вышли за корень» тут мало: "2026/.." схлопывается обратно в
// корень, остаётся под ним и всё же кладёт файл не туда, где его потом ищет реестр.
// Легальные значения такого не приносят - санитайзер blankpath меняет разделители
// на дефис, - поэтому компонент с разделителем означает ошибку вызывающего.
func validatePathComponents(levels []string) error {
	for _, level := range levels {
		switch {
		case level == "":
			return errors.New("archive path level is empty")
		case level == "." || level == "..":
			return fmt.Errorf("archive path level %q is a relative reference", level)
		case strings.ContainsRune(level, '/'), strings.ContainsRune(level, filepath.Separator):
			return fmt.Errorf("archive path level %q contains a separator", level)
		}
	}
	return nil
}

// Exists сообщает, лежит ли файл на своём месте. Нужен сверке: строка реестра со
// статусом «записан» и пропавший файл - расхождение, которое надо увидеть, а не
// узнать в момент, когда файл понадобился.
func (w *ArchiveWriter) Exists(levels []string, name string) (bool, error) {
	full, err := w.Resolve(append(append([]string{}, levels...), name)...)
	if err != nil {
		return false, err
	}
	switch _, err := os.Stat(full); {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("failed to stat archive file: %w", err)
	}
}

// WriteFile атомарно кладёт файл в архив, создавая недостающие уровни каталогов.
//
// Порядок шагов важен целиком. Временный файл создаётся В ТОМ ЖЕ каталоге - rename
// между файловыми системами не работает, а /tmp в контейнере лежит на другом слое.
// Sync идёт ДО Rename: место кончается именно на сбросе буферов, и без него файл
// переехал бы на боевое имя пустым, а клиент по сети прочитал бы битый .xlsx.
// Каталог синхронизируется после переименования - иначе при внезапной перезагрузке
// содержимое файла уцелеет, а записи о нём в каталоге не останется.
func (w *ArchiveWriter) WriteFile(levels []string, name string, data []byte) error {
	// Шифруем до записи: на диск не должно попасть даже временного файла с
	// открытым содержимым - подметатель убирает такие с задержкой.
	data, err := w.crypto.Encrypt(data)
	if err != nil {
		return err
	}

	dir, err := w.EnsureDir(levels)
	if err != nil {
		return err
	}
	target, err := w.Resolve(append(append([]string{}, levels...), name)...)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, archiveTempPrefix+"*")
	if err != nil {
		return fmt.Errorf("failed to create temp file in archive: %w", err)
	}
	tmpName := tmp.Name()
	// Уборка на любом неуспешном выходе: незакрытый временный файл иначе остался бы
	// в папке заявки и попал бы оператору на глаза вместе с бланками.
	committed := false
	defer func() {
		if !committed {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("failed to write archive file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("failed to flush archive file: %w", err)
	}
	// Chmod по дескриптору, а не по имени: режим не зависит от umask процесса, и
	// файл не окажется доступным на чтение всем, если umask окажется мягким.
	if err := tmp.Chmod(archiveFileMode); err != nil {
		return fmt.Errorf("failed to set archive file mode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close archive file: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("failed to place archive file: %w", err)
	}
	committed = true

	return syncDir(dir)
}

// RemoveFile убирает файл архива. Отсутствие файла успехом и остаётся: вызов идёт
// из уборки, а повторная уборка не должна выглядеть ошибкой.
func (w *ArchiveWriter) RemoveFile(levels []string, name string) error {
	full, err := w.Resolve(append(append([]string{}, levels...), name)...)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to remove archive file: %w", err)
	}
	return nil
}

// MoveFile переименовывает файл внутри одного каталога архива. Нужен дошифровке:
// содержимое уже закрыто, менять его незачем, а имя обязано получить суффикс.
// Каталоги не создаются намеренно - переезд между папками делает MoveDir.
func (w *ArchiveWriter) MoveFile(levels []string, from, to string) error {
	src, err := w.Resolve(append(append([]string{}, levels...), from)...)
	if err != nil {
		return err
	}
	dst, err := w.Resolve(append(append([]string{}, levels...), to)...)
	if err != nil {
		return err
	}
	if src == dst {
		return nil
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("failed to rename archive file: %w", err)
	}
	return syncDir(filepath.Dir(dst))
}

// EnsureDir создаёт уровни каталогов под корнем и возвращает путь самого глубокого.
//
// Уровни создаются по одному, а режим выставляется только на созданные нами: чужой
// каталог мог быть намеренно открыт администратором для сетевой папки, и молча
// сужать ему права нельзя.
func (w *ArchiveWriter) EnsureDir(levels []string) (string, error) {
	full, err := w.Resolve(levels...)
	if err != nil {
		return "", err
	}
	if err := w.ensureRoot(); err != nil {
		return "", err
	}

	cur := w.root
	for _, level := range levels {
		cur = filepath.Join(cur, level)
		if err := mkdirWithMode(cur); err != nil {
			return "", err
		}
	}
	return full, nil
}

// ensureRoot создаёт корень архива, если его ещё нет.
//
// Готовый каталог не трогаем ни режимом, ни владельцем: на боевом сервере его
// заводит администратор bind-mount'ом с владельцем 1001, и Chmod по чужому каталогу
// вернул бы EPERM - процесс не root. Создали сами - выставляем режим явно, потому
// что Mkdir пропускает запрошенные права через umask, и при жёстком umask каталог
// молча остался бы без группового чтения, ради которого архив и отдают в сетевую папку.
func (w *ArchiveWriter) ensureRoot() error {
	switch err := os.Mkdir(w.root, archiveDirMode); {
	case err == nil:
		return chmodDir(w.root)
	case errors.Is(err, fs.ErrExist):
		return nil
	case errors.Is(err, fs.ErrNotExist):
		// Корня нет вместе с родительскими каталогами - штатно для локального
		// ./archive в разработке, на боевом пути такого не бывает.
		if err := os.MkdirAll(w.root, archiveDirMode); err != nil {
			return fmt.Errorf("failed to create archive root: %w", err)
		}
		return chmodDir(w.root)
	default:
		return fmt.Errorf("failed to create archive root: %w", err)
	}
}

// mkdirWithMode создаёт один каталог и выставляет режим только на созданный.
func mkdirWithMode(dir string) error {
	switch err := os.Mkdir(dir, archiveDirMode); {
	case err == nil:
		return chmodDir(dir)
	case errors.Is(err, fs.ErrExist):
		// Уже есть - ни режим, ни владельца не трогаем: каталог мог быть намеренно
		// открыт администратором под сетевую папку.
		return nil
	default:
		return fmt.Errorf("failed to create archive directory: %w", err)
	}
}

func chmodDir(dir string) error {
	if err := os.Chmod(dir, archiveDirMode); err != nil {
		return fmt.Errorf("failed to set archive directory mode: %w", err)
	}
	return nil
}

// MoveDir переносит папку заявки из фактического положения в желаемое: организацию
// в заявке поправили, и дерево обязано это отразить, иначе рядом появится вторая
// папка с теми же бланками.
//
// Порядок «диск, потом база» задан снаружи и здесь важен: падение сразу после
// переноса чинится само собой на следующем прогоне (фактического каталога уже нет,
// файлы просто перепишутся), а обратный порядок оставил бы каталог-сироту.
func (w *ArchiveWriter) MoveDir(from, to []string) error {
	src, err := w.Resolve(from...)
	if err != nil {
		return err
	}
	dst, err := w.Resolve(to...)
	if err != nil {
		return err
	}
	if src == dst || src == w.root || dst == w.root {
		return nil
	}

	if len(to) > 0 {
		if _, err := w.EnsureDir(to[:len(to)-1]); err != nil {
			return err
		}
	}

	// Отсутствие источника определяется по ошибке самого Rename, а не проверкой
	// заранее: между Stat и Rename папку успели бы удалить, и вызывающий получил бы
	// вместо понятного сигнала «папки нет» невнятную ошибку переноса. Родитель цели
	// к этому моменту уже создан, поэтому ENOENT здесь означает именно источник.
	switch err := os.Rename(src, dst); {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		return ErrArchiveSourceMissing
	case isDirNotEmpty(err):
		// Целевая папка занята: та же заявка уже писалась по новому пути, либо
		// администратор создал каталог руками. Сливаем пофайлово - потерять бланк
		// из-за занятого имени хуже, чем оставить оператору папку со смесью.
		if err := mergeDir(src, dst); err != nil {
			return err
		}
	default:
		return fmt.Errorf("failed to move archive directory: %w", err)
	}

	w.pruneEmptyParents(filepath.Dir(src))
	if parent := filepath.Dir(dst); parent != w.root {
		if err := syncDir(parent); err != nil {
			return err
		}
	}
	return nil
}

// CleanupTemp убирает временные файлы старше указанного возраста. Свежие не трогает:
// в этот момент их может дописывать соседний проход. Возвращает число удалённых.
func (w *ArchiveWriter) CleanupTemp(olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	removed := 0

	err := filepath.WalkDir(w.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Каталог могли снести прямо во время обхода - это не повод бросать
			// уборку остального дерева.
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasPrefix(d.Name(), archiveTempPrefix) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if info.ModTime().After(cutoff) {
			return nil
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		removed++
		return nil
	})
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Корня ещё нет - архив ни разу не писали, убирать нечего.
			return removed, nil
		}
		return removed, fmt.Errorf("failed to clean archive temp files: %w", err)
	}
	return removed, nil
}

// pruneEmptyParents подчищает опустевшие уровни день/месяц/год вверх до корня.
// Best-effort: непустой каталог просто останавливает подъём, а прочие ошибки
// игнорируются - пустая папка в дереве неприятна, но останавливать из-за неё
// выгрузку бланков не за что.
func (w *ArchiveWriter) pruneEmptyParents(dir string) {
	prefix := w.root + string(filepath.Separator)
	for dir != w.root && strings.HasPrefix(dir, prefix) {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// mergeDir переносит содержимое src в существующий dst и убирает опустевший src.
// Вложенные каталоги обрабатываются рекурсивно: в папке заявки лежат только файлы,
// но ронять перенос из-за неожиданного подкаталога незачем.
func mergeDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read archive directory: %w", err)
	}
	for _, entry := range entries {
		from := filepath.Join(src, entry.Name())
		to := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := mkdirWithMode(to); err != nil {
				return err
			}
			if err := mergeDir(from, to); err != nil {
				return err
			}
			continue
		}
		if err := os.Rename(from, to); err != nil {
			return fmt.Errorf("failed to move archive file: %w", err)
		}
	}
	if err := os.Remove(src); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to remove merged archive directory: %w", err)
	}
	return syncDir(dst)
}

// syncDir сбрасывает на диск саму запись каталога. Без этого после аварийного
// выключения питания файл может существовать, а имени в каталоге не быть.
func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("failed to open archive directory: %w", err)
	}
	defer f.Close()

	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync archive directory: %w", err)
	}
	return nil
}

// isDirNotEmpty распознаёт отказ переименовать каталог поверх занятого имени.
// Ядро возвращает здесь ENOTEMPTY или EEXIST в зависимости от файловой системы,
// поэтому проверяются оба.
func isDirNotEmpty(err error) bool {
	return errors.Is(err, fs.ErrExist) || errors.Is(err, syscall.ENOTEMPTY)
}
