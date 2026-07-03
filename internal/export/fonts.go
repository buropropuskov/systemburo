package export

import _ "embed"

// dejaVuSans - встроенный TTF DejaVu Sans (свободная лицензия). Нужен для рендера
// кириллицы в PDF: alpine-образ бэкенда без системных шрифтов, а core-шрифты fpdf
// (Helvetica/Arial) кириллицу не содержат. Встраиваем в бинарь, чтобы экспорт не
// зависел от окружения.
//
//go:embed fonts/DejaVuSans.ttf
var dejaVuSans []byte
