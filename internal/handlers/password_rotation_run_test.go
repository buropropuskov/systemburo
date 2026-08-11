package handlers_test

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// rotationEnv - сервис плановой смены с настроенной почтой поверх тестовой базы.
// Почтовый сервис настоящий, но с несуществующим сервером: письма ставятся в
// очередь, отправку проверяет отдельный тест почтового слоя.
func rotationEnv(t *testing.T, db *gorm.DB, rotationDays int) (*services.PasswordRotationService, services.SettingsService) {
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
	// Конструктор сам наполняет кэш настроек из базы, отдельного шага не нужно.
	settings := services.NewSettingsService(db, cfg)
	mail := services.NewMailService(db, cfg)
	notifications := services.NewNotificationService(db)
	svc := services.NewPasswordRotationService(db, settings, mail, notifications, nil, "https://example.org")
	return svc, settings
}

// mkRotationUser заводит работника с нужной датой смены пароля.
func mkRotationUser(t *testing.T, db *gorm.DB, td testutil.TestData, username, email string, changedAt time.Time) models.User {
	t.Helper()
	u := models.User{
		Username: username, Password: "старый-хэш", TypeID: 1,
		OrganizationID: &td.OrgID, CompanyID: &td.CompanyID,
		PasswordChangedAt: &changedAt,
	}
	if email != "" {
		u.Email = &email
	}
	require.NoError(t, db.Create(&u).Error)
	return u
}

// TestRotation_ChangesOnlyExpired: плановый прогон трогает только тех, у кого
// срок вышел. Свежий пароль менять нельзя - человек сменил его сам вчера.
func TestRotation_ChangesOnlyExpired(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db, 90)
	expired := mkRotationUser(t, db, td, "rot_run_expired", "expired@example.org", time.Now().AddDate(0, 0, -200))
	fresh := mkRotationUser(t, db, td, "rot_run_fresh", "fresh@example.org", time.Now().AddDate(0, 0, -3))

	result, err := svc.Run(context.Background(), false, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Changed, "меняем только просроченный пароль")

	var after models.User
	require.NoError(t, db.First(&after, expired.ID).Error)
	assert.NotEqual(t, "старый-хэш", after.Password, "пароль должен смениться")
	require.NotNil(t, after.PasswordRotatedAt)
	assert.True(t, after.MustChangePassword, "по умолчанию требуем сменить пароль при входе")

	var untouched models.User
	require.NoError(t, db.First(&untouched, fresh.ID).Error)
	assert.Equal(t, "старый-хэш", untouched.Password, "свежий пароль трогать нельзя")
}

// TestRotation_QueuesLetterWithPassword: письмо с паролем ставится в очередь той
// же транзакцией. Ради этого очередь и заведена.
func TestRotation_QueuesLetterWithPassword(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db, 90)
	u := mkRotationUser(t, db, td, "rot_run_letter", "letter@example.org", time.Now().AddDate(0, 0, -200))

	_, err := svc.Run(context.Background(), false, 0)
	require.NoError(t, err)

	var letter models.EmailMessage
	require.NoError(t, db.Where("user_id = ? AND template_code = ?", u.ID, services.MailTemplatePasswordRotated).
		First(&letter).Error)
	assert.Equal(t, "letter@example.org", letter.ToAddress)
	assert.Equal(t, models.EmailStatusPending, letter.Status)
	assert.Contains(t, letter.Body, "rot_run_letter", "в письме должен быть логин")
	assert.Contains(t, letter.Body, "Пароль:")
}

// TestRotation_SkipsUsersWithoutEmail: работника без адреса не трогаем - смена
// пароля без канала доставки запирает человека снаружи. Он попадает в отчёт.
func TestRotation_SkipsUsersWithoutEmail(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db, 90)
	noMail := mkRotationUser(t, db, td, "rot_run_nomail", "", time.Now().AddDate(0, 0, -200))

	result, err := svc.Run(context.Background(), false, 0)
	require.NoError(t, err)

	var after models.User
	require.NoError(t, db.First(&after, noMail.ID).Error)
	assert.Equal(t, "старый-хэш", after.Password, "пароль без адреса менять нельзя")
	assert.Contains(t, result.NoMailLogins, "rot_run_nomail")
	assert.GreaterOrEqual(t, result.SkippedNoMail, 1)
}

// TestRotation_SkipsArchivedAndBanned: архивных и заблокированных смена не
// касается - им и входить некуда.
func TestRotation_SkipsArchivedAndBanned(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db, 90)
	archived := mkRotationUser(t, db, td, "rot_run_archived", "arch@example.org", time.Now().AddDate(0, 0, -200))
	banned := mkRotationUser(t, db, td, "rot_run_banned", "ban@example.org", time.Now().AddDate(0, 0, -200))
	// is_active=false и is_banned=true дописываем отдельно: нулевое значение при
	// default:true gorm при вставке пропускает.
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", archived.ID).
		Update("is_active", false).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", banned.ID).
		Update("is_banned", true).Error)

	_, err := svc.Run(context.Background(), false, 0)
	require.NoError(t, err)

	for _, id := range []int{archived.ID, banned.ID} {
		var after models.User
		require.NoError(t, db.First(&after, id).Error)
		assert.Equal(t, "старый-хэш", after.Password)
	}
}

// TestRotation_ManualIgnoresDeadline: ручной прогон меняет пароли всем
// подходящим, не дожидаясь срока.
func TestRotation_ManualIgnoresDeadline(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db, 90)
	fresh := mkRotationUser(t, db, td, "rot_run_manual", "manual@example.org", time.Now().AddDate(0, 0, -1))

	result, err := svc.Run(context.Background(), true, 42)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Changed, 1)
	assert.True(t, result.Manual)
	assert.Equal(t, 42, result.StartedBy)

	var after models.User
	require.NoError(t, db.First(&after, fresh.ID).Error)
	assert.NotEqual(t, "старый-хэш", after.Password, "ручной прогон меняет и свежие пароли")
}

// TestRotation_RefusesWithoutMail: без настроенной почты прогон не начинается.
// Молча пропустить нельзя: администратор будет считать, что пароли меняются.
func TestRotation_RefusesWithoutMail(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	cfg := &config.Config{UploadMaxFileSize: 10485760} // SMTP_HOST пуст
	settings := services.NewSettingsService(db, cfg)
	svc := services.NewPasswordRotationService(db, settings, services.NewMailService(db, cfg),
		services.NewNotificationService(db), nil, "")

	u := mkRotationUser(t, db, td, "rot_run_nomailsrv", "x@example.org", time.Now().AddDate(0, 0, -200))

	_, err := svc.Run(context.Background(), false, 0)
	require.ErrorIs(t, err, services.ErrRotationMailNotConfigured)

	var after models.User
	require.NoError(t, db.First(&after, u.ID).Error)
	assert.Equal(t, "старый-хэш", after.Password, "без почты пароли не трогаются")
}

// TestRotation_RevokesSessions: после смены пароля прежние маркеры продления
// отзываются - иначе старая сессия доживёт до своего срока.
func TestRotation_RevokesSessions(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db, 90)
	u := mkRotationUser(t, db, td, "rot_run_sessions", "sess@example.org", time.Now().AddDate(0, 0, -200))
	require.NoError(t, db.Create(&models.RefreshToken{
		UserID: u.ID, FamilyID: "test-family", TokenHash: "тестовый-хэш-маркера",
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error)

	_, err := svc.Run(context.Background(), false, 0)
	require.NoError(t, err)

	var alive int64
	require.NoError(t, db.Model(&models.RefreshToken{}).
		Where("user_id = ? AND is_revoked = false", u.ID).Count(&alive).Error)
	assert.EqualValues(t, 0, alive, "прежние сессии должны быть оборваны")
}

// TestRotation_CreatesNotification: работник узнаёт о смене и внутри системы, не
// только письмом.
func TestRotation_CreatesNotification(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db, 90)
	u := mkRotationUser(t, db, td, "rot_run_notify", "notify@example.org", time.Now().AddDate(0, 0, -200))

	_, err := svc.Run(context.Background(), false, 0)
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", u.ID, services.NotificationTypePasswordRotated).
		Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

// TestRotation_SecondRunChangesNobody: идемпотентность. После прогона дата смены
// сдвинута, и повторный проход того же дня никого не выбирает.
func TestRotation_SecondRunChangesNobody(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db, 90)
	mkRotationUser(t, db, td, "rot_run_twice", "twice@example.org", time.Now().AddDate(0, 0, -200))

	first, err := svc.Run(context.Background(), false, 0)
	require.NoError(t, err)
	require.Equal(t, 1, first.Changed)

	second, err := svc.Run(context.Background(), false, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, second.Changed, "повторный прогон никого не трогает")
}

// TestRotation_ExpiringWarningWithoutPassword: предупреждение уходит заранее и
// пароля не содержит - письмо может пролежать в ящике неделю.
func TestRotation_ExpiringWarningWithoutPassword(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, settings := rotationEnv(t, db, 90)
	// Включаем смену: предупреждения без неё не имеют смысла.
	_, err := settings.Update(context.Background(), "password.rotation_enabled", "true")
	require.NoError(t, err)
	defer settings.Update(context.Background(), "password.rotation_enabled", "false")

	// Срок 90 суток, предупреждение за 7: пароль, сменённый 85 суток назад,
	// попадает в окно.
	u := mkRotationUser(t, db, td, "rot_warn_user", "warn@example.org", time.Now().AddDate(0, 0, -85))

	svc.NotifyExpiring(context.Background())

	var letter models.EmailMessage
	require.NoError(t, db.Where("user_id = ? AND template_code = ?", u.ID, services.MailTemplatePasswordExpiring).
		First(&letter).Error)
	assert.NotContains(t, letter.Body, "Пароль:", "в предупреждении пароля быть не должно")
	assert.Contains(t, letter.Body, "Сменить пароль")

	// Повторный вызов в тот же день письмо не дублирует.
	svc.NotifyExpiring(context.Background())
	var letters int64
	require.NoError(t, db.Model(&models.EmailMessage{}).
		Where("user_id = ? AND template_code = ?", u.ID, services.MailTemplatePasswordExpiring).
		Count(&letters).Error)
	assert.EqualValues(t, 1, letters, "предупреждение шлётся раз в сутки")
}
