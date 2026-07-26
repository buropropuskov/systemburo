package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Пути, на которых пользователь видит персональные данные (152-ФЗ). Префикс /api
// обязателен: роутер вешает всё на api := e.Group("/api"), а nginx проксирует без
// среза префикса - без него сверка не совпадала ни разу и журнал стоял пустым (#1472).
var pdPaths = []string{"/api/employees", "/api/unique-employees", "/api/attachments"}

// auditWriteTimeout - максимальное время на запись лога в БД. Если БД легла или
// горутина зависла, не блокируем graceful shutdown навсегда.
const auditWriteTimeout = 5 * time.Second

func PDAudit(db *gorm.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			path := c.Request().URL.Path
			if !isPDPath(path) {
				return err
			}

			// Снапшот данных запроса ДО горутины: c.Response()/c.RealIP() могут
			// быть невалидны после возврата из handler-а (data race).
			username, _ := c.Get("username").(string)
			userID := pdUserID(c)
			method := c.Request().Method
			ip := c.RealIP()
			statusCode := pdStatusCode(c, err)

			go func() {
				// Отдельный context с таймаутом, не привязанный к request-context
				// (тот отменится сразу после ответа). Защита от висящих горутин
				// при медленной БД.
				ctx, cancel := context.WithTimeout(context.Background(), auditWriteTimeout)
				defer cancel()
				log := models.PDAuditLog{
					UserID:     userID,
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

// isPDPath отвечает, ведёт ли запрос к персональным данным. Кроме перечня префиксов
// сюда попадают два адреса с идентификатором в середине: выгрузка бланка (один .xlsx
// уносит ФИО, паспорта и патенты всех сотрудников заявки) и деталь доступного
// вложения, где охрана видит те же паспортные данные.
func isPDPath(path string) bool {
	for _, p := range pdPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return isBlankPath(path) || isAvailableAttachmentPath(path)
}

func isBlankPath(path string) bool {
	return strings.HasPrefix(path, "/api/applications/") && strings.HasSuffix(path, "/blank")
}

func isAvailableAttachmentPath(path string) bool {
	const prefix = "/api/applications/available-attachments/"
	return strings.HasPrefix(path, prefix) && len(path) > len(prefix)
}

// pdStatusCode возвращает код, который увидит клиент. Echo вызывает обработчик
// ошибок уже после цепочки middleware, поэтому у неудачного запроса Response().Status
// здесь ещё дефолтные 200 - и отказ в доступе попадал в журнал как успешный просмотр.
func pdStatusCode(c echo.Context, err error) int {
	var he *echo.HTTPError
	if errors.As(err, &he) && he.Code != 0 {
		return he.Code
	}
	if err != nil && !c.Response().Committed {
		return http.StatusInternalServerError
	}
	return c.Response().Status
}

// pdUserID достаёт идентификатор пользователя из контекста JWT: по одному имени
// запись не привязать к учётке, если пользователя переименовали или архивировали.
func pdUserID(c echo.Context) *int {
	id, ok := c.Get("user_id").(int)
	if !ok || id == 0 {
		return nil
	}
	return &id
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
	switch {
	case isBlankPath(path):
		return "attachment_blank"
	case isAvailableAttachmentPath(path):
		return "available_attachment"
	case strings.HasPrefix(path, "/api/unique-employees"):
		return "unique_employee"
	case strings.HasPrefix(path, "/api/employees"):
		return "employee"
	case strings.HasPrefix(path, "/api/attachments"):
		return "attachment"
	}
	return "unknown"
}
