package handlers_test

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
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

// fakeSMTP - минимальный SMTP-сервер для тестов очереди. Отвечает ровно столько,
// сколько нужно клиенту для отправки письма, и умеет отказывать на RCPT, чтобы
// проверить путь неудачной доставки.
type fakeSMTP struct {
	listener net.Listener
	wg       sync.WaitGroup

	mu       sync.Mutex
	received []string // тела принятых писем
	rejectTo bool     // отвечать 550 на RCPT TO
}

func startFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := &fakeSMTP{listener: ln}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // слушатель закрыт
			}
			go s.handle(conn)
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		s.wg.Wait()
	})
	return s
}

func (s *fakeSMTP) port() int { return s.listener.Addr().(*net.TCPAddr).Port }

func (s *fakeSMTP) setRejectTo(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rejectTo = v
}

func (s *fakeSMTP) messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.received...)
}

func (s *fakeSMTP) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)
	write := func(format string, args ...any) {
		_, _ = fmt.Fprintf(conn, format+"\r\n", args...)
	}

	write("220 fake ESMTP")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			// Ни STARTTLS, ни AUTH не объявляем: тестовый режим - none, без
			// аутентификации, и клиент не должен их требовать.
			write("250-fake\r\n250 SIZE 10485760")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			write("250 OK")
		case strings.HasPrefix(cmd, "RCPT TO"):
			s.mu.Lock()
			reject := s.rejectTo
			s.mu.Unlock()
			if reject {
				write("550 5.7.1 Sender address rejected")
				continue
			}
			write("250 OK")
		case strings.HasPrefix(cmd, "DATA"):
			write("354 End data with <CR><LF>.<CR><LF>")
			var body strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(dataLine, "\r\n") == "." {
					break
				}
				body.WriteString(dataLine)
			}
			s.mu.Lock()
			s.received = append(s.received, body.String())
			s.mu.Unlock()
			write("250 OK: queued")
		case strings.HasPrefix(cmd, "RSET"):
			write("250 OK")
		case strings.HasPrefix(cmd, "NOOP"):
			write("250 OK")
		case strings.HasPrefix(cmd, "QUIT"):
			write("221 Bye")
			return
		default:
			write("500 unknown command")
		}
	}
}

// mailCfg собирает конфигурацию под тестовый сервер: без шифрования и без
// аутентификации, всё остальное как в бою.
func mailCfg(port int, retries int) *config.Config {
	return &config.Config{
		SMTPHost:          "127.0.0.1",
		SMTPPort:          port,
		SMTPFrom:          "bureau@example.org",
		SMTPFromName:      "Бюро пропусков",
		SMTPTLSMode:       "none",
		SMTPTimeoutSec:    5,
		SMTPRatePerHour:   100,
		MailRetryAttempts: retries,
	}
}

func loadMessage(t *testing.T, db *gorm.DB, id int) models.EmailMessage {
	t.Helper()
	var row models.EmailMessage
	require.NoError(t, db.First(&row, id).Error)
	return row
}

// TestMailQueue_DeliversAndMarksSent: письмо из очереди доходит до сервера, тело
// сохраняется целиком, строка помечается отправленной.
func TestMailQueue_DeliversAndMarksSent(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	srv := startFakeSMTP(t)
	svc := services.NewMailService(db, mailCfg(srv.port(), 5))

	require.NoError(t, svc.Enqueue(t.Context(), nil, services.MailMessage{
		To:           "worker@example.org",
		Subject:      "Плановая смена пароля",
		Body:         "Ваш новый пароль: секрет",
		TemplateCode: "password_rotated",
	}))

	sent, failed := svc.ProcessQueue(t.Context())
	assert.Equal(t, 1, sent)
	assert.Equal(t, 0, failed)

	msgs := srv.messages()
	require.Len(t, msgs, 1, "сервер должен принять одно письмо")
	assert.Contains(t, msgs[0], "worker@example.org")

	var row models.EmailMessage
	require.NoError(t, db.Where("to_address = ?", "worker@example.org").First(&row).Error)
	assert.Equal(t, models.EmailStatusSent, row.Status)
	assert.Equal(t, 1, row.Attempts)
	require.NotNil(t, row.SentAt)
	assert.Empty(t, row.LastError)
}

// TestMailQueue_RetriesOnRejection: отказ сервера не теряет письмо - оно остаётся
// в очереди с отложенной следующей попыткой и записанной причиной.
func TestMailQueue_RetriesOnRejection(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	srv := startFakeSMTP(t)
	srv.setRejectTo(true)
	svc := services.NewMailService(db, mailCfg(srv.port(), 5))

	require.NoError(t, svc.Enqueue(t.Context(), nil, services.MailMessage{
		To: "worker@example.org", Subject: "Тема", Body: "Текст", TemplateCode: "test",
	}))

	sent, failed := svc.ProcessQueue(t.Context())
	assert.Equal(t, 0, sent)
	assert.Equal(t, 1, failed)

	var row models.EmailMessage
	require.NoError(t, db.Where("to_address = ?", "worker@example.org").First(&row).Error)
	assert.Equal(t, models.EmailStatusPending, row.Status, "письмо должно остаться в очереди")
	assert.Equal(t, 1, row.Attempts)
	require.NotNil(t, row.NextAttemptAt, "следующая попытка должна быть отложена")
	assert.True(t, row.NextAttemptAt.After(time.Now()), "повтор не должен быть немедленным")
	assert.Contains(t, row.LastError, "550")
}

// TestMailQueue_GivesUpAfterAttempts: исчерпав попытки, письмо помечается
// недоставленным, а не крутится в очереди вечно.
func TestMailQueue_GivesUpAfterAttempts(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	srv := startFakeSMTP(t)
	srv.setRejectTo(true)
	svc := services.NewMailService(db, mailCfg(srv.port(), 1))

	require.NoError(t, svc.Enqueue(t.Context(), nil, services.MailMessage{
		To: "worker@example.org", Subject: "Тема", Body: "Текст", TemplateCode: "test",
	}))

	svc.ProcessQueue(t.Context())

	var row models.EmailMessage
	require.NoError(t, db.Where("to_address = ?", "worker@example.org").First(&row).Error)
	assert.Equal(t, models.EmailStatusFailed, row.Status)
	assert.Nil(t, row.NextAttemptAt, "у отказавшегося письма повтора быть не должно")
}

// TestMailQueue_RespectsNextAttempt: письмо, чей срок повтора ещё не настал,
// в текущий проход не берётся.
func TestMailQueue_RespectsNextAttempt(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	srv := startFakeSMTP(t)
	svc := services.NewMailService(db, mailCfg(srv.port(), 5))

	future := time.Now().Add(time.Hour)
	row := models.EmailMessage{
		ToAddress: "later@example.org", Subject: "Позже", Body: "Текст",
		TemplateCode: "test", Status: models.EmailStatusPending,
		Attempts: 1, NextAttemptAt: &future,
	}
	require.NoError(t, db.Create(&row).Error)

	sent, failed := svc.ProcessQueue(t.Context())
	assert.Equal(t, 0, sent)
	assert.Equal(t, 0, failed)
	assert.Empty(t, srv.messages(), "письмо со сроком в будущем брать рано")
}

// TestMailQueue_EnqueueJoinsCallerTransaction: письмо ставится в очередь в
// транзакции вызывающего. Ради этого очередь и заведена: смена пароля и письмо о
// ней должны попасть на диск вместе либо не попасть вовсе.
func TestMailQueue_EnqueueJoinsCallerTransaction(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	srv := startFakeSMTP(t)
	svc := services.NewMailService(db, mailCfg(srv.port(), 5))

	// Транзакция вызывающего откатывается - письма быть не должно.
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := svc.Enqueue(t.Context(), tx, services.MailMessage{
			To: "rollback@example.org", Subject: "Тема", Body: "Текст", TemplateCode: "test",
		}); err != nil {
			return err
		}
		return fmt.Errorf("вызывающий передумал")
	})
	require.Error(t, err)

	var count int64
	require.NoError(t, db.Model(&models.EmailMessage{}).
		Where("to_address = ?", "rollback@example.org").Count(&count).Error)
	assert.EqualValues(t, 0, count, "откат транзакции должен унести и письмо")

	// Та же транзакция, но успешная - письмо на месте.
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return svc.Enqueue(t.Context(), tx, services.MailMessage{
			To: "committed@example.org", Subject: "Тема", Body: "Текст", TemplateCode: "test",
		})
	}))
	require.NoError(t, db.Model(&models.EmailMessage{}).
		Where("to_address = ?", "committed@example.org").Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

// TestMailStatus_ReportsNotConfigured: пока почта не настроена, интерфейс обязан
// это знать - иначе администратор включит плановую рассылку вслепую.
func TestMailStatus_ReportsNotConfigured(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/api/settings/mail/status", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"configured":false`)
}

// TestSendTestMail_RefusesWhenNotConfigured: проверочное письмо при ненастроенной
// почте отвечает понятной причиной, а не таймаутом.
func TestSendTestMail_RefusesWhenNotConfigured(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/api/settings/mail/test", `{"to":"admin@example.org"}`,
		testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "не настроена")
}

// TestSendTestMail_ValidatesAddress: адрес проверяется до попытки отправки.
func TestSendTestMail_ValidatesAddress(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/api/settings/mail/test", `{"to":"не адрес"}`,
		testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestMailRoutes_RequirePermission: обе ручки закрыты правом настроек. Обычный
// работник не должен ни знать параметры почты, ни рассылать письма от системы.
func TestMailRoutes_RequirePermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userToken := testutil.RegisterAndLogin(t, e, "mail_plain_user", "password12345678", 1, td.OrgID, td.CompanyID)

	statusRec := testutil.GET(t, e, "/api/settings/mail/status", testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, statusRec.Code)

	testRec := testutil.POST(t, e, "/api/settings/mail/test", `{"to":"admin@example.org"}`,
		testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, testRec.Code)
}
