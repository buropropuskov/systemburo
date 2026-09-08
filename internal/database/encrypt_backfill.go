package database

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
)

// Дошифровка значений, оставшихся открытыми (#2351).
//
// Поле, которое начали шифровать позже, чем завели, оставляет в базе две породы
// записей: новые лежат шифротекстом, старые - как есть. Пока читают по одной, это
// незаметно (AfterFind отдаёт открытое значение без изменений), но поиск по почте
// и телефону идёт по свёртке, а у старой записи свёртки нет. На стенде это
// выглядело так: человек есть, а по собственному адресу не находится. Тесты этого
// не видели - в них база создаётся с нуля и разнородных записей в ней не бывает.
//
// Команда server reencrypt здесь не помогает: она переводит таблицу целиком с
// одного ключа на другой и на смешанной базе либо испортит уже зашифрованные
// записи, либо остановится на открытых.

// encryptionBackfillKey - отметка о выполненной дошифровке в system_settings.
const encryptionBackfillKey = "security.encryption_backfill"

// encryptionBackfillGeneration - поколение перечня шифруемых столбцов. Поднимать
// при добавлении нового столбца в encryptedTables: пока отметка отстаёт от
// поколения, проход повторяется и добирает то, что лежит открытым.
const encryptionBackfillGeneration = "1"

// encryptBackfillBatch - сколько строк столбца читается за раз.
const encryptBackfillBatch = 1000

// EncryptPlaintextValues дошифровывает значения, лежащие открытыми, и считает им
// свёртки. Возвращает число переведённых значений.
//
// Признак «значение открыто» - оно не расшифровывается действующим ключом. Отсюда
// идемпотентность: уже зашифрованная запись под условие не попадает, а повторный
// запуск ничего не делает. Проверка надёжна, потому что расшифровка AES-GCM
// проверяет имитовставку: открытый текст её не проходит.
//
// Вызывать только после EnsureEncryptionKeyMatches. Иначе при неверном ключе
// открытые значения были бы зашифрованы им же, и прежний ключ их уже не открыл бы.
//
// Транзакции нет намеренно: проход идёт значение за значением и безопасен на
// прерывании - недоделанное доберёт следующий запуск. Отметка ставится последней,
// только после полного прохода.
func EncryptPlaintextValues(ctx context.Context, db *gorm.DB, key []byte) (int, error) {
	if key == nil {
		// Шифрование выключено: значения и должны лежать открытыми.
		return 0, nil
	}
	done, err := encryptBackfillDone(ctx, db)
	if err != nil {
		return 0, err
	}
	if done {
		return 0, nil
	}

	total := 0
	for _, table := range encryptedTables {
		for _, col := range table.columns {
			n, err := encryptPlaintextColumn(ctx, db, table.name, col, key)
			if err != nil {
				return total, err
			}
			total += n
		}
	}

	if err := markEncryptBackfillDone(ctx, db); err != nil {
		return total, err
	}
	if total > 0 {
		slog.Info("значения, лежавшие открытыми, зашифрованы", "значений", total)
	}
	return total, nil
}

// encryptPlaintextColumn переводит один столбец, читая его пачками по id.
func encryptPlaintextColumn(ctx context.Context, db *gorm.DB, table string, col encryptedColumn, key []byte) (int, error) {
	type row struct {
		ID    int
		Value string
	}

	converted := 0
	lastID := 0
	query := fmt.Sprintf(
		`SELECT id, %s AS value FROM %s WHERE %s IS NOT NULL AND %s <> '' AND id > ? ORDER BY id LIMIT %d`,
		col.value, table, col.value, col.value, encryptBackfillBatch)

	for {
		var rows []row
		if err := db.WithContext(ctx).Raw(query, lastID).Scan(&rows).Error; err != nil {
			return converted, fmt.Errorf("чтение %s.%s: %w", table, col.value, err)
		}
		if len(rows) == 0 {
			return converted, nil
		}
		lastID = rows[len(rows)-1].ID

		for _, r := range rows {
			value, hmac, plaintext, err := encryptIfPlaintext(r.Value, key, col.normalize)
			if err != nil {
				return converted, fmt.Errorf("шифрование %s.%s, запись %d: %w", table, col.value, r.ID, err)
			}
			if !plaintext {
				if looksEncrypted(r.Value) && !decryptsWith(r.Value, key) {
					// Значение похоже на шифротекст, но действующим ключом не
					// читается: скорее всего оно осталось от прежнего ключа.
					// Шифровать его повторно нельзя - после этого не поможет уже
					// ни один ключ. Пропускаем и говорим об этом вслух.
					slog.Warn("значение не читается действующим ключом, оставлено как есть",
						"таблица", table, "столбец", col.value, "запись", r.ID)
				}
				continue
			}
			var update string
			var args []any
			if col.hmac == "" {
				update = fmt.Sprintf(`UPDATE %s SET %s = ? WHERE id = ?`, table, col.value)
				args = []any{value, r.ID}
			} else {
				update = fmt.Sprintf(`UPDATE %s SET %s = ?, %s = ? WHERE id = ?`, table, col.value, col.hmac)
				args = []any{value, hmac, r.ID}
			}
			if err := db.WithContext(ctx).Exec(update, args...).Error; err != nil {
				return converted, fmt.Errorf("запись %s.%s id=%d: %w", table, col.value, r.ID, err)
			}
			converted++
		}
	}
}

// encryptBackfillDone сообщает, покрыто ли текущее поколение перечня.
func encryptBackfillDone(ctx context.Context, db *gorm.DB) (bool, error) {
	var stored string
	err := db.WithContext(ctx).Raw(
		`SELECT value FROM system_settings WHERE key = ?`, encryptionBackfillKey,
	).Scan(&stored).Error
	if err != nil {
		return false, fmt.Errorf("чтение отметки о дошифровке: %w", err)
	}
	return stored == encryptionBackfillGeneration, nil
}

// markEncryptBackfillDone запоминает поколение, чтобы следующий запуск не читал
// шифруемые столбцы целиком.
func markEncryptBackfillDone(ctx context.Context, db *gorm.DB) error {
	setting := models.SystemSetting{
		Key:   encryptionBackfillKey,
		Value: encryptionBackfillGeneration,
		Type:  "string",
	}
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&setting).Error
	if err != nil {
		return fmt.Errorf("отметка о дошифровке: %w", err)
	}
	return nil
}

// encryptIfPlaintext решает судьбу одного значения: plaintext=false означает, что
// оно уже зашифровано и трогать его нельзя.
//
// Решение отделено от базы намеренно - на нём держится вся идемпотентность прохода,
// а тесты пакета делят одну базу и не могут опираться на её содержимое.
func encryptIfPlaintext(stored string, key []byte, normalize func(string) string) (value string, hmac string, plaintext bool, err error) {
	if decryptsWith(stored, key) || looksEncrypted(stored) {
		return "", "", false, nil
	}

	value, err = crypto.Encrypt(stored, key)
	if err != nil {
		return "", "", false, err
	}
	return value, computeColumnHMAC(stored, key, normalize), true, nil
}

// decryptsWith - значение читается этим ключом.
func decryptsWith(stored string, key []byte) bool {
	_, err := crypto.Decrypt(stored, key)
	return err == nil
}

// looksEncrypted - значение по виду шифротекст: разбирается из base64 и длиннее
// суммы nonce и имитовставки AES-GCM.
//
// Нужно как страховка от значения, зашифрованного прежним ключом: расшифровка его
// не берёт, и без проверки вида проход счёл бы его открытым и зашифровал повторно -
// после чего не помог бы уже ни один ключ. Открытые данные под условие не подходят:
// в почте есть @ и точка, в паспорте пробел, в телефоне скобки, а короткая строка
// из одних цифр не дотягивает по длине.
func looksEncrypted(stored string) bool {
	data, err := base64.StdEncoding.DecodeString(stored)
	if err != nil {
		return false
	}
	return len(data) >= aesGCMNonceSize+aesGCMTagSize
}

// Размеры служебных частей AES-GCM: значение короче их суммы шифротекстом быть не
// может.
const (
	aesGCMNonceSize = 12
	aesGCMTagSize   = 16
)
