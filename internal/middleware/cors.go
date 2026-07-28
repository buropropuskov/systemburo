package middleware

import (
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// CORS returns CORS middleware configured with the given allowed origins.
func CORS(allowedOrigins []string) echo.MiddlewareFunc {
	return echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders: []string{"Authorization", "Content-Type", "X-Request-ID", "Accept"},
		// Заголовки rate-limit/лимита попыток должны быть читаемы фронтом (по CORS
		// они не в safelist): Retry-After -> таймер, X-Auth-Attempts-Remaining -> счётчик.
		ExposeHeaders:    []string{"Retry-After", "X-Auth-Attempts-Remaining"},
		AllowCredentials: true,
		MaxAge:           3600,
	})
}
