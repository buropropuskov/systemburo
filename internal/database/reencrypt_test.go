package database

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"systemburo/internal/crypto"
)

// Проверяется перевод одного значения - на нём держится вся команда. Поход в базу
// сюда не нужен: таблицы перебираются циклом, а ошибиться можно именно в том, каким
// ключом расшифровали, каким зашифровали и от чего посчитали HMAC.

func testKey(seed byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = seed + byte(i)
	}
	return key
}

// TestReencryptValue_MovesToNewKey - значение переезжает на новый ключ: читается им
// и перестаёт читаться прежним.
func TestReencryptValue_MovesToNewKey(t *testing.T) {
	oldKey, newKey := testKey(1), testKey(200)
	const passport = "4510 123456"

	stored, err := crypto.Encrypt(passport, oldKey)
	require.NoError(t, err)

	value, hmac, err := reencryptValue(stored, oldKey, newKey)
	require.NoError(t, err)

	back, err := crypto.Decrypt(value, newKey)
	require.NoError(t, err)
	require.Equal(t, passport, back, "новым ключом значение должно читаться как исходное")

	_, err = crypto.Decrypt(value, oldKey)
	require.Error(t, err, "прежним ключом значение читаться уже не должно")

	require.Equal(t, crypto.ComputeHMAC(passport, newKey), hmac,
		"HMAC обязан считаться от открытого значения новым ключом: иначе поиск по паспорту перестанет находить")
}

// TestReencryptValue_WrongOldKey - неверный прежний ключ останавливает перевод, а не
// портит значение. Без этого команда прошлась бы по базе, зашифровав шифротекст
// повторно, и данные не открыл бы уже ни один ключ.
func TestReencryptValue_WrongOldKey(t *testing.T) {
	stored, err := crypto.Encrypt("4510 123456", testKey(1))
	require.NoError(t, err)

	_, _, err = reencryptValue(stored, testKey(50), testKey(200))
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrReencryptSourceKey))
}

// TestReencryptValue_FromCleartext - включение шифрования на базе, работавшей без
// него: прежнего ключа нет, значения лежат открытыми.
func TestReencryptValue_FromCleartext(t *testing.T) {
	newKey := testKey(7)
	const patent = "7712 3456789"

	value, hmac, err := reencryptValue(patent, nil, newKey)
	require.NoError(t, err)
	require.NotEqual(t, patent, value, "после перевода значение обязано быть шифротекстом")

	back, err := crypto.Decrypt(value, newKey)
	require.NoError(t, err)
	require.Equal(t, patent, back)
	require.Equal(t, crypto.ComputeHMAC(patent, newKey), hmac)
}

// TestReencryptValue_ToCleartext - снятие шифрования: новый ключ не задан, значение
// возвращается открытым.
func TestReencryptValue_ToCleartext(t *testing.T) {
	oldKey := testKey(3)
	const passport = "4510 987654"

	stored, err := crypto.Encrypt(passport, oldKey)
	require.NoError(t, err)

	value, _, err := reencryptValue(stored, oldKey, nil)
	require.NoError(t, err)
	require.Equal(t, passport, value, "без нового ключа значение остаётся открытым")
}

// TestEncryptedTables_CoverPassportModels - перечень таблиц не должен разъезжаться с
// моделями, где стоят хуки шифрования: пропущенная таблица останется на старом ключе
// молча, и обнаружится это только когда оператор откроет карточку.
func TestEncryptedTables_CoverPassportModels(t *testing.T) {
	want := map[string]bool{
		"employees":             false,
		"unique_employees":      false,
		"application_employees": false,
	}
	for _, table := range encryptedTables {
		_, known := want[table.name]
		require.True(t, known, "таблица %s в перечне лишняя либо переименована", table.name)
		want[table.name] = true

		require.NotEmpty(t, table.columns, "у таблицы %s не указаны столбцы", table.name)
		for _, col := range table.columns {
			require.NotEmpty(t, col.value)
			require.NotEmpty(t, col.hmac, "столбец %s.%s без парного HMAC: поиск сломается после перевода",
				table.name, col.value)
		}
	}
	for name, covered := range want {
		require.True(t, covered, "таблица %s выпала из перевода", name)
	}
}
