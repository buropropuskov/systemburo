package crypto

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

/*
Потоковое шифрование файлов (#1721).

Строковый Encrypt для файлов не годится: он держит в памяти и plaintext, и
base64 целиком. Здесь поток режется на чанки по 64 КБ, каждый запечатывается
AES-256-GCM своим счётчиком nonce, а признак «последний» уходит в
дополнительные данные. Без этого признака обрезанный на середине файл
расшифровался бы молча и выглядел бы целым - как раз то, чего нельзя допустить
для документа, приложенного к заявке.

Формат: "SBF1" | nonce_prefix(8) | чанк* , где чанк = GCM(64 КБ plaintext).
Ключ nil означает passthrough - ту же семантику, что у строкового Encrypt.
*/

const (
	streamMagic     = "SBF1"
	streamNoncePfx  = 8
	streamChunkSize = 64 * 1024
)

var (
	aadChunk = []byte{0}
	aadFinal = []byte{1}

	// ErrStreamTruncated -- поток кончился без чанка, помеченного последним.
	ErrStreamTruncated = errors.New("зашифрованный файл обрезан")
)

// IsStreamEncrypted отвечает, начинается ли содержимое с заголовка потока.
func IsStreamEncrypted(header []byte) bool {
	return len(header) >= len(streamMagic) && string(header[:len(streamMagic)]) == streamMagic
}

// StreamWriter шифрует записываемые байты чанками. Close обязателен: он
// дописывает последний чанк, без которого файл не прочитается.
type StreamWriter struct {
	dst    io.Writer
	gcm    cipher.AEAD
	prefix []byte
	buf    []byte
	num    uint32
	closed bool
}

// NewStreamWriter оборачивает dst шифрующим writer-ом. При nil-ключе возвращает
// passthrough-обёртку, чтобы вызывающий не разветвлял код.
func NewStreamWriter(dst io.Writer, key []byte) (io.WriteCloser, error) {
	if key == nil {
		return nopWriteCloser{dst}, nil
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	prefix := make([]byte, streamNoncePfx)
	if _, err := io.ReadFull(rand.Reader, prefix); err != nil {
		return nil, fmt.Errorf("rand nonce prefix: %w", err)
	}
	if _, err := dst.Write([]byte(streamMagic)); err != nil {
		return nil, fmt.Errorf("write stream magic: %w", err)
	}
	if _, err := dst.Write(prefix); err != nil {
		return nil, fmt.Errorf("write nonce prefix: %w", err)
	}
	return &StreamWriter{dst: dst, gcm: gcm, prefix: prefix, buf: make([]byte, 0, streamChunkSize)}, nil
}

func (w *StreamWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, errors.New("запись в закрытый поток")
	}
	written := len(p)
	for len(p) > 0 {
		// Полный чанк уходит лениво - в тот момент, когда пришли новые данные.
		// Иначе последний чанк файла, кратного размеру чанка, уехал бы без метки
		// финала, и чтение объявляло бы целый файл обрезанным.
		if len(w.buf) == streamChunkSize {
			if err := w.flush(aadChunk); err != nil {
				return 0, err
			}
		}
		take := min(streamChunkSize-len(w.buf), len(p))
		w.buf = append(w.buf, p[:take]...)
		p = p[take:]
	}
	return written, nil
}

func (w *StreamWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return w.flush(aadFinal)
}

func (w *StreamWriter) flush(aad []byte) error {
	sealed := w.gcm.Seal(nil, w.nonce(), w.buf, aad)
	w.buf = w.buf[:0]
	w.num++
	if _, err := w.dst.Write(sealed); err != nil {
		return fmt.Errorf("write chunk: %w", err)
	}
	return nil
}

func (w *StreamWriter) nonce() []byte {
	nonce := make([]byte, 12)
	copy(nonce, w.prefix)
	binary.BigEndian.PutUint32(nonce[streamNoncePfx:], w.num)
	return nonce
}

// NewStreamReader расшифровывает поток, записанный StreamWriter. Ключ nil и
// содержимое без заголовка отдаются как есть: до появления шифрования файлы
// писались открытыми, и читать их всё равно нужно.
func NewStreamReader(src io.Reader, key []byte) (io.Reader, error) {
	buffered := bufio.NewReaderSize(src, streamChunkSize+64)
	header, err := buffered.Peek(len(streamMagic))
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read stream header: %w", err)
	}
	if key == nil || !IsStreamEncrypted(header) {
		return buffered, nil
	}

	if _, err := buffered.Discard(len(streamMagic)); err != nil {
		return nil, fmt.Errorf("skip stream magic: %w", err)
	}
	prefix := make([]byte, streamNoncePfx)
	if _, err := io.ReadFull(buffered, prefix); err != nil {
		return nil, fmt.Errorf("read nonce prefix: %w", err)
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	return &streamReader{src: buffered, gcm: gcm, prefix: prefix}, nil
}

type streamReader struct {
	src    *bufio.Reader
	gcm    cipher.AEAD
	prefix []byte
	num    uint32
	plain  bytes.Reader
	done   bool
	err    error
}

func (r *streamReader) Read(p []byte) (int, error) {
	for {
		if r.plain.Len() > 0 {
			return r.plain.Read(p)
		}
		if r.err != nil {
			return 0, r.err
		}
		if r.done {
			return 0, io.EOF
		}
		if err := r.readChunk(); err != nil {
			r.err = err
			return 0, err
		}
	}
}

func (r *streamReader) readChunk() error {
	sealed := make([]byte, streamChunkSize+r.gcm.Overhead())
	n, err := io.ReadFull(r.src, sealed)
	switch {
	case err == nil:
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		// Неполный чанк бывает только последним.
		sealed = sealed[:n]
	default:
		return fmt.Errorf("read chunk: %w", err)
	}
	if len(sealed) < r.gcm.Overhead() {
		return ErrStreamTruncated
	}

	// Финальным считается чанк, за которым в потоке ничего нет. Метка проверяется
	// самим GCM: подмена признака ломает аутентификацию.
	_, peekErr := r.src.Peek(1)
	aad := aadChunk
	final := errors.Is(peekErr, io.EOF)
	if final {
		aad = aadFinal
	}

	plain, err := r.gcm.Open(nil, r.nonce(), sealed, aad)
	if err != nil {
		if !final {
			return fmt.Errorf("decrypt chunk: %w", err)
		}
		return ErrStreamTruncated
	}
	r.num++
	r.plain.Reset(plain)
	r.done = final
	return nil
}

func (r *streamReader) nonce() []byte {
	nonce := make([]byte, 12)
	copy(nonce, r.prefix)
	binary.BigEndian.PutUint32(nonce[streamNoncePfx:], r.num)
	return nonce
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	return gcm, nil
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
