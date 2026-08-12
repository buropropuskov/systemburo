package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/models"

	"github.com/wneessen/go-mail"
	"gorm.io/gorm"
)

// mailRetryBackoff - через сколько пробовать снова после очередного отказа.
// Последнее значение повторяется, если попыток настроено больше: минута ловит
// моргнувшую сеть, час - обслуживание почтового сервера.
var mailRetryBackoff = []time.Duration{
	time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	time.Hour,
}

// mailBatchSize - сколько писем берётся за один проход воркера. Ограничение не
// про память, а про удержание соединения: пачка отправляется по одному
// SMTP-соединению, и слишком длинная пачка рискует получить таймаут сервера.
const mailBatchSize = 50

// MailMessage - письмо, которое ставят в очередь.
type MailMessage struct {
	To           string
	Subject      string
	Body         string
	TemplateCode string
	UserID       *int
}

// MailSender - постановка писем в очередь и разбор очереди. Узкий интерфейс:
// вызывающим сервисам нужен только Enqueue, воркеру - ProcessQueue.
type MailSender interface {
	// Enqueue кладёт письмо в очередь. exec - транзакция вызывающего: письмо
	// обязано попасть на диск вместе с событием, которое его породило, иначе
	// смена пароля переживёт сбой, а письмо о ней - нет. nil означает "вне
	// транзакции", тогда используется общее соединение.
	Enqueue(ctx context.Context, exec *gorm.DB, msg MailMessage) error
	// SendNow отправляет письмо мимо очереди и возвращает ошибку сервера как есть.
	// Нужен единственному сценарию - проверочному письму при настройке почты:
	// там человек стоит перед экраном и ждёт ответа, а не отчёта через минуту.
	SendNow(ctx context.Context, msg MailMessage) error
	// ProcessQueue разбирает пачку писем, возвращает число отправленных и упавших.
	ProcessQueue(ctx context.Context) (sent int, failed int)
	// Enabled сообщает, настроена ли почта.
	Enabled() bool
}

type mailService struct {
	db  *gorm.DB
	cfg *config.Config
	// limiter держит темп отправки ниже лимита почтового провайдера.
	limiter *hourlyRateLimiter
}

// NewMailService создаёт сервис почты. Работает и с ненастроенной почтой:
// Enqueue тогда отказывает явной ошибкой, а воркер не запускается.
func NewMailService(db *gorm.DB, cfg *config.Config) MailSender {
	return &mailService{
		db:      db,
		cfg:     cfg,
		limiter: newHourlyRateLimiter(cfg.SMTPRatePerHour),
	}
}

func (s *mailService) Enabled() bool { return s.cfg.MailEnabled() }

// ErrMailDisabled возвращается, когда почта не настроена. Отдельная ошибка, а не
// тихий пропуск: вызывающий (плановая смена паролей) обязан остановиться, а не
// менять пароли в пустоту.
var ErrMailDisabled = errors.New("почта не настроена: задан пустой SMTP_HOST")

func (s *mailService) Enqueue(ctx context.Context, exec *gorm.DB, msg MailMessage) error {
	if !s.Enabled() {
		return ErrMailDisabled
	}
	if msg.To == "" {
		return fmt.Errorf("письмо без адреса получателя (шаблон %q)", msg.TemplateCode)
	}
	db := s.db
	if exec != nil {
		db = exec
	}
	row := models.EmailMessage{
		ToAddress:    msg.To,
		UserID:       msg.UserID,
		TemplateCode: msg.TemplateCode,
		Subject:      msg.Subject,
		Body:         msg.Body,
		Status:       models.EmailStatusPending,
	}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("не удалось поставить письмо в очередь: %w", err)
	}
	return nil
}

func (s *mailService) SendNow(ctx context.Context, msg MailMessage) error {
	if !s.Enabled() {
		return ErrMailDisabled
	}
	client, err := s.newClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	letter, err := s.buildMessage(msg)
	if err != nil {
		return err
	}
	if err := client.DialAndSendWithContext(ctx, letter); err != nil {
		return fmt.Errorf("почтовый сервер отклонил письмо: %w", err)
	}
	return nil
}

// ProcessQueue отправляет пачку писем по одному соединению. Ошибка отдельного
// письма не прерывает пачку: недоступный адрес одного человека не должен
// задерживать рассылку остальным.
func (s *mailService) ProcessQueue(ctx context.Context) (int, int) {
	if !s.Enabled() {
		return 0, 0
	}
	rows, err := s.claimBatch(ctx)
	if err != nil {
		slog.Error("почта: не удалось выбрать письма из очереди", "error", err)
		return 0, 0
	}
	if len(rows) == 0 {
		return 0, 0
	}

	client, err := s.newClient()
	if err != nil {
		slog.Error("почта: неверные параметры подключения", "error", err)
		return 0, 0
	}
	if err := client.DialWithContext(ctx); err != nil {
		// Сервер недоступен целиком - письма остаются в очереди, отметим отказ,
		// чтобы следующий проход не долбился в ту же секунду.
		slog.Error("почта: не удалось подключиться к серверу", "host", s.cfg.SMTPHost, "error", err)
		for i := range rows {
			s.markFailure(ctx, &rows[i], err)
		}
		return 0, len(rows)
	}
	defer func() { _ = client.Close() }()

	sent, failed := 0, 0
	for i := range rows {
		row := &rows[i]
		if !s.limiter.allow() {
			// Потолок отправки выбран - остаток пачки ждёт следующего прохода.
			slog.Info("почта: достигнут потолок отправки в час, остаток ждёт", "limit", s.cfg.SMTPRatePerHour)
			break
		}
		letter, err := s.buildMessage(MailMessage{
			To:           row.ToAddress,
			Subject:      row.Subject,
			Body:         row.Body,
			TemplateCode: row.TemplateCode,
		})
		if err != nil {
			s.markFailure(ctx, row, err)
			failed++
			continue
		}
		if err := client.Send(letter); err != nil {
			s.markFailure(ctx, row, err)
			failed++
			continue
		}
		s.markSent(ctx, row)
		sent++
	}
	return sent, failed
}

// claimBatch выбирает письма, которым пора: ни разу не пробованные и те, чей срок
// повтора настал. Блокировки строк нет намеренно - разбирает очередь единственная
// горутина единственной реплики. Появится вторая - сюда добавится FOR UPDATE SKIP
// LOCKED, иначе оба процесса отправят одни и те же письма.
func (s *mailService) claimBatch(ctx context.Context) ([]models.EmailMessage, error) {
	var rows []models.EmailMessage
	err := s.db.WithContext(ctx).
		Where("status = ?", models.EmailStatusPending).
		Where("next_attempt_at IS NULL OR next_attempt_at <= ?", time.Now()).
		Order("created_at").
		Limit(mailBatchSize).
		Find(&rows).Error
	return rows, err
}

// markSent помечает письмо отправленным и СТИРАЕТ его текст.
//
// Текст нужен ровно до отправки. Дальше он остаётся копией того, что уже ушло
// адресату, и для писем о пароле это пароль открытым текстом - в базе, которая
// по всем прочим правилам хранит пароли только вычислением Argon2id. Очередь
// ничего не удаляет по сроку, поэтому без стирания такая копия жила бы вечно.
//
// Остальное о письме сохраняется: адрес, тема, шаблон, время отправки, число
// попыток. По ним разбирают доставку, а текст для этого не нужен.
func (s *mailService) markSent(ctx context.Context, row *models.EmailMessage) {
	now := time.Now()
	err := s.db.WithContext(ctx).Model(&models.EmailMessage{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"status":     models.EmailStatusSent,
			"sent_at":    now,
			"attempts":   row.Attempts + 1,
			"last_error": "",
			"body":       "",
		}).Error
	if err != nil {
		slog.Error("почта: письмо отправлено, но статус не записан", "id", row.ID, "error", err)
	}
}

// markFailure увеличивает счётчик попыток и отодвигает следующую. Исчерпал
// попытки - статус failed и запись в журнал: молча потерянное письмо с паролем
// означает работника, запертого снаружи, и об этом обязан узнать администратор.
func (s *mailService) markFailure(ctx context.Context, row *models.EmailMessage, cause error) {
	attempts := row.Attempts + 1
	updates := map[string]any{
		"attempts":   attempts,
		"last_error": truncateMailError(cause.Error()),
	}
	if attempts >= s.cfg.MailRetryAttempts {
		updates["status"] = models.EmailStatusFailed
		// Повторов больше не будет - текст письма не нужен, а хранить его нельзя:
		// в письмах о пароле он содержит сам пароль открытым текстом.
		updates["body"] = ""
		slog.Error("почта: письмо не доставлено, попытки исчерпаны",
			"id", row.ID, "to", row.ToAddress, "template", row.TemplateCode, "attempts", attempts, "error", cause)
	} else {
		next := time.Now().Add(mailBackoff(attempts))
		updates["next_attempt_at"] = next
		slog.Warn("почта: отправка не удалась, повторим позже",
			"id", row.ID, "attempts", attempts, "next_attempt_at", next.Format(time.RFC3339), "error", cause)
	}
	if err := s.db.WithContext(ctx).Model(&models.EmailMessage{}).
		Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		slog.Error("почта: не удалось записать неудачную попытку", "id", row.ID, "error", err)
	}
}

// mailBackoff возвращает паузу перед попыткой номер attempts (нумерация с единицы).
func mailBackoff(attempts int) time.Duration {
	idx := attempts - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(mailRetryBackoff) {
		idx = len(mailRetryBackoff) - 1
	}
	return mailRetryBackoff[idx]
}

// truncateMailError режет текст отказа под размер колонки. Начало сообщения
// информативнее хвоста: код ответа сервера идёт первым.
func truncateMailError(s string) string {
	const limit = 500
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit])
}

// newClient собирает SMTP-клиент по параметрам конфигурации.
func (s *mailService) newClient() (*mail.Client, error) {
	opts := []mail.Option{
		mail.WithPort(s.cfg.SMTPPort),
		mail.WithTimeout(time.Duration(s.cfg.SMTPTimeoutSec) * time.Second),
	}

	switch s.cfg.SMTPTLSMode {
	case "tls":
		// WithSSL, а не WithSSLPort: порт задан выше явно, и подмена его на 465
		// вместе с откатом на 25 нам не нужна - молчаливый откат на нешифрованное
		// соединение как раз то, чего здесь нельзя допускать.
		opts = append(opts, mail.WithSSL())
	case "none":
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	default:
		// starttls: шифрование обязательно. TLSMandatory вместо TLSOpportunistic
		// намеренно - в письме плановой смены лежит пароль открытым текстом, и
		// тихий откат на нешифрованное соединение отдал бы его сети.
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	}

	if s.cfg.SMTPUsername != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(s.cfg.SMTPUsername),
			mail.WithPassword(s.cfg.SMTPPassword),
		)
	}

	client, err := mail.NewClient(s.cfg.SMTPHost, opts...)
	if err != nil {
		return nil, fmt.Errorf("не удалось собрать почтовый клиент: %w", err)
	}
	return client, nil
}

// buildMessage собирает письмо. Только текст, без вложений и картинок: письма
// системы читают в том числе на служебных почтовых клиентах организаций, где
// разметку всё равно вырежут.
func (s *mailService) buildMessage(msg MailMessage) (*mail.Msg, error) {
	letter := mail.NewMsg()
	if err := letter.FromFormat(s.cfg.SMTPFromName, s.cfg.SMTPFrom); err != nil {
		return nil, fmt.Errorf("некорректный адрес отправителя %q: %w", s.cfg.SMTPFrom, err)
	}
	if err := letter.To(msg.To); err != nil {
		return nil, fmt.Errorf("некорректный адрес получателя %q: %w", msg.To, err)
	}
	letter.Subject(msg.Subject)
	letter.SetBodyString(mail.TypeTextPlain, msg.Body)
	return letter, nil
}

// hourlyRateLimiter - скользящий счётчик отправленных писем за час.
type hourlyRateLimiter struct {
	limit int
	sent  []time.Time
}

func newHourlyRateLimiter(limit int) *hourlyRateLimiter {
	return &hourlyRateLimiter{limit: limit}
}

// allow отмечает отправку и сообщает, можно ли её делать. Вызывается из
// единственной горутины воркера, поэтому без блокировки.
func (l *hourlyRateLimiter) allow() bool {
	if l.limit <= 0 {
		return true
	}
	cutoff := time.Now().Add(-time.Hour)
	kept := l.sent[:0]
	for _, t := range l.sent {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	l.sent = kept
	if len(l.sent) >= l.limit {
		return false
	}
	l.sent = append(l.sent, time.Now())
	return true
}

// Коды шаблонов писем. Хранятся в очереди, по ним отбирают письма одного вида
// для отчёта администратору и повторной отправки.
const (
	MailTemplateTest = "test"
)

// ExplainMailError переводит отказ почтового сервера в понятный администратору
// текст. Голое "550 5.7.1" в интерфейсе не говорит ничего, а причина у каждого
// кода ровно одна и типовая: сначала проверяют логин, потом отправителя, потом
// доступность порта.
func ExplainMailError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	switch {
	case errors.Is(err, ErrMailDisabled):
		return "Почта не настроена: задан пустой SMTP_HOST"
	case containsAny(text, "535", "authentication failed", "Authentication credentials invalid"):
		return "Почтовый сервер не принял логин или пароль (535). Проверьте SMTP_USERNAME и SMTP_PASSWORD; у Яндекса нужен пароль приложения, а не пароль от учётной записи. Ответ сервера: " + text
	case containsAny(text, "550", "553", "not allowed", "sender"):
		return "Почтовый сервер отказался принимать письмо от этого отправителя (550). SMTP_FROM должен совпадать с ящиком, под которым система входит на сервер. Ответ сервера: " + text
	case containsAny(text, "timeout", "i/o timeout", "deadline exceeded"):
		return "Почтовый сервер не ответил вовремя. Обычно это закрытый исходящий порт: проверьте с сервера доступность " + "SMTP_HOST:SMTP_PORT. Подробности: " + text
	case containsAny(text, "connection refused", "no such host", "lookup"):
		return "Не удалось подключиться к почтовому серверу. Проверьте SMTP_HOST и SMTP_PORT. Подробности: " + text
	case containsAny(text, "tls", "certificate", "x509"):
		return "Не удалось установить защищённое соединение. Проверьте SMTP_TLS_MODE: 587 обычно starttls, 465 - tls. Подробности: " + text
	default:
		return "Письмо отправить не удалось. Ответ сервера: " + text
	}
}

// containsAny сообщает, встречается ли в тексте хотя бы одна из подстрок
// (регистронезависимо).
func containsAny(text string, needles ...string) bool {
	lower := strings.ToLower(text)
	for _, n := range needles {
		if strings.Contains(lower, strings.ToLower(n)) {
			return true
		}
	}
	return false
}
