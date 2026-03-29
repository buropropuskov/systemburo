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
func BindAndValidate(c echo.Context, dst interface{}) error {
	if err := c.Bind(dst); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if c.Echo().Validator != nil {
		// Skip validation for slices — Echo validator works only with structs
		if reflect.TypeOf(dst).Elem().Kind() != reflect.Slice {
			if err := c.Validate(dst); err != nil {
				return err
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

// GetTypeID безопасно извлекает type_id из контекста.
func GetTypeID(c echo.Context) int {
	id, _ := c.Get("type_id").(int)
	return id
}
