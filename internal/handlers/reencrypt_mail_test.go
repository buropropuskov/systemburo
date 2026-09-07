package handlers_test

import (
	"testing"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Смена ключа шифрования переводит и письма в очереди (#2377).
//
// Без этого письма, не ушедшие адресату к моменту смены, оставались бы на прежнем
// ключе: сработала бы мягкая деградация, письмо не отправилось бы вовсе, а причину
// искали бы в почтовом сервере, а не в смене ключа неделей раньше.
func TestReencrypt_MovesQueuedMailToNewKey(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	// Тестовое приложение поднимается без ключа, тело лежит открытым - значит
	// проверяем включение шифрования: прежний ключ nil, новый задан. Механизм тот
	// же самый, что и при смене ключа, отличается только источник.
	oldKey := crypto.GetGlobalKey()

	const secret = "Пароль для входа: Qwerty-2026-Xy"
	msg := models.EmailMessage{
		ToAddress:    "worker@example.org",
		TemplateCode: "password_rotation",
		Subject:      "Новый пароль",
		Body:         secret,
		Status:       models.EmailStatusPending,
		CreatedAt:    time.Now().UTC(),
	}
	require.NoError(t, db.Create(&msg).Error)

	var stored string
	require.NoError(t, db.Raw("SELECT body FROM email_messages WHERE id = ?", msg.ID).Row().Scan(&stored))

	newKey := make([]byte, 32)
	for i := range newKey {
		newKey[i] = byte(i + 7)
	}

	results, err := database.Reencrypt(t.Context(), db, database.ReencryptOptions{
		OldKey: oldKey,
		NewKey: newKey,
		Apply:  true,
	})
	require.NoError(t, err)

	var mailResult *database.ReencryptResult
	for i := range results {
		if results[i].Table == "email_messages" {
			mailResult = &results[i]
		}
	}
	require.NotNil(t, mailResult, "письма попали в перевод")
	assert.EqualValues(t, 1, mailResult.Values, "переведено одно тело")

	// Новым ключом читается, прежним - нет.
	require.NoError(t, db.Raw("SELECT body FROM email_messages WHERE id = ?", msg.ID).Row().Scan(&stored))
	decoded, err := crypto.Decrypt(stored, newKey)
	require.NoError(t, err, "после перевода тело читается новым ключом")
	assert.Equal(t, secret, decoded)

	assert.NotContains(t, stored, "Qwerty", "после перевода пароль в базе не читается глазами")
}
