package database

import (
	"testing"

	"github.com/stretchr/testify/require"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
)

// Проверяется решение по одному значению - на нём держится идемпотентность прохода.
// Поход в базу сюда не нужен: строки перебираются циклом, а ошибиться можно в том,
// считать ли значение открытым и от чего посчитать свёртку.

// TestEncryptIfPlaintext_EncryptsCleartext - открытое значение переводится в
// шифротекст, свёртка считается по правилу столбца.
func TestEncryptIfPlaintext_EncryptsCleartext(t *testing.T) {
	key := testKey(11)
	const email = "Ivanov@Example.COM"

	value, hmac, plaintext, err := encryptIfPlaintext(email, key, models.NormalizeEmailForHMAC)
	require.NoError(t, err)
	require.True(t, plaintext, "значение лежало открытым и должно быть переведено")
	require.NotEqual(t, email, value)

	back, err := crypto.Decrypt(value, key)
	require.NoError(t, err)
	require.Equal(t, email, back)

	require.Equal(t, crypto.ComputeHMAC(models.NormalizeEmailForHMAC(email), key), hmac,
		"свёртка обязана считаться от нормализованного значения: иначе поиск по почте не найдёт запись")
}

// TestEncryptIfPlaintext_SkipsEncrypted - уже зашифрованное значение не трогается.
// Без этой развилки проход зашифровал бы шифротекст повторно, и запись перестала бы
// читаться действующим ключом.
func TestEncryptIfPlaintext_SkipsEncrypted(t *testing.T) {
	key := testKey(23)
	stored, err := crypto.Encrypt("+7 (916) 123-45-67", key)
	require.NoError(t, err)

	_, _, plaintext, err := encryptIfPlaintext(stored, key, models.NormalizePhoneForHMAC)
	require.NoError(t, err)
	require.False(t, plaintext, "значение уже зашифровано, второй раз шифровать нельзя")
}

// TestEncryptIfPlaintext_PhoneWritingIrrelevant - телефон, записанный по-разному,
// даёт одну свёртку. Ради этого нормализация и живёт в перечне столбцов: иначе
// человек не находился бы по собственному номеру, набранному в другом виде.
func TestEncryptIfPlaintext_PhoneWritingIrrelevant(t *testing.T) {
	key := testKey(31)

	_, first, _, err := encryptIfPlaintext("+7 (916) 123-45-67", key, models.NormalizePhoneForHMAC)
	require.NoError(t, err)
	_, second, _, err := encryptIfPlaintext("89161234567", key, models.NormalizePhoneForHMAC)
	require.NoError(t, err)

	require.Equal(t, first, second)
}

// TestEncryptedTables_ContactColumnsNormalize - у контактов правило нормализации
// обязано быть задано. Пропуск здесь не виден ни сборке, ни расшифровке: значение
// переведётся правильно, а свёртка посчитается от сырого написания, и поиск по
// почте и телефону молча перестанет находить записи.
func TestEncryptedTables_ContactColumnsNormalize(t *testing.T) {
	want := map[string]bool{
		"users.email":                       true,
		"users.phone":                       true,
		"employees.passport_series_number":  false,
		"applications.contact_phone":        false,
		"unique_employees.other_permission": false,
	}

	for _, table := range encryptedTables {
		for _, col := range table.columns {
			need, listed := want[table.name+"."+col.value]
			if !listed {
				continue
			}
			if need {
				require.NotNil(t, col.normalize, "%s.%s: свёртка должна считаться от нормализованного значения", table.name, col.value)
				continue
			}
			require.Nil(t, col.normalize, "%s.%s: нормализация здесь не предусмотрена", table.name, col.value)
		}
	}
}

// TestEncryptIfPlaintext_SkipsForeignCiphertext - значение, зашифрованное прежним
// ключом, остаётся нетронутым. Приняв его за открытое, проход зашифровал бы
// шифротекст повторно, и запись не открыл бы уже ни один ключ.
func TestEncryptIfPlaintext_SkipsForeignCiphertext(t *testing.T) {
	stored, err := crypto.Encrypt("4510 123456", testKey(41))
	require.NoError(t, err)

	_, _, plaintext, err := encryptIfPlaintext(stored, testKey(97), nil)
	require.NoError(t, err)
	require.False(t, plaintext, "чужой шифротекст шифровать повторно нельзя")
}

// TestLooksEncrypted_RealValuesAreCleartext - страховка от чужого шифротекста не
// должна принимать за него настоящие данные, иначе они так и останутся открытыми.
func TestLooksEncrypted_RealValuesAreCleartext(t *testing.T) {
	cleartext := []string{
		"ivanov@example.com",
		"+7 (916) 123-45-67",
		"89161234567",
		"4510 123456",
		"7712 3456789",
		"Разрешение на временное проживание",
		"Иванов Иван Иванович",
	}
	for _, v := range cleartext {
		require.False(t, looksEncrypted(v), "значение %q обязано считаться открытым", v)
	}

	stored, err := crypto.Encrypt("4510 123456", testKey(5))
	require.NoError(t, err)
	require.True(t, looksEncrypted(stored))
}
