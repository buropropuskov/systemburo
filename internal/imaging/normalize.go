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
	// MaxPixels limits decoded allocation; zero uses the safe default.
	MaxPixels int
	// MaxEncodedBytes is supplied by the caller's upload configuration.
	MaxEncodedBytes int64
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
func Normalize(r io.ReadSeeker, mime string, opts Options) ([]byte, string, error) {
	// The seekable contract allows replay without buffering an arbitrary stream.
	start, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, "", fmt.Errorf("image position: %w", err)
	}
	end, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, "", fmt.Errorf("image size: %w", err)
	}
	if start < 0 || end < start || (opts.MaxEncodedBytes > 0 && end-start > opts.MaxEncodedBytes) {
		return nil, "", fmt.Errorf("image input exceeds limit or has invalid size")
	}
	if _, err := r.Seek(start, io.SeekStart); err != nil {
		return nil, "", err
	}
	cfg, _, err := image.DecodeConfig(io.LimitReader(r, end-start))
	if err != nil {
		return nil, "", fmt.Errorf("decode image config: %w", err)
	}
	limit := opts.MaxPixels
	if limit <= 0 {
		limit = 25_000_000
	}
	if !validDimensions(cfg.Width, cfg.Height, limit) {
		return nil, "", fmt.Errorf("image dimensions exceed pixel limit")
	}
	if _, err := r.Seek(start, io.SeekStart); err != nil {
		return nil, "", err
	}
	src, _, err := image.Decode(io.LimitReader(r, end-start))
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

func validDimensions(width, height, limit int) bool {
	return width > 0 && height > 0 && limit > 0 && width <= limit/height
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
	dst := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}
