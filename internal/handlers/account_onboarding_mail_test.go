package handlers_test

import (
	"context"
	"strings"
	"testing"

	"systemburo/internal/config"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// onboardingUserService - сервис пользователей с настроенной почтой поверх
// тестовой базы. Почтовый сервис настоящий, но с несуществующим сервером: письма
// ставятся в очередь, отправку проверяет отдельный тест почтового слоя.
//
// withMail=false повторяет стенд без почты: пароль там задаёт администратор.
func onboardingUserService(t *testing.T, db *gorm.DB, withMail bool) services.UserService {
	t.Helper()
	cfg := &config.Config{
		UploadMaxFileSize: 10485760,
		SMTPPort:          2525,
		SMTPFrom:          "bureau@example.org",
		SMTPFromName:      "Бюро пропусков",
		SMTPTLSMode:       "none",
		SMTPTimeoutSec:    2,
		SMTPRatePerHour:   100,
		MailRetryAttempts: 3,
	}
	if withMail {
		cfg.SMTPHost = "127.0.0.1"
	}
	svc := services.NewUserService(db, services.NewNotificationService(db))
	svc.SetPasswordPolicyProvider(services.NewSettingsService(db, cfg))
	svc.SetMailSender(services.NewMailService(db, cfg), "https://example.org")
	return svc
}

// letterPassword достаёт пароль из строки письма «Пароль: ...». Проверять письмо
// на непустоту мало: сравнение с тем, чем работник реально войдёт, - и есть
// смысл этих тестов.
func letterPassword(t *testing.T, body string) string {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if _, after, ok := strings.Cut(line, "Пароль:"); ok {
			return strings.TrimSpace(after)
		}
	}
	t.Fatalf("в письме нет строки с паролем:\n%s", body)
	return ""
}

// userLetters возвращает письма работника, свежие первыми.
func userLetters(t *testing.T, db *gorm.DB, userID int) []models.EmailMessage {
	t.Helper()
	var rows []models.EmailMessage
	require.NoError(t, db.Where("user_id = ?", userID).Order("id DESC").Find(&rows).Error)
	return rows
}

func fetchUser(t *testing.T, db *gorm.DB, username string) models.User {
	t.Helper()
	var u models.User
	require.NoError(t, db.Where("username = ?", username).First(&u).Error)
	return u
}

// TestCreateUser_WithEmail_SendsCredentials: при заведении с адресом почты
// пароль придумывает система и высылает работнику. Проверяем не факт письма, а
// его пригодность: паролем из письма работник должен войти.
func TestCreateUser_WithEmail_SendsCredentials(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := onboardingUserService(t, db, true)
	email := "onboard.mail@example.org"
	name := "Пётр"
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "onboard_with_mail", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID,
		Email: &email, FirstName: &name,
	}))

	created := fetchUser(t, db, "onboard_with_mail")
	assert.True(t, created.MustChangePassword, "придуманный системой пароль работник меняет при первом входе")

	letters := userLetters(t, db, created.ID)
	require.Len(t, letters, 1, "письмо с учётными данными должно быть ровно одно")
	letter := letters[0]
	assert.Equal(t, services.MailTemplateAccountCreated, letter.TemplateCode)
	assert.Equal(t, email, letter.ToAddress)
	assert.Equal(t, models.EmailStatusPending, letter.Status)
	assert.Contains(t, letter.Body, "onboard_with_mail", "в письме должен быть логин")
	assert.Contains(t, letter.Body, "https://example.org", "в письме должен быть адрес системы")
	assert.Contains(t, letter.Body, "При первом входе система попросит задать свой пароль")

	password := letterPassword(t, letter.Body)
	require.NotEmpty(t, password)
	token, _ := testutil.LoginUser(t, e, "onboard_with_mail", password)
	assert.NotEmpty(t, token, "паролем из письма работник должен войти")
}

// TestCreateUser_WithoutEmail_RequiresPassword: без адреса пароль обязан задать
// администратор - придумывать его некому, доставить некуда.
func TestCreateUser_WithoutEmail_RequiresPassword(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := onboardingUserService(t, db, true)
	err := svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "onboard_no_mail_no_pass", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "пароль", "ошибка должна называть причину отказа")

	var count int64
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "onboard_no_mail_no_pass").Count(&count).Error)
	assert.EqualValues(t, 0, count, "отказ не должен оставлять учётную запись")
}

// TestCreateUser_WithoutEmail_ManualPassword: пароль задан руками, адреса нет -
// учётная запись заводится, письма не уходит, сменить пароль работник всё равно
// обязан: придумал его не он.
func TestCreateUser_WithoutEmail_ManualPassword(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := onboardingUserService(t, db, true)
	require.NoError(t, svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "onboard_manual_pass", Password: "manualpass12345", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID,
	}))

	created := fetchUser(t, db, "onboard_manual_pass")
	assert.True(t, created.MustChangePassword)
	assert.Empty(t, userLetters(t, db, created.ID), "без адреса письму уходить некуда")
}

// TestCreateUser_MailNotConfigured_RequiresPassword: адрес есть, а почта не
// настроена - пароль всё равно задаёт администратор. Придуманный системой пароль
// на стенде без почты никто бы не прочитал.
func TestCreateUser_MailNotConfigured_RequiresPassword(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := onboardingUserService(t, db, false)
	email := "no.smtp@example.org"
	err := svc.Create(context.Background(), 0, models.RegisterRequest{
		Username: "onboard_no_smtp", TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID, Email: &email,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Почта не настроена")
}

// TestCreateUser_NoPasswordRejectedByAPI: ручка создания принимает пустой пароль
// (его придумывает система), поэтому отказ без адреса обязан приходить с
// объяснением, а не пустым 400 от валидатора.
func TestCreateUser_NoPasswordRejectedByAPI(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	body := `{"username":"api_no_pass","type_id":1,"organization_id":` + itoa(td.OrgID) + `}`
	rec := testutil.POST(t, e, "/users", body, h)

	require.Equal(t, 400, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "пароль")
}

// TestUsersTable_MustChangePasswordDefaultsFalse: учётная запись, заведённая мимо
// сервиса прямым INSERT, признака обязательной смены не получает.
//
// Так заводится супер-администратор при установке (`cmd/seed`): столбца в его
// INSERT нет, значение берётся из умолчания схемы. Поменяй кто-нибудь умолчание на
// true - и свежепоставленная система встретила бы владельца требованием сменить
// пароль, а помочь ему было бы некому: других учётных записей ещё нет.
func TestUsersTable_MustChangePasswordDefaultsFalse(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	require.NoError(t, db.Exec(
		`INSERT INTO users (username, password, type_id, organization_id, company_id, is_super_admin)
		 VALUES ('seed_like_admin', 'хэш', 1, ?, ?, true)`, td.OrgID, td.CompanyID).Error)

	assert.False(t, fetchUser(t, db, "seed_like_admin").MustChangePassword,
		"учётная запись установки не должна упираться в требование сменить пароль")
}

// TestUpdatePassword_ByAdmin_SendsThatPassword: администратор задал пароль руками -
// работник получает письмом именно его, а не придуманный заново.
func TestUpdatePassword_ByAdmin_SendsThatPassword(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := onboardingUserService(t, db, true)
	email := "reset.target@example.org"
	target := models.User{
		Username: "pwd_letter_target", Password: "старый-хэш", TypeID: 1,
		OrganizationID: &td.OrgID, CompanyID: &td.CompanyID, Email: &email,
	}
	require.NoError(t, db.Create(&target).Error)

	const newPassword = "adminchosen12345"
	require.NoError(t, svc.UpdatePassword(context.Background(), target.ID+1000, target.Username,
		models.UpdatePasswordRequest{Password: newPassword}, nil))

	after := fetchUser(t, db, target.Username)
	assert.True(t, after.MustChangePassword, "заданный администратором пароль работник меняет при первом входе")

	letters := userLetters(t, db, target.ID)
	require.Len(t, letters, 1)
	assert.Equal(t, services.MailTemplatePasswordSetByAdmin, letters[0].TemplateCode)
	assert.Equal(t, email, letters[0].ToAddress)
	assert.Equal(t, newPassword, letterPassword(t, letters[0].Body), "в письме должен быть пароль, который задал администратор")
	assert.Contains(t, letters[0].Body, target.Username)
}

// TestUpdatePassword_ByAdmin_NoEmail_NoLetter: адреса нет - письму уходить
// некуда, но сменить пароль при первом входе работник всё равно обязан.
func TestUpdatePassword_ByAdmin_NoEmail_NoLetter(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := onboardingUserService(t, db, true)
	target := models.User{
		Username: "pwd_letter_nomail", Password: "старый-хэш", TypeID: 1,
		OrganizationID: &td.OrgID, CompanyID: &td.CompanyID,
	}
	require.NoError(t, db.Create(&target).Error)

	require.NoError(t, svc.UpdatePassword(context.Background(), target.ID+1000, target.Username,
		models.UpdatePasswordRequest{Password: "adminchosen12345"}, nil))

	after := fetchUser(t, db, target.Username)
	assert.True(t, after.MustChangePassword)
	assert.Empty(t, userLetters(t, db, target.ID))
}

// TestUpdatePassword_SelfChange_NoLetterNoFlag: свою смену это не касается -
// человек задал пароль сам, требовать сменить его снова незачем, и слать его
// себе же письмом тоже.
func TestUpdatePassword_SelfChange_NoLetterNoFlag(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := onboardingUserService(t, db, true)
	email := "self.change@example.org"
	target := models.User{
		Username: "pwd_letter_self", Password: "старый-хэш", TypeID: 1,
		OrganizationID: &td.OrgID, CompanyID: &td.CompanyID, Email: &email,
		MustChangePassword: true,
	}
	require.NoError(t, db.Create(&target).Error)

	require.NoError(t, svc.UpdatePassword(context.Background(), target.ID, target.Username,
		models.UpdatePasswordRequest{Password: "selfchosen12345"}, nil))

	after := fetchUser(t, db, target.Username)
	assert.False(t, after.MustChangePassword, "требование должно сниматься сменой пароля самим работником")
	assert.Empty(t, userLetters(t, db, target.ID))
}
