package handlers_test

import (
	"testing"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Текст письма нужен ровно до отправки. Дальше он остаётся копией уже ушедшего,
// и для писем о пароле это пароль открытым текстом - в базе, которая по всем
// прочим правилам хранит пароли только вычислением Argon2id. Очередь ничего не
// удаляет по сроку, поэтому такая копия жила бы вечно.
//
// Замечено при разборе стенда: письмо о заведении учётной записи лежало в
// очереди с паролем в теле спустя сутки после отправки.

// TestClearDeliveredMailBodies_ClearsTerminalOnly: разовая очистка снимает текст
// у отправленных и окончательно не доставленных, но не трогает ожидающих очереди -
// им текст ещё предстоит отправить.
func TestClearDeliveredMailBodies_ClearsTerminalOnly(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	const secret = "Пароль: icLvVdVWKu9S"

	rows := []models.EmailMessage{
		{ToAddress: "sent@example.org", Subject: "тема", Body: secret, Status: models.EmailStatusSent},
		{ToAddress: "failed@example.org", Subject: "тема", Body: secret, Status: models.EmailStatusFailed},
		{ToAddress: "pending@example.org", Subject: "тема", Body: secret, Status: models.EmailStatusPending},
	}
	for i := range rows {
		require.NoError(t, db.Create(&rows[i]).Error)
	}

	require.NoError(t, database.ClearDeliveredMailBodies(db))

	// Каждое чтение в свою переменную: gorm подмешивает первичный ключ из уже
	// заполненной структуры, и повторное чтение искало бы сразу два разных id.
	var sent, failed, pending models.EmailMessage
	require.NoError(t, db.First(&sent, rows[0].ID).Error)
	assert.Empty(t, sent.Body, "у отправленного письма текст обязан быть стёрт")
	assert.Equal(t, "sent@example.org", sent.ToAddress, "остальное о письме сохраняется")
	assert.Equal(t, "тема", sent.Subject)

	require.NoError(t, db.First(&failed, rows[1].ID).Error)
	assert.Empty(t, failed.Body, "у недоставленного повторов больше не будет - текст не нужен")

	require.NoError(t, db.First(&pending, rows[2].ID).Error)
	assert.Equal(t, secret, pending.Body, "ожидающему отправки текст ещё нужен")
}

// TestMailQueue_ClearsBodyAfterDelivery: отправитель стирает текст сам, как только
// письмо ушло. До сервера при этом уходит полный текст - стирается копия в базе,
// а не то, что читает работник.
func TestMailQueue_ClearsBodyAfterDelivery(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	srv := startFakeSMTP(t)
	svc := services.NewMailService(db, mailCfg(srv.port(), 5))

	require.NoError(t, svc.Enqueue(t.Context(), nil, services.MailMessage{
		To:           "cleared@example.org",
		Subject:      "Учётная запись в системе бюро пропусков",
		Body:         "Логин: probe\nПароль: icLvVdVWKu9S",
		TemplateCode: "account_created",
	}))

	sent, failed := svc.ProcessQueue(t.Context())
	require.Equal(t, 1, sent)
	require.Equal(t, 0, failed)

	require.Len(t, srv.messages(), 1)
	assert.Contains(t, srv.messages()[0], "cleared@example.org",
		"адресату уходит полное письмо: стирается копия в базе, а не отправляемое")

	var row models.EmailMessage
	require.NoError(t, db.Where("to_address = ?", "cleared@example.org").First(&row).Error)
	assert.Empty(t, row.Body, "после отправки текст письма в базе хранить нельзя")
	assert.NotEmpty(t, row.Subject, "остальное о письме сохраняется для разбора доставки")
	require.NotNil(t, row.SentAt)
}

// TestMailQueue_ClearsBodyWhenAttemptsExhausted: у письма, которое так и не ушло,
// повторов больше не будет - текст не нужен и хранить его нельзя.
func TestMailQueue_ClearsBodyWhenAttemptsExhausted(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	srv := startFakeSMTP(t)
	srv.setRejectTo(true)
	svc := services.NewMailService(db, mailCfg(srv.port(), 1))

	require.NoError(t, svc.Enqueue(t.Context(), nil, services.MailMessage{
		To:           "exhausted@example.org",
		Subject:      "Новый пароль в системе бюро пропусков",
		Body:         "Пароль: icLvVdVWKu9S",
		TemplateCode: "password_set_by_admin",
	}))

	_, failed := svc.ProcessQueue(t.Context())
	require.Equal(t, 1, failed)

	var row models.EmailMessage
	require.NoError(t, db.Where("to_address = ?", "exhausted@example.org").First(&row).Error)
	require.Equal(t, models.EmailStatusFailed, row.Status)
	assert.Empty(t, row.Body, "повторов не будет - текст хранить нельзя")
	assert.NotEmpty(t, row.LastError, "причина отказа обязана сохраниться: по ней и разбирают")
}

// TestClearDeliveredMailBodies_Idempotent: проход идёт при каждом запуске сервера
// и на второй раз не должен ни падать, ни трогать лишнего.
func TestClearDeliveredMailBodies_Idempotent(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	row := models.EmailMessage{
		ToAddress: "twice@example.org", Subject: "тема",
		Body: "Пароль: secret123", Status: models.EmailStatusSent,
	}
	require.NoError(t, db.Create(&row).Error)

	require.NoError(t, database.ClearDeliveredMailBodies(db))
	require.NoError(t, database.ClearDeliveredMailBodies(db))

	var after models.EmailMessage
	require.NoError(t, db.First(&after, row.ID).Error)
	assert.Empty(t, after.Body)
}
