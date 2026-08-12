package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// Коды шаблонов писем о паролях. Хранятся в очереди писем и по ним же
// отбираются письма одного вида для отчёта.
//
// Значения берутся из кодов уведомлений намеренно: письмо и уведомление - два
// канала одного события, и общий код позволяет сопоставить их в разборе
// («уведомление создано, а письмо не ушло»). Замок каталога уведомлений заодно
// не даёт написать здесь строку мимо каталога.
const (
	MailTemplatePasswordRotated  = NotificationTypePasswordRotated
	MailTemplatePasswordExpiring = NotificationTypePasswordExpiring
	MailTemplateRotationReport   = NotificationTypePasswordRotationReport
)

// reportLoginsLimit - сколько логинов без адреса перечислять в отчёте. Полный
// список смотрят в разделе «Пользователи», режим «Без почты»; письмо на триста
// строк никто не читает.
const reportLoginsLimit = 20

// ErrRotationInProgress - прогон уже идёт. Замок общий у проверки сроков и
// обновления паролей: второй одновременный запуск выдал бы работнику два пароля
// подряд и два письма, из которых рабочим окажется только второе, а первое он
// успел бы попробовать.
var ErrRotationInProgress = errors.New("прогон по паролям уже выполняется")

// ErrRotationMailNotConfigured - почта не настроена. Придумывать пароль за
// человека, не имея канала доставки, значит запереть его снаружи. Плановой
// проверки сроков не касается: она паролей не придумывает и писем не шлёт.
var ErrRotationMailNotConfigured = errors.New("почта не настроена, смена паролей не запускается")

// RotationResult - итог прогона. Плановая проверка и ручное обновление делают
// разное, поэтому счётчика два: Marked - скольким пароль помечен истёкшим,
// Changed - скольким пароль сменён и выслан.
type RotationResult struct {
	Changed       int       `json:"changed"`
	Marked        int       `json:"marked"`
	SkippedNoMail int       `json:"skipped_no_email"`
	Failed        int       `json:"failed"`
	NoMailLogins  []string  `json:"no_email_logins"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	// Manual - прогон запущен человеком, а не расписанием.
	Manual bool `json:"manual"`
	// StartedBy - кто запустил ручной прогон (0 у планового).
	StartedBy int `json:"started_by,omitempty"`
}

// PasswordRotationService помечает истёкшие пароли по сроку и меняет их по кнопке.
type PasswordRotationService struct {
	db            *gorm.DB
	settings      SettingsService
	mail          MailSender
	notifications NotificationService
	resolver      *PermissionResolver
	recorder      AuditRecorder
	// baseURL - адрес системы для писем. Пустой означает «ссылку не вставлять»:
	// ссылка на localhost у получателя всё равно не откроется.
	baseURL string

	mu      sync.Mutex
	running bool
	last    *RotationResult
}

// NewPasswordRotationService конструирует сервис. resolver нужен, чтобы найти
// администраторов для отчёта; без него отчёт просто не уходит.
func NewPasswordRotationService(db *gorm.DB, settings SettingsService, mail MailSender,
	notifications NotificationService, resolver *PermissionResolver, baseURL string) *PasswordRotationService {
	return &PasswordRotationService{
		db:            db,
		settings:      settings,
		mail:          mail,
		notifications: notifications,
		resolver:      resolver,
		recorder:      NewAuditRecorder(db),
		baseURL:       normalizeBaseURL(baseURL),
	}
}

// LastResult возвращает итог последнего прогона в этом процессе (nil, если
// прогонов не было). Нужен экрану настроек.
func (s *PasswordRotationService) LastResult() *RotationResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

// RunScheduled - плановый прогон. Выключенная настройка не ошибка: человек её
// осознанно не включил, и жаловаться в журнал незачем.
func (s *PasswordRotationService) RunScheduled(ctx context.Context) {
	policy := s.settings.GetPasswordPolicy()
	if !policy.RotationEnabled {
		return
	}
	if _, err := s.MarkExpired(ctx); err != nil && !errors.Is(err, ErrRotationInProgress) {
		slog.Error("плановая проверка сроков паролей не выполнена", "error", err)
	}
}

// MarkExpired - плановый прогон по сроку. Паролей не придумывает и писем не шлёт:
// тем, у кого срок вышел, поднимается признак обязательной смены. Дальше человек
// входит своим прежним паролем, а гейт (#1911) не пускает его никуда, кроме формы
// смены. Так пароль перестал путешествовать по почте открытым текстом - главный
// риск прежней схемы, когда система придумывала пароль и высылала его письмом.
//
// Почта здесь не нужна вовсе, поэтому и работники без адреса больше не
// пропускаются: адрес требовался только для доставки нового пароля.
func (s *PasswordRotationService) MarkExpired(ctx context.Context) (*RotationResult, error) {
	if err := s.begin(); err != nil {
		return nil, err
	}
	defer s.finish()

	policy := s.settings.GetPasswordPolicy()
	result := &RotationResult{StartedAt: time.Now()}

	targets, err := s.expiredTargets(ctx, policy)
	if err != nil {
		return nil, err
	}

	for _, u := range targets {
		if ctx.Err() != nil {
			// Сервер останавливают - прекращаем. Уже помеченные остаются
			// помеченными, остальных возьмёт следующий прогон.
			break
		}
		if err := s.markOne(ctx, u, policy); err != nil {
			result.Failed++
			slog.Error("плановая проверка сроков: пароль не помечен истёкшим",
				"user_id", u.ID, "username", u.Username, "error", err)
			continue
		}
		result.Marked++
	}

	result.FinishedAt = time.Now()
	s.mu.Lock()
	s.last = result
	s.mu.Unlock()

	s.reportMarkedToAdmins(ctx, result)
	slog.Info("плановая проверка сроков паролей завершена",
		"marked", result.Marked, "failed", result.Failed)
	return result, nil
}

// Run - ручное обновление паролей всем работникам. В отличие от планового
// прогона придумывает пароль и высылает его письмом, поэтому срок действия здесь
// ни при чём: берутся все действующие работники с адресом почты. Инструмент на
// случай инцидента, когда прежние пароли нужно обнулить разом.
func (s *PasswordRotationService) Run(ctx context.Context, startedBy int) (*RotationResult, error) {
	if err := s.begin(); err != nil {
		return nil, err
	}
	defer s.finish()

	if s.mail == nil || !s.mail.Enabled() {
		// Громко, а не молча: администратор должен узнать, что прогон не состоялся,
		// иначе он будет считать, что пароли сменились.
		s.notifyAdmins(ctx, "Обновление паролей не выполнено",
			"Почта не настроена, поэтому прогон не начинался. Пароли не менялись.")
		return nil, ErrRotationMailNotConfigured
	}

	policy := s.settings.GetPasswordPolicy()
	result := &RotationResult{StartedAt: time.Now(), Manual: true, StartedBy: startedBy}

	noMail, err := s.loginsWithoutEmail(ctx)
	if err != nil {
		return nil, err
	}
	result.SkippedNoMail = len(noMail)
	result.NoMailLogins = noMail

	targets, err := s.selectTargets(ctx)
	if err != nil {
		return nil, err
	}

	for _, u := range targets {
		if ctx.Err() != nil {
			// Сервер останавливают - прекращаем, уже смененные пароли остаются
			// смененными, письма к ним лежат в очереди.
			break
		}
		if err := s.rotateOne(ctx, u, policy, "обновление паролей всем работникам"); err != nil {
			result.Failed++
			slog.Error("обновление паролей: пароль не сменён",
				"user_id", u.ID, "username", u.Username, "error", err)
			continue
		}
		result.Changed++
		s.notifyRotated(ctx, u)
	}

	result.FinishedAt = time.Now()
	s.mu.Lock()
	s.last = result
	s.mu.Unlock()

	s.reportToAdmins(ctx, result)
	slog.Info("обновление паролей завершено",
		"changed", result.Changed, "failed", result.Failed,
		"skipped_no_email", result.SkippedNoMail, "started_by", startedBy)
	return result, nil
}

// begin занимает сервис под прогон. Возвращает ErrRotationInProgress, если
// прогон уже идёт: ручная кнопка и планировщик могут совпасть по времени.
func (s *PasswordRotationService) begin() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return ErrRotationInProgress
	}
	s.running = true
	return nil
}

func (s *PasswordRotationService) finish() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

// IsRunning сообщает, идёт ли прогон прямо сейчас.
func (s *PasswordRotationService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// selectTargets отбирает работников для ручного обновления. Условия те же, по
// которым считает экран настроек: действующая незаблокированная учётная запись с
// адресом почты - придуманный пароль надо куда-то выслать. Архивных и
// заблокированных обновление не касается - им и входить некуда.
func (s *PasswordRotationService) selectTargets(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := s.db.WithContext(ctx).
		Where("is_active = ? AND is_banned = ?", true, false).
		Where("email IS NOT NULL AND email <> ''").
		Order("id").Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("отбор работников для смены пароля: %w", err)
	}
	return users, nil
}

// expiredTargets отбирает тех, у кого срок действия пароля вышел. Адрес почты в
// условии не участвует: плановый прогон писем не шлёт. Архивных и заблокированных
// не берём - им и входить некуда.
//
// Уже помеченные исключены, и на этом держится идемпотентность: пометка даты
// смены пароля не двигает, поэтому без этого условия прогон следующих суток
// перечислял бы тех же людей заново и врал бы в отчёте.
//
// Пустая дата смены сюда не попадает намеренно: она означает учётную запись,
// заведённую до появления столбца, и такие получили дату при миграции.
func (s *PasswordRotationService) expiredTargets(ctx context.Context, policy models.PasswordPolicy) ([]models.User, error) {
	deadline := time.Now().AddDate(0, 0, -policy.RotationDays)
	var users []models.User
	err := s.db.WithContext(ctx).
		Where("is_active = ? AND is_banned = ?", true, false).
		Where("must_change_password = ?", false).
		Where("password_changed_at IS NOT NULL AND password_changed_at < ?", deadline).
		Order("id").Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("отбор работников с истёкшим паролем: %w", err)
	}
	return users, nil
}

// markOne помечает пароль одного работника истёкшим. Сессии здесь не обрываются,
// в отличие от смены пароля: гейт висит на каждом защищённом запросе, поэтому уже
// открытая вкладка упирается в форму смены на первом же обращении к серверу, а
// маркер продления после смены пароля отзовётся сам.
func (s *PasswordRotationService) markOne(ctx context.Context, u models.User, policy models.PasswordPolicy) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).Where("id = ?", u.ID).
			Update("must_change_password", true).Error; err != nil {
			return fmt.Errorf("отметка истёкшего пароля: %w", err)
		}
		s.recorder.Log(ctx, tx, models.AuditEntityUser, &u.ID, models.UserActionPasswordExpired, nil,
			map[string]any{"rotation_days": policy.RotationDays})
		return nil
	})
}

// loginsWithoutEmail собирает логины действующих работников без адреса почты.
// Ручное обновление их пропускает: придуманный пароль выслать некуда, а сменить
// его молча - значит запереть человека снаружи.
func (s *PasswordRotationService) loginsWithoutEmail(ctx context.Context) ([]string, error) {
	var logins []string
	err := s.db.WithContext(ctx).Model(&models.User{}).
		Where("is_active = ? AND is_banned = ?", true, false).
		Where("email IS NULL OR email = ''").
		Order("username").
		Pluck("username", &logins).Error
	if err != nil {
		return nil, fmt.Errorf("сбор работников без адреса почты: %w", err)
	}
	return logins, nil
}

// rotateOne меняет пароль одному работнику. Пароль и письмо о нём попадают на
// диск одной транзакцией: иначе сбой между ними оставит человека с паролем,
// которого он не видел. reason попадает в журнал действий - обновление всем и
// сброс из карточки выглядят там одинаково, различает их только он.
func (s *PasswordRotationService) rotateOne(ctx context.Context, u models.User, policy models.PasswordPolicy, reason string) error {
	password, err := s.generateUnusedPassword(ctx, u.ID, policy)
	if err != nil {
		return err
	}
	hashed := hashPassword(password)
	now := time.Now()

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"password":             hashed,
			"password_changed_at":  now,
			"password_rotated_at":  now,
			"must_change_password": policy.ForceChangeOnNextLogin,
		}
		if err := tx.Model(&models.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("обновление пароля: %w", err)
		}

		// Старые сессии обрываем: пароль сменился, и прежние маркеры продления
		// не должны доживать до своего срока.
		if err := tx.Model(&models.RefreshToken{}).
			Where("user_id = ? AND is_revoked = false", u.ID).
			Update("is_revoked", true).Error; err != nil {
			return fmt.Errorf("отзыв маркеров продления: %w", err)
		}

		// Выданный пароль запоминается наравне с придуманным вручную: иначе
		// человек, получив письмо, сможет «сменить» пароль на тот же самый.
		if err := recordUsedPassword(ctx, tx, u.ID, hashed); err != nil {
			return err
		}

		s.recorder.Log(ctx, tx, models.AuditEntityUser, &u.ID, models.UserActionPasswordReset, nil,
			map[string]any{"reason": reason})

		email := ""
		if u.Email != nil {
			email = *u.Email
		}
		letter := MailMessage{
			To:           email,
			Subject:      "Новый пароль в системе бюро пропусков",
			Body:         s.rotatedLetterBody(u, password, policy),
			TemplateCode: MailTemplatePasswordRotated,
			UserID:       &u.ID,
		}
		if err := s.mail.Enqueue(ctx, tx, letter); err != nil {
			return fmt.Errorf("постановка письма в очередь: %w", err)
		}
		return nil
	})
}

// generateUnusedPassword придумывает пароль, которым эта учётная запись ещё не
// пользовалась. Запрет на повтор одинаков для всех путей смены, и выданный
// системой пароль не исключение: работник иначе получил бы письмом пароль,
// который система же и не даст ему подтвердить при следующей смене.
//
// Совпадение случайного пароля с одним из десяти прежних встречается примерно
// никогда, поэтому попыток пять, а не бесконечность: исчерпать их можно только
// при сломанном генераторе, и тогда честная ошибка полезнее вечного цикла.
func (s *PasswordRotationService) generateUnusedPassword(ctx context.Context, userID int, policy models.PasswordPolicy) (string, error) {
	for attempt := 0; attempt < passwordGenerateAttempts; attempt++ {
		password := GeneratePassword(policy)
		used, err := passwordAlreadyUsed(ctx, s.db, userID, password)
		if err != nil {
			return "", err
		}
		if !used {
			return password, nil
		}
		slog.Warn("плановая смена: сгенерированный пароль уже использовался, пробуем ещё",
			"user_id", userID, "attempt", attempt+1)
	}
	return "", fmt.Errorf("не удалось придумать неиспользованный пароль за %d попыток", passwordGenerateAttempts)
}

// rotatedLetterBody собирает письмо с новым паролем. Пароль отдельной строкой и
// без знаков препинания вплотную - его переписывают руками.
func (s *PasswordRotationService) rotatedLetterBody(u models.User, password string, policy models.PasswordPolicy) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Здравствуйте, %s.\n\n", addressee(u))
	b.WriteString("По правилам безопасности система сменила пароль вашей учётной записи:\n")
	b.WriteString("срок действия прежнего истёк.\n\n")
	fmt.Fprintf(&b, "  Логин:  %s\n", u.Username)
	fmt.Fprintf(&b, "  Пароль: %s\n\n", password)
	if s.baseURL != "" {
		fmt.Fprintf(&b, "Адрес системы: %s\n\n", s.baseURL)
	}
	if policy.ForceChangeOnNextLogin {
		b.WriteString("При первом входе система попросит задать свой пароль - придумайте его\n")
		b.WriteString("сами и никому не сообщайте. Пароль из этого письма после этого\n")
		b.WriteString("перестанет действовать.\n\n")
	}
	b.WriteString("Если вы не пользуетесь системой или письмо пришло по ошибке, сообщите\n")
	b.WriteString("в бюро пропусков.\n")
	return b.String()
}

// notifyRotated кладёт уведомление внутри системы. Best-effort: письмо уже в
// очереди и является основным каналом, а провал уведомления не должен отменять
// уже сменённый пароль.
func (s *PasswordRotationService) notifyRotated(ctx context.Context, u models.User) {
	if s.notifications == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"rotated_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return
	}
	data := string(payload)
	if err := s.notifications.CreateForUser(ctx, u.ID, NotificationTypePasswordRotated,
		"Пароль изменён",
		"Система сменила ваш пароль по правилам безопасности и отправила новый на вашу почту.",
		&data); err != nil {
		slog.Warn("обновление паролей: уведомление не создано", "user_id", u.ID, "error", err)
	}
}

// reportMarkedToAdmins рассылает итог плановой проверки сроков. Письмо уходит,
// только если почта настроена: сам прогон в ней не нуждается, и без неё
// администратор получит один лишь отчёт внутри системы.
func (s *PasswordRotationService) reportMarkedToAdmins(ctx context.Context, r *RotationResult) {
	message := fmt.Sprintf("Помечено истёкшими паролей: %d. При следующем входе система попросит этих работников задать новый пароль.", r.Marked)
	if r.Failed > 0 {
		message += fmt.Sprintf(" Не удалось пометить: %d.", r.Failed)
	}
	s.notifyAdmins(ctx, "Плановая проверка сроков паролей выполнена", message)

	if s.mail == nil || !s.mail.Enabled() {
		return
	}
	for _, admin := range s.adminUsers(ctx) {
		if admin.Email == nil || *admin.Email == "" {
			continue
		}
		letter := MailMessage{
			To:           *admin.Email,
			Subject:      "Плановая проверка сроков паролей: итог прогона",
			Body:         s.markReportLetterBody(r),
			TemplateCode: MailTemplateRotationReport,
			UserID:       &admin.ID,
		}
		if err := s.mail.Enqueue(ctx, nil, letter); err != nil {
			slog.Warn("плановая проверка сроков: отчёт администратору не поставлен в очередь",
				"user_id", admin.ID, "error", err)
		}
	}
}

func (s *PasswordRotationService) markReportLetterBody(r *RotationResult) string {
	var b strings.Builder
	b.WriteString("Плановая проверка сроков действия паролей завершена.\n\n")
	fmt.Fprintf(&b, "  Помечено истёкшими: %d\n", r.Marked)
	fmt.Fprintf(&b, "  Не удалось пометить: %d\n\n", r.Failed)
	b.WriteString("Пароли этих работников не менялись и письмами не рассылались: каждый\n")
	b.WriteString("входит своим прежним паролем, после чего система просит задать новый и\n")
	b.WriteString("до этого никуда не пускает.\n")
	return b.String()
}

// reportToAdmins рассылает итог ручного обновления паролей тем, кто отвечает за
// настройки.
func (s *PasswordRotationService) reportToAdmins(ctx context.Context, r *RotationResult) {
	title := "Обновление паролей выполнено"
	message := fmt.Sprintf("Сменено паролей: %d.", r.Changed)
	if r.SkippedNoMail > 0 {
		message += fmt.Sprintf(" Без адреса почты пропущено: %d.", r.SkippedNoMail)
	}
	if r.Failed > 0 {
		message += fmt.Sprintf(" Не удалось сменить: %d.", r.Failed)
	}
	s.notifyAdmins(ctx, title, message)

	// Письмо администраторам - со списком тех, кому нужно проставить адрес.
	for _, admin := range s.adminUsers(ctx) {
		if admin.Email == nil || *admin.Email == "" {
			continue
		}
		letter := MailMessage{
			To:           *admin.Email,
			Subject:      "Обновление паролей: итог прогона",
			Body:         s.reportLetterBody(r),
			TemplateCode: MailTemplateRotationReport,
			UserID:       &admin.ID,
		}
		if err := s.mail.Enqueue(ctx, nil, letter); err != nil {
			slog.Warn("обновление паролей: отчёт администратору не поставлен в очередь",
				"user_id", admin.ID, "error", err)
		}
	}
}

func (s *PasswordRotationService) reportLetterBody(r *RotationResult) string {
	var b strings.Builder
	b.WriteString("Прогон обновления паролей завершён.\n\n")
	fmt.Fprintf(&b, "  Сменено паролей: %d\n", r.Changed)
	fmt.Fprintf(&b, "  Не удалось сменить: %d\n", r.Failed)
	fmt.Fprintf(&b, "  Пропущено без адреса почты: %d\n\n", r.SkippedNoMail)

	if len(r.NoMailLogins) > 0 {
		b.WriteString("Работники без адреса почты (пароль не менялся):\n")
		shown := r.NoMailLogins
		if len(shown) > reportLoginsLimit {
			shown = shown[:reportLoginsLimit]
		}
		for _, login := range shown {
			fmt.Fprintf(&b, "  - %s\n", login)
		}
		if len(r.NoMailLogins) > len(shown) {
			fmt.Fprintf(&b, "  ... и ещё %d\n", len(r.NoMailLogins)-len(shown))
		}
		b.WriteString("\nИм нужно указать адрес в карточке работника, иначе обновление паролей\n")
		b.WriteString("будет обходить их и дальше: высылать придуманный пароль некуда. Полный\n")
		b.WriteString("список - в разделе «Пользователи», режим «Без почты».\n")
	}
	return b.String()
}

// notifyAdmins шлёт уведомление тем, у кого есть право на настройки системы:
// плановую смену настраивают там же, и разбираться с её итогом им.
func (s *PasswordRotationService) notifyAdmins(ctx context.Context, title, message string) {
	if s.notifications == nil {
		return
	}
	for _, admin := range s.adminUsers(ctx) {
		if err := s.notifications.CreateForUser(ctx, admin.ID,
			NotificationTypePasswordRotationReport, title, message, nil); err != nil {
			slog.Warn("плановая смена: уведомление администратору не создано",
				"user_id", admin.ID, "error", err)
		}
	}
}

// adminUsers - действующие работники с правом на настройки системы. Право
// спрашиваем у резолвера, того же источника, что стоит за гейтом раздела:
// «пришло уведомление, а разбираться нечем» так не бывает.
func (s *PasswordRotationService) adminUsers(ctx context.Context) []models.User {
	if s.resolver == nil {
		return nil
	}
	var users []models.User
	if err := s.db.WithContext(ctx).
		Where("is_active = ? AND is_banned = ?", true, false).
		Order("id").Find(&users).Error; err != nil {
		slog.Warn("плановая смена: не удалось собрать администраторов", "error", err)
		return nil
	}
	out := make([]models.User, 0, 4)
	for _, u := range users {
		set, err := s.resolver.Resolve(ctx, u.ID)
		if err != nil {
			continue
		}
		if set.Has(KeyPageAdminSettings) {
			out = append(out, u)
		}
	}
	return out
}

// RotateOne меняет пароль одному работнику по кнопке в его карточке и шлёт
// письмо. Закрывает случай «работник потерял пароль», ради которого пароль
// сейчас придумывают руками и диктуют по телефону.
func (s *PasswordRotationService) RotateOne(ctx context.Context, username string, startedBy int) error {
	if s.mail == nil || !s.mail.Enabled() {
		return ErrRotationMailNotConfigured
	}
	var u models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return fmt.Errorf("работник %q не найден: %w", username, err)
	}
	// Архивные и заблокированные исключены здесь так же, как в прогоне обновления:
	// входить им некуда, а смена пароля отправила бы человеку письмо с доступом,
	// которого у него нет. В интерфейсе кнопка у таких учётных записей и не
	// показывается, но проверка нужна и на стороне сервера - иначе она обходится
	// прямым запросом.
	if !u.IsActive {
		return fmt.Errorf("учётная запись %q в архиве, пароль не меняется", username)
	}
	if u.IsBanned {
		return fmt.Errorf("учётная запись %q заблокирована, пароль не меняется", username)
	}
	if u.Email == nil || *u.Email == "" {
		return fmt.Errorf("у работника %q не указан адрес почты, отправить новый пароль некуда", username)
	}
	policy := s.settings.GetPasswordPolicy()
	if err := s.rotateOne(ctx, u, policy, "сброс пароля из карточки работника"); err != nil {
		return err
	}
	s.notifyRotated(ctx, u)
	slog.Info("пароль сменён по кнопке в карточке", "user_id", u.ID, "started_by", startedBy)
	return nil
}

// NotifyExpiring предупреждает тех, у кого срок истекает в ближайшие
// rotation_notify_days_before суток. Письмо без пароля: оно уходит заранее и
// может пролежать в ящике неделю.
//
// Адрес почты обязателен - это и есть канал предупреждения. Работник без адреса
// узнает об истечении на входе, когда система попросит задать новый пароль.
func (s *PasswordRotationService) NotifyExpiring(ctx context.Context) {
	policy := s.settings.GetPasswordPolicy()
	if !policy.RotationEnabled || policy.RotationNotifyDaysBefore <= 0 {
		return
	}
	if s.mail == nil || !s.mail.Enabled() {
		return
	}

	from := time.Now().AddDate(0, 0, -policy.RotationDays)
	to := time.Now().AddDate(0, 0, -(policy.RotationDays - policy.RotationNotifyDaysBefore))

	var users []models.User
	if err := s.db.WithContext(ctx).
		Where("is_active = ? AND is_banned = ?", true, false).
		Where("email IS NOT NULL AND email <> ''").
		Where("password_changed_at IS NOT NULL AND password_changed_at >= ? AND password_changed_at < ?", from, to).
		Order("id").Find(&users).Error; err != nil {
		slog.Error("предупреждение об истечении пароля: отбор не удался", "error", err)
		return
	}

	for _, u := range users {
		if s.alreadyWarned(ctx, u.ID) {
			continue
		}
		expiresAt := u.PasswordChangedAt.AddDate(0, 0, policy.RotationDays)
		letter := MailMessage{
			To:           *u.Email,
			Subject:      "Пароль в системе бюро пропусков скоро истекает",
			Body:         s.expiringLetterBody(u, expiresAt),
			TemplateCode: MailTemplatePasswordExpiring,
			UserID:       &u.ID,
		}
		if err := s.mail.Enqueue(ctx, nil, letter); err != nil {
			slog.Warn("предупреждение об истечении пароля: письмо не поставлено", "user_id", u.ID, "error", err)
			continue
		}
		if s.notifications != nil {
			msg := fmt.Sprintf("Срок действия пароля истекает %s. Смените его заранее, иначе при следующем входе система попросит сделать это сразу.",
				expiresAt.Format("02.01.2006"))
			if err := s.notifications.CreateForUser(ctx, u.ID, NotificationTypePasswordExpiring,
				"Пароль скоро истечёт", msg, nil); err != nil {
				slog.Warn("предупреждение об истечении пароля: уведомление не создано", "user_id", u.ID, "error", err)
			}
		}
	}
}

// alreadyWarned защищает от повторов при перезапуске: предупреждение уходит раз
// в сутки на человека. Считаем по уже созданному уведомлению - отдельной
// таблицы отметок для этого заводить незачем.
func (s *PasswordRotationService) alreadyWarned(ctx context.Context, userID int) bool {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", userID, NotificationTypePasswordExpiring).
		Where("created_at > ?", time.Now().Add(-24*time.Hour)).
		Count(&count).Error
	if err != nil {
		// Не смогли проверить - лучше не слать: повтор раздражает сильнее пропуска.
		slog.Warn("предупреждение об истечении пароля: проверка повтора не удалась", "user_id", userID, "error", err)
		return true
	}
	return count > 0
}

func (s *PasswordRotationService) expiringLetterBody(u models.User, expiresAt time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Здравствуйте, %s.\n\n", addressee(u))
	fmt.Fprintf(&b, "Срок действия вашего пароля истекает %s. После этого система при\n", expiresAt.Format("02.01.2006"))
	b.WriteString("следующем входе попросит задать новый пароль и до тех пор никуда\n")
	b.WriteString("не пустит. Прежний пароль остаётся рабочим - им вы и войдёте.\n\n")
	b.WriteString("Чтобы не делать это на бегу, смените пароль заранее: личный кабинет,\n")
	b.WriteString("кнопка «Сменить пароль». Отсчёт срока начнётся заново.\n")
	if s.baseURL != "" {
		fmt.Fprintf(&b, "\nАдрес системы: %s\n", s.baseURL)
	}
	return b.String()
}

// addressee - как обратиться к человеку в письме: по имени, если оно известно,
// иначе по логину.
func addressee(u models.User) string {
	parts := make([]string, 0, 2)
	if u.FirstName != nil && *u.FirstName != "" {
		parts = append(parts, *u.FirstName)
	}
	if u.MiddleName != nil && *u.MiddleName != "" {
		parts = append(parts, *u.MiddleName)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	return u.Username
}

// normalizeBaseURL отбрасывает адрес разработчика: ссылка на localhost в письме
// у получателя не откроется, и лучше без ссылки, чем с нерабочей.
func normalizeBaseURL(raw string) string {
	url := strings.TrimSpace(strings.TrimSuffix(raw, "/"))
	if url == "" {
		return ""
	}
	if strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1") {
		return ""
	}
	return url
}
