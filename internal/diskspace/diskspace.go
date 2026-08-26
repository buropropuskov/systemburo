// Package diskspace измеряет место на файловых системах, к которым принадлежат
// каталоги приложения (#1615, срез B2). Платформенно-зависимая часть (Statfs)
// разведена по build-тегам: прод разворачивается только в Docker на Linux, но
// пакет обязан собираться и на других GOOS ради локальной разработки и CI.
package diskspace

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Usage - место на файловой системе, которой принадлежит каталог.
type Usage struct {
	// Device - идентификатор физического устройства раздела (st_dev). По нему
	// разводятся разные каталоги приложения, оказавшиеся на одном разделе:
	// список для выбора не должен показывать администратору два "разных"
	// варианта там, где физически один диск.
	Device uint64
	// TotalBytes - общий размер раздела.
	TotalBytes int64
	// FreeBytes - доступно ОБЫЧНОМУ процессу (statfs.Bavail), а не общий остаток
	// (Bfree): Bfree включает запас, зарезервированный ядром для root, и
	// "свободно" по нему соврало бы в большую сторону там, где процесс архива
	// пишет не от root.
	FreeBytes int64
}

// UsedBytes - занято на разделе. Не может уйти в минус даже при рассинхроне
// снимков total/free между двумя системными вызовами (гонка с чужой записью).
func (u Usage) UsedBytes() int64 {
	if u.TotalBytes < u.FreeBytes {
		return 0
	}
	return u.TotalBytes - u.FreeBytes
}

// UsedPercent - доля занятого места, 0-100. 0 при неизвестном общем размере -
// раздел с TotalBytes=0 не бывает переполнен, это ошибка измерения, а не 100%.
func (u Usage) UsedPercent() float64 {
	if u.TotalBytes <= 0 {
		return 0
	}
	return float64(u.UsedBytes()) / float64(u.TotalBytes) * 100
}

// Statfs возвращает место на разделе, которому принадлежит path. Платформенная
// реализация (statfs) - в statfs_linux.go / statfs_other.go.
//
// Каталога может ещё не быть на диске (архив, например, создаёт первый уровень
// сам писатель при первой записи) - в этом случае берём статистику ближайшего
// существующего предка: раздел от этого не меняется, а отказ стартовать сводку
// был бы неверен - каталог появится сам при первой выгрузке.
func Statfs(path string) (Usage, error) {
	existing, err := nearestExisting(path)
	if err != nil {
		return Usage{}, fmt.Errorf("diskspace: %s: %w", path, err)
	}
	return statfs(existing)
}

// nearestExisting поднимается по родителям path до первого существующего
// каталога. path приводится к абсолютному: рабочий каталог процесса не должен
// влиять на то, какой раздел определён.
func nearestExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for cur := abs; ; {
		if _, err := os.Stat(cur); err == nil {
			return cur, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("no existing ancestor for %q", abs)
		}
		cur = parent
	}
}

// Dir - один каталог приложения с подписью для интерфейса (архив, загрузки, ...).
type Dir struct {
	Label string
	Path  string
}

// Partition - физический раздел, на котором оказался один или несколько
// каталогов приложения, дедуплицированные по устройству.
type Partition struct {
	Usage
	// Labels - подписи каталогов, оказавшихся на этом разделе.
	Labels []string
}

// Collect считает Usage по каждому каталогу и группирует их по разделу: два
// каталога на одном физическом разделе не должны попасть в список выбора как
// два самостоятельных варианта (#1615, срез B2).
//
// Каталог, которого нет и статистику по которому снять не удалось (например,
// каталог базы данных живёт в соседнем контейнере на своём томе и процессу
// архива вовсе не виден), молча пропускается - выбирать администратору есть из
// чего только среди РЕАЛЬНО видимых процессу каталогов.
func Collect(dirs []Dir) []Partition {
	byDevice := make(map[uint64]int, len(dirs))
	out := make([]Partition, 0, len(dirs))
	for _, d := range dirs {
		if strings.TrimSpace(d.Path) == "" {
			continue
		}
		usage, err := Statfs(d.Path)
		if err != nil {
			slog.Debug("diskspace: каталог недоступен для статистики раздела",
				"label", d.Label, "path", d.Path, "error", err)
			continue
		}
		if idx, ok := byDevice[usage.Device]; ok {
			out[idx].Labels = append(out[idx].Labels, d.Label)
			continue
		}
		byDevice[usage.Device] = len(out)
		out = append(out, Partition{Usage: usage, Labels: []string{d.Label}})
	}
	return out
}

// DirSize суммирует видимый размер обычных файлов каталога рекурсивно.
// Упрощённый эквивалент `du -sb`: разреженные файлы считаются по логическому
// размеру, а не по числу занятых блоков - точности достаточно для состава
// занятого места на полосе интерфейса, не для биллинга.
//
// Отсутствующий каталог не ошибка - 0 байт: часть каталогов (например, логи)
// настроена не всегда, и вызывающему не нужно отдельно проверять существование
// перед каждым вызовом.
func DirSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if os.IsNotExist(err) {
				// Файл исчез между обходом каталога и Info() - обычное дело для
				// временных файлов писателя архива, не повод рвать весь подсчёт.
				return nil
			}
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("diskspace: dir size %s: %w", path, err)
	}
	return total, nil
}
