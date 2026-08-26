package crypto

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func streamKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, key)
	require.NoError(t, err)
	return key
}

func sealStream(t *testing.T, key, payload []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	w, err := NewStreamWriter(&out, key)
	require.NoError(t, err)
	_, err = w.Write(payload)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return out.Bytes()
}

// TestStream_RoundTripAcrossChunkBoundaries: размеры вокруг границы чанка ловят
// ошибки набора буфера, из-за которых файл кратной длины читался бы обрезанным.
func TestStream_RoundTripAcrossChunkBoundaries(t *testing.T) {
	key := streamKey(t)
	sizes := []int{0, 1, streamChunkSize - 1, streamChunkSize, streamChunkSize + 1, 3*streamChunkSize + 17}

	for _, size := range sizes {
		payload := make([]byte, size)
		_, err := io.ReadFull(rand.Reader, payload)
		require.NoError(t, err)

		sealed := sealStream(t, key, payload)
		require.True(t, IsStreamEncrypted(sealed), "размер %d", size)
		// Проверка «данные не лежат открытыми» осмысленна только на куске, который
		// не мог совпасть случайно: один-два байта встречаются в шифротексте сами
		// собой и делали бы тест плавающим.
		if size >= 32 {
			require.NotContains(t, string(sealed), string(payload[:32]), "размер %d: данные не должны лежать открытыми", size)
		}

		r, err := NewStreamReader(bytes.NewReader(sealed), key)
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err, "размер %d", size)
		require.Equal(t, payload, got, "размер %d", size)
	}
}

// TestStream_WriteInSmallPiecesMatchesSingleWrite: io.Copy кормит writer кусками по
// 32 КБ, и результат обязан совпасть с одной большой записью.
func TestStream_WriteInSmallPiecesMatchesSingleWrite(t *testing.T) {
	key := streamKey(t)
	payload := make([]byte, 5*streamChunkSize+123)
	_, err := io.ReadFull(rand.Reader, payload)
	require.NoError(t, err)

	var out bytes.Buffer
	w, err := NewStreamWriter(&out, key)
	require.NoError(t, err)
	_, err = io.Copy(w, bytes.NewReader(payload))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	r, err := NewStreamReader(bytes.NewReader(out.Bytes()), key)
	require.NoError(t, err)
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

// TestStream_TruncatedFileFailsLoudly: обрезанный файл не должен читаться как целый.
// Ради этого признак последнего чанка и уходит в дополнительные данные GCM.
func TestStream_TruncatedFileFailsLoudly(t *testing.T) {
	key := streamKey(t)
	payload := make([]byte, 3*streamChunkSize)
	_, err := io.ReadFull(rand.Reader, payload)
	require.NoError(t, err)
	sealed := sealStream(t, key, payload)

	// Отрезаем последний чанк целиком: получается поток из полных чанков, каждый
	// из которых сам по себе расшифровывается.
	cut := sealed[:len(streamMagic)+streamNoncePfx+(streamChunkSize+16)]
	r, err := NewStreamReader(bytes.NewReader(cut), key)
	require.NoError(t, err)
	_, err = io.ReadAll(r)
	require.ErrorIs(t, err, ErrStreamTruncated)
}

// TestStream_TamperedByteFailsDecryption: правка шифротекста обязана ломать чтение,
// а не отдавать испорченный документ.
func TestStream_TamperedByteFailsDecryption(t *testing.T) {
	key := streamKey(t)
	sealed := sealStream(t, key, []byte("паспортные данные внутри"))
	sealed[len(sealed)-1] ^= 0xFF

	r, err := NewStreamReader(bytes.NewReader(sealed), key)
	require.NoError(t, err)
	_, err = io.ReadAll(r)
	require.Error(t, err)
}

// TestStream_ForeignKeyCannotRead: копия без ключа бесполезна - ради этого файлы и
// шифруются, ключ в резервные копии намеренно не попадает.
func TestStream_ForeignKeyCannotRead(t *testing.T) {
	sealed := sealStream(t, streamKey(t), []byte("содержимое"))

	r, err := NewStreamReader(bytes.NewReader(sealed), streamKey(t))
	require.NoError(t, err)
	_, err = io.ReadAll(r)
	require.Error(t, err)
}

// TestStream_NilKeyIsPassthrough: без ключа система работает как раньше, файл
// пишется и читается открытым.
func TestStream_NilKeyIsPassthrough(t *testing.T) {
	payload := []byte("открытый файл")

	var out bytes.Buffer
	w, err := NewStreamWriter(&out, nil)
	require.NoError(t, err)
	_, err = w.Write(payload)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.Equal(t, payload, out.Bytes())

	r, err := NewStreamReader(bytes.NewReader(out.Bytes()), nil)
	require.NoError(t, err)
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

// TestStream_PlainFileReadableWithKey: файлы, записанные до появления шифрования,
// продолжают читаться и при поднятом ключе.
func TestStream_PlainFileReadableWithKey(t *testing.T) {
	payload := []byte("файл из прошлой версии")

	r, err := NewStreamReader(bytes.NewReader(payload), streamKey(t))
	require.NoError(t, err)
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, payload, got)
}
