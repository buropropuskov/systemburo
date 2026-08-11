package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/config"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// userSvcWithMail - сервис пользователей с настроенной почтой. Почтовый сервер
// недостижим намеренно: письма проверяются в очереди, отправку проверяет тест
// почтового слоя.
func userSvcWithMail(t *testing.T, db *gorm.DB) services.UserService {
	t.Helper()
	cfg := &config.Config{
		UploadMaxFileSize: 10485760,
		SMTPHost:          "127.0.0.1",
		SMTPPort:          2525,
		SMTPFrom:          "bureau@example.org",
		SMTPFromName:      "Бюро пропусков",
		SMTPTLSMode:       "none",
		SMTPTimeoutSec:    2,
		SMTPRatePerHour:   100,
		MailRetryAttempts: 3,
	}
	svc := services.NewUserService(db, services.NewNotificationService(db))
	svc.SetPasswordPolicyProvider(services.NewSettingsService(db, cfg))
	svc.SetMailSender(services.NewMailService(db, cfg))
	svc.SetPublicBaseURL("https://example.org")
	return svc
}

func letterFor(t *testing.T, db *gorm.DB, userID int, template string) models.EmailMessage {
	t.Helper()
	var letter models.EmailMessage
	require.NoError(t, db.Where("user_id = ? AND template_code = ?", userID, template).
		First(&letter).Error, "письмо %s не поставлено в очередь", template)
	return letter
}

// TestCreateUser_WithEmail_GeneratesPasswordAndSendsLetter: при заведении учётной
// записи с адресом почты пароль придумывает система и отправляет письмом.
// Администратору не нужно ничего придумывать и диктовать.
func TestCreateUser_WithEmail_GeneratesPasswordAndSendsLetter(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := userSvcWithMail(t, db)
	email := "newcomer@example.org"
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "acct_with_mail", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID, Email: &email,
	}))

	var created models.User
	require.NoError(t, db.Where("username = ?", "acct_with_mail").First(&created).Error)
	assert.NotEmpty(t, created.Password, "пароль должен быть придуман системой")
	assert.True(t, created.MustChangePassword, "пароль, который работник не выбирал, годится на один вход")

	letter := letterFor(t, db, created.ID, services.MailTemplateAccountCreated)
	assert.Equal(t, email, letter.ToAddress)
	assert.Contains(t, letter.Body, "acct_with_mail", "в письме должен быть логин")
	assert.Contains(t, letter.Body, "Пароль:")
	assert.Contains(t, letter.Body, "https://example.org")
}

// TestCreateUser_WithoutEmail_RequiresManualPassword: без адреса почты отправить
// пароль некуда, поэтому его задаёт администратор. Пустой пароль отклоняется с
// объяснением, а не молча.
func TestCreateUser_WithoutEmail_RequiresManualPassword(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := userSvcWithMail(t, db)

	err := svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "acct_no_mail_nopass", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "адрес почты")

	// С паролем от администратора - проходит, письма нет, смена при входе нужна.
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "acct_no_mail", Password: "manualpass12345", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID,
	}))

	var created models.User
	require.NoError(t, db.Where("username = ?", "acct_no_mail").First(&created).Error)
	assert.True(t, created.MustChangePassword)

	var letters int64
	require.NoError(t, db.Model(&models.EmailMessage{}).
		Where("user_id = ?", created.ID).Count(&letters).Error)
	assert.EqualValues(t, 0, letters, "без адреса письму уйти некуда")
}

// TestUpdatePassword_ByAdmin_SendsLetter: пароль, заданный администратором,
// работник получает письмом - иначе его пришлось бы диктовать по телефону.
func TestUpdatePassword_ByAdmin_SendsLetter(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := userSvcWithMail(t, db)
	email := "worker@example.org"
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "acct_admin_set", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID, Email: &email,
	}))
	var user models.User
	require.NoError(t, db.Where("username = ?", "acct_admin_set").First(&user).Error)

	const adminID = 999
	require.NoError(t, svc.UpdatePassword(context.Background(), adminID, user.Username,
		models.UpdatePasswordRequest{Password: "adminchosen12345"}, nil))

	letter := letterFor(t, db, user.ID, services.MailTemplatePasswordSetByAdmin)
	assert.Contains(t, letter.Body, "adminchosen12345", "в письме должен быть заданный пароль")
	assert.Contains(t, letter.Body, "Прежний пароль больше не действует")

	var after models.User
	require.NoError(t, db.First(&after, user.ID).Error)
	assert.True(t, after.MustChangePassword, "пароль от администратора годится на один вход")
}

// TestUpdatePassword_AdminForSelf_NoLetterNoFlag: администратор, меняющий пароль
// себе, письма себе не шлёт и смены при входе не требует - пароль он и придумал.
func TestUpdatePassword_AdminForSelf_NoLetterNoFlag(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := userSvcWithMail(t, db)
	email := "self@example.org"
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "acct_self", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID, Email: &email,
	}))
	var user models.User
	require.NoError(t, db.Where("username = ?", "acct_self").First(&user).Error)

	require.NoError(t, svc.UpdatePassword(context.Background(), user.ID, user.Username,
		models.UpdatePasswordRequest{Password: "ownchosen12345"}, nil))

	var after models.User
	require.NoError(t, db.First(&after, user.ID).Error)
	assert.False(t, after.MustChangePassword, "свой пароль менять повторно незачем")

	var letters int64
	require.NoError(t, db.Model(&models.EmailMessage{}).
		Where("user_id = ? AND template_code = ?", user.ID, services.MailTemplatePasswordSetByAdmin).
		Count(&letters).Error)
	assert.EqualValues(t, 0, letters, "самому себе пароль письмом не пересылают")
}

// TestCreateUser_LetterFormat: письмо читают в почтовом клиенте без разметки,
// поэтому логин и пароль вынесены в отдельный блок - в сплошном тексте они теряются.
func TestCreateUser_LetterFormat(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := userSvcWithMail(t, db)
	email := "format@example.org"
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "acct_format", TypeID: 1, LastName: nil,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID, Email: &email,
	}))
	var created models.User
	require.NoError(t, db.Where("username = ?", "acct_format").First(&created).Error)

	body := letterFor(t, db, created.ID, services.MailTemplateAccountCreated).Body
	assert.Contains(t, body, "  Логин:", "поля выровнены отступом, а не слиты с текстом")
	assert.Contains(t, body, "----", "блок с данными отделён линейкой")
	assert.Contains(t, body, "Отвечать на это письмо не нужно")
	assert.NotContains(t, body, "<", "письмо простым текстом, без разметки")
}
