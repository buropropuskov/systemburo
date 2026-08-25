// Package download - единая точка отдачи файлов (CMS-симметрия к internal/upload).
// Доступ проверяется вызывающим хендлером ДО Serve/StreamZip (через сервис-слой,
// который и резолвит сущность); пакет отвечает только за заголовки, 404 при
// отсутствии файла и саму отдачу.
package download

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"systemburo/internal/crypto"
	"systemburo/internal/httpx"

	"github.com/labstack/echo/v4"
)

// allowLongResponse снимает с соединения общий срок записи.
//
// Отдача файла идёт в темпе клиента: nginx для маршрутов файлового архива
// буферизацию снимает намеренно (иначе он копил бы весь архив в памяти на машине с
// 8 ГБ) и отводит на них час. Значит, скорость мобильного интернета на той стороне
// напрямую решает, уложится ли запись в HTTP_WRITE_TIMEOUT, а обрыв на середине
// выглядит как испорченный файл, а не как отказ.
func allowLongResponse(c echo.Context, what string) {
	if err := httpx.AllowLongResponse(c); err != nil {
		slog.Warn("download: не удалось снять срок записи, отдача оборвётся по HTTP_WRITE_TIMEOUT",
			"what", what, "path", c.Path(), "error", err)
	}
}

// File описывает файл к отдаче.
type File struct {
	// Path - абсолютный путь к файлу на диске.
	Path string
	// Open - как получить содержимое, когда на диске лежит не оно само. Задан у
	// файлов файлового архива: там на диске шифротекст, и отдать его байт в байт
	// значит выдать пользователю то, что он не откроет. Пусто - отдаём файл как есть.
	Open func() (io.ReadCloser, error)
	// Size - размер отдаваемого содержимого. Нужен вместе с Open: размер файла на
	// диске к расшифрованному потоку отношения не имеет. 0 - заголовок не ставим.
	Size int64
	// Name - имя файла для Content-Disposition. Пусто - заголовок не ставится
	// (браузер сам решит, как показать; для inline-предпросмотра по расширению).
	Name string
	// Mime - Content-Type. Пусто - не выставляем, Echo определит по расширению.
	Mime string
	// Inline - просмотр в браузере (inline) вместо скачивания (attachment).
	Inline bool
}

// Encrypted описывает зашифрованный на диске файл к отдаче.
type Encrypted struct {
	File
	// Key - ключ расшифровки. nil отдаёт содержимое как есть.
	Key []byte
	// Size - размер исходного содержимого. Нужен для Content-Length: на диске
	// лежит шифротекст, он длиннее, и его размер браузеру не годится.
	Size int64
}

// ServeEncrypted расшифровывает файл на лету и отдаёт потоком. Целиком в память
// он не читается: расшифровка идёт чанками, поэтому десяток одновременных
// скачиваний не выносит процесс.
func ServeEncrypted(c echo.Context, f Encrypted) error {
	if f.Path == "" {
		return echo.NewHTTPError(http.StatusNotFound, "файл не найден")
	}
	src, err := os.Open(f.Path)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "файл не найден")
	}
	defer src.Close()

	reader, err := crypto.NewStreamReader(src, f.Key)
	if err != nil {
		return fmt.Errorf("open encrypted file: %w", err)
	}
	allowLongResponse(c, "encrypted file")

	if f.Name != "" {
		disposition := "attachment"
		if f.Inline {
			disposition = "inline"
		}
		c.Response().Header().Set(echo.HeaderContentDisposition,
			fmt.Sprintf(`%s; filename="%s"`, disposition, sanitizeName(f.Name)))
	}
	if f.Size > 0 {
		c.Response().Header().Set(echo.HeaderContentLength, strconv.FormatInt(f.Size, 10))
	}
	mime := f.Mime
	if mime == "" {
		mime = echo.MIMEOctetStream
	}
	return c.Stream(http.StatusOK, mime, reader)
}

// Serve отдаёт файл с корректными заголовками. Возвращает 404, если путь пуст
// или файла нет на диске. Доступ должен быть проверен вызывающим до Serve.
func Serve(c echo.Context, f File) error {
	if f.Path == "" {
		return echo.NewHTTPError(http.StatusNotFound, "файл не найден")
	}
	if info, err := os.Stat(f.Path); err != nil || info.IsDir() {
		return echo.NewHTTPError(http.StatusNotFound, "файл не найден")
	}
	allowLongResponse(c, "file")

	if f.Mime != "" {
		c.Response().Header().Set(echo.HeaderContentType, f.Mime)
	}
	if f.Name != "" {
		disposition := "attachment"
		if f.Inline {
			disposition = "inline"
		}
		c.Response().Header().Set(echo.HeaderContentDisposition,
			fmt.Sprintf(`%s; filename="%s"`, disposition, sanitizeName(f.Name)))
	}

	if f.Open == nil {
		return c.File(f.Path)
	}

	reader, err := f.Open()
	if err != nil {
		return fmt.Errorf("open file content: %w", err)
	}
	defer reader.Close()

	if f.Size > 0 {
		c.Response().Header().Set(echo.HeaderContentLength, strconv.FormatInt(f.Size, 10))
	}
	mime := f.Mime
	if mime == "" {
		mime = echo.MIMEOctetStream
	}
	return c.Stream(http.StatusOK, mime, reader)
}

// sanitizeName экранирует кавычки и убирает переносы строк из имени файла,
// чтобы оно не ломало заголовок Content-Disposition (header injection).
func sanitizeName(s string) string {
	return strings.NewReplacer("\r", "", "\n", "", `"`, `\"`).Replace(s)
}

// ZipEntry - один файл потокового ZIP-архива (#1615, B3). Доступ к нему уже
// проверен вызывающим - StreamZip таких проверок не делает.
type ZipEntry struct {
	// Path - абсолютный путь файла на диске.
	Path string
	// Open - как получить содержимое, если файл нельзя просто открыть. Нужен для
	// зашифрованного архива: на диске лежит шифротекст, а в ZIP должен уехать
	// читаемый документ. nil означает обычное открытие по Path.
	Open func() (io.ReadCloser, error)
	// Name - путь файла внутри архива. Разделитель "/" - вложенные каталоги
	// (raw-строка, не filepath.Join): формат ZIP всегда использует прямой слэш
	// независимо от ОС, на которой архив потом откроют.
	Name string
}

// zipErrorSuffix - метка файла-заметки об ошибке внутри архива. С кириллицей,
// чтобы отличаться от настоящих файлов оператора на глаз, не разбирая расширение.
const zipErrorSuffix = "_ОШИБКА.txt"

// StreamZip отдаёт набор файлов единым потоковым ZIP без буферизации в память -
// на сервере 8 ГБ, и держать гигабайты выгрузки целиком в памяти нельзя.
//
// Метод Store (без сжатия): бланки уже сжаты как .xlsx, повторное сжатие тратит
// CPU без выигрыша в размере. Заголовки уходят до первого байта тела; каждая
// запись сбрасывается в сеть сразу после записи (Flush) - у потока нет заранее
// известного Content-Length, и клиент должен видеть прогресс, а не ждать один
// большой ответ разом.
//
// Файл, который не удалось открыть, не прерывает архив целиком: вместо него
// кладётся текстовая заметка "<имя>_ОШИБКА.txt" с причиной - молчаливый пропуск
// неотличим от "файла никогда не было", а администратор должен увидеть пробел.
func StreamZip(c echo.Context, archiveName string, entries []ZipEntry) error {
	allowLongResponse(c, "zip stream")

	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "application/zip")
	res.Header().Set(echo.HeaderContentDisposition,
		fmt.Sprintf(`attachment; filename="%s"`, sanitizeName(archiveName)))
	res.WriteHeader(http.StatusOK)

	zw := zip.NewWriter(res)
	for _, entry := range entries {
		if err := writeZipEntry(zw, entry); err != nil {
			// Ошибка добавляется НОВОЙ записью, а не заменой: writeZipEntry падает
			// до первого байта содержимого (открытие файла), поэтому исходная
			// запись ещё не начата и дублирования имени в архиве не возникает.
			writeZipErrorNote(zw, entry.Name, err)
		}
		res.Flush()
	}
	return zw.Close()
}

// writeZipEntry копирует один файл в архив. Ошибка возвращается ДО первого
// байта содержимого записи (открытие файла) во всех штатных сценариях сбоя
// (файл убрали с диска, нет прав на чтение) - только так вызывающий может
// безопасно заменить запись на заметку об ошибке, не оставив в потоке половину
// валидной, половину битой записи.
func writeZipEntry(zw *zip.Writer, entry ZipEntry) error {
	open := entry.Open
	if open == nil {
		open = func() (io.ReadCloser, error) { return os.Open(entry.Path) }
	}
	f, err := open()
	if err != nil {
		return err
	}
	defer f.Close()

	w, err := zw.CreateHeader(&zip.FileHeader{Name: entry.Name, Method: zip.Store})
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

// writeZipErrorNote кладёт в архив текстовую заметку вместо не открывшегося
// файла. Best-effort: если и заметку создать не удалось, поток уже пошёл в
// сеть и обрывать его из-за одной пропущенной строки хуже, чем отдать архив
// без неё.
func writeZipErrorNote(zw *zip.Writer, name string, cause error) {
	w, err := zw.Create(name + zipErrorSuffix)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "Файл %q не удалось включить в архив: %v\n", name, cause)
}
