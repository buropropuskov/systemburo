package middleware

import (
	"context"
	"log/slog"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// requestLogWriteTimeout - таймаут на запись лога в БД из фоновой горутины.
// Защита от висящих горутин при медленной БД (например, во время shutdown).
const requestLogWriteTimeout = 5 * time.Second

// RequestLogger записывает все HTTP-запросы в таблицу request_logs.
func RequestLogger(db *gorm.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now().UTC()

			err := next(c)

			duration := time.Since(start).Milliseconds()

			method := c.Request().Method
			url := c.Request().URL.String()
			status := c.Response().Status
			durationInt := int(duration)

			var userID *int
			var username *string
			if uid, ok := c.Get("user_id").(int); ok && uid > 0 {
				userID = &uid
			}
			if uname, ok := c.Get("username").(string); ok && uname != "" {
				username = &uname
			}

			go func() {
				// Отдельный context с таймаутом - request-context уже отменён
				// после возврата из handler-а.
				ctx, cancel := context.WithTimeout(context.Background(), requestLogWriteTimeout)
				defer cancel()
				log := models.RequestLogs{
					UserID:         userID,
					Username:       username,
					Method:         &method,
					URL:            &url,
					ResponseStatus: &status,
					DurationMs:     &durationInt,
					CreatedAt:      start,
				}
				if dbErr := db.WithContext(ctx).Create(&log).Error; dbErr != nil {
					slog.Error("failed to write request log", "error", dbErr, "url", url)
				}
			}()

			return err
		}
	}
}
