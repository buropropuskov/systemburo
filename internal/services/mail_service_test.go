package services

import (
	"errors"
	"strings"
	"testing"
	"time"

	"systemburo/internal/config"
)

func mailTestConfig() *config.Config {
	return &config.Config{
		SMTPHost:          "smtp.example.org",
		SMTPPort:          587,
		SMTPFrom:          "bureau@example.org",
		SMTPFromName:      "Бюро пропусков",
		SMTPTLSMode:       "starttls",
		SMTPTimeoutSec:    15,
		SMTPRatePerHour:   400,
		MailRetryAttempts: 5,
	}
}

// TestMailBackoff_GrowsAndSaturates: пауза растёт с каждой попыткой и упирается
// в потолок, а не уходит в бесконечность и не сбрасывается в ноль.
func TestMailBackoff_GrowsAndSaturates(t *testing.T) {
	prev := time.Duration(0)
	for attempt := 1; attempt <= len(mailRetryBackoff); attempt++ {
		got := mailBackoff(attempt)
		if got <= prev {
			t.Fatalf("попытка %d: пауза %s не больше предыдущей %s", attempt, got, prev)
		}
		prev = got
	}
	// За пределами таблицы повторяется последнее значение.
	last := mailRetryBackoff[len(mailRetryBackoff)-1]
	for _, attempt := range []int{len(mailRetryBackoff) + 1, 99} {
		if got := mailBackoff(attempt); got != last {
			t.Errorf("попытка %d: ожидали %s, получили %s", attempt, last, got)
		}
	}
	// Нулевая и отрицательная попытка не должны паниковать на индексе.
	if got := mailBackoff(0); got != mailRetryBackoff[0] {
		t.Errorf("попытка 0: ожидали %s, получили %s", mailRetryBackoff[0], got)
	}
}

// TestHourlyRateLimiter_StopsAtLimit: потолок отправки соблюдается, иначе первая
// же массовая рассылка выбирает часовой лимит провайдера и он начинает отвечать
// отказом на всё подряд.
func TestHourlyRateLimiter_StopsAtLimit(t *testing.T) {
	l := newHourlyRateLimiter(3)
	for i := 0; i < 3; i++ {
		if !l.allow() {
			t.Fatalf("отправка %d должна быть разрешена", i+1)
		}
	}
	if l.allow() {
		t.Error("четвёртая отправка должна упереться в потолок")
	}
}

// TestHourlyRateLimiter_ForgetsOldSends: окно скользящее - отправки старше часа
// перестают занимать место.
func TestHourlyRateLimiter_ForgetsOldSends(t *testing.T) {
	l := newHourlyRateLimiter(2)
	l.sent = []time.Time{
		time.Now().Add(-2 * time.Hour),
		time.Now().Add(-90 * time.Minute),
	}
	if !l.allow() {
		t.Fatal("отправки старше часа не должны занимать место в окне")
	}
}

// TestHourlyRateLimiter_ZeroLimitDisabled: нулевой потолок означает «без
// ограничения», а не «ничего не отправлять».
func TestHourlyRateLimiter_ZeroLimitDisabled(t *testing.T) {
	l := newHourlyRateLimiter(0)
	for i := 0; i < 100; i++ {
		if !l.allow() {
			t.Fatalf("нулевой потолок не должен ограничивать (отправка %d)", i+1)
		}
	}
}

// TestExplainMailError_NamesTheCause: администратор должен видеть причину, а не
// код. Проверяем, что типовые отказы разбираются по существу.
func TestExplainMailError_NamesTheCause(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		expect string
	}{
		{"неверный пароль", errors.New("535 5.7.8 Error: authentication failed"), "логин или пароль"},
		{"чужой отправитель", errors.New("550 5.7.1 Sender address rejected"), "отправителя"},
		{"закрытый порт", errors.New("dial tcp 1.2.3.4:587: i/o timeout"), "закрытый исходящий порт"},
		{"нет хоста", errors.New("dial tcp: lookup smtp.example.org: no such host"), "подключиться"},
		{"проблема TLS", errors.New("tls: first record does not look like a TLS handshake"), "SMTP_TLS_MODE"},
		{"почта выключена", ErrMailDisabled, "не настроена"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExplainMailError(tc.err)
			if !strings.Contains(got, tc.expect) {
				t.Errorf("ожидали упоминание %q, получили: %s", tc.expect, got)
			}
		})
	}
	if ExplainMailError(nil) != "" {
		t.Error("nil-ошибка не должна давать текст")
	}
}

// TestExplainMailError_KeepsServerAnswer: свой текст не должен прятать ответ
// сервера - без него разбор нетипового отказа упирается в догадки.
func TestExplainMailError_KeepsServerAnswer(t *testing.T) {
	raw := "521 5.2.1 mailbox disabled by administrator"
	if got := ExplainMailError(errors.New(raw)); !strings.Contains(got, raw) {
		t.Errorf("ответ сервера потерян: %s", got)
	}
}

// TestBuildMessage_UsesConfiguredSender: отправитель собирается из параметров,
// адрес получателя проверяется.
func TestBuildMessage_UsesConfiguredSender(t *testing.T) {
	s := &mailService{cfg: mailTestConfig()}

	letter, err := s.buildMessage(MailMessage{To: "worker@example.org", Subject: "Тема", Body: "Текст"})
	if err != nil {
		t.Fatalf("письмо не собралось: %v", err)
	}
	from := letter.GetFromString()
	if len(from) != 1 || !strings.Contains(from[0], "bureau@example.org") {
		t.Errorf("отправитель собран неверно: %v", from)
	}
	to := letter.GetToString()
	if len(to) != 1 || !strings.Contains(to[0], "worker@example.org") {
		t.Errorf("получатель собран неверно: %v", to)
	}

	if _, err := s.buildMessage(MailMessage{To: "не адрес", Subject: "Тема"}); err == nil {
		t.Error("некорректный адрес получателя должен давать ошибку")
	}
}

// TestNewClient_TLSModes: каждый режим шифрования собирает клиент без ошибки, а
// неизвестный режим до сервиса не доходит - его отсекает валидация конфигурации.
func TestNewClient_TLSModes(t *testing.T) {
	for _, mode := range []string{"starttls", "tls", "none"} {
		t.Run(mode, func(t *testing.T) {
			cfg := mailTestConfig()
			cfg.SMTPTLSMode = mode
			s := &mailService{cfg: cfg}
			if _, err := s.newClient(); err != nil {
				t.Errorf("режим %s: клиент не собрался: %v", mode, err)
			}
		})
	}
}

// TestTruncateMailError_KeepsHead: текст отказа режется под размер колонки, но
// начало (код ответа) сохраняется.
func TestTruncateMailError_KeepsHead(t *testing.T) {
	long := "550 " + strings.Repeat("длинный ответ сервера ", 100)
	got := truncateMailError(long)
	if len([]rune(got)) != 500 {
		t.Errorf("ожидали 500 рун, получили %d", len([]rune(got)))
	}
	if !strings.HasPrefix(got, "550 ") {
		t.Errorf("код ответа потерян: %s", got[:20])
	}
	short := "550 отказ"
	if got := truncateMailError(short); got != short {
		t.Errorf("короткий текст не должен меняться: %s", got)
	}
}

// TestMailDisabled_RefusesLoudly: при пустом SMTP_HOST постановка в очередь и
// отправка отказывают явной ошибкой. Тихий пропуск здесь недопустим: плановая
// смена паролей обязана остановиться, а не менять пароли в пустоту.
func TestMailDisabled_RefusesLoudly(t *testing.T) {
	cfg := mailTestConfig()
	cfg.SMTPHost = ""
	s := NewMailService(nil, cfg)

	if s.Enabled() {
		t.Fatal("пустой SMTP_HOST должен означать выключенную почту")
	}
	if err := s.Enqueue(t.Context(), nil, MailMessage{To: "a@example.org"}); !errors.Is(err, ErrMailDisabled) {
		t.Errorf("Enqueue: ожидали ErrMailDisabled, получили %v", err)
	}
	if err := s.SendNow(t.Context(), MailMessage{To: "a@example.org"}); !errors.Is(err, ErrMailDisabled) {
		t.Errorf("SendNow: ожидали ErrMailDisabled, получили %v", err)
	}
	if sent, failed := s.ProcessQueue(t.Context()); sent != 0 || failed != 0 {
		t.Errorf("ProcessQueue при выключенной почте: ожидали 0/0, получили %d/%d", sent, failed)
	}
}

// TestEnqueue_RequiresRecipient: письмо без адреса не должно попадать в очередь -
// иначе воркер будет вечно возить его по кругу и отчёт наполнится мусором.
func TestEnqueue_RequiresRecipient(t *testing.T) {
	s := NewMailService(nil, mailTestConfig())
	err := s.Enqueue(t.Context(), nil, MailMessage{TemplateCode: "test"})
	if err == nil {
		t.Fatal("ожидали ошибку на пустом адресе")
	}
	if !strings.Contains(err.Error(), "адреса") {
		t.Errorf("текст ошибки должен называть причину, получили: %v", err)
	}
}

// TestMailTemplateCodesUnique: коды шаблонов уходят в базу и в отчёты, дубль
// означал бы два разных письма под одним именем.
func TestMailTemplateCodesUnique(t *testing.T) {
	codes := []string{MailTemplateTest}
	seen := map[string]bool{}
	for _, c := range codes {
		if c == "" {
			t.Fatal("пустой код шаблона")
		}
		if seen[c] {
			t.Fatalf("код шаблона %q объявлен дважды", c)
		}
		seen[c] = true
	}
}
