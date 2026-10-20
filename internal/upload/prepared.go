package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"systemburo/internal/apperr"
	"systemburo/internal/crypto"
	"systemburo/internal/imaging"
)

// PreparedFile holds one bounded file; callers release it before preparing the next.
type PreparedFile struct {
	Metadata SavedFile
	Content  []byte
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// MeasureFile counts parser-owned input bytes without trusting request headers.
func MeasureFile(ctx context.Context, file *multipart.FileHeader, max int64) (int64, error) {
	src, err := file.Open()
	if err != nil {
		return 0, apperr.Validation("Не удалось прочитать файл")
	}
	defer src.Close()
	n, err := io.Copy(io.Discard, io.LimitReader(contextReader{ctx, src}, max+1))
	if err != nil {
		return 0, err
	}
	if n <= 0 || n > max {
		return 0, apperr.Validation("Файл пуст или превышает допустимый размер")
	}
	return n, nil
}

// PrepareFile validates and normalizes content without creating a permanent path.
func PrepareFile(ctx context.Context, file *multipart.FileHeader, opts Options, expected int64) (PreparedFile, error) {
	var result PreparedFile
	if err := ctx.Err(); err != nil {
		return result, err
	}
	src, err := file.Open()
	if err != nil {
		return result, apperr.Validation("Не удалось прочитать файл")
	}
	data, readErr := io.ReadAll(io.LimitReader(contextReader{ctx, src}, opts.MaxFileSize+1))
	closeErr := src.Close()
	if err = errors.Join(readErr, closeErr); err != nil {
		return result, err
	}
	if int64(len(data)) != expected || int64(len(data)) > opts.MaxFileSize {
		return result, apperr.Validation("Размер файла изменился или превышает допустимый")
	}
	detected, err := ValidateFileType(bytes.NewReader(data), opts.AllowedTypes)
	if err != nil {
		return result, apperr.Validation("Недопустимый тип файла")
	}
	stored := OfficeMimeByName(detected, file.Filename)
	if opts.Normalize != nil && imaging.Normalizable(detected) {
		normalize := *opts.Normalize
		normalize.MaxEncodedBytes = opts.MaxFileSize
		data, stored, err = imaging.Normalize(bytes.NewReader(data), detected, normalize)
		if err != nil {
			return result, apperr.Validation("Не удалось обработать изображение")
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if len(data) == 0 || int64(len(data)) > opts.MaxFileSize {
		return result, apperr.Validation("Файл превышает допустимый размер")
	}
	result.Content = data
	result.Metadata = SavedFile{FileName: file.Filename, Size: int64(len(data)), MimeType: file.Header.Get("Content-Type"), DetectedMime: stored, Encrypted: opts.EncryptionKey != nil}
	return result, nil
}

// WritePrepared is called only after the path and normalized size are reserved.
func WritePrepared(ctx context.Context, file PreparedFile, opts Options, name string) (SavedFile, error) {
	result := file.Metadata
	if err := ctx.Err(); err != nil {
		return result, err
	}
	target, err := StoredPath(filepath.Dir(opts.Dir), filepath.Base(opts.Dir), name)
	if err != nil {
		return result, err
	}
	if err = os.MkdirAll(opts.Dir, 0755); err != nil {
		return result, err
	}
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return result, err
	}
	writer, err := crypto.NewStreamWriter(dst, opts.EncryptionKey)
	if err != nil {
		return result, errors.Join(err, dst.Close(), os.Remove(target))
	}
	n, err := finishWrite(writer, dst, contextReader{ctx, bytes.NewReader(file.Content)}, opts.MaxFileSize, func() error { return os.Remove(target) })
	if err != nil {
		return result, err
	}
	if n != result.Size {
		return result, apperr.Internal("Не удалось записать файл целиком")
	}
	result.StoredName = name
	return result, nil
}
