package handlers

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"github.com/labstack/echo/v4"
)

// ParseID парсит числовой параметр из URL path и возвращает ошибку 400 если невалидный.
func ParseID(c echo.Context, param string) (int, error) {
	id, err := strconv.Atoi(c.Param(param))
	if err != nil || id <= 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid %s", param))
	}
	return id, nil
}

// BindAndValidate биндит JSON body и валидирует его (если валидатор зарегистрирован).
// Для слайсов валидация пропускается (Echo validator не поддерживает слайсы напрямую).
//
// Конвенция границы (#888): хендлер, читающий ТЕЛО запроса (POST/PUT/PATCH), использует
// BindAndValidate с DTO, поля которого размечены validate-тегами - единая точка bind +
// валидации + 400. Голый c.Bind допустим только там, где валидировать нечего, и только
// с комментарием почему:
//   - query-параметры фильтров/пагинации (GET): поля опциональны;
//   - намеренно опциональное тело, где пустое = валидный кейс (напр. user_ban: бан без причины);
//   - бинд в слайс []T (валидатор работает по структурам);
//   - доменная/условная валидация, не выражаемая статичным тегом (обязательность зависит от
//     другого поля, напр. statistics.RunReport) - остаётся в хендлере/сервисе, но через явный 400.
// Подробнее - docs/BACKEND.md, раздел "Валидация входа (boundary)".
func BindAndValidate(c echo.Context, dst interface{}) error {
	if err := c.Bind(dst); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if c.Echo().Validator != nil {
		// Skip validation for slices — Echo validator works only with structs
		if reflect.TypeOf(dst).Elem().Kind() != reflect.Slice {
			if err := c.Validate(dst); err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "Validation failed")
			}
		}
	}
	return nil
}

// GetUsername безопасно извлекает username из контекста.
func GetUsername(c echo.Context) string {
	username, _ := c.Get("username").(string)
	return username
}

// GetUserID безопасно извлекает user_id из контекста.
func GetUserID(c echo.Context) int {
	id, _ := c.Get("user_id").(int)
	return id
}

// IsSuperAdmin безопасно извлекает is_super_admin из контекста (#231).
// Используется вместо хардкода type_id == 6.
func IsSuperAdmin(c echo.Context) bool {
	v, _ := c.Get("is_super_admin").(bool)
	return v
}

// viewerUserID возвращает id просматривающего для фильтра пер-вложенного пересыла (#680):
// реальный user_id, либо 0 для супер-админа (без фильтра - видит все вложения).
func viewerUserID(c echo.Context) int {
	if IsSuperAdmin(c) {
		return 0
	}
	id, _ := c.Get("user_id").(int)
	return id
}
