package imaging

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
)

// sampleImage рисует картинку заданного размера с неоднородным содержимым: сплошная
// заливка сжимается почти в ничто и не показала бы разницы размеров.
func sampleImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 251), G: uint8(y % 241), B: uint8((x * y) % 233), A: 255})
		}
	}
	return img
}

func TestNormalizePixelBounds(t *testing.T) {
	for _, tc := range []struct {
		w, h int
		ok   bool
	}{{5000, 5000, true}, {5001, 5000, false}, {0, 1, false}, {1, 0, false}, {int(^uint(0) >> 1), int(^uint(0) >> 1), false}} {
		require.Equal(t, tc.ok, validDimensions(tc.w, tc.h, 25_000_000))
	}
	data := encodeJPEG(t, sampleImage(10, 10))
	_, _, err := Normalize(bytes.NewReader(data), "image/jpeg", Options{MaxPixels: 100})
	require.NoError(t, err)
	_, _, err = Normalize(bytes.NewReader(data), "image/jpeg", Options{MaxPixels: 99})
	require.ErrorContains(t, err, "pixel limit")
	_, _, err = Normalize(bytes.NewReader(data), "image/jpeg", Options{MaxEncodedBytes: int64(len(data))})
	require.NoError(t, err)
	_, _, err = Normalize(bytes.NewReader(data), "image/jpeg", Options{MaxEncodedBytes: int64(len(data) - 1)})
	require.ErrorContains(t, err, "input exceeds limit")
}

func TestNormalizeRejectsLargeConfigBeforeDecode(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, sampleImage(1, 1)))
	data := append([]byte(nil), buf.Bytes()[:33]...)
	binary.BigEndian.PutUint32(data[16:20], 5001)
	binary.BigEndian.PutUint32(data[20:24], 5000)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	_, _, err := Normalize(bytes.NewReader(data), "image/png", Options{})
	require.ErrorContains(t, err, "pixel limit")
}

func TestNormalizeWebP(t *testing.T) {
	// Synthetic one-pixel lossless WebP fixture.
	data, err := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	require.NoError(t, err)
	out, mime, err := Normalize(bytes.NewReader(data), "image/webp", Options{})
	require.NoError(t, err)
	require.Equal(t, "image/jpeg", mime)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	require.NoError(t, err)
	require.Equal(t, 1, cfg.Width)
	require.Equal(t, 1, cfg.Height)
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}))
	return buf.Bytes()
}

// TestNormalize_ShrinksToMaxSide: длинная сторона ужимается, пропорции сохраняются.
func TestNormalize_ShrinksToMaxSide(t *testing.T) {
	data := encodeJPEG(t, sampleImage(3000, 1500))

	out, mime, err := Normalize(bytes.NewReader(data), "image/jpeg", Options{MaxSide: 2000, JPEGQuality: 82})
	require.NoError(t, err)
	require.Equal(t, "image/jpeg", mime)

	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	require.NoError(t, err)
	require.Equal(t, 2000, cfg.Width)
	require.Equal(t, 1000, cfg.Height)
	require.Less(t, len(out), len(data), "ужатый снимок должен занимать меньше исходного")
}

// TestNormalize_KeepsSmallImageSize: маленький скан не растягивается - растянутый
// документ читается хуже исходного.
func TestNormalize_KeepsSmallImageSize(t *testing.T) {
	data := encodeJPEG(t, sampleImage(800, 600))

	out, _, err := Normalize(bytes.NewReader(data), "image/jpeg", Options{MaxSide: 2000, JPEGQuality: 82})
	require.NoError(t, err)

	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	require.NoError(t, err)
	require.Equal(t, 800, cfg.Width)
	require.Equal(t, 600, cfg.Height)
}

// TestNormalize_DropsExif: перекодирование обязано убрать метаданные. EXIF снимка с
// телефона несёт координаты съёмки, то есть иногда больше сведений о заявителе, чем
// сам приложенный документ.
func TestNormalize_DropsExif(t *testing.T) {
	base := encodeJPEG(t, sampleImage(1200, 900))
	marker := []byte("GPSLatitudeRef")
	// APP1-сегмент с exif-подписью вставляется сразу после SOI.
	payload := append([]byte("Exif\x00\x00"), marker...)
	segment := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte((len(payload) + 2) & 0xFF)}
	withExif := append([]byte{}, base[:2]...)
	withExif = append(withExif, segment...)
	withExif = append(withExif, payload...)
	withExif = append(withExif, base[2:]...)
	require.Contains(t, string(withExif), string(marker), "подготовка теста: метка должна быть в исходнике")

	out, _, err := Normalize(bytes.NewReader(withExif), "image/jpeg", Options{MaxSide: 2000, JPEGQuality: 82})
	require.NoError(t, err)
	require.NotContains(t, string(out), string(marker), "метаданные не должны переживать перекодирование")
}

// TestNormalize_PngStaysPng: скриншот документа в jpeg замылился бы, поэтому png
// остаётся png.
func TestNormalize_PngStaysPng(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, sampleImage(2400, 1200)))

	out, mime, err := Normalize(bytes.NewReader(buf.Bytes()), "image/png", Options{MaxSide: 2000, JPEGQuality: 82})
	require.NoError(t, err)
	require.Equal(t, "image/png", mime)

	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	require.NoError(t, err)
	require.Equal(t, "png", format)
	require.Equal(t, 2000, cfg.Width)
}

// TestNormalizable_SkipsDocuments: pdf и офисные документы конвейер не трогает.
func TestNormalizable_SkipsDocuments(t *testing.T) {
	require.True(t, Normalizable("image/jpeg"))
	require.True(t, Normalizable("image/webp"))
	require.False(t, Normalizable("application/pdf"))
	require.False(t, Normalizable("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"))
}
