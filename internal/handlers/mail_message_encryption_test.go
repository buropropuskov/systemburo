package handlers_test

import (
	"encoding/hex"
	"testing"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testMailEncryptionKey - детерминированный ключ AES-256 для тестов очереди писем.
// setupTestApp сбрасывает глобальный ключ на nil (пропускной режим) при каждом
// вызове testutil.SetupTestApp, поэтому реальный ключ выставляется уже после него
// и всегда возвращается в nil через t.Cleanup - иначе он утёк бы в соседние тесты
// пакета.
func testMailEncryptionKey() []byte {
	key, _ := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	return key
}

func withMailEncryptionKey(t *testing.T) {
	t.Helper()
	crypto.SetGlobalKey(testMailEncryptionKey())
	t.Cleanup(func() { crypto.SetGlobalKey(nil) })
}

// TestMailQueue_BodyStoredEncrypted - пароль плановой смены не должен читаться в
// дампе базы (#2351): постановка в очередь шифрует тело, в колонке лежит шифротекст.
func TestMailQueue_BodyStoredEncrypted(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	withMailEncryptionKey(t)

	svc := services.NewMailService(db, mailCfg(0, 5))
	const password = "секретный-пароль-42"
	require.NoError(t, svc.Enqueue(t.Context(), nil, services.MailMessage{
		To: "encrypted@example.org", Subject: "Новый пароль", Body: "Пароль: " + password,
		TemplateCode: "password_rotated",
	}))

	var stored string
	require.NoError(t, db.Raw(`SELECT body FROM email_messages WHERE to_address = ?`, "encrypted@example.org").
		Scan(&stored).Error)
	assert.NotContains(t, stored, password, "пароль не должен читаться в столбце body напрямую")

	decrypted, err := crypto.Decrypt(stored, testMailEncryptionKey())
	require.NoError(t, err)
	assert.Contains(t, decrypted, password, "под ключом тело должно расшифровываться в исходный текст")
}

// TestMailQueue_RetryReDecryptsBodyAndDelivers - повторная отправка после отказа
// доставляет настоящий пароль (#2351): тело остаётся зашифрованным между попытками,
// а не пропадает и не портится.
func TestMailQueue_RetryReDecryptsBodyAndDelivers(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	withMailEncryptionKey(t)

	srv := startFakeSMTP(t)
	srv.setRejectTo(true)
	svc := services.NewMailService(db, mailCfg(srv.port(), 5))

	// Пароль латиницей и цифрами - как настоящий сгенерированный (password_generator.go).
	// Кириллица в теле письма уходит на проводе quoted-printable и режется мягкими
	// переносами строки, так что искать подстроку в сыром SMTP-тексте можно только
	// по ASCII-содержимому - иначе тест ловит артефакт кодировки, а не свой баг.
	const password = "Retry777Xk"
	require.NoError(t, svc.Enqueue(t.Context(), nil, services.MailMessage{
		To: "retry-encrypted@example.org", Subject: "Новый пароль", Body: "Пароль: " + password,
		TemplateCode: "password_rotated",
	}))

	sent, failed := svc.ProcessQueue(t.Context())
	assert.Equal(t, 0, sent)
	assert.Equal(t, 1, failed)

	var row models.EmailMessage
	require.NoError(t, db.Where("to_address = ?", "retry-encrypted@example.org").First(&row).Error)
	require.Equal(t, models.EmailStatusPending, row.Status)

	var storedBody string
	require.NoError(t, db.Raw(`SELECT body FROM email_messages WHERE id = ?`, row.ID).Scan(&storedBody).Error)
	assert.NotEmpty(t, storedBody, "тело должно остаться доступным для повтора")
	assert.NotContains(t, storedBody, password, "между попытками тело хранится шифротекстом")

	// Срок повтора настал, сервер больше не отказывает.
	require.NoError(t, db.Model(&models.EmailMessage{}).Where("id = ?", row.ID).
		Update("next_attempt_at", time.Now().Add(-time.Minute)).Error)
	srv.setRejectTo(false)

	sent, failed = svc.ProcessQueue(t.Context())
	assert.Equal(t, 1, sent)
	assert.Equal(t, 0, failed)

	msgs := srv.messages()
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], password, "повтор должен доставить настоящий пароль, расшифровав тело")

	require.NoError(t, db.Where("id = ?", row.ID).First(&row).Error)
	assert.Equal(t, models.EmailStatusSent, row.Status)
	assert.Empty(t, row.Body, "после доставки тело стирается")
}
