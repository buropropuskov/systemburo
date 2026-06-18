package services

import (
	"context"
	"fmt"

	"systemburo/internal/models"

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
