package middleware

import (
	"strings"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

var pdPaths = []string{"/employees", "/unique-employees"}

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

			username, _ := c.Get("username").(string)
			method := c.Request().Method

			go func() {
				log := models.PDAuditLog{
					Username:   username,
					Action:     methodToAction(method),
					Resource:   pathToResource(path),
					IPAddress:  c.RealIP(),
					Method:     method,
					Path:       path,
					StatusCode: c.Response().Status,
				}
				db.Create(&log)
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
	return "unknown"
}
