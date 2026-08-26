package services

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/stretchr/testify/require"
)

// keyPair отдаёт публичный ключ «получателя» и приватный ключ системы.
func keyPair(t *testing.T) (recipient, identity string) {
	t.Helper()
	ext, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	own, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	return ext.Recipient().String(), own.String()
}

// TestArchiveCrypto_ReadableByBothSides: файл архива обязан открываться и внешним
// получателем, и самой системой. Первое - смысл архива, второе - выгрузка ZIP из
// карточки заявки, которая иначе перестала бы работать.
func TestArchiveCrypto_ReadableByBothSides(t *testing.T) {
	extIdentity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	ownIdentity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	crypto, err := NewArchiveCrypto(extIdentity.Recipient().String(), ownIdentity.String())
	require.NoError(t, err)
	require.True(t, crypto.Enabled())

	payload := []byte("паспорт 4509 123456, патент 77 №1234567")
	sealed, err := crypto.Encrypt(payload)
	require.NoError(t, err)
	require.NotContains(t, string(sealed), "паспорт", "содержимое не должно лежать открытым")

	// Внешняя сторона читает своим ключом.
	r, err := age.Decrypt(strings.NewReader(string(sealed)), extIdentity)
	require.NoError(t, err)
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, payload, got)

	// Система читает своим - через тот же путь, каким отдаёт файл в ZIP.
	dir := t.TempDir()
	path := filepath.Join(dir, "бланк.xlsx"+EncryptedSuffix)
	require.NoError(t, os.WriteFile(path, sealed, 0o600))

	rc, err := crypto.Open(path)
	require.NoError(t, err)
	defer rc.Close()
	got, err = io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

// TestArchiveCrypto_ForeignKeyCannotRead: посторонний ключ бесполезен - ради этого
// шифрование и вводилось, каталог архива отдают в сетевой доступ.
func TestArchiveCrypto_ForeignKeyCannotRead(t *testing.T) {
	recipient, identity := keyPair(t)
	crypto, err := NewArchiveCrypto(recipient, identity)
	require.NoError(t, err)

	sealed, err := crypto.Encrypt([]byte("сведения о работнике"))
	require.NoError(t, err)

	stranger, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	_, err = age.Decrypt(strings.NewReader(string(sealed)), stranger)
	require.Error(t, err)
}

// TestArchiveCrypto_DisabledKeepsPlainBehaviour: без ключей поведение прежнее -
// площадка, которой шифрование не нужно, не ломается обновлением.
func TestArchiveCrypto_DisabledKeepsPlainBehaviour(t *testing.T) {
	crypto, err := NewArchiveCrypto("", "")
	require.NoError(t, err)
	require.False(t, crypto.Enabled())

	payload := []byte("бланк как есть")
	out, err := crypto.Encrypt(payload)
	require.NoError(t, err)
	require.Equal(t, payload, out)
	require.Equal(t, "бланк.xlsx", crypto.FileName("бланк.xlsx"))

	dir := t.TempDir()
	path := filepath.Join(dir, "бланк.xlsx")
	require.NoError(t, os.WriteFile(path, payload, 0o600))
	rc, err := crypto.Open(path)
	require.NoError(t, err)
	defer rc.Close()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

// TestArchiveCrypto_HalfKeyRejected: получатель без ключа системы означал бы архив,
// который система не может прочитать сама, - выгрузка ZIP молча перестала бы работать.
func TestArchiveCrypto_HalfKeyRejected(t *testing.T) {
	recipient, identity := keyPair(t)

	_, err := NewArchiveCrypto(recipient, "")
	require.Error(t, err)
	_, err = NewArchiveCrypto("", identity)
	require.Error(t, err)

	_, err = NewArchiveCrypto("не ключ", identity)
	require.Error(t, err)
	_, err = NewArchiveCrypto(recipient, "не ключ")
	require.Error(t, err)
}

// TestArchiveCrypto_PlainFileStillReadable: файлы, записанные до включения режима,
// продолжают открываться - признак берётся из имени, а не из настройки.
func TestArchiveCrypto_PlainFileStillReadable(t *testing.T) {
	recipient, identity := keyPair(t)
	crypto, err := NewArchiveCrypto(recipient, identity)
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "старый.xlsx")
	require.NoError(t, os.WriteFile(path, []byte("записан до шифрования"), 0o600))

	rc, err := crypto.Open(path)
	require.NoError(t, err)
	defer rc.Close()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "записан до шифрования", string(got))
}

// TestArchiveWriter_WritesEncrypted: писатель кладёт на диск шифротекст, а имя
// получает суффикс - по нему оператор видит, что файл не откроется двойным кликом.
func TestArchiveWriter_WritesEncrypted(t *testing.T) {
	recipient, identity := keyPair(t)
	crypto, err := NewArchiveCrypto(recipient, identity)
	require.NoError(t, err)

	root := filepath.Join(t.TempDir(), "archive")
	writer, err := NewArchiveWriter(root)
	require.NoError(t, err)
	writer.SetCrypto(crypto)

	name := crypto.FileName("Автозаявка.xlsx")
	require.NoError(t, writer.WriteFile([]string{"2026", "08"}, name, []byte("паспортные данные")))

	onDisk, err := os.ReadFile(filepath.Join(root, "2026", "08", name))
	require.NoError(t, err)
	require.NotContains(t, string(onDisk), "паспортные данные")

	rc, err := crypto.Open(filepath.Join(root, "2026", "08", name))
	require.NoError(t, err)
	defer rc.Close()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "паспортные данные", string(got))
}

// TestArchiveWriter_SnapshotRoundTrip: слепок заявки - самый чувствительный файл
// архива, в нём паспорта всех людей заявки. Он обязан шифроваться наравне с
// бланками и читаться обратно: имя без суффикса заставило бы чтение отдать
// шифротекст как есть, то есть выдать принимающей стороне нечитаемый мусор.
func TestArchiveWriter_SnapshotRoundTrip(t *testing.T) {
	recipient, identity := keyPair(t)
	crypto, err := NewArchiveCrypto(recipient, identity)
	require.NoError(t, err)

	root := filepath.Join(t.TempDir(), "archive")
	writer, err := NewArchiveWriter(root)
	require.NoError(t, err)
	writer.SetCrypto(crypto)

	levels := []string{"2026", "08", "заявка"}
	name := crypto.FileName(archiveSnapshotFileName)
	require.Equal(t, "заявка.json.age", name)

	payload := []byte(`{"passport":"4509 123456"}`)
	require.NoError(t, writer.WriteFile(levels, name, payload))

	full := filepath.Join(append([]string{root}, append(levels, name)...)...)
	raw, err := os.ReadFile(full)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "4509", "слепок не должен лежать открытым")

	rc, err := crypto.Open(full)
	require.NoError(t, err)
	defer rc.Close()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

// TestSnapshotContentChanged_StableForSameData: сравнение идёт по расшифрованному
// содержимому. Шифрование берёт новый ключ потока на каждую запись, поэтому байты
// на диске у неизменного слепка каждый раз разные - сравнение шифротекстов
// переписывало бы файл на каждом прогоне и двигало mtime, ломая инкрементальную
// синхронизацию на рабочий компьютер.
func TestSnapshotContentChanged_StableForSameData(t *testing.T) {
	recipient, identity := keyPair(t)
	crypto, err := NewArchiveCrypto(recipient, identity)
	require.NoError(t, err)

	root := filepath.Join(t.TempDir(), "archive")
	writer, err := NewArchiveWriter(root)
	require.NoError(t, err)
	writer.SetCrypto(crypto)

	levels := []string{"2026", "08", "заявка"}
	payload := []byte(`{"application":"№ 1"}`)
	require.NoError(t, writer.WriteFile(levels, crypto.FileName(archiveSnapshotFileName), payload))

	changed, err := snapshotContentChanged(writer, levels, payload)
	require.NoError(t, err)
	require.False(t, changed, "неизменный слепок не должен переписываться")

	changed, err = snapshotContentChanged(writer, levels, []byte(`{"application":"№ 2"}`))
	require.NoError(t, err)
	require.True(t, changed, "изменённый слепок обязан переписаться")
}
