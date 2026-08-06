// Package imaging приводит загруженные изображения к предсказуемому виду:
// ограничивает размер, перекодирует и тем самым срезает метаданные.
package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // декодер webp: кодера нет, такие снимки уходят в jpeg
)

// Options -- параметры приведения изображения.
type Options struct {
	// MaxSide -- предел длинной стороны в пикселях. 0 отключает уменьшение.
	MaxSide int
	// JPEGQuality -- качество для jpeg на выходе.
	JPEGQuality int
}

// Normalizable отвечает, умеет ли пакет обрабатывать этот тип. Документы (pdf,
// docx) проходят мимо: их содержимое трогать нечем и незачем.
func Normalizable(mime string) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/webp":
		return true
	}
	return false
}

// Normalize декодирует изображение, при необходимости уменьшает его и кодирует
// заново, возвращая содержимое и итоговый MIME-тип.
//
// Перекодирование - и есть способ убрать метаданные: EXIF снимка с телефона
// несёт координаты съёмки и модель устройства, то есть про заявителя иногда
// больше, чем сам документ. Отдельного «стирателя EXIF» поэтому нет.
//
// webp уходит в jpeg: в стандартной библиотеке и x/image есть декодер webp, но
// нет кодера.
func Normalize(r io.Reader, mime string, opts Options) ([]byte, string, error) {
	src, _, err := image.Decode(r)
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	dst := resize(src, opts.MaxSide)

	var out bytes.Buffer
	outMime := mime
	switch mime {
	case "image/png":
		if err := png.Encode(&out, dst); err != nil {
			return nil, "", fmt.Errorf("encode png: %w", err)
		}
	default:
		outMime = "image/jpeg"
		quality := opts.JPEGQuality
		if quality <= 0 {
			quality = jpeg.DefaultQuality
		}
		if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: quality}); err != nil {
			return nil, "", fmt.Errorf("encode jpeg: %w", err)
		}
	}
	return out.Bytes(), outMime, nil
}

// resize уменьшает изображение до maxSide по длинной стороне. Увеличением не
// занимается: растянутый скан документа читается хуже исходного.
func resize(src image.Image, maxSide int) image.Image {
	if maxSide <= 0 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	longest := max(w, h)
	if longest <= maxSide {
		return src
	}

	scale := float64(maxSide) / float64(longest)
	dst := image.NewRGBA(image.Rect(0, 0, int(float64(w)*scale), int(float64(h)*scale)))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}
