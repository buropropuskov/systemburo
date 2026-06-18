package services

import (
	"context"
	"fmt"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// OnboardingService управляет per-user статусом онбординг-тура.
// Хранит версию тура, которую юзер прошёл (null = не проходил), чтобы при
// подъёме версии шагов показать тур заново и не сбрасывать его при смене браузера.
type OnboardingService struct {
	db *gorm.DB
}

// NewOnboardingService конструирует сервис.
func NewOnboardingService(db *gorm.DB) *OnboardingService {
	return &OnboardingService{db: db}
}

// GetCompletedVersion возвращает версию тура, которую прошёл юзер.
// nil-значение означает "не проходил". Если юзера нет - возвращает ошибку,
// чтобы handler не выдавал "не проходил" для несуществующего id.
func (s *OnboardingService) GetCompletedVersion(ctx context.Context, userID int) (*int, error) {
	var user models.User
	if err := s.db.WithContext(ctx).
		Select("id", "onboarding_completed_version").
		Where("id = ?", userID).
		First(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to get onboarding version for user %d: %w", userID, err)
	}
	return user.OnboardingCompletedVersion, nil
}

// SetCompleted помечает прохождение тура указанной версии для юзера.
func (s *OnboardingService) SetCompleted(ctx context.Context, userID int, version int) error {
	res := s.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("onboarding_completed_version", version)
	if res.Error != nil {
		return fmt.Errorf("failed to set onboarding version for user %d: %w", userID, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("failed to set onboarding version: user %d not found: %w", userID, gorm.ErrRecordNotFound)
	}
	return nil
}

// ResetByUsername сбрасывает статус прохождения тура (NULL) для пользователя по
// username. Админ-действие: после сброса у юзера снова сработает автозапуск.
func (s *OnboardingService) ResetByUsername(ctx context.Context, username string) error {
	res := s.db.WithContext(ctx).
		Model(&models.User{}).
		Where("username = ?", username).
		Update("onboarding_completed_version", gorm.Expr("NULL"))
	if res.Error != nil {
		return fmt.Errorf("failed to reset onboarding for user %q: %w", username, res.Error)
	}
	if res.RowsAffected == 0 {
		// echo.HTTPError, чтобы error-handler отдал 404 (а не 500): username
		// приходит из path, несуществующий юзер - валидный клиентский кейс.
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("пользователь %q не найден", username))
	}
	return nil
}
