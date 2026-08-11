package services

import (
	"context"
	"fmt"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// PasswordRotationStatus - состояние плановой проверки сроков для экрана настроек.
// Включать проверку вслепую нельзя: администратор должен видеть, скольких
// работников затронет ближайший прогон и уйдут ли предупреждения о скором
// истечении.
type PasswordRotationStatus struct {
	// MailConfigured - настроена ли отправка почты. Плановой проверке она не нужна,
	// от неё зависят предупреждения заранее и ручное обновление паролей.
	MailConfigured bool `json:"mail_configured"`
	Enabled        bool `json:"enabled"`
	RotationDays   int  `json:"rotation_days"`
	// Eligible - активные незаблокированные работники с адресом почты, то есть те,
	// кому ручное обновление может выслать новый пароль.
	Eligible int `json:"eligible"`
	// WithoutEmail - активные незаблокированные без адреса. Плановая проверка их
	// берёт наравне со всеми, а вот предупреждение и новый пароль слать им некуда.
	WithoutEmail int `json:"without_email"`
	// Expired - у скольких срок уже вышел. Часть из них прогон мог пометить в
	// прошлые сутки, поэтому это ответ на вопрос «сколько людей упрётся в форму
	// смены», а не размер ближайшего прогона.
	Expired int `json:"expired"`
	// ExpiringSoon - скольким уйдёт предупреждение в окне перед истечением. Считается
	// только по тем, у кого есть адрес: предупреждение - это письмо.
	ExpiringSoon int `json:"expiring_soon"`
	// NextRunAt - когда планировщик проснётся в следующий раз.
	NextRunAt time.Time `json:"next_run_at"`
}

// PasswordRotationStatusService считает это состояние.
type PasswordRotationStatusService struct {
	db       *gorm.DB
	settings SettingsService
	mail     MailSender
	location *time.Location
}

// NewPasswordRotationStatusService конструирует сервис. mail может быть nil -
// тогда почта считается ненастроенной.
func NewPasswordRotationStatusService(db *gorm.DB, settings SettingsService, mail MailSender, loc *time.Location) *PasswordRotationStatusService {
	if loc == nil {
		loc = time.UTC
	}
	return &PasswordRotationStatusService{db: db, settings: settings, mail: mail, location: loc}
}

// RotationRunHour - час, в который просыпается планировщик проверки сроков.
// 03:00 занят сверкой файлового архива, 06:00 - сбросом территориальных статусов,
// поэтому 04:00.
const RotationRunHour = 4

// Get собирает состояние. Считает по тем же условиям, по которым будет отбирать
// работников сам прогон, - иначе число на экране разойдётся с тем, что произойдёт.
func (s *PasswordRotationStatusService) Get(ctx context.Context) (PasswordRotationStatus, error) {
	policy := s.settings.GetPasswordPolicy()
	status := PasswordRotationStatus{
		MailConfigured: s.mail != nil && s.mail.Enabled(),
		Enabled:        policy.RotationEnabled,
		RotationDays:   policy.RotationDays,
		NextRunAt:      nextRotationRun(time.Now().In(s.location), s.location),
	}

	// Базовое условие: работник в системе и может войти. Архивных и заблокированных
	// плановая проверка не касается - им и входить некуда.
	base := s.db.WithContext(ctx).Model(&models.User{}).
		Where("is_active = ?", true).
		Where("is_banned = ?", false)

	var eligible, withoutEmail, expired, expiringSoon int64
	hasEmail := "email IS NOT NULL AND email <> ''"

	if err := base.Session(&gorm.Session{}).Where(hasEmail).Count(&eligible).Error; err != nil {
		return status, fmt.Errorf("count eligible users: %w", err)
	}
	if err := base.Session(&gorm.Session{}).Where("email IS NULL OR email = ''").Count(&withoutEmail).Error; err != nil {
		return status, fmt.Errorf("count users without email: %w", err)
	}

	// Истёкшие считаем без оглядки на почту и на уже поднятый признак смены: на
	// экране это ответ на вопрос «у скольких работников срок вышел», а не «сколько
	// строк тронет ближайший прогон».
	deadline := time.Now().AddDate(0, 0, -policy.RotationDays)
	if err := base.Session(&gorm.Session{}).
		Where("password_changed_at IS NOT NULL AND password_changed_at < ?", deadline).
		Count(&expired).Error; err != nil {
		return status, fmt.Errorf("count expired passwords: %w", err)
	}

	if policy.RotationNotifyDaysBefore > 0 {
		soonFrom := deadline
		soonTo := time.Now().AddDate(0, 0, -(policy.RotationDays - policy.RotationNotifyDaysBefore))
		if err := base.Session(&gorm.Session{}).Where(hasEmail).
			Where("password_changed_at IS NOT NULL AND password_changed_at >= ? AND password_changed_at < ?", soonFrom, soonTo).
			Count(&expiringSoon).Error; err != nil {
			return status, fmt.Errorf("count expiring passwords: %w", err)
		}
	}

	status.Eligible = int(eligible)
	status.WithoutEmail = int(withoutEmail)
	status.Expired = int(expired)
	status.ExpiringSoon = int(expiringSoon)
	return status, nil
}

// nextRotationRun возвращает ближайшее срабатывание планировщика после now.
func nextRotationRun(now time.Time, loc *time.Location) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), RotationRunHour, 0, 0, 0, loc)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
