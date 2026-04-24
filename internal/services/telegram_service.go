package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"
)

// TelegramService отправляет сообщения в канал/чат через Telegram Bot API.
// Используется best-effort: при неудаче логируем slog.Error, но не блокируем
// основной флоу (bug_report уже записан в БД).
type TelegramService interface {
	SendBugReport(ctx context.Context, report *models.BugReport, username string) error
}

type telegramService struct {
	botToken   string
	chatID     string
	apiBaseURL string
	httpClient *http.Client
	// backoffBase - стартовый интервал retry. В проде 1с, в тестах выставляется
	// через newTelegramServiceForTest для ускорения.
	backoffBase time.Duration
}

const telegramAPIBaseURL = "https://api.telegram.org"

// NewTelegramService создаёт TG-клиент. Если botToken или chatID пустые -
// возвращает nil-подобный сервис, который в SendBugReport логирует предупреждение
// и возвращает nil. Это позволяет запускаться без TG в dev-окружении.
func NewTelegramService(botToken, chatID string) TelegramService {
	return &telegramService{
		botToken:    botToken,
		chatID:      chatID,
		apiBaseURL:  telegramAPIBaseURL,
		backoffBase: time.Second,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type telegramSendRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// SendBugReport форматирует отчёт и отправляет в TG. Retry до 3 попыток с exp backoff.
// Если botToken/chatID не настроены - только warn и return nil (не error), чтобы
// bug_report сохранялся в БД даже без TG-интеграции.
func (s *telegramService) SendBugReport(ctx context.Context, report *models.BugReport, username string) error {
	if s.botToken == "" || s.chatID == "" {
		slog.Warn("telegram not configured, bug_report stored only in db",
			"bug_hash", report.BugHash)
		return nil
	}

	text := formatBugReport(report, username)
	payload := telegramSendRequest{
		ChatID:    s.chatID,
		Text:      text,
		ParseMode: "Markdown",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", s.apiBaseURL, s.botToken)
	backoff := s.backoffBase
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		err := s.sendOnce(ctx, url, body)
		if err == nil {
			return nil
		}
		lastErr = err
		slog.Warn("telegram send failed, will retry",
			"attempt", attempt,
			"error", err,
			"bug_hash", report.BugHash)
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}
	}
	return fmt.Errorf("telegram send failed after 3 attempts: %w", lastErr)
}

func (s *telegramService) sendOnce(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("unexpected status: %d", resp.StatusCode)
}

// formatBugReport - Markdown-форматирование для TG.
// Экранирование Markdown в Telegram упрощено: сообщения от клиентов
// не содержат пользовательского ввода (кроме UserAgent), поэтому
// достаточно базовой проверки на backticks в message.
func formatBugReport(r *models.BugReport, username string) string {
	return fmt.Sprintf(
		"*Bug report* `%s`\n\n"+
			"*Пользователь:* %s (id=%d)\n"+
			"*Маршрут:* `%s`\n"+
			"*Статус:* %d\n"+
			"*Сообщение:* %s\n"+
			"*User-Agent:* %s\n"+
			"*Время:* %s",
		r.BugHash,
		sanitizeForMarkdown(username),
		r.UserID,
		sanitizeForMarkdown(r.Route),
		r.HTTPStatus,
		sanitizeForMarkdown(r.Message),
		sanitizeForMarkdown(r.UserAgent),
		r.CreatedAt.Format("2006-01-02 15:04:05 MST"),
	)
}

// sanitizeForMarkdown удаляет символы, ломающие TG Markdown.
func sanitizeForMarkdown(s string) string {
	// Убираем backticks и звёздочки - достаточно для целостности форматирования
	replacer := map[rune]rune{'`': '\'', '*': '·'}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if repl, ok := replacer[r]; ok {
			out = append(out, repl)
		} else {
			out = append(out, r)
		}
	}
	return string(out)
}
