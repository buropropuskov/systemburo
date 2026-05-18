package middleware

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

var pdPaths = []string{"/employees", "/unique-employees", "/attachments"}

// auditWriteTimeout - максимальное время на запись лога в БД. Если БД легла или
// горутина зависла, не блокируем graceful shutdown навсегда.
const auditWriteTimeout = 5 * time.Second

func PDAudit(db *gorm.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			path := c.Request().URL.Path
			isPD := false
			for _, p := range pdPaths {
				if strings.HasPrefix(path, p) {
					isPD = true
					break
				}
			}
			if !isPD {
				return err
			}

			// Снапшот данных запроса ДО горутины: c.Response()/c.RealIP() могут
			// быть невалидны после возврата из handler-а (data race).
			username, _ := c.Get("username").(string)
			method := c.Request().Method
			ip := c.RealIP()
			statusCode := c.Response().Status

			go func() {
				// Отдельный context с таймаутом, не привязанный к request-context
				// (тот отменится сразу после ответа). Защита от висящих горутин
				// при медленной БД.
				ctx, cancel := context.WithTimeout(context.Background(), auditWriteTimeout)
				defer cancel()
				log := models.PDAuditLog{
					Username:   username,
					Action:     methodToAction(method),
					Resource:   pathToResource(path),
					IPAddress:  ip,
					Method:     method,
					Path:       path,
					StatusCode: statusCode,
				}
				if err := db.WithContext(ctx).Create(&log).Error; err != nil {
					slog.Error("failed to write PD audit log", "error", err, "path", path)
				}
			}()

			return err
		}
	}
}

func methodToAction(method string) string {
	switch method {
	case "GET":
		return "view"
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	default:
		return method
	}
}

func pathToResource(path string) string {
	if strings.HasPrefix(path, "/unique-employees") {
		return "unique_employee"
	}
	if strings.HasPrefix(path, "/employees") {
		return "employee"
	}
	if strings.HasPrefix(path, "/attachments") {
		return "attachment"
	}
	return "unknown"
}
