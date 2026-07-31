package services

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Уровни каталогов боевой раскладки: год / месяц / день / папка заявки. Взяты с
// кириллицей и знаком номера намеренно - именно такие имена приходят из шаблона по
// умолчанию, и на них проверяется вся работа с путями.
func archiveLevels(day, folder string) []string {
	return []string{"2026", "7 ИЮЛЬ 2026", day, folder}
}

func newTestWriter(t *testing.T) *ArchiveWriter {
	t.Helper()
	w, err := NewArchiveWriter(filepath.Join(t.TempDir(), "archive"))
	if err != nil {
		t.Fatalf("NewArchiveWriter: %v", err)
	}
	return w
}

func TestNewArchiveWriter_RejectsEmptyRoot(t *testing.T) {
	if _, err := NewArchiveWriter("   "); err == nil {
		t.Fatal("пустой корень архива обязан отвергаться: запись пошла бы в рабочий каталог процесса")
	}
}

func TestArchiveWriter_WriteFile_ModesAndNoLeftovers(t *testing.T) {
	w := newTestWriter(t)
	levels := archiveLevels("31.07.2026", "31.07.2026 №001 Мегобари")
	name := "Заявка на работы - Мегобари.xlsx"

	if err := w.WriteFile(levels, name, []byte("бланк")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	full := filepath.Join(append([]string{w.Root()}, append(levels, name)...)...)
	info, err := os.Stat(full)
	if err != nil {
		t.Fatalf("файл не появился: %v", err)
	}
	if got := info.Mode().Perm(); got != archiveFileMode {
		t.Errorf("режим файла = %v, ожидался %v", got, archiveFileMode)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "бланк" {
		t.Errorf("содержимое = %q, ожидалось %q", data, "бланк")
	}

	dir := filepath.Dir(full)
	for dir != w.Root() {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("Stat %s: %v", dir, err)
		}
		if got := info.Mode().Perm(); got != archiveDirMode {
			t.Errorf("режим каталога %s = %v, ожидался %v", dir, got, archiveDirMode)
		}
		dir = filepath.Dir(dir)
	}

	entries, err := os.ReadDir(filepath.Dir(full))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("в папке заявки %d записей, ожидался только бланк: %v", len(entries), entries)
	}
}

// Жёсткий umask не должен отбирать у архива групповое чтение: ради него каталог и
// отдают в сетевую папку. Mkdir запрошенные права через umask пропускает, поэтому
// режим выставляется отдельным Chmod - проверяем это на всех созданных уровнях,
// включая сам корень.
func TestArchiveWriter_EnsureDir_ModesSurviveStrictUmask(t *testing.T) {
	w := newTestWriter(t)
	levels := archiveLevels("31.07.2026", "31.07.2026 №001 Мегобари")

	prev := syscall.Umask(0o077)
	_, err := w.EnsureDir(levels)
	syscall.Umask(prev)
	if err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	dir := filepath.Join(append([]string{w.Root()}, levels...)...)
	for {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("Stat %s: %v", dir, err)
		}
		if got := info.Mode().Perm(); got != archiveDirMode {
			t.Errorf("режим каталога %s = %v, ожидался %v", dir, got, archiveDirMode)
		}
		if dir == w.Root() {
			break
		}
		dir = filepath.Dir(dir)
	}
}

// Чужой каталог мог быть намеренно открыт администратором под сетевую папку -
// повторная запись не должна молча сужать ему права.
func TestArchiveWriter_EnsureDir_KeepsForeignMode(t *testing.T) {
	w := newTestWriter(t)
	levels := archiveLevels("31.07.2026", "31.07.2026 №001 Мегобари")

	if _, err := w.EnsureDir(levels); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	existing := filepath.Join(append([]string{w.Root()}, levels...)...)
	if err := os.Chmod(existing, 0o755); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	if err := w.WriteFile(levels, "бланк.xlsx", []byte("x")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	info, err := os.Stat(existing)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("режим существующего каталога = %v, ожидался нетронутый 0755", got)
	}
}

func TestArchiveWriter_WriteFile_Overwrites(t *testing.T) {
	w := newTestWriter(t)
	levels := archiveLevels("31.07.2026", "31.07.2026 №001 Мегобари")

	if err := w.WriteFile(levels, "бланк.xlsx", []byte("первый")); err != nil {
		t.Fatalf("первая запись: %v", err)
	}
	if err := w.WriteFile(levels, "бланк.xlsx", []byte("второй")); err != nil {
		t.Fatalf("вторая запись: %v", err)
	}

	full := filepath.Join(append([]string{w.Root()}, append(levels, "бланк.xlsx")...)...)
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "второй" {
		t.Errorf("содержимое = %q, ожидалось %q", data, "второй")
	}

	entries, err := os.ReadDir(filepath.Dir(full))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("после перезаписи %d записей, ожидалась одна", len(entries))
	}
}

// Каталог только на чтение - модель полного диска и потерянных прав: на боевом
// имени не должно появиться ни пустого, ни временного файла.
func TestArchiveWriter_WriteFile_ReadOnlyDir_LeavesNothing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("под root права каталога не ограничивают запись - проверка бессмысленна")
	}
	w := newTestWriter(t)
	levels := archiveLevels("31.07.2026", "31.07.2026 №001 Мегобари")

	dir, err := w.EnsureDir(levels)
	if err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, archiveDirMode) })

	if err := w.WriteFile(levels, "бланк.xlsx", []byte("данные")); err == nil {
		t.Fatal("запись в каталог только на чтение обязана вернуть ошибку")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("после неудачной записи в каталоге %d записей, ожидалась пустая папка: %v", len(entries), entries)
	}
}

func TestArchiveWriter_Resolve_RejectsEscape(t *testing.T) {
	w := newTestWriter(t)

	if _, err := w.Resolve("..", "..", "etc"); err == nil {
		t.Fatal("путь за пределы корня архива обязан отвергаться")
	}
	// Схлопывается обратно в корень и формально его не покидает, но файл ляжет не
	// туда, где его потом ищет реестр, - такой уровень тоже недопустим.
	if err := w.WriteFile([]string{"2026", ".."}, "чужой.xlsx", []byte("x")); err == nil {
		t.Fatal("уровень \"..\" обязан отвергаться")
	}
	if err := w.WriteFile([]string{"2026/07"}, "бланк.xlsx", []byte("x")); err == nil {
		t.Fatal("уровень с разделителем обязан отвергаться: имена уровней приходят уже очищенными")
	}
	if _, err := w.EnsureDir([]string{"2026", ""}); err == nil {
		t.Fatal("пустой уровень обязан отвергаться")
	}
}

func TestArchiveWriter_Exists(t *testing.T) {
	w := newTestWriter(t)
	levels := archiveLevels("31.07.2026", "31.07.2026 №001 Мегобари")

	switch ok, err := w.Exists(levels, "бланк.xlsx"); {
	case err != nil:
		t.Fatalf("Exists до записи: %v", err)
	case ok:
		t.Fatal("Exists = true до записи файла")
	}

	if err := w.WriteFile(levels, "бланк.xlsx", []byte("x")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	switch ok, err := w.Exists(levels, "бланк.xlsx"); {
	case err != nil:
		t.Fatalf("Exists после записи: %v", err)
	case !ok:
		t.Fatal("Exists = false для записанного файла")
	}
}

func TestArchiveWriter_RemoveFile_Idempotent(t *testing.T) {
	w := newTestWriter(t)
	levels := archiveLevels("31.07.2026", "31.07.2026 №001 Мегобари")

	if err := w.WriteFile(levels, "бланк.xlsx", []byte("x")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := w.RemoveFile(levels, "бланк.xlsx"); err != nil {
		t.Fatalf("первое удаление: %v", err)
	}
	if err := w.RemoveFile(levels, "бланк.xlsx"); err != nil {
		t.Fatalf("повторное удаление обязано быть успешным: %v", err)
	}
}

// Организацию в заявке сменили - папка переезжает, а опустевшие день, месяц и год
// подчищаются. Корень при этом обязан уцелеть.
func TestArchiveWriter_MoveDir_PrunesEmptyLevels(t *testing.T) {
	w := newTestWriter(t)
	from := archiveLevels("31.07.2026", "31.07.2026 №001 Ромашка")
	to := []string{"2026", "8 АВГУСТ 2026", "01.08.2026", "01.08.2026 №001 Мегобари"}

	if err := w.WriteFile(from, "бланк.xlsx", []byte("данные")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := w.MoveDir(from, to); err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	moved := filepath.Join(append([]string{w.Root()}, append(to, "бланк.xlsx")...)...)
	data, err := os.ReadFile(moved)
	if err != nil {
		t.Fatalf("файл не переехал: %v", err)
	}
	if string(data) != "данные" {
		t.Errorf("содержимое = %q, ожидалось %q", data, "данные")
	}

	oldYear := filepath.Join(w.Root(), "2026")
	if _, err := os.Stat(oldYear); err != nil {
		t.Fatalf("год обязан уцелеть - в нём новый месяц: %v", err)
	}
	oldMonth := filepath.Join(w.Root(), "2026", "7 ИЮЛЬ 2026")
	if _, err := os.Stat(oldMonth); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("опустевший месяц остался на диске: %v", err)
	}
	if _, err := os.Stat(w.Root()); err != nil {
		t.Fatalf("корень архива снесён уборкой: %v", err)
	}
}

// Целевая папка уже занята: та же заявка писалась по новому пути раньше. Бланки
// сливаются, исходная папка исчезает - потерять файл из-за занятого имени нельзя.
func TestArchiveWriter_MoveDir_MergesIntoOccupied(t *testing.T) {
	w := newTestWriter(t)
	from := archiveLevels("31.07.2026", "31.07.2026 №001 Ромашка")
	to := archiveLevels("31.07.2026", "31.07.2026 №001 Мегобари")

	if err := w.WriteFile(from, "работы.xlsx", []byte("работы")); err != nil {
		t.Fatalf("WriteFile источника: %v", err)
	}
	if err := w.WriteFile(to, "ввоз.xlsx", []byte("ввоз")); err != nil {
		t.Fatalf("WriteFile цели: %v", err)
	}

	if err := w.MoveDir(from, to); err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	dst := filepath.Join(append([]string{w.Root()}, to...)...)
	entries, err := os.ReadDir(dst)
	if err != nil {
		t.Fatalf("ReadDir цели: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("в целевой папке %d файлов, ожидалось 2 (слияние)", len(entries))
	}
	src := filepath.Join(append([]string{w.Root()}, from...)...)
	if _, err := os.Stat(src); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("исходная папка осталась после слияния: %v", err)
	}
}

// Слияние идёт вглубь: подкаталог в папке заявки не теряется, а созданный при
// переносе каталог получает тот же режим, что и остальное дерево.
func TestArchiveWriter_MoveDir_MergesNestedDirs(t *testing.T) {
	w := newTestWriter(t)
	from := archiveLevels("31.07.2026", "31.07.2026 №001 Ромашка")
	to := archiveLevels("31.07.2026", "31.07.2026 №001 Мегобари")

	if err := w.WriteFile(append(from, "приложения"), "скан.xlsx", []byte("скан")); err != nil {
		t.Fatalf("WriteFile во вложенный каталог: %v", err)
	}
	if err := w.WriteFile(to, "ввоз.xlsx", []byte("ввоз")); err != nil {
		t.Fatalf("WriteFile цели: %v", err)
	}

	prev := syscall.Umask(0o077)
	err := w.MoveDir(from, to)
	syscall.Umask(prev)
	if err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	nested := filepath.Join(append([]string{w.Root()}, append(to, "приложения")...)...)
	info, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("вложенный каталог не переехал: %v", err)
	}
	if got := info.Mode().Perm(); got != archiveDirMode {
		t.Errorf("режим перенесённого каталога = %v, ожидался %v", got, archiveDirMode)
	}
	data, err := os.ReadFile(filepath.Join(nested, "скан.xlsx"))
	if err != nil {
		t.Fatalf("файл во вложенном каталоге не переехал: %v", err)
	}
	if string(data) != "скан" {
		t.Errorf("содержимое = %q, ожидалось %q", data, "скан")
	}
	src := filepath.Join(append([]string{w.Root()}, from...)...)
	if _, err := os.Stat(src); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("исходная папка осталась после слияния: %v", err)
	}
}

// Одноимённый бланк есть и в переносимой папке, и в целевой. Побеждает переносимый:
// он записан последним прогоном выгрузки, а лежащий в цели остался от экспорта по
// новому пути до переименования.
func TestArchiveWriter_MoveDir_MergeOverwritesSameName(t *testing.T) {
	w := newTestWriter(t)
	from := archiveLevels("31.07.2026", "31.07.2026 №001 Ромашка")
	to := archiveLevels("31.07.2026", "31.07.2026 №001 Мегобари")

	if err := w.WriteFile(from, "работы.xlsx", []byte("актуальный")); err != nil {
		t.Fatalf("WriteFile источника: %v", err)
	}
	if err := w.WriteFile(to, "работы.xlsx", []byte("устаревший")); err != nil {
		t.Fatalf("WriteFile цели: %v", err)
	}
	if err := w.WriteFile(to, "ввоз.xlsx", []byte("ввоз")); err != nil {
		t.Fatalf("WriteFile соседа: %v", err)
	}

	if err := w.MoveDir(from, to); err != nil {
		t.Fatalf("MoveDir: %v", err)
	}

	dst := filepath.Join(append([]string{w.Root()}, to...)...)
	data, err := os.ReadFile(filepath.Join(dst, "работы.xlsx"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "актуальный" {
		t.Errorf("содержимое = %q, ожидался файл из переносимой папки", data)
	}
	if _, err := os.Stat(filepath.Join(dst, "ввоз.xlsx")); err != nil {
		t.Errorf("сосед по целевой папке пострадал при слиянии: %v", err)
	}
}

// Папку удалили руками. Это штатный случай: писатель сообщает отдельной ошибкой,
// чтобы вызывающий очистил фактический путь и записал файлы заново.
func TestArchiveWriter_MoveDir_MissingSource(t *testing.T) {
	w := newTestWriter(t)
	from := archiveLevels("31.07.2026", "31.07.2026 №001 Ромашка")
	to := archiveLevels("31.07.2026", "31.07.2026 №001 Мегобари")

	err := w.MoveDir(from, to)
	if !errors.Is(err, ErrArchiveSourceMissing) {
		t.Fatalf("MoveDir без источника = %v, ожидался ErrArchiveSourceMissing", err)
	}
}

func TestArchiveWriter_MoveDir_SamePathIsNoop(t *testing.T) {
	w := newTestWriter(t)
	levels := archiveLevels("31.07.2026", "31.07.2026 №001 Мегобари")

	if err := w.WriteFile(levels, "бланк.xlsx", []byte("данные")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := w.MoveDir(levels, levels); err != nil {
		t.Fatalf("MoveDir в тот же путь: %v", err)
	}

	full := filepath.Join(append([]string{w.Root()}, append(levels, "бланк.xlsx")...)...)
	if _, err := os.Stat(full); err != nil {
		t.Fatalf("файл пропал при переносе в тот же путь: %v", err)
	}
}

func TestArchiveWriter_CleanupTemp(t *testing.T) {
	w := newTestWriter(t)
	levels := archiveLevels("31.07.2026", "31.07.2026 №001 Мегобари")

	dir, err := w.EnsureDir(levels)
	if err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	stale := filepath.Join(dir, archiveTempPrefix+"старый")
	fresh := filepath.Join(dir, archiveTempPrefix+"свежий")
	blank := filepath.Join(dir, "бланк.xlsx")
	for _, p := range []string{stale, fresh, blank} {
		if err := os.WriteFile(p, []byte("x"), archiveFileMode); err != nil {
			t.Fatalf("WriteFile %s: %v", p, err)
		}
	}
	old := time.Now().Add(-3 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	removed, err := w.CleanupTemp(time.Hour)
	if err != nil {
		t.Fatalf("CleanupTemp: %v", err)
	}
	if removed != 1 {
		t.Errorf("удалено %d временных файлов, ожидался 1", removed)
	}
	if _, err := os.Stat(stale); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("старый временный файл остался: %v", err)
	}
	for _, p := range []string{fresh, blank} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("файл %s не должен был пострадать: %v", p, err)
		}
	}
}

// Архива на диске ещё нет: уборка обязана промолчать, а не сорвать старт воркера.
func TestArchiveWriter_CleanupTemp_MissingRoot(t *testing.T) {
	w := newTestWriter(t)

	removed, err := w.CleanupTemp(time.Hour)
	if err != nil {
		t.Fatalf("CleanupTemp на пустом корне: %v", err)
	}
	if removed != 0 {
		t.Errorf("удалено %d файлов на несуществующем корне", removed)
	}
}
