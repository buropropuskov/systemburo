package database

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"systemburo/internal/crypto"
)

// Перевод паспортных данных с одного ключа шифрования на другой (#2253).
//
// Смена DATA_ENCRYPTION_KEY правкой файла параметров не переводит данные, а делает
// их нечитаемыми: расшифровка чужим ключом молча отдаёт шифротекст, и первое же
// сохранение записи зашифрует его повторно. Поэтому смена ключа идёт только через
// эту команду, а контрольная запись не пускает установку с ключом, который данным
// не соответствует.

// encryptedColumn - столбец с шифрованным значением и парный ему столбец HMAC.
type encryptedColumn struct {
	value string
	hmac  string
}

// encryptedTable - таблица, хранящая шифрованные поля.
type encryptedTable struct {
	name    string
	columns []encryptedColumn
}

// encryptedTables - перечень того, что переводится. Держится рядом с самой
// перешифровкой, потому что пропуск таблицы здесь означает молча недопереведённые
// данные: они останутся на старом ключе и после смены станут нечитаемыми.
var encryptedTables = []encryptedTable{
	{name: "employees", columns: passportColumns()},
	{name: "unique_employees", columns: passportColumns()},
	{name: "application_employees", columns: passportColumns()},
}

func passportColumns() []encryptedColumn {
	return []encryptedColumn{
		{value: "passport_series_number", hmac: "passport_series_number_hmac"},
		{value: "patent_number", hmac: "patent_number_hmac"},
	}
}

// ErrReencryptSourceKey - старый ключ не подходит к данным этой базы.
var ErrReencryptSourceKey = errors.New("указанный прежний ключ не подходит к данным этой базы")

// ReencryptOptions - что и куда переводить.
type ReencryptOptions struct {
	// OldKey - ключ, которым данные зашифрованы сейчас. nil означает, что данные
	// лежат открытыми и шифрование включается впервые.
	OldKey []byte
	// NewKey - ключ, на который переводим. nil означает снятие шифрования.
	NewKey []byte
	// Apply - выполнять запись. Без него команда только считает.
	Apply bool
}

// ReencryptResult - итог по одной таблице.
type ReencryptResult struct {
	Table string
	// Rows - строк, где есть хотя бы одно непустое шифруемое поле.
	Rows int64
	// Values - значений, подлежащих переводу.
	Values int64
}

// reencryptValue переводит одно значение со старого ключа на новый и считает новый
// HMAC. Возвращает ошибку, если старым ключом значение не читается: это и есть
// признак, что прежний ключ указан неверно.
//
// Функция отделена от базы намеренно - на ней держится вся проверка перевода, а
// тесты пакета делят одну базу и не могут опираться на её содержимое.
func reencryptValue(stored string, oldKey, newKey []byte) (value string, hmac string, err error) {
	plain, err := crypto.Decrypt(stored, oldKey)
	if err != nil {
		return "", "", fmt.Errorf("%w: значение не расшифровано", ErrReencryptSourceKey)
	}

	value, err = crypto.Encrypt(plain, newKey)
	if err != nil {
		return "", "", fmt.Errorf("шифрование новым ключом: %w", err)
	}
	return value, crypto.ComputeHMAC(plain, newKey), nil
}

// Reencrypt переводит паспортные и патентные поля на новый ключ.
//
// Вся работа идёт одной транзакцией: прерванный на середине перевод оставил бы
// часть записей на старом ключе, а часть на новом, и ни один из ключей не открывал
// бы базу целиком. Контрольная запись обновляется в той же транзакции последней -
// она и подтверждает, что перевод дошёл до конца.
//
// Запросы идут сырым SQL мимо моделей: хуки BeforeSave и AfterFind работают с
// глобальным ключом и зашифровали бы значения повторно.
func Reencrypt(ctx context.Context, db *gorm.DB, opts ReencryptOptions) ([]ReencryptResult, error) {
	if err := verifyReencryptSourceKey(ctx, db, opts.OldKey); err != nil {
		return nil, err
	}

	results := make([]ReencryptResult, 0, len(encryptedTables))

	run := func(tx *gorm.DB) error {
		for _, table := range encryptedTables {
			res, err := reencryptTable(ctx, tx, table, opts)
			if err != nil {
				return err
			}
			results = append(results, res)
		}
		if !opts.Apply {
			return nil
		}
		return writeEncryptionCanaryTx(tx, opts.NewKey)
	}

	if !opts.Apply {
		if err := run(db); err != nil {
			return nil, err
		}
		return results, nil
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		results = results[:0]
		return run(tx)
	}); err != nil {
		return nil, err
	}
	return results, nil
}

// verifyReencryptSourceKey сверяет заявленный прежний ключ с контрольной записью.
// Без этой проверки ошибка в ключе обнаружилась бы на середине перевода, когда
// часть значений уже переписана.
func verifyReencryptSourceKey(ctx context.Context, db *gorm.DB, oldKey []byte) error {
	var stored string
	err := db.WithContext(ctx).Raw(
		`SELECT value FROM system_settings WHERE key = ?`, encryptionCanaryKey,
	).Scan(&stored).Error
	if err != nil {
		return fmt.Errorf("чтение контрольной записи шифрования: %w", err)
	}
	if stored == "" {
		// Записи нет: установка ни разу не поднималась с этой сборкой. Сверять не с
		// чем, перевод разрешаем - данные проверит сама расшифровка.
		return nil
	}

	if _, err := decideCanary(stored, true, oldKey); err != nil {
		return fmt.Errorf("%w: %s", ErrReencryptSourceKey, err)
	}
	return nil
}

// reencryptTable переводит одну таблицу.
func reencryptTable(ctx context.Context, tx *gorm.DB, table encryptedTable, opts ReencryptOptions) (ReencryptResult, error) {
	res := ReencryptResult{Table: table.name}

	for _, col := range table.columns {
		type row struct {
			ID    int
			Value string
		}
		var rows []row
		query := fmt.Sprintf(
			`SELECT id, %s AS value FROM %s WHERE %s IS NOT NULL AND %s <> '' ORDER BY id`,
			col.value, table.name, col.value, col.value)
		if err := tx.WithContext(ctx).Raw(query).Scan(&rows).Error; err != nil {
			return res, fmt.Errorf("чтение %s.%s: %w", table.name, col.value, err)
		}

		res.Values += int64(len(rows))
		if !opts.Apply {
			continue
		}

		for _, r := range rows {
			value, hmac, err := reencryptValue(r.Value, opts.OldKey, opts.NewKey)
			if err != nil {
				return res, fmt.Errorf("%s.%s, запись %d: %w", table.name, col.value, r.ID, err)
			}
			update := fmt.Sprintf(`UPDATE %s SET %s = ?, %s = ? WHERE id = ?`,
				table.name, col.value, col.hmac)
			if err := tx.WithContext(ctx).Exec(update, value, hmac, r.ID).Error; err != nil {
				return res, fmt.Errorf("запись %s.%s id=%d: %w", table.name, col.value, r.ID, err)
			}
		}
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, table.name, notEmptyCondition(table.columns))
	if err := tx.WithContext(ctx).Raw(countQuery).Scan(&res.Rows).Error; err != nil {
		return res, fmt.Errorf("подсчёт строк %s: %w", table.name, err)
	}
	return res, nil
}

// notEmptyCondition собирает условие "хотя бы одно шифруемое поле заполнено".
func notEmptyCondition(columns []encryptedColumn) string {
	cond := ""
	for i, col := range columns {
		if i > 0 {
			cond += " OR "
		}
		cond += fmt.Sprintf("(%s IS NOT NULL AND %s <> '')", col.value, col.value)
	}
	return cond
}
