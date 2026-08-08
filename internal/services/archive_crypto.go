package services

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"filippo.io/age"
)

/*
Шифрование файлового архива.

Бланки и слепки заявок лежат в архиве для чтения снаружи: их забирает
корпоративный сервер. Поэтому шифровать их собственным ключом системы нельзя -
получатель не расшифрует. Решение стандартное: шифруем на ПАРУ получателей.

  - публичный ключ принимающей стороны (ARCHIVE_AGE_RECIPIENT) - им архив читает
    тот, кому он предназначен; приватной половины на сервере бюро нет вовсе;
  - собственный ключ системы (ARCHIVE_AGE_IDENTITY) - иначе система не смогла бы
    отдать ZIP по кнопке в карточке заявки, то есть потеряла бы уже работающую
    возможность.

Формат age выбран не случайно: им же шифруются резервные копии
(BACKUP_AGE_RECIPIENT в scripts/backup.sh), то есть на площадке уже есть и
инструмент, и порядок хранения ключей.

Выключенный режим (ключи не заданы) оставляет прежнее поведение: файлы пишутся
как есть. Так площадка, которой шифрование не нужно, не ломается обновлением.
*/

// ArchiveCrypto шифрует файлы архива при записи и расшифровывает при чтении.
// Нулевое значение (nil) означает выключенный режим.
type ArchiveCrypto struct {
	recipients []age.Recipient
	identity   age.Identity
}

// EncryptedSuffix - расширение зашифрованного файла архива. Оператор по нему
// сразу видит, что файл не откроется двойным кликом без ключа.
const EncryptedSuffix = ".age"

// NewArchiveCrypto собирает шифрование архива из ключей.
//
// recipient - публичный ключ получателя (age1...), identity - приватный ключ
// системы (AGE-SECRET-KEY-...). Пустая пара выключает шифрование. Заданный
// получатель без ключа системы отклоняется: такой архив система не прочитает и
// молча потеряет выгрузку ZIP.
func NewArchiveCrypto(recipient, identity string) (*ArchiveCrypto, error) {
	recipient = strings.TrimSpace(recipient)
	identity = strings.TrimSpace(identity)
	if recipient == "" && identity == "" {
		return nil, nil
	}
	if recipient == "" || identity == "" {
		return nil, fmt.Errorf("ARCHIVE_AGE_RECIPIENT и ARCHIVE_AGE_IDENTITY задаются только вместе: без ключа системы архив не прочитать из интерфейса")
	}

	rcpt, err := age.ParseX25519Recipient(recipient)
	if err != nil {
		return nil, fmt.Errorf("ARCHIVE_AGE_RECIPIENT: %w", err)
	}
	ident, err := age.ParseX25519Identity(identity)
	if err != nil {
		return nil, fmt.Errorf("ARCHIVE_AGE_IDENTITY: %w", err)
	}

	return &ArchiveCrypto{
		// Себя тоже кладём в получатели: age позволяет несколько, и каждый
		// расшифровывает своим ключом независимо от остальных.
		recipients: []age.Recipient{rcpt, ident.Recipient()},
		identity:   ident,
	}, nil
}

// Enabled отвечает, включено ли шифрование архива.
func (c *ArchiveCrypto) Enabled() bool { return c != nil && len(c.recipients) > 0 }

// FileName возвращает имя файла с учётом режима: при шифровании добавляется суффикс.
func (c *ArchiveCrypto) FileName(name string) string {
	if !c.Enabled() {
		return name
	}
	return name + EncryptedSuffix
}

// Encrypt шифрует содержимое файла архива. При выключенном режиме отдаёт как есть.
func (c *ArchiveCrypto) Encrypt(data []byte) ([]byte, error) {
	if !c.Enabled() {
		return data, nil
	}
	var out bytes.Buffer
	w, err := age.Encrypt(&out, c.recipients...)
	if err != nil {
		return nil, fmt.Errorf("encrypt archive file: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("write archive file: %w", err)
	}
	// Close дописывает завершение потока: без него файл не расшифруется.
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("finish archive file: %w", err)
	}
	return out.Bytes(), nil
}

// Open открывает файл архива на чтение, расшифровывая при необходимости.
// Признак шифрования берётся из имени, а не из настройки: в архиве могут лежать
// файлы, записанные до включения режима, и их всё равно нужно отдавать.
func (c *ArchiveCrypto) Open(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(path, EncryptedSuffix) {
		return f, nil
	}
	if !c.Enabled() {
		f.Close()
		return nil, fmt.Errorf("файл зашифрован, а ключ системы не задан: проверьте ARCHIVE_AGE_IDENTITY")
	}

	r, err := age.Decrypt(f, c.identity)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("decrypt archive file: %w", err)
	}
	return readCloser{Reader: r, closer: f}, nil
}

// readCloser связывает поток расшифровки с закрытием исходного файла: сам
// age.Decrypt отдаёт Reader без Close, и без обёртки дескриптор бы утекал.
type readCloser struct {
	io.Reader
	closer io.Closer
}

func (r readCloser) Close() error { return r.closer.Close() }
