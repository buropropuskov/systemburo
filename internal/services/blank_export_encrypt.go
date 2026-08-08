package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"systemburo/internal/models"
)

/*
Разовая дошифровка файлов, записанных до включения ключей архива.

Шифрование архива смотрит на переменные окружения, и площадка, где их задали не
сразу, остаётся с двумя поколениями файлов: старые лежат открытыми, новые - под
age. Обычные строки реестра догоняет бэкфилл, потому что он ПЕРЕСОБИРАЕТ бланк, а
пересобранный файл пишется уже через шифрование. Замороженные так не догнать:
перезапись документа задним числом - ровно то, что заморозка запрещает.

Отсюда отдельный проход: он не генерирует ничего заново, а берёт лежащий на диске
файл и заворачивает его как есть. Содержимое документа не меняется ни на байт,
меняется только оболочка и имя, поэтому смысл заморозки не нарушен. Хэш и размер в
реестре считаются от ИСХОДНОГО содержимого (см. exportOne), так что их трогать не
нужно - и это же делает проход безопасным для ночной сверки.
*/

// ArchiveEncryptResult - сводка прохода дошифровки.
type ArchiveEncryptResult struct {
	// Candidates - строк реестра с открытым файлом на момент старта.
	Candidates int
	// Encrypted - файлов закрыто этим проходом.
	Encrypted int
	// Recovered - строк, где файл уже лежал закрытым, а реестр знал прежнее имя:
	// прошлый проход упал между диском и базой.
	Recovered int
	// Missing - файла нет на диске. Строку не трогаем: расхождение реестра с диском
	// разбирает ночная сверка, а не эта команда.
	Missing int
	// Failed - файлов, которые закрыть не удалось.
	Failed int
}

// ErrArchiveCryptoDisabled - ключи архива не заданы, шифровать нечем.
var ErrArchiveCryptoDisabled = errors.New("archive encryption keys are not set")

// EncryptExisting закрывает файлы архива, записанные до включения шифрования.
//
// dryRun считает и печатает, не трогая ни диск, ни реестр: на боевом каталоге
// первый запуск должен показывать объём работы, а не выполнять её.
func (s *BlankExportService) EncryptExisting(ctx context.Context, dryRun bool) (ArchiveEncryptResult, error) {
	var res ArchiveEncryptResult
	if !s.writer.Crypto().Enabled() {
		return res, ErrArchiveCryptoDisabled
	}

	var rows []models.BlankExport
	err := s.db.WithContext(ctx).
		Where("file_name <> '' AND file_name NOT LIKE ?", "%"+EncryptedSuffix).
		Order("id").
		Find(&rows).Error
	if err != nil {
		return res, fmt.Errorf("failed to load archive registry rows: %w", err)
	}
	res.Candidates = len(rows)

	for i := range rows {
		outcome, err := s.encryptExistingRow(ctx, &rows[i], dryRun)
		if err != nil {
			res.Failed++
			slog.Error("не удалось закрыть файл архива",
				"application_id", rows[i].ApplicationID, "attachment_id", rows[i].AttachmentID,
				"file", rows[i].FileName, "error", err)
			continue
		}
		switch outcome {
		case encryptOutcomeEncrypted:
			res.Encrypted++
		case encryptOutcomeRecovered:
			res.Recovered++
		case encryptOutcomeMissing:
			res.Missing++
		}
	}
	return res, nil
}

type encryptOutcome int

const (
	encryptOutcomeEncrypted encryptOutcome = iota
	encryptOutcomeRecovered
	encryptOutcomeMissing
)

// agePrefix - первые байты формата age. Файл с ним уже закрыт, и повторное
// шифрование сделало бы матрёшку, которую получатель распакует только дважды.
var agePrefix = []byte("age-encryption.org/v1")

// encryptExistingRow закрывает файл одной строки реестра.
//
// Порядок «диск, потом база» как во всей выгрузке. Падение между записью нового
// файла и обновлением строки лечится следующим прогоном: закрытый файл уже лежит
// рядом, и проход опознаёт это состояние по нему, а не по реестру.
func (s *BlankExportService) encryptExistingRow(ctx context.Context, row *models.BlankExport, dryRun bool) (encryptOutcome, error) {
	levels := splitRelDir(row.RelDir)
	target := row.FileName + EncryptedSuffix

	sealedExists, err := s.writer.Exists(levels, target)
	if err != nil {
		return 0, err
	}
	if sealedExists {
		// Незавершённый прошлый проход: закрытый файл на месте, реестр помнит старое
		// имя. Открытый остаток, если он ещё лежит рядом, убираем - иначе смысл
		// прохода теряется, данные так и останутся читаемыми.
		if dryRun {
			return encryptOutcomeRecovered, nil
		}
		if err := s.writer.RemoveFile(levels, row.FileName); err != nil {
			return 0, err
		}
		if err := s.renameRegistryFile(ctx, row.ID, target); err != nil {
			return 0, err
		}
		return encryptOutcomeRecovered, nil
	}

	full, err := s.writer.Resolve(append(append([]string{}, levels...), row.FileName)...)
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(full)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return encryptOutcomeMissing, nil
	case err != nil:
		return 0, fmt.Errorf("failed to read archive file: %w", err)
	}
	if dryRun {
		return encryptOutcomeEncrypted, nil
	}

	// Содержимое уже в формате age при открытом имени - шифровать второй раз нельзя,
	// достаточно переименовать. Так выглядит файл, записанный в короткое окно между
	// включением ключей и правкой имён.
	if bytes.HasPrefix(data, agePrefix) {
		if err := s.writer.MoveFile(levels, row.FileName, target); err != nil {
			return 0, err
		}
		return encryptOutcomeRecovered, s.renameRegistryFile(ctx, row.ID, target)
	}

	// WriteFile шифрует сам и кладёт файл атомарно - через временный файл, sync и
	// rename. Своя запись здесь означала бы вторую копию этих гарантий.
	if err := s.writer.WriteFile(levels, target, data); err != nil {
		return 0, err
	}
	if err := s.writer.RemoveFile(levels, row.FileName); err != nil {
		return 0, err
	}
	return encryptOutcomeEncrypted, s.renameRegistryFile(ctx, row.ID, target)
}

// renameRegistryFile правит в строке реестра только имя файла. Ни хэш, ни размер,
// ни момент заморозки не трогаются: они описывают содержимое документа, а оно тем
// же и осталось.
func (s *BlankExportService) renameRegistryFile(ctx context.Context, id int, name string) error {
	err := s.db.WithContext(ctx).Model(&models.BlankExport{}).
		Where("id = ?", id).
		Update("file_name", name).Error
	if err != nil {
		return fmt.Errorf("failed to update archive registry file name: %w", err)
	}
	return nil
}
