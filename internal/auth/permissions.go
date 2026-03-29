package auth

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// CheckAdminByTypeID проверяет, что тип пользователя с данным typeID
// является администраторским (код "manager" или "buropropuskov").
func CheckAdminByTypeID(db *gorm.DB, ctx context.Context, typeID int) error {
	var code string
	err := db.WithContext(ctx).
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

// CheckBuroByUsername проверяет, что пользователь с данным username
// имеет тип "buropropuskov".
func CheckBuroByUsername(db *gorm.DB, ctx context.Context, username string) error {
	var result struct {
		Code string
	}
	err := db.WithContext(ctx).
		Table("users").
		Select("user_types.code").
		Joins("JOIN user_types ON users.type_id = user_types.id").
		Where("users.username = ?", username).
		Scan(&result).Error
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}
	if result.Code != "buropropuskov" {
		return echo.NewHTTPError(http.StatusForbidden, "Insufficient permissions")
	}
	return nil
}
