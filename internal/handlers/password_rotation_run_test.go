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

// rotationEnv - сервис работы с паролями по сроку поверх тестовой базы, с
// настроенной почтой. Почтовый сервис настоящий, но с несуществующим сервером:
// письма ставятся в очередь, отправку проверяет отдельный тест почтового слоя.
func rotationEnv(t *testing.T, db *gorm.DB) (*services.PasswordRotationService, services.SettingsService) {
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

// TestMarkExpired_MarksOnlyExpired: плановый прогон трогает только тех, у кого
// срок вышел, и трогает единственным способом - поднимает признак обязательной
// смены. Сам пароль остаётся прежним: человек войдёт им и задаст новый.
func TestMarkExpired_MarksOnlyExpired(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	expired := mkRotationUser(t, db, td, "rot_run_expired", "expired@example.org", time.Now().AddDate(0, 0, -200))
	fresh := mkRotationUser(t, db, td, "rot_run_fresh", "fresh@example.org", time.Now().AddDate(0, 0, -3))

	result, err := svc.MarkExpired(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, result.Marked, "помечаем только просроченный пароль")
	assert.Equal(t, 0, result.Changed, "плановый прогон паролей не меняет")

	var after models.User
	require.NoError(t, db.First(&after, expired.ID).Error)
	assert.True(t, after.MustChangePassword, "истёкший пароль требует смены при входе")
	assert.Equal(t, "старый-хэш", after.Password, "пароль остаётся прежним - им человек и войдёт")
	assert.Nil(t, after.PasswordRotatedAt, "система пароль не меняла, отметке смены взяться неоткуда")

	var untouched models.User
	require.NoError(t, db.First(&untouched, fresh.ID).Error)
	assert.False(t, untouched.MustChangePassword, "свежий пароль трогать нельзя")
	assert.Equal(t, "старый-хэш", untouched.Password)
}

// TestMarkExpired_SendsNothing: плановый прогон не ставит в очередь ни одного
// письма. Ради этого схему и меняли - пароль перестал ходить по почте открытым
// текстом.
func TestMarkExpired_SendsNothing(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	u := mkRotationUser(t, db, td, "rot_run_letter", "letter@example.org", time.Now().AddDate(0, 0, -200))

	_, err := svc.MarkExpired(context.Background())
	require.NoError(t, err)

	var letters int64
	require.NoError(t, db.Model(&models.EmailMessage{}).
		Where("user_id = ?", u.ID).Count(&letters).Error)
	assert.EqualValues(t, 0, letters, "работнику писем не уходит")

	var withPassword int64
	require.NoError(t, db.Model(&models.EmailMessage{}).
		Where("template_code = ?", services.MailTemplatePasswordRotated).Count(&withPassword).Error)
	assert.EqualValues(t, 0, withPassword, "письма с паролем плановый прогон не порождает")
}

// TestMarkExpired_TakesUsersWithoutEmail: работник без адреса теперь участвует
// наравне со всеми. Адрес требовался, только чтобы доставить придуманный пароль,
// а придумывать его больше некому.
func TestMarkExpired_TakesUsersWithoutEmail(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	noMail := mkRotationUser(t, db, td, "rot_run_nomail", "", time.Now().AddDate(0, 0, -200))

	result, err := svc.MarkExpired(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, result.Marked)
	assert.Empty(t, result.NoMailLogins, "пропускать некого, список пуст")
	assert.Equal(t, 0, result.SkippedNoMail)

	var after models.User
	require.NoError(t, db.First(&after, noMail.ID).Error)
	assert.True(t, after.MustChangePassword, "отсутствие адреса больше не защищает от отметки")
}

// TestMarkExpired_SkipsArchivedAndBanned: архивных и заблокированных проверка не
// касается - им и входить некуда. Требование владельца.
func TestMarkExpired_SkipsArchivedAndBanned(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	archived := mkRotationUser(t, db, td, "rot_run_archived", "arch@example.org", time.Now().AddDate(0, 0, -200))
	banned := mkRotationUser(t, db, td, "rot_run_banned", "ban@example.org", time.Now().AddDate(0, 0, -200))
	// is_active=false и is_banned=true дописываем отдельно: нулевое значение при
	// default:true gorm при вставке пропускает.
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", archived.ID).
		Update("is_active", false).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", banned.ID).
		Update("is_banned", true).Error)

	_, err := svc.MarkExpired(context.Background())
	require.NoError(t, err)

	for _, id := range []int{archived.ID, banned.ID} {
		var after models.User
		require.NoError(t, db.First(&after, id).Error)
		assert.False(t, after.MustChangePassword)
		assert.Equal(t, "старый-хэш", after.Password)
	}
}

// TestMarkExpired_WorksWithoutMail: ненастроенная почта плановому прогону больше
// не помеха. Прежняя схема без неё останавливалась, потому что рассылала пароли,
// а этой рассылать нечего.
func TestMarkExpired_WorksWithoutMail(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	cfg := &config.Config{UploadMaxFileSize: 10485760} // SMTP_HOST пуст
	settings := services.NewSettingsService(db, cfg)
	svc := services.NewPasswordRotationService(db, settings, services.NewMailService(db, cfg),
		services.NewNotificationService(db), nil, "")

	u := mkRotationUser(t, db, td, "rot_run_nomailsrv", "x@example.org", time.Now().AddDate(0, 0, -200))

	result, err := svc.MarkExpired(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, result.Marked)

	var after models.User
	require.NoError(t, db.First(&after, u.ID).Error)
	assert.True(t, after.MustChangePassword)
}

// TestMarkExpired_KeepsSessions: открытые сессии не обрываются. Пароль не менялся,
// а уже открытая вкладка упрётся в форму смены на первом же запросе - этим
// занимается гейт, а не отзыв маркеров.
func TestMarkExpired_KeepsSessions(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	u := mkRotationUser(t, db, td, "rot_run_sessions", "sess@example.org", time.Now().AddDate(0, 0, -200))
	require.NoError(t, db.Create(&models.RefreshToken{
		UserID: u.ID, FamilyID: "test-family", TokenHash: "тестовый-хэш-маркера",
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error)

	_, err := svc.MarkExpired(context.Background())
	require.NoError(t, err)

	var alive int64
	require.NoError(t, db.Model(&models.RefreshToken{}).
		Where("user_id = ? AND is_revoked = false", u.ID).Count(&alive).Error)
	assert.EqualValues(t, 1, alive, "маркер продления остаётся живым")
}

// TestMarkExpired_WritesAudit: отметка попадает в журнал отдельным действием.
// Иначе на вопрос «почему у меня вдруг потребовали сменить пароль» ответить
// нечем, а сбросом пароля это называть нельзя - пароль не менялся.
func TestMarkExpired_WritesAudit(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	u := mkRotationUser(t, db, td, "rot_run_audit", "audit@example.org", time.Now().AddDate(0, 0, -200))

	_, err := svc.MarkExpired(context.Background())
	require.NoError(t, err)

	var expiredEntries int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityUser, u.ID, models.UserActionPasswordExpired).
		Count(&expiredEntries).Error)
	assert.EqualValues(t, 1, expiredEntries)

	var resetEntries int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityUser, u.ID, models.UserActionPasswordReset).
		Count(&resetEntries).Error)
	assert.EqualValues(t, 0, resetEntries, "сбросом пароля отметка не притворяется")
}

// TestMarkExpired_CreatesNoNotification: уведомления работнику проверка не шлёт.
// Он увидит требование на входе, а прочитать уведомление сможет только после
// смены пароля - к тому моменту оно уже неправда.
func TestMarkExpired_CreatesNoNotification(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	u := mkRotationUser(t, db, td, "rot_run_notify", "notify@example.org", time.Now().AddDate(0, 0, -200))

	_, err := svc.MarkExpired(context.Background())
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ?", u.ID).Count(&count).Error)
	assert.EqualValues(t, 0, count)
}

// TestMarkExpired_SecondRunMarksNobody: идемпотентность. Дату смены пароля
// отметка не двигает, поэтому от повторного счёта спасает только исключение уже
// помеченных - без него прогон следующих суток перечислил бы тех же людей.
func TestMarkExpired_SecondRunMarksNobody(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	mkRotationUser(t, db, td, "rot_run_twice", "twice@example.org", time.Now().AddDate(0, 0, -200))

	first, err := svc.MarkExpired(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, first.Marked)

	second, err := svc.MarkExpired(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, second.Marked, "повторный прогон никого не трогает")
}

// TestRotation_ManualChangesAndSendsPassword: ручное обновление осталось прежним -
// придумывает пароль, шлёт письмо и не смотрит на срок действия.
func TestRotation_ManualChangesAndSendsPassword(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	fresh := mkRotationUser(t, db, td, "rot_run_manual", "manual@example.org", time.Now().AddDate(0, 0, -1))

	result, err := svc.Run(context.Background(), 42)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Changed, 1)
	assert.True(t, result.Manual)
	assert.Equal(t, 42, result.StartedBy)

	var after models.User
	require.NoError(t, db.First(&after, fresh.ID).Error)
	assert.NotEqual(t, "старый-хэш", after.Password, "ручной прогон меняет и свежие пароли")
	require.NotNil(t, after.PasswordRotatedAt)

	var letter models.EmailMessage
	require.NoError(t, db.Where("user_id = ? AND template_code = ?", fresh.ID, services.MailTemplatePasswordRotated).
		First(&letter).Error)
	assert.Equal(t, "manual@example.org", letter.ToAddress)
	assert.Equal(t, models.EmailStatusPending, letter.Status)
	assert.Contains(t, letter.Body, "rot_run_manual", "в письме должен быть логин")
	assert.Contains(t, letter.Body, "Пароль:")
}

// TestRotation_ManualSkipsUsersWithoutEmail: у ручного обновления адрес почты
// по-прежнему обязателен - придуманный пароль без него доставить нечем.
func TestRotation_ManualSkipsUsersWithoutEmail(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	noMail := mkRotationUser(t, db, td, "rot_manual_nomail", "", time.Now().AddDate(0, 0, -200))

	result, err := svc.Run(context.Background(), 1)
	require.NoError(t, err)

	var after models.User
	require.NoError(t, db.First(&after, noMail.ID).Error)
	assert.Equal(t, "старый-хэш", after.Password, "пароль без адреса менять нельзя")
	assert.Contains(t, result.NoMailLogins, "rot_manual_nomail")
	assert.GreaterOrEqual(t, result.SkippedNoMail, 1)
}

// TestRotation_ManualRefusesWithoutMail: без настроенной почты ручное обновление
// не начинается. Молча пропустить нельзя: администратор будет считать, что пароли
// сменились.
func TestRotation_ManualRefusesWithoutMail(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	cfg := &config.Config{UploadMaxFileSize: 10485760} // SMTP_HOST пуст
	settings := services.NewSettingsService(db, cfg)
	svc := services.NewPasswordRotationService(db, settings, services.NewMailService(db, cfg),
		services.NewNotificationService(db), nil, "")

	u := mkRotationUser(t, db, td, "rot_manual_nomailsrv", "x@example.org", time.Now().AddDate(0, 0, -200))

	_, err := svc.Run(context.Background(), 1)
	require.ErrorIs(t, err, services.ErrRotationMailNotConfigured)

	var after models.User
	require.NoError(t, db.First(&after, u.ID).Error)
	assert.Equal(t, "старый-хэш", after.Password, "без почты пароли не трогаются")
}

// TestRotation_ManualRevokesSessions: после смены пароля прежние маркеры продления
// отзываются - иначе старая сессия доживёт до своего срока с паролем, которого
// владелец уже не знает.
func TestRotation_ManualRevokesSessions(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	u := mkRotationUser(t, db, td, "rot_manual_sessions", "msess@example.org", time.Now().AddDate(0, 0, -200))
	require.NoError(t, db.Create(&models.RefreshToken{
		UserID: u.ID, FamilyID: "test-family", TokenHash: "тестовый-хэш-маркера",
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error)

	_, err := svc.Run(context.Background(), 1)
	require.NoError(t, err)

	var alive int64
	require.NoError(t, db.Model(&models.RefreshToken{}).
		Where("user_id = ? AND is_revoked = false", u.ID).Count(&alive).Error)
	assert.EqualValues(t, 0, alive, "прежние сессии должны быть оборваны")
}

// TestRotation_ManualCreatesNotification: работник узнаёт о смене и внутри
// системы, не только письмом.
func TestRotation_ManualCreatesNotification(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	u := mkRotationUser(t, db, td, "rot_manual_notify", "mnotify@example.org", time.Now().AddDate(0, 0, -200))

	_, err := svc.Run(context.Background(), 1)
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", u.ID, services.NotificationTypePasswordRotated).
		Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

// TestRotation_ExpiringWarningWithoutPassword: предупреждение уходит заранее,
// пароля не содержит и обещает то, что произойдёт на самом деле - просьбу задать
// новый пароль на входе, а не присланный системой.
func TestRotation_ExpiringWarningWithoutPassword(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, settings := rotationEnv(t, db)
	// Включаем проверку сроков: предупреждения без неё не имеют смысла.
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
	assert.Contains(t, letter.Body, "попросит задать новый пароль")
	assert.NotContains(t, letter.Body, "пришлёт новый письмом", "система больше не присылает пароль сама")

	// Повторный вызов в тот же день письмо не дублирует.
	svc.NotifyExpiring(context.Background())
	var letters int64
	require.NoError(t, db.Model(&models.EmailMessage{}).
		Where("user_id = ? AND template_code = ?", u.ID, services.MailTemplatePasswordExpiring).
		Count(&letters).Error)
	assert.EqualValues(t, 1, letters, "предупреждение шлётся раз в сутки")
}

// TestRotateOne_RefusesArchivedAndBanned: точечная смена пароля из карточки не
// должна трогать архивные и заблокированные учётные записи. В интерфейсе кнопка
// у них не показывается, но проверка нужна и на сервере - иначе она обходится
// прямым запросом к ручке.
func TestRotateOne_RefusesArchivedAndBanned(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)

	archived := mkRotationUser(t, db, td, "rot_one_archived", "arch1@example.org", time.Now().AddDate(0, 0, -200))
	banned := mkRotationUser(t, db, td, "rot_one_banned", "ban1@example.org", time.Now().AddDate(0, 0, -200))
	// is_active=false и is_banned=true дописываем отдельным обновлением: нулевое
	// значение при default:true gorm при вставке пропускает.
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", archived.ID).
		Update("is_active", false).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", banned.ID).
		Update("is_banned", true).Error)

	err := svc.RotateOne(context.Background(), "rot_one_archived", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "архиве")

	err = svc.RotateOne(context.Background(), "rot_one_banned", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "заблокирован")

	// Пароли не тронуты, писем не поставлено.
	for _, id := range []int{archived.ID, banned.ID} {
		var after models.User
		require.NoError(t, db.First(&after, id).Error)
		assert.Equal(t, "старый-хэш", after.Password)
	}
	var letters int64
	require.NoError(t, db.Model(&models.EmailMessage{}).
		Where("template_code = ?", services.MailTemplatePasswordRotated).Count(&letters).Error)
	assert.EqualValues(t, 0, letters, "письма недействующим учётным записям не ставятся")
}

// TestRotateOne_WorksForActive: действующему работнику точечная смена по-прежнему
// доступна - проверка не должна закрыть основной сценарий.
func TestRotateOne_WorksForActive(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	u := mkRotationUser(t, db, td, "rot_one_active", "active1@example.org", time.Now().AddDate(0, 0, -1))

	require.NoError(t, svc.RotateOne(context.Background(), "rot_one_active", 1))

	var after models.User
	require.NoError(t, db.First(&after, u.ID).Error)
	assert.NotEqual(t, "старый-хэш", after.Password)
}
