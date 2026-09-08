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
	// Ожидаемый состав: таблица -> нужна ли парная свёртка. Свёртка есть там, где по
	// значению ищут (документы), и её нет у тела письма - по нему не ищут, шифруется
	// оно ради самого хранения (#2377). Различать обязательно: пропавшая свёртка у
	// паспорта ломает поиск молча, а требование свёртки у письма не дало бы внести
	// его в перевод вовсе.
	// Признак нужен по столбцу, а не по таблице: у документов свёртка обязана быть,
	// у «иного разрешения» и тела письма её быть не должно - по ним не ищут.
	wantHMAC := map[string]bool{
		"passport_series_number": true,
		"patent_number":          true,
		"other_permission":       false,
		"body":                   false,
		"email":                  true,
		"phone":                  true,
		"contact_phone":          false,
	}
	want := map[string]bool{
		"employees":             true,
		"unique_employees":      true,
		"application_employees": true,
		"email_messages":        false,
		"users":                 true,
		"applications":          false,
	}
	seen := map[string]bool{}
	for _, table := range encryptedTables {
		_, known := want[table.name]
		require.True(t, known, "таблица %s в перечне лишняя либо переименована", table.name)
		seen[table.name] = true

		require.NotEmpty(t, table.columns, "у таблицы %s не указаны столбцы", table.name)
		for _, col := range table.columns {
			require.NotEmpty(t, col.value)
			need, known := wantHMAC[col.value]
			require.True(t, known, "столбец %s.%s не описан в ожиданиях: решите, нужна ли ему свёртка",
				table.name, col.value)
			if need {
				require.NotEmpty(t, col.hmac, "столбец %s.%s без парного HMAC: поиск сломается после перевода",
					table.name, col.value)
			} else {
				require.Empty(t, col.hmac, "у %s.%s свёртки быть не должно: по нему не ищут",
					table.name, col.value)
			}
		}
	}
	for name := range want {
		require.True(t, seen[name], "таблица %s выпала из перевода", name)
	}
}
