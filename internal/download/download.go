// Package download - единая точка отдачи файлов (CMS-симметрия к internal/upload).
// Доступ проверяется вызывающим хендлером ДО Serve (через сервис-слой, который и
// резолвит сущность); Serve отвечает за заголовки, 404 при отсутствии файла и саму отдачу.
package download

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

// File описывает файл к отдаче.
type File struct {
	// Path - абсолютный путь к файлу на диске.
	Path string
	// Name - имя файла для Content-Disposition. Пусто - заголовок не ставится
	// (браузер сам решит, как показать; для inline-предпросмотра по расширению).
	Name string
	// Mime - Content-Type. Пусто - не выставляем, Echo определит по расширению.
	Mime string
	// Inline - просмотр в браузере (inline) вместо скачивания (attachment).
	Inline bool
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

	return c.File(f.Path)
}

// sanitizeName экранирует кавычки и убирает переносы строк из имени файла,
// чтобы оно не ломало заголовок Content-Disposition (header injection).
func sanitizeName(s string) string {
	return strings.NewReplacer("\r", "", "\n", "", `"`, `\"`).Replace(s)
}
