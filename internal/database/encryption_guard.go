package database

import (
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
)

// encryptionCanaryKey - строка в system_settings, по которой сверяется ключ шифрования.
// Отдельная таблица не заводится намеренно: system_settings уже мигрируется, а
// контрольная запись это ровно одна пара ключ-значение.
const encryptionCanaryKey = "security.encryption_canary"

// encryptionCanaryProbe - то, что кладётся в контрольную запись зашифрованным.
// Значение произвольное, важна только неизменность: расшифровали и сравнили.
const encryptionCanaryProbe = "systemburo-encryption-canary-v1"

// ErrEncryptionKeyMismatch - действующий ключ не тот, которым зашифрована база.
var ErrEncryptionKeyMismatch = errors.New("ключ шифрования не подходит к данным этой базы")

// canaryVerdict - что делать с контрольной записью по итогам сверки.
type canaryVerdict int

const (
	// canaryOK - ключ подходит, писать ничего не надо.
	canaryOK canaryVerdict = iota
	// canaryWrite - записи ещё нет либо она открытая при заданном ключе; её
	// нужно сохранить под действующим ключом.
	canaryWrite
)

// decideCanary - решение по контрольной записи, отделённое от базы.
//
// Отдельная функция здесь не ради стиля: строка контрольной записи одна на всю
// базу, а тесты пакета делят базу между собой. Проверяя решение без похода в
// базу, тест не зависит от того, кто и когда эту строку трогал.
//
// stored - значение из базы, exists - была ли запись найдена.
func decideCanary(stored string, exists bool, key []byte) (canaryVerdict, error) {
	if !exists {
		return canaryWrite, nil
	}

	if key == nil {
		// Значение лежит открытым ровно тогда, когда его писали без ключа. Всё
		// остальное означает зашифрованную базу, для которой ключ забыли задать:
		// без отказа она поднимется и покажет операторам шифротекст вместо
		// паспортов.
		if stored == encryptionCanaryProbe {
			return canaryOK, nil
		}
		return canaryOK, fmt.Errorf("%w: база зашифрована, а DATA_ENCRYPTION_KEY не задан", ErrEncryptionKeyMismatch)
	}

	// Открытая контрольная запись при заданном ключе - законный случай: шифрование
	// включили на установке, которая работала без него. Данные при этом остались
	// открытыми, их переводит команда reencrypt.
	if stored == encryptionCanaryProbe {
		return canaryWrite, nil
	}

	probe, err := crypto.Decrypt(stored, key)
	if err != nil || probe != encryptionCanaryProbe {
		return canaryOK, fmt.Errorf("%w: DATA_ENCRYPTION_KEY не совпадает с ключом, которым зашифрованы паспортные данные", ErrEncryptionKeyMismatch)
	}
	return canaryOK, nil
}

// EnsureEncryptionKeyMatches сверяет ключ шифрования с контрольной записью и
// заводит её, если записи ещё нет.
//
// Зачем: расшифровка неподходящим ключом молча возвращает шифротекст
// (crypto.DecryptOptional), поэтому подмена ключа сама себя не обнаруживает.
// Система стартует, оператор видит в поле паспорта набор символов, сохраняет
// запись - и BeforeSave шифрует уже зашифрованное вторым ключом. После этого
// не помогает и первый ключ. Проверка ставится до приёма трафика, чтобы такая
// установка не поднялась вовсе и данные остались нетронутыми.
func EnsureEncryptionKeyMatches(db *gorm.DB, key []byte) error {
	var setting models.SystemSetting
	err := db.Where("key = ?", encryptionCanaryKey).First(&setting).Error

	exists := true
	if errors.Is(err, gorm.ErrRecordNotFound) {
		exists = false
	} else if err != nil {
		return fmt.Errorf("чтение контрольной записи шифрования: %w", err)
	}

	verdict, err := decideCanary(setting.Value, exists, key)
	if err != nil {
		return err
	}
	if verdict == canaryOK {
		return nil
	}

	switch {
	case !exists:
		slog.Info("заведена контрольная запись ключа шифрования")
	case key != nil:
		slog.Warn("шифрование включено на базе, которая работала без него: ранее сохранённые паспорта и патенты лежат открытыми, перевести их можно командой server reencrypt")
	}

	var existing *models.SystemSetting
	if exists {
		existing = &setting
	}
	return writeEncryptionCanary(db, key, existing)
}

// writeEncryptionCanaryTx переписывает контрольную запись под новый ключ, заводя её
// при отсутствии. Вызывается перешифровкой последним шагом её транзакции: пока
// запись не обновлена, перевод не считается состоявшимся.
func writeEncryptionCanaryTx(tx *gorm.DB, key []byte) error {
	var setting models.SystemSetting
	err := tx.Where("key = ?", encryptionCanaryKey).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return writeEncryptionCanary(tx, key, nil)
	}
	if err != nil {
		return fmt.Errorf("чтение контрольной записи шифрования: %w", err)
	}
	return writeEncryptionCanary(tx, key, &setting)
}

// writeEncryptionCanary сохраняет пробу под действующим ключом. Без ключа проба
// ложится открытой - так же, как ложатся сами данные в этом режиме.
func writeEncryptionCanary(db *gorm.DB, key []byte, existing *models.SystemSetting) error {
	value, err := crypto.Encrypt(encryptionCanaryProbe, key)
	if err != nil {
		return fmt.Errorf("шифрование контрольной записи: %w", err)
	}

	if existing != nil {
		existing.Value = value
		if err := db.Save(existing).Error; err != nil {
			return fmt.Errorf("обновление контрольной записи шифрования: %w", err)
		}
		return nil
	}

	setting := models.SystemSetting{
		Key:   encryptionCanaryKey,
		Value: value,
		Type:  "string",
	}
	if err := db.Create(&setting).Error; err != nil {
		return fmt.Errorf("создание контрольной записи шифрования: %w", err)
	}
	return nil
}
