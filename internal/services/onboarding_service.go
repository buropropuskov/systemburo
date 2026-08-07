package services

import (
	"context"
	"errors"
	"fmt"

	"systemburo/internal/apperr"
	"systemburo/internal/models"

	"gorm.io/gorm"
)

// OnboardingService управляет per-user прогрессом онбординг-туров.
// Хранит версию КАЖДОГО тура, которую юзер прошёл (нет строки = не проходил), чтобы
// при подъёме версии шагов одного тура показать заново только его, не сбрасывая
// остальные, и чтобы прогресс не терялся при смене браузера.
type OnboardingService struct {
	db *gorm.DB
}

// NewOnboardingService конструирует сервис.
func NewOnboardingService(db *gorm.DB) *OnboardingService {
	return &OnboardingService{db: db}
}

// unknownTourError - единый отказ по неизвестному ключу тура для всех входов.
func unknownTourError(tour string) error {
	return apperr.Validation(fmt.Sprintf("неизвестный тур %q", tour))
}

// GetCompleted возвращает версию по КАЖДОМУ туру (ключ присутствует всегда, nil =
// тур не показывали) и отдельно список туров, пройденных до конца. Разведены они
// потому, что гасит автозапуск сам факт показа, а бейдж «Пройден» заслуживает
// только доведённый до финала. Если юзера нет - ошибка, чтобы handler не выдавал
// "не проходил" для несуществующего id.
func (s *OnboardingService) GetCompleted(ctx context.Context, userID int) (map[string]*int, []string, error) {
	var exists int64
	if err := s.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Count(&exists).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to check user %d for onboarding progress: %w", userID, err)
	}
	if exists == 0 {
		return nil, nil, apperr.NotFound(fmt.Sprintf("пользователь %d не найден", userID))
	}

	var rows []models.UserOnboardingProgress
	if err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&rows).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to get onboarding progress for user %d: %w", userID, err)
	}

	finished := make([]string, 0, len(rows))
	completed := make(map[string]*int, len(models.TourKeys))
	for _, key := range models.TourKeys {
		completed[key] = nil
	}
	for _, row := range rows {
		// Тур, выведенный из обращения, остаётся строкой в БД, но в ответе его быть
		// не должно: состав ключей задаёт TourKeys, а не содержимое таблицы.
		if _, known := completed[row.TourKey]; !known {
			continue
		}
		version := row.CompletedVersion
		completed[row.TourKey] = &version
		if row.Finished {
			finished = append(finished, row.TourKey)
		}
	}
	return completed, finished, nil
}

// SetCompleted помечает прохождение тура указанной версии. Версия только растёт:
// отметка меньшей версией (открытая со вчера вкладка) прогресс не понижает.
//
// finished=false - тур закрыли на середине. Строка всё равно появляется, иначе
// автозапуск показывал бы его при каждом входе, но «Пройден» такой тур не считается.
// Обратный переход (был finished, стал нет) невозможен: повторный просмотр не должен
// отнимать уже заработанную отметку.
func (s *OnboardingService) SetCompleted(ctx context.Context, userID int, tour string, version int, finished bool) error {
	if !models.IsValidTourKey(tour) {
		return unknownTourError(tour)
	}
	if version < 1 {
		return apperr.Validation("версия тура должна быть не меньше 1")
	}

	const q = `
		INSERT INTO user_onboarding_progress (user_id, tour_key, completed_version, finished, completed_at)
		VALUES (?, ?, ?, ?, NOW())
		ON CONFLICT (user_id, tour_key) DO UPDATE
		SET completed_version = GREATEST(user_onboarding_progress.completed_version, EXCLUDED.completed_version),
		    finished = user_onboarding_progress.finished OR EXCLUDED.finished,
		    completed_at = EXCLUDED.completed_at`
	if err := s.db.WithContext(ctx).Exec(q, userID, tour, version, finished).Error; err != nil {
		return fmt.Errorf("failed to set onboarding tour %q for user %d: %w", tour, userID, err)
	}
	return nil
}

// ResetByUsername сбрасывает прохождение туров пользователя по username: пустой tour
// снимает ВСЕ туры, иначе только указанный. Админ-действие: после сброса у юзера
// снова сработает автозапуск. Сброс уже непройденного тура - не ошибка.
func (s *OnboardingService) ResetByUsername(ctx context.Context, username, tour string) error {
	if tour != "" && !models.IsValidTourKey(tour) {
		return unknownTourError(tour)
	}

	// Отдельным запросом, потому что удаление нулевого числа строк одинаково
	// выглядит и у несуществующего юзера, и у юзера без прогресса, а различать их надо.
	var user models.User
	if err := s.db.WithContext(ctx).
		Select("id").
		Where("username = ?", username).
		First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound(fmt.Sprintf("пользователь %q не найден", username))
		}
		return fmt.Errorf("failed to load user %q for onboarding reset: %w", username, err)
	}

	q := s.db.WithContext(ctx).Where("user_id = ?", user.ID)
	if tour != "" {
		q = q.Where("tour_key = ?", tour)
	}
	if err := q.Delete(&models.UserOnboardingProgress{}).Error; err != nil {
		return fmt.Errorf("failed to reset onboarding for user %q: %w", username, err)
	}
	return nil
}
