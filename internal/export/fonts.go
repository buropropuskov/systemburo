package export

import _ "embed"

// dejaVuSans - встроенный TTF DejaVu Sans. Нужен для рендера кириллицы в PDF:
// alpine-образ бэкенда без системных шрифтов, а core-шрифты fpdf (Helvetica/Arial)
// кириллицу не содержат. Встраиваем в бинарь, чтобы экспорт не зависел от окружения.
// Лицензия (Bitstream Vera + public domain) - fonts/LICENSE.
//
//go:embed fonts/DejaVuSans.ttf
var dejaVuSans []byte
