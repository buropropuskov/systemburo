package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Барьер против пакета открытым текстом. Один каталог с паспортами всей организации -
// это ровно то, что нельзя записать «случайно», поэтому отказ по умолчанию, а согласие
// оператора выражается флагом.
func TestExportEncryptionGate(t *testing.T) {
	require.NoError(t, exportEncryptionGate(true, false), "ключи заданы - выгрузка идёт молча")
	require.NoError(t, exportEncryptionGate(true, true), "флаг при заданных ключах ничего не ломает")
	require.NoError(t, exportEncryptionGate(false, true), "оператор согласился на открытый пакет явно")

	err := exportEncryptionGate(false, false)
	require.Error(t, err, "без ключей и без согласия пакет писать нельзя")
	assert.Contains(t, err.Error(), "ARCHIVE_AGE_RECIPIENT", "отказ обязан называть, что задать")
	assert.Contains(t, err.Error(), "-plaintext", "и как поступить, если открытый пакет здесь уместен")
}
