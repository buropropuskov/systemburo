package crypto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDecryptOptional_CountsFailures - неудачная расшифровка попадает в счётчик, а
// значение возвращается как есть. Раньше она не оставляла никакого следа, и
// проблема с ключом доходила до оператора набором символов вместо паспорта.
func TestDecryptOptional_CountsFailures(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	other := make([]byte, 32)
	for i := range other {
		other[i] = byte(200 - i)
	}

	stored, err := Encrypt("4510 123456", key)
	require.NoError(t, err)

	SetGlobalKey(other)
	defer SetGlobalKey(nil)
	ResetDecryptFailures()

	got := DecryptOptional(&stored)
	require.NotNil(t, got)
	require.Equal(t, stored, *got, "нечитаемое значение возвращается как есть")
	require.EqualValues(t, 1, DecryptFailures(), "неудача обязана попасть в счётчик")

	DecryptOptional(&stored)
	require.EqualValues(t, 2, DecryptFailures(), "счётчик считает каждую неудачу, а не только первую")
}

// TestDecryptOptional_SuccessLeavesCounterAlone - удачная расшифровка счётчик не
// трогает, иначе сводка в журнале превратится в шум на исправной установке.
func TestDecryptOptional_SuccessLeavesCounterAlone(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	const passport = "4510 987654"

	stored, err := Encrypt(passport, key)
	require.NoError(t, err)

	SetGlobalKey(key)
	defer SetGlobalKey(nil)
	ResetDecryptFailures()

	got := DecryptOptional(&stored)
	require.NotNil(t, got)
	require.Equal(t, passport, *got)
	require.EqualValues(t, 0, DecryptFailures())
}

// TestDecryptOptional_PassthroughIsNotFailure - без ключа расшифровки нет вовсе, и
// это штатный режим: счётчик обязан молчать, иначе установка без шифрования будет
// писать предупреждение на каждое чтение.
func TestDecryptOptional_PassthroughIsNotFailure(t *testing.T) {
	SetGlobalKey(nil)
	ResetDecryptFailures()

	value := "4510 111222"
	got := DecryptOptional(&value)
	require.NotNil(t, got)
	require.Equal(t, value, *got)
	require.EqualValues(t, 0, DecryptFailures())
}
