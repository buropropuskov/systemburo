package crypto_test

import (
	"testing"

	"systemburo/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- EmailMessage (#2351) ----------
//
// Тело письма плановой смены пароля содержит сам пароль. testKey32/setKey живут в
// encryption_integration_test.go этого же пакета.

func TestEmailMessage_BeforeSave_EncryptsBody(t *testing.T) {
	setKey(t, testKey32())

	password := "секретный-пароль-1234"
	m := &models.EmailMessage{
		Body: "Здравствуйте.\n\n  Логин:  ivanov\n  Пароль: " + password + "\n",
	}

	require.NoError(t, m.BeforeSave(nil))

	assert.NotContains(t, m.Body, password, "пароль не должен читаться в сохраняемом теле")
}

func TestEmailMessage_AfterFind_DecryptsBody(t *testing.T) {
	setKey(t, testKey32())

	original := "Пароль: секрет-9999"
	m := &models.EmailMessage{Body: original}

	require.NoError(t, m.BeforeSave(nil))
	require.NotEqual(t, original, m.Body, "тело должно стать шифротекстом после BeforeSave")

	require.NoError(t, m.AfterFind(nil))
	assert.Equal(t, original, m.Body, "AfterFind должен вернуть исходный текст письма")
}

func TestEmailMessage_RoundTrip(t *testing.T) {
	setKey(t, testKey32())

	tests := []struct {
		name string
		body string
	}{
		{"обычный пароль", "Логин: ivanov\nПароль: Aa1!защита"},
		{"юникод", "Пароль: пропуск-777"},
		{"длинный текст", "письмо " + string(make([]byte, 2000))},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &models.EmailMessage{Body: tc.body}
			require.NoError(t, m.BeforeSave(nil))
			require.NoError(t, m.AfterFind(nil))
			assert.Equal(t, tc.body, m.Body)
		})
	}
}

// TestEmailMessage_BeforeSave_SkipsEmptyBody защищает стирание тела (mail_service.go
// markSent/markFailure): пустая строка после отправки не должна превращаться в
// непустой шифротекст пустого значения - тогда "стёрто" перестало бы значить "пусто".
func TestEmailMessage_BeforeSave_SkipsEmptyBody(t *testing.T) {
	setKey(t, testKey32())

	m := &models.EmailMessage{Body: ""}
	require.NoError(t, m.BeforeSave(nil))
	assert.Equal(t, "", m.Body)
}

// TestEmailMessage_AfterFind_SkipsEmptyBody: пустое тело у отправленного или
// окончательно недоставленного письма - не ошибка ключа, decrypt не запускается.
func TestEmailMessage_AfterFind_SkipsEmptyBody(t *testing.T) {
	setKey(t, testKey32())

	m := &models.EmailMessage{Body: ""}
	require.NoError(t, m.AfterFind(nil))
	assert.Equal(t, "", m.Body)
}

// TestEmailMessage_AfterFind_TeratesUndecryptableBody: письмо, поставленное в очередь
// до включения шифрования (или после смены ключа без перевода), не должно ронять
// выборку - воркер разбирает письма пачкой (mail_service.go claimBatch), и одна
// нечитаемая запись не обязана останавливать остальные.
func TestEmailMessage_AfterFind_ToleratesUndecryptableBody(t *testing.T) {
	setKey(t, testKey32())

	legacyPlainBody := "Пароль: старое-письмо-до-шифрования"
	m := &models.EmailMessage{Body: legacyPlainBody}

	err := m.AfterFind(nil)
	require.NoError(t, err, "нечитаемое тело не должно возвращать ошибку из хука")
	assert.Equal(t, legacyPlainBody, m.Body, "нерасшифрованное значение возвращается как есть")
}
