package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// ErrBugAlreadyReported возвращается когда юзер уже отправил репорт на этот bug_hash.
// Маппится в handler'е в 409 Conflict.
var ErrBugAlreadyReported = errors.New("bug already reported by this user")

// BugReportService принимает репорт со страницы Error500 и:
//  1. Проверяет per-user rate limit (3 репорта за 5 минут).
//  2. INSERT .. ON CONFLICT DO NOTHING (uniq на user_id+bug_hash).
//  3. Если вставка прошла - асинхронно отправляет в Telegram (fire-and-forget).
type BugReportService interface {
	Submit(ctx context.Context, userID int, username string, req models.BugReportRequest, userAgent string) (*models.BugReport, error)
}

type bugReportService struct {
	db    *gorm.DB
	tg    TelegramService
	limit int           // сколько репортов разрешено за окно
	win   time.Duration // окно rate limit
}

// NewBugReportService конструирует сервис. Rate limit: 3 репорта за 5 минут на юзера.
// Значения фиксированы - тут нет смысла выносить в env.
func NewBugReportService(db *gorm.DB, tg TelegramService) BugReportService {
	return &bugReportService{
		db:    db,
		tg:    tg,
		limit: 3,
		win:   5 * time.Minute,
	}
}

func (s *bugReportService) Submit(ctx context.Context, userID int, username string, req models.BugReportRequest, userAgent string) (*models.BugReport, error) {
	// Rate limit: считаем последние N запросов за окно.
	since := time.Now().UTC().Add(-s.win)
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&models.BugReport{}).
		Where("user_id = ? AND created_at > ?", userID, since).
		Count(&count).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "не удалось проверить частоту репортов")
	}
	if count >= int64(s.limit) {
		return nil, echo.NewHTTPError(http.StatusTooManyRequests,
			fmt.Sprintf("лимит: %d репортов за %s", s.limit, s.win))
	}

	report := &models.BugReport{
		UserID:     userID,
		BugHash:    req.BugHash,
		Route:      req.Route,
		HTTPStatus: req.HTTPStatus,
		Message:    req.Message,
		UserAgent:  userAgent,
		CreatedAt:  time.Now().UTC(),
	}

	// INSERT .. ON CONFLICT DO NOTHING эквивалент: Create с ошибкой по uniq index.
	// GORM возвращает ошибку Duplicate key - проверяем по RowsAffected=0 после
	// Clauses(OnConflict{DoNothing}).
	res := s.db.WithContext(ctx).
		Exec(`INSERT INTO bug_reports (user_id, bug_hash, route, http_status, message, user_agent, created_at)
		      VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (user_id, bug_hash) DO NOTHING`,
			report.UserID, report.BugHash, report.Route, report.HTTPStatus,
			report.Message, report.UserAgent, report.CreatedAt)
	if res.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "не удалось сохранить репорт")
	}
	if res.RowsAffected == 0 {
		return nil, ErrBugAlreadyReported
	}

	// Подгрузим ID свежесозданной записи (нужно для ответа и TG).
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND bug_hash = ?", report.UserID, report.BugHash).
		First(report).Error; err != nil {
		// Запись точно есть (мы только что вставили) - если тут ошибка,
		// это серьёзно, но TG уже смысла слать нет.
		slog.Error("bug_report saved but readback failed", "error", err)
		return report, nil
	}

	// Fire-and-forget: TG-отправка не должна блокировать API.
	go func() {
		// Свежий context - родительский мог завершиться.
		sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.tg.SendBugReport(sendCtx, report, username); err != nil {
			slog.Error("telegram delivery failed",
				"bug_hash", report.BugHash,
				"user_id", report.UserID,
				"error", err)
		}
	}()

	return report, nil
}
