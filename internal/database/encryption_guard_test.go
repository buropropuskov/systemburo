package database

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"systemburo/internal/crypto"
)

// Проверяется решение по контрольной записи, а не поход в базу: строка одна на всю
// базу, а тесты пакета её делят. Запись и чтение самой строки закрыты сборкой и
// проверкой запуска, здесь важна развилка "пускать или остановиться".

// encryptProbe шифрует пробу заданным ключом - так же, как её кладёт сама сверка.
func encryptProbe(t *testing.T, key []byte) string {
	t.Helper()
	v, err := crypto.Encrypt(encryptionCanaryProbe, key)
	require.NoError(t, err)
	return v
}

func TestDecideCanary(t *testing.T) {
	keyA := make([]byte, 32)
	for i := range keyA {
		keyA[i] = byte(i)
	}
	keyB := make([]byte, 32)
	for i := range keyB {
		keyB[i] = byte(255 - i)
	}

	tests := []struct {
		name    string
		stored  string
		exists  bool
		key     []byte
		want    canaryVerdict
		wantErr bool
	}{
		{
			name:   "первый запуск с ключом - запись заводится",
			exists: false,
			key:    keyA,
			want:   canaryWrite,
		},
		{
			name:   "первый запуск без ключа - запись заводится открытой",
			exists: false,
			key:    nil,
			want:   canaryWrite,
		},
		{
			name:   "тот же ключ - запуск проходит",
			stored: encryptProbe(t, keyA),
			exists: true,
			key:    keyA,
			want:   canaryOK,
		},
		{
			name:   "работа без ключа продолжается",
			stored: encryptionCanaryProbe,
			exists: true,
			key:    nil,
			want:   canaryOK,
		},
		{
			name:   "шифрование включили на открытой базе - запись обновляется",
			stored: encryptionCanaryProbe,
			exists: true,
			key:    keyA,
			want:   canaryWrite,
		},
		{
			name:    "ключ подменили - запуск останавливается",
			stored:  encryptProbe(t, keyA),
			exists:  true,
			key:     keyB,
			wantErr: true,
		},
		{
			name:    "ключ убрали с зашифрованной базы - запуск останавливается",
			stored:  encryptProbe(t, keyA),
			exists:  true,
			key:     nil,
			wantErr: true,
		},
		{
			name:    "значение испорчено - запуск останавливается",
			stored:  "не шифротекст и не проба",
			exists:  true,
			key:     keyA,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decideCanary(tt.stored, tt.exists, tt.key)

			if tt.wantErr {
				require.Error(t, err)
				require.True(t, errors.Is(err, ErrEncryptionKeyMismatch),
					"отказ должен опознаваться как несовпадение ключа, иначе вызывающий не отличит его от сбоя базы")
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestDecideCanary_SameKeyDifferentCiphertext - шифротекст пробы каждый раз новый
// (случайный nonce), поэтому сверять значения строкой нельзя, только расшифровкой.
// Без этого замка сравнение "в лоб" прошло бы тесты выше и роняло бы запуск на
// втором старте с тем же ключом.
func TestDecideCanary_SameKeyDifferentCiphertext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	first := encryptProbe(t, key)
	second := encryptProbe(t, key)
	require.NotEqual(t, first, second, "шифротекст пробы обязан отличаться между вызовами")

	got, err := decideCanary(first, true, key)
	require.NoError(t, err)
	require.Equal(t, canaryOK, got)
}
