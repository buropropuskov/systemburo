package services

import (
	"context"
	"fmt"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// ThemeService управляет per-user темой оформления (#1415).
// Тема лежит в профиле, а не только в localStorage, чтобы выбор переживал смену
// устройства; клиент применяет её мгновенно и сверяется с профилем после входа.
type ThemeService struct {
	db *gorm.DB
}

// NewThemeService конструирует сервис.
func NewThemeService(db *gorm.DB) *ThemeService {
	return &ThemeService{db: db}
}

// Get возвращает выбранную юзером тему; nil означает "не выбирал" (клиент
// показывает models.DefaultTheme). Несуществующий юзер - ошибка, а не nil-тема.
func (s *ThemeService) Get(ctx context.Context, userID int) (*string, error) {
	var user models.User
	if err := s.db.WithContext(ctx).
		Select("id", "theme").
		Where("id = ?", userID).
		First(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to get theme for user %d: %w", userID, err)
	}
	return user.Theme, nil
}

// Set сохраняет тему юзера. Неизвестный id отклоняется 400-й: в БД должны
// лежать только значения, для которых у фронта есть палитра.
func (s *ThemeService) Set(ctx context.Context, userID int, theme string) error {
	if !models.IsValidTheme(theme) {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("неизвестная тема оформления %q", theme))
	}
	res := s.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("theme", theme)
	if res.Error != nil {
		return fmt.Errorf("failed to set theme for user %d: %w", userID, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("failed to set theme: user %d not found: %w", userID, gorm.ErrRecordNotFound)
	}
	return nil
}
