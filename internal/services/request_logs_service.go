package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RequestLogsService -- интерфейс бизнес-логики логов запросов.
type RequestLogsService interface {
	GetLogs(ctx context.Context, typeID int, q models.RequestLogsQuery) ([]models.RequestLogs, int64, error)
	GetUsers(ctx context.Context, typeID int) ([]models.RequestLogsUser, error)
	GetStats(ctx context.Context, typeID int) (*models.RequestLogsStats, error)
	GetRealtime(ctx context.Context, typeID int) (*models.RealtimeStats, error)
	GetTimeline(ctx context.Context, typeID int, q models.TimelineQuery) ([]models.TimelinePoint, error)
	Export(ctx context.Context, typeID int, q models.RequestLogsQuery) (string, error)
}

type requestLogsService struct {
	db *gorm.DB
}

// NewRequestLogsService создаёт реализацию RequestLogsService.
func NewRequestLogsService(db *gorm.DB) RequestLogsService {
	return &requestLogsService{db: db}
}

func (s *requestLogsService) checkAdmin(_ context.Context, typeID int) error {
	var code string
	err := s.db.
		Table("user_types").
		Select("code").
		Where("id = ?", typeID).
		Row().
		Scan(&code)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}
	if code != "manager" && code != "buropropuskov" {
		return echo.NewHTTPError(http.StatusForbidden, "Insufficient permissions")
	}
	return nil
}

func (s *requestLogsService) applyFilters(tx *gorm.DB, q models.RequestLogsQuery) *gorm.DB {
	if q.UserID != nil {
		tx = tx.Where("user_id = ?", *q.UserID)
	}
	if q.Method != "" {
		tx = tx.Where("method = ?", strings.ToUpper(q.Method))
	}
	if q.Status != nil {
		tx = tx.Where("response_status = ?", *q.Status)
	}
	if q.From != "" {
		if t, err := time.Parse(time.RFC3339, q.From); err == nil {
			tx = tx.Where("created_at >= ?", t)
		}
	}
	if q.To != "" {
		if t, err := time.Parse(time.RFC3339, q.To); err == nil {
			tx = tx.Where("created_at <= ?", t)
		}
	}
	if q.Search != "" {
		pattern := "%" + q.Search + "%"
		tx = tx.Where("(url ILIKE ? OR username ILIKE ?)", pattern, pattern)
	}
	return tx
}

// GetLogs возвращает логи с пагинацией и фильтрацией.
func (s *requestLogsService) GetLogs(ctx context.Context, typeID int, q models.RequestLogsQuery) ([]models.RequestLogs, int64, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, 0, err
	}

	if q.Page < 1 {
		q.Page = 1
	}
	if q.PerPage < 1 {
		q.PerPage = 20
	}
	if q.PerPage > 100 {
		q.PerPage = 100
	}

	tx := s.db.WithContext(ctx).Table("request_logs")
	tx = s.applyFilters(tx, q)

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "failed to count request logs")
	}

	logs := make([]models.RequestLogs, 0)
	offset := (q.Page - 1) * q.PerPage
	if err := tx.Order("created_at DESC").Offset(offset).Limit(q.PerPage).Find(&logs).Error; err != nil {
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch request logs")
	}

	return logs, total, nil
}

// GetUsers возвращает уникальных пользователей из логов.
func (s *requestLogsService) GetUsers(ctx context.Context, typeID int) ([]models.RequestLogsUser, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

	users := make([]models.RequestLogsUser, 0)
	err := s.db.WithContext(ctx).
		Table("request_logs").
		Select("DISTINCT user_id AS id, username").
		Where("user_id IS NOT NULL AND username IS NOT NULL").
		Order("username").
		Scan(&users).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch users")
	}

	return users, nil
}

// GetStats возвращает агрегированную статистику.
func (s *requestLogsService) GetStats(ctx context.Context, typeID int) (*models.RequestLogsStats, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

	var stats models.RequestLogsStats

	// Total
	if err := s.db.WithContext(ctx).Table("request_logs").Count(&stats.Total).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch stats")
	}

	// Today (по UTC -- created_at в БД хранится в UTC).
	todayStart := time.Now().UTC().Truncate(24 * time.Hour)
	s.db.WithContext(ctx).Table("request_logs").Where("created_at >= ?", todayStart).Count(&stats.Today)

	// Avg duration
	s.db.WithContext(ctx).Table("request_logs").
		Select("COALESCE(AVG(duration_ms), 0)").
		Row().
		Scan(&stats.AvgDuration)

	// Error rate (4xx + 5xx)
	if stats.Total > 0 {
		var errorCount int64
		s.db.WithContext(ctx).Table("request_logs").
			Where("response_status >= 400").
			Count(&errorCount)
		stats.ErrorRate = float64(errorCount) / float64(stats.Total) * 100
	}

	// Requests per minute (за последний час)
	hourAgo := time.Now().UTC().Add(-1 * time.Hour)
	var lastHourCount int64
	s.db.WithContext(ctx).Table("request_logs").
		Where("created_at >= ?", hourAgo).
		Count(&lastHourCount)
	stats.RequestsPerMinute = float64(lastHourCount) / 60.0

	return &stats, nil
}

// GetRealtime возвращает количество запросов за последнюю секунду и минуту.
func (s *requestLogsService) GetRealtime(ctx context.Context, typeID int) (*models.RealtimeStats, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var stats models.RealtimeStats

	s.db.WithContext(ctx).Table("request_logs").
		Where("created_at >= ?", now.Add(-1*time.Second)).
		Count(&stats.LastSecondCount)

	s.db.WithContext(ctx).Table("request_logs").
		Where("created_at >= ?", now.Add(-1*time.Minute)).
		Count(&stats.LastMinuteCount)

	return &stats, nil
}

// GetTimeline возвращает точки для графика, сгруппированные по интервалу.
func (s *requestLogsService) GetTimeline(ctx context.Context, typeID int, q models.TimelineQuery) ([]models.TimelinePoint, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

	if q.Interval < 1 {
		q.Interval = 60
	}
	if q.Limit < 1 {
		q.Limit = 24
	}

	tx := s.db.WithContext(ctx).Table("request_logs")

	if q.From != "" {
		if t, err := time.Parse(time.RFC3339, q.From); err == nil {
			tx = tx.Where("created_at >= ?", t)
		}
	}
	if q.To != "" {
		if t, err := time.Parse(time.RFC3339, q.To); err == nil {
			tx = tx.Where("created_at <= ?", t)
		}
	}

	bucketExpr := fmt.Sprintf(
		"to_timestamp(floor(EXTRACT(EPOCH FROM created_at) / %d) * %d)",
		q.Interval, q.Interval,
	)

	points := make([]models.TimelinePoint, 0)
	err := tx.
		Select(fmt.Sprintf(
			"to_char(%s, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') AS timestamp, COUNT(*) AS count, COALESCE(AVG(duration_ms), 0) AS avg_duration",
			bucketExpr,
		)).
		Group(bucketExpr).
		Order(bucketExpr + " DESC").
		Limit(q.Limit).
		Scan(&points).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch timeline")
	}

	// Reverse to chronological order
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}

	return points, nil
}

// Export экспортирует логи в текстовый формат.
func (s *requestLogsService) Export(ctx context.Context, typeID int, q models.RequestLogsQuery) (string, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return "", err
	}

	// Снимаем лимит пагинации для экспорта, но ограничиваем 10000 записей
	q.Page = 1
	q.PerPage = 10000

	tx := s.db.WithContext(ctx).Table("request_logs")
	tx = s.applyFilters(tx, q)

	logs := make([]models.RequestLogs, 0)
	if err := tx.Order("created_at DESC").Limit(q.PerPage).Find(&logs).Error; err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to export request logs")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Request Logs Export (%d records)\n", len(logs)))
	sb.WriteString(strings.Repeat("=", 80) + "\n\n")

	for _, l := range logs {
		sb.WriteString(fmt.Sprintf("[%s] ", l.CreatedAt.Format("2006-01-02 15:04:05")))

		if l.Method != nil {
			sb.WriteString(*l.Method + " ")
		}
		if l.URL != nil {
			sb.WriteString(*l.URL)
		}
		sb.WriteString(" -> ")
		if l.ResponseStatus != nil {
			sb.WriteString(fmt.Sprintf("%d", *l.ResponseStatus))
		}
		if l.DurationMs != nil {
			sb.WriteString(fmt.Sprintf(" (%dms)", *l.DurationMs))
		}
		if l.Username != nil {
			sb.WriteString(fmt.Sprintf(" [%s]", *l.Username))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}
